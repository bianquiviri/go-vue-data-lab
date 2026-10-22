package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/jobs"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// fakeJobControl implements service.JobControl for handler tests.
type fakeJobControl struct {
	t *testing.T

	start     func(context.Context, string, any) (uuid.UUID, error)
	stop      func() error
	status    func() (jobs.JobStatus, error)
	progress  func() (jobs.Progress, error)
	subscribe func(context.Context) <-chan jobs.Progress
	history   func(context.Context, int, int) ([]models.JobExecution, int64, error)
}

func newFakeJobControl(t *testing.T) *fakeJobControl {
	t.Helper()
	unexpected := func(method string) {
		t.Errorf("unexpected call to JobControl.%s", method)
	}
	return &fakeJobControl{
		t: t,
		start: func(context.Context, string, any) (uuid.UUID, error) {
			unexpected("Start")
			return uuid.Nil, nil
		},
		stop: func() error {
			unexpected("Stop")
			return nil
		},
		status: func() (jobs.JobStatus, error) {
			unexpected("Status")
			return jobs.JobStatus{}, nil
		},
		progress: func() (jobs.Progress, error) {
			unexpected("Progress")
			return jobs.Progress{}, nil
		},
		subscribe: func(context.Context) <-chan jobs.Progress {
			unexpected("Subscribe")
			ch := make(chan jobs.Progress)
			close(ch)
			return ch
		},
		history: func(context.Context, int, int) ([]models.JobExecution, int64, error) {
			unexpected("History")
			return nil, 0, nil
		},
	}
}

func (f *fakeJobControl) Start(ctx context.Context, jobType string, params any) (uuid.UUID, error) {
	return f.start(ctx, jobType, params)
}
func (f *fakeJobControl) Stop() error                      { return f.stop() }
func (f *fakeJobControl) Status() (jobs.JobStatus, error)  { return f.status() }
func (f *fakeJobControl) Progress() (jobs.Progress, error) { return f.progress() }
func (f *fakeJobControl) Subscribe(ctx context.Context) <-chan jobs.Progress {
	return f.subscribe(ctx)
}
func (f *fakeJobControl) History(ctx context.Context, page, perPage int) ([]models.JobExecution, int64, error) {
	return f.history(ctx, page, perPage)
}

var _ service.JobControl = (*fakeJobControl)(nil)

// jobsEngine mounts the jobs routes on a fresh engine with the opaque id
// codec wired, so job ids are sealed in responses.
func jobsEngine(svc service.JobControl) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewJobsHandler(svc)
	engine := gin.New()
	engine.Use(handler.WithCodec(testCodec))
	engine.POST("/api/v1/jobs/generate/start", h.StartGenerate)
	engine.POST("/api/v1/jobs/migrate/start", h.StartMigrate)
	engine.POST("/api/v1/jobs/stop", h.Stop)
	engine.GET("/api/v1/jobs/status", h.Status)
	engine.GET("/api/v1/jobs/history", h.History)
	engine.GET("/api/v1/jobs/progress", h.Progress)
	return engine
}

func doJSON(t *testing.T, engine *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestJobsStartGenerateAccepted(t *testing.T) {
	svc := newFakeJobControl(t)
	id := uuid.New()
	var gotType string
	var gotParams any
	svc.start = func(_ context.Context, jobType string, params any) (uuid.UUID, error) {
		gotType, gotParams = jobType, params
		return id, nil
	}

	rec := doJSON(t, jobsEngine(svc), http.MethodPost, "/api/v1/jobs/generate/start", `{"customers":10}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusAccepted, rec.Code, rec.Body.String())
	}
	if gotType != jobs.TypeGenerate {
		t.Errorf("expected generate job, got %q", gotType)
	}
	p, ok := gotParams.(jobs.GenerateParams)
	if !ok || p.Customers != 10 {
		t.Errorf("unexpected params: %#v", gotParams)
	}

	var body struct {
		Data struct {
			JobID  string `json:"job_id"`
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.JobID != sealID(id) || body.Data.Type != jobs.TypeGenerate {
		t.Errorf("unexpected body: %+v", body.Data)
	}
}

func TestJobsStartMigrateAccepted(t *testing.T) {
	svc := newFakeJobControl(t)
	id := uuid.New()
	var gotType string
	svc.start = func(_ context.Context, jobType string, _ any) (uuid.UUID, error) {
		gotType = jobType
		return id, nil
	}

	rec := doJSON(t, jobsEngine(svc), http.MethodPost, "/api/v1/jobs/migrate/start", `{"entities":["customers"]}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusAccepted, rec.Code, rec.Body.String())
	}
	if gotType != jobs.TypeMigrate {
		t.Errorf("expected migrate job, got %q", gotType)
	}
}

func TestJobsStartValidationError(t *testing.T) {
	svc := newFakeJobControl(t)
	svc.start = func(context.Context, string, any) (uuid.UUID, error) {
		return uuid.Nil, jobs.ErrInvalidParams
	}

	rec := doJSON(t, jobsEngine(svc), http.MethodPost, "/api/v1/jobs/generate/start", `{"customers":-1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if got := decodeError(t, rec).Error.Code; got != handler.CodeValidation {
		t.Errorf("expected validation_failed, got %q", got)
	}
}

func TestJobsStartMalformedJSON(t *testing.T) {
	svc := newFakeJobControl(t)

	rec := doJSON(t, jobsEngine(svc), http.MethodPost, "/api/v1/jobs/generate/start", `{`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if got := decodeError(t, rec).Error.Code; got != handler.CodeValidation {
		t.Errorf("expected validation_failed, got %q", got)
	}
}

func TestJobsStartConflict(t *testing.T) {
	svc := newFakeJobControl(t)
	svc.start = func(context.Context, string, any) (uuid.UUID, error) {
		return uuid.Nil, jobs.ErrJobRunning
	}

	rec := doJSON(t, jobsEngine(svc), http.MethodPost, "/api/v1/jobs/generate/start", `{}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected %d, got %d", http.StatusConflict, rec.Code)
	}
	if got := decodeError(t, rec).Error.Code; got != handler.CodeConflict {
		t.Errorf("expected conflict, got %q", got)
	}
}

func TestJobsStartInternalError(t *testing.T) {
	svc := newFakeJobControl(t)
	svc.start = func(context.Context, string, any) (uuid.UUID, error) {
		return uuid.Nil, errors.New("db down")
	}

	rec := doJSON(t, jobsEngine(svc), http.MethodPost, "/api/v1/jobs/generate/start", `{}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestJobsStop(t *testing.T) {
	svc := newFakeJobControl(t)
	svc.stop = func() error { return nil }

	rec := doJSON(t, jobsEngine(svc), http.MethodPost, "/api/v1/jobs/stop", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var body struct {
		Data struct {
			Stopped bool `json:"stopped"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Data.Stopped {
		t.Error("expected stopped=true")
	}
}

func TestJobsStopNoRunningJob(t *testing.T) {
	svc := newFakeJobControl(t)
	svc.stop = func() error { return jobs.ErrNoRunningJob }

	rec := doJSON(t, jobsEngine(svc), http.MethodPost, "/api/v1/jobs/stop", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
	if got := decodeError(t, rec).Error.Code; got != handler.CodeNotFound {
		t.Errorf("expected not_found, got %q", got)
	}
}

func TestJobsStatus(t *testing.T) {
	svc := newFakeJobControl(t)
	svc.status = func() (jobs.JobStatus, error) {
		return jobs.JobStatus{ID: uuid.New(), Type: jobs.TypeGenerate, Status: jobs.StatusRunning, Running: true}, nil
	}

	rec := doJSON(t, jobsEngine(svc), http.MethodGet, "/api/v1/jobs/status", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var body struct {
		Data struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Running bool   `json:"running"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Status != string(jobs.StatusRunning) || !body.Data.Running {
		t.Errorf("unexpected status: %+v", body.Data)
	}
}

func TestJobsProgress(t *testing.T) {
	svc := newFakeJobControl(t)
	svc.progress = func() (jobs.Progress, error) {
		return jobs.Progress{Status: jobs.StatusRunning, Percent: 42, Entities: []jobs.EntityProgress{{Entity: "customers", Done: 4, Total: 10}}}, nil
	}

	rec := doJSON(t, jobsEngine(svc), http.MethodGet, "/api/v1/jobs/progress", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	// job_id is a sealed token, so decode the envelope loosely.
	var body struct {
		Data struct {
			JobID    string  `json:"job_id"`
			Percent  float64 `json:"percent"`
			Entities []struct {
				Entity string `json:"entity"`
				Done   int64  `json:"done"`
			} `json:"entities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Percent != 42 || len(body.Data.Entities) != 1 || body.Data.JobID == "" {
		t.Errorf("unexpected progress: %+v", body.Data)
	}
}

func TestJobsHistoryPaginated(t *testing.T) {
	svc := newFakeJobControl(t)
	var gotPage, gotPerPage int
	svc.history = func(_ context.Context, page, perPage int) ([]models.JobExecution, int64, error) {
		gotPage, gotPerPage = page, perPage
		return []models.JobExecution{{ID: uuid.New(), JobName: "generate", Status: "completed"}}, 42, nil
	}

	rec := doJSON(t, jobsEngine(svc), http.MethodGet, "/api/v1/jobs/history?page=2&per_page=5", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if gotPage != 2 || gotPerPage != 5 {
		t.Errorf("expected page=2 per_page=5, got %d/%d", gotPage, gotPerPage)
	}
	var body struct {
		Data []struct {
			ID      string `json:"id"`
			JobName string `json:"job_name"`
			Status  string `json:"status"`
		} `json:"data"`
		Meta struct {
			Page       int   `json:"page"`
			PerPage    int   `json:"per_page"`
			Total      int64 `json:"total"`
			TotalPages int   `json:"total_pages"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].JobName != "generate" || body.Meta.Total != 42 || body.Meta.TotalPages != 9 {
		t.Errorf("unexpected history envelope: %+v", body)
	}
}

func TestJobsHistoryInvalidPaging(t *testing.T) {
	svc := newFakeJobControl(t)

	rec := doJSON(t, jobsEngine(svc), http.MethodGet, "/api/v1/jobs/history?page=-1", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
