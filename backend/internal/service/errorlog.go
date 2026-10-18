package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"
)

// LogEntry is a single structured log record surfaced by the admin log
// endpoints. Time is zero when the record carries an unparsable timestamp.
type LogEntry struct {
	Time    time.Time      `json:"time"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Caller  string         `json:"caller,omitempty"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// ErrorLogs is the read port over the rotating JSON log. It is satisfied by
// FileErrorLogs and by test doubles.
type ErrorLogs interface {
	// RecentErrors returns all matching entries (total) plus the slice in
	// [offset, offset+limit), ordered newest first.
	RecentErrors(ctx context.Context, limit, offset int) ([]LogEntry, int, error)
}

// maxTailBytes bounds how much of the log file is scanned per request: only
// the trailing window is read, so the endpoint stays cheap on a 100 MB file.
const maxTailBytes int64 = 4 << 20 // 4 MiB

// defaultErrorLogPage is the page size used when the caller asks for a
// non-positive limit (the handler already normalizes pagination).
const defaultErrorLogPage = 20

// errorLevels are the zap levels that count as "errors" for the admin
// endpoint (capitalized, as emitted by the production encoder).
var errorLevels = map[string]struct{}{
	"ERROR":  {},
	"FATAL":  {},
	"PANIC":  {},
	"DPANIC": {},
}

// FileErrorLogs reads zap JSON records from the rotating log file at path.
type FileErrorLogs struct {
	path string
}

// NewFileErrorLogs builds an ErrorLogs over the log file at path.
func NewFileErrorLogs(path string) *FileErrorLogs {
	return &FileErrorLogs{path: path}
}

// RecentErrors scans the tail of the log file, keeps the error-level records
// and returns the requested page. A missing file is not an error: it simply
// means no errors have been logged yet.
func (f *FileErrorLogs) RecentErrors(ctx context.Context, limit, offset int) ([]LogEntry, int, error) {
	if limit <= 0 {
		limit = defaultErrorLogPage
	}
	if offset < 0 {
		offset = 0
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, fmt.Errorf("errorlog: %w", err)
	}

	lines, err := f.tailLines()
	if err != nil {
		return nil, 0, err
	}

	entries := make([]LogEntry, 0, len(lines)/4)
	for _, line := range lines {
		if err := ctx.Err(); err != nil {
			return nil, 0, fmt.Errorf("errorlog: %w", err)
		}
		if entry, ok := decodeErrorRecord(line); ok {
			entries = append(entries, entry)
		}
	}

	// The file is written oldest first; the API returns newest first.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	total := len(entries)
	page := make([]LogEntry, 0, limit)
	for i := offset; i < total && len(page) < limit; i++ {
		page = append(page, entries[i])
	}
	return page, total, nil
}

// tailLines reads at most maxTailBytes from the end of the log file and
// returns them split by newline. The first line is dropped when the read
// started mid-file so no partial record is ever parsed.
func (f *FileErrorLogs) tailLines() ([][]byte, error) {
	file, err := os.Open(f.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("errorlog: open %q: %w", f.path, err)
	}
	defer func() { _ = file.Close() }()

	// Read only the trailing window so a large rotated file stays cheap to
	// scan; when truncated, the first line is partial and must be dropped.
	truncated := false
	if info, statErr := file.Stat(); statErr == nil && info.Size() > maxTailBytes {
		if _, seekErr := file.Seek(info.Size()-maxTailBytes, 0); seekErr != nil {
			return nil, fmt.Errorf("errorlog: seek %q: %w", f.path, seekErr)
		}
		truncated = true
	}

	raw, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("errorlog: read %q: %w", f.path, err)
	}

	lines := splitLines(raw)
	if truncated && len(lines) > 0 {
		lines = lines[1:]
	}
	return lines, nil
}

// splitLines returns the non-empty lines of raw, with trailing CR removed.
func splitLines(raw []byte) [][]byte {
	parts := strings.Split(string(raw), "\n")
	lines := make([][]byte, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimRight(p, "\r")
		if strings.TrimSpace(p) == "" {
			continue
		}
		lines = append(lines, []byte(p))
	}
	return lines
}

// decodeErrorRecord parses one zap JSON record and returns it when its level
// is error-level. Unparsable lines, records without a message and non-error
// levels are skipped.
func decodeErrorRecord(line []byte) (LogEntry, bool) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return LogEntry{}, false
	}

	var level string
	if err := json.Unmarshal(raw["level"], &level); err != nil || level == "" {
		return LogEntry{}, false
	}
	if _, ok := errorLevels[strings.ToUpper(level)]; !ok {
		return LogEntry{}, false
	}

	entry := LogEntry{Level: level, Time: decodeLogTime(raw["ts"])}

	msg, err := unmarshalString(raw["msg"])
	if err != nil || strings.TrimSpace(msg) == "" {
		return LogEntry{}, false
	}
	entry.Message = msg

	if caller, err := unmarshalString(raw["caller"]); err == nil {
		entry.Caller = caller
	}

	fields := make(map[string]any, len(raw))
	for key, value := range raw {
		switch key {
		case "level", "ts", "msg", "caller":
			continue
		}
		var v any
		if json.Unmarshal(value, &v) == nil {
			fields[key] = v
		}
	}
	if len(fields) > 0 {
		entry.Fields = fields
	}
	return entry, true
}

// decodeLogTime accepts zap's ISO8601 string, RFC3339 and a unix-seconds
// number; it returns the zero time when the value cannot be parsed.
func decodeLogTime(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05.000Z0700",
			"2006-01-02T15:04:05Z0700",
			time.RFC3339Nano + "Z0700",
		} {
			if t, err := time.Parse(layout, s); err == nil {
				return t
			}
		}
		return time.Time{}
	}

	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		secs := int64(n)
		nanos := int64((n - float64(secs)) * float64(time.Second))
		return time.Unix(secs, nanos).UTC()
	}
	return time.Time{}
}

func unmarshalString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("missing")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}
