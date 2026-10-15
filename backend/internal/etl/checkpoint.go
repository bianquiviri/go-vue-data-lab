package etl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EntityCheckpoint is the resumable state of a single entity.
type EntityCheckpoint struct {
	// LastKey is the primary key of the last fully confirmed batch. Empty means
	// "start from the beginning".
	LastKey string `json:"last_key,omitempty"`
	// Processed counts the rows confirmed in previous runs.
	Processed int64 `json:"processed"`
	// Completed is true once the entity was fully migrated.
	Completed bool `json:"completed"`
}

// Checkpoint is the resumable state of a whole run.
type Checkpoint struct {
	Entities  map[Entity]EntityCheckpoint `json:"entities"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

// NewCheckpoint returns an empty, initialised checkpoint.
func NewCheckpoint() Checkpoint {
	return Checkpoint{Entities: make(map[Entity]EntityCheckpoint)}
}

// Key returns the last confirmed key for entity ("" when unset).
func (c Checkpoint) Key(e Entity) string { return c.Entities[e].LastKey }

// IsCompleted reports whether entity finished in a previous run.
func (c Checkpoint) IsCompleted(e Entity) bool { return c.Entities[e].Completed }

// SetKey records the last confirmed key for entity.
func (c *Checkpoint) SetKey(e Entity, key string) {
	if c.Entities == nil {
		c.Entities = make(map[Entity]EntityCheckpoint)
	}
	ec := c.Entities[e]
	ec.LastKey = key
	c.Entities[e] = ec
}

// MarkCompleted flags entity as fully migrated.
func (c *Checkpoint) MarkCompleted(e Entity) {
	if c.Entities == nil {
		c.Entities = make(map[Entity]EntityCheckpoint)
	}
	ec := c.Entities[e]
	ec.Completed = true
	c.Entities[e] = ec
}

// Clone returns a deep copy of the checkpoint.
func (c Checkpoint) Clone() Checkpoint {
	out := Checkpoint{Entities: make(map[Entity]EntityCheckpoint, len(c.Entities)), UpdatedAt: c.UpdatedAt}
	for k, v := range c.Entities {
		out.Entities[k] = v
	}
	return out
}

// CheckpointStore persists checkpoints between runs.
type CheckpointStore interface {
	// Load returns the stored checkpoint, or an empty one when none exists.
	Load(ctx context.Context) (Checkpoint, error)
	// Save persists cp, replacing any previous state.
	Save(ctx context.Context, cp Checkpoint) error
}

// MemoryStore keeps the checkpoint in memory. It is the default store and is
// mainly useful for tests and single-run embedders.
type MemoryStore struct {
	mu sync.Mutex
	cp Checkpoint
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{cp: NewCheckpoint()} }

// Load implements CheckpointStore.
func (s *MemoryStore) Load(_ context.Context) (Checkpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cp.Clone(), nil
}

// Save implements CheckpointStore.
func (s *MemoryStore) Save(_ context.Context, cp Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cp = cp.Clone()
	return nil
}

// FileStore persists the checkpoint as JSON in a file. Writes are atomic
// (temp file + rename) so a crash never leaves a truncated checkpoint.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore returns a store backed by path.
func NewFileStore(path string) *FileStore { return &FileStore{path: path} }

// Load implements CheckpointStore. A missing file yields an empty checkpoint.
func (s *FileStore) Load(ctx context.Context) (Checkpoint, error) {
	if err := ctx.Err(); err != nil {
		return Checkpoint{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return NewCheckpoint(), nil
	}
	if err != nil {
		return Checkpoint{}, fmt.Errorf("etl: leer checkpoint %q: %w", s.path, err)
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return Checkpoint{}, fmt.Errorf("etl: decodificar checkpoint %q: %w", s.path, err)
	}
	if cp.Entities == nil {
		cp.Entities = make(map[Entity]EntityCheckpoint)
	}
	return cp, nil
}

// Save implements CheckpointStore.
func (s *FileStore) Save(ctx context.Context, cp Checkpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	cp.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("etl: codificar checkpoint: %w", err)
	}
	if dir := filepath.Dir(s.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("etl: crear directorio de checkpoint %q: %w", dir, err)
		}
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("etl: escribir checkpoint %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("etl: reemplazar checkpoint %q: %w", s.path, err)
	}
	return nil
}

// parseUUIDKey parses a UUID checkpoint key; an empty string means uuid.Nil
// (start of the table).
func parseUUIDKey(s string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("etl: clave UUID inválida %q: %w", s, err)
	}
	return id, nil
}

// parseInt64Key parses an int64 checkpoint key; an empty string means 0 (start
// of the table).
func parseInt64Key(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("etl: clave int64 inválida %q: %w", s, err)
	}
	return n, nil
}
