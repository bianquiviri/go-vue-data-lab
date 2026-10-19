package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
	"github.com/josebianco/go-vue-data-lab/internal/handler"
	"github.com/josebianco/go-vue-data-lab/internal/jobs"
	"github.com/josebianco/go-vue-data-lab/internal/scheduler"
	"github.com/josebianco/go-vue-data-lab/internal/service"
)

// fakeSchedulerControl implements service.SchedulerControl for handler tests.
type fakeSchedulerControl struct {
	t *testing.T

	list    func(context.Context) ([]models.CronSchedule, error)
	create  func(context.Context, scheduler.CreateParams) (models.CronSchedule, error)
	update  func(context.Context, uuid.UUID, scheduler.UpdateParams) (models.CronSchedule, error)
	del     func(context.Context, uuid.UUID) error
	enable  func(context.Context, uuid.UUID) (models.CronSchedule, error)
	disable func(context.Context, uuid.UUID) (models.CronSchedule, error)
	runNow  func(context.Context, uuid.UUID) (uuid.UUID, error)
}

func newFakeSchedulerControl(t *testing.T) *fakeSchedulerControl {
	t.Helper()
	unexpected := func(method string) {
		t.Errorf("unexpected call to SchedulerControl.%s", method)
	}
	return &fakeSchedulerControl{
		t: t,
		list: func(context.Context) ([]models.CronSchedule, error) {
			unexpected("List")
			return nil, nil
		},
		create: func(context.Context, scheduler.CreateParams) (models.CronSchedule, error) {
			unexpected("Create")
			return models.CronSchedule{}, nil
		},
		update: func(context.Context, uuid.UUID, scheduler.UpdateParams) (models.CronSchedule, error) {
			unexpected("Update")
			return models.CronSchedule{}, nil
		},
		del: func(context.Context, uuid.UUID) error {
			unexpected("Delete")
			return nil
		},
		enable: func(context.Context, uuid.UUID) (models.CronSchedule, error) {
			unexpected("Enable")
			return models.CronSchedule{}, nil
		},
		disable: func(context.Context, uuid.UUID) (models.CronSchedule, error) {
			unexpected("Disable")
			return models.CronSchedule{}, nil
		},
		runNow: func(context.Context, uuid.UUID) (uuid.UUID, error) {
			unexpected("RunNow")
			return uuid.Nil, nil
		},
	}
}

func (f *fakeSchedulerControl) List(ctx context.Context) ([]models.CronSchedule, error) {
	return f.list(ctx)
}
func (f *fakeSchedulerControl) Create(ctx context.Context, p scheduler.CreateParams) (models.CronSchedule, error) {
	return f.create(ctx, p)
}
func (f *fakeSchedulerControl) Update(ctx context.Context, id uuid.UUID, p scheduler.UpdateParams) (models.CronSchedule, error) {
	return f.update(ctx, id, p)
}
func (f *fakeSchedulerControl) Delete(ctx context.Context, id uuid.UUID) error {
	return f.del(ctx, id)
}
func (f *fakeSchedulerControl) Enable(ctx context.Context, id uuid.UUID) (models.CronSchedule, error) {
	return f.enable(ctx, id)
}
func (f *fakeSchedulerControl) Disable(ctx context.Context, id uuid.UUID) (models.CronSchedule, error) {
	return f.disable(ctx, id)
}
func (f *fakeSchedulerControl) RunNow(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return f.runNow(ctx, id)
}

var _ service.SchedulerControl = (*fakeSchedulerControl)(nil)

func schedulerEngine(svc service.SchedulerControl) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewSchedulerHandler(svc)
	engine := gin.New()
	engine.GET("/api/v1/scheduler/jobs", h.List)
	engine.POST("/api/v1/scheduler/jobs", h.Create)
	engine.PUT("/api/v1/scheduler/jobs/:id", h.Update)
	engine.DELETE("/api/v1/scheduler/jobs/:id", h.Delete)
	engine.POST("/api/v1/scheduler/jobs/:id/enable", h.Enable)
	engine.POST("/api/v1/scheduler/jobs/:id/disable", h.Disable)
	engine.POST("/api/v1/scheduler/jobs/:id/run", h.Run)
	return engine
}

func TestSchedulerList(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	svc.list = func(context.Context) ([]models.CronSchedule, error) {
		return []models.CronSchedule{{ID: uuid.New(), Name: "nightly", CronExpr: "0 3 * * *", Command: "generate", Enabled: true}}, nil
	}

	rec := doJSON(t, schedulerEngine(svc), http.MethodGet, "/api/v1/scheduler/jobs", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var body struct {
		Data []models.CronSchedule `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].Name != "nightly" {
		t.Errorf("unexpected list: %+v", body.Data)
	}
}

func TestSchedulerCreate(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	var got scheduler.CreateParams
	svc.create = func(_ context.Context, p scheduler.CreateParams) (models.CronSchedule, error) {
		got = p
		return models.CronSchedule{ID: uuid.New(), Name: p.Name, CronExpr: p.CronExpr, Command: p.Command, Enabled: true}, nil
	}

	rec := doJSON(t, schedulerEngine(svc), http.MethodPost, "/api/v1/scheduler/jobs",
		`{"name":"nightly","cron_expr":"0 3 * * *","command":"generate"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusCreated, rec.Code, rec.Body.String())
	}
	if got.Name != "nightly" || got.CronExpr != "0 3 * * *" || got.Command != "generate" {
		t.Errorf("unexpected params: %+v", got)
	}
}

func TestSchedulerCreateInvalidCron(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	svc.create = func(context.Context, scheduler.CreateParams) (models.CronSchedule, error) {
		return models.CronSchedule{}, scheduler.ErrInvalidCron
	}

	rec := doJSON(t, schedulerEngine(svc), http.MethodPost, "/api/v1/scheduler/jobs",
		`{"name":"x","cron_expr":"nope","command":"generate"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if got := decodeError(t, rec).Error.Code; got != handler.CodeValidation {
		t.Errorf("expected validation_failed, got %q", got)
	}
}

func TestSchedulerCreateMissingFields(t *testing.T) {
	svc := newFakeSchedulerControl(t)

	rec := doJSON(t, schedulerEngine(svc), http.MethodPost, "/api/v1/scheduler/jobs", `{"name":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if got := decodeError(t, rec).Error.Code; got != handler.CodeValidation {
		t.Errorf("expected validation_failed, got %q", got)
	}
}

func TestSchedulerUpdate(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	id := uuid.New()
	var gotID uuid.UUID
	svc.update = func(_ context.Context, got uuid.UUID, p scheduler.UpdateParams) (models.CronSchedule, error) {
		gotID = got
		return models.CronSchedule{ID: got, Name: p.Name, CronExpr: p.CronExpr, Command: p.Command, Enabled: true}, nil
	}

	rec := doJSON(t, schedulerEngine(svc), http.MethodPut, "/api/v1/scheduler/jobs/"+id.String(),
		`{"name":"new","cron_expr":"0 4 * * *","command":"migrate"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
	}
	if gotID != id {
		t.Errorf("expected id %s, got %s", id, gotID)
	}
}

func TestSchedulerUpdateInvalidUUID(t *testing.T) {
	svc := newFakeSchedulerControl(t)

	rec := doJSON(t, schedulerEngine(svc), http.MethodPut, "/api/v1/scheduler/jobs/not-a-uuid",
		`{"name":"new","cron_expr":"0 4 * * *","command":"migrate"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if got := decodeError(t, rec).Error.Code; got != handler.CodeValidation {
		t.Errorf("expected validation_failed, got %q", got)
	}
}

func TestSchedulerUpdateNotFound(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	svc.update = func(context.Context, uuid.UUID, scheduler.UpdateParams) (models.CronSchedule, error) {
		return models.CronSchedule{}, scheduler.ErrNotFound
	}

	rec := doJSON(t, schedulerEngine(svc), http.MethodPut, "/api/v1/scheduler/jobs/"+uuid.NewString(),
		`{"name":"new","cron_expr":"0 4 * * *","command":"migrate"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSchedulerDelete(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	svc.del = func(context.Context, uuid.UUID) error { return nil }

	rec := doJSON(t, schedulerEngine(svc), http.MethodDelete, "/api/v1/scheduler/jobs/"+uuid.NewString(), "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body, got %q", rec.Body.String())
	}
}

func TestSchedulerDeleteNotFound(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	svc.del = func(context.Context, uuid.UUID) error { return scheduler.ErrNotFound }

	rec := doJSON(t, schedulerEngine(svc), http.MethodDelete, "/api/v1/scheduler/jobs/"+uuid.NewString(), "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSchedulerEnableDisable(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	id := uuid.New()
	svc.enable = func(_ context.Context, got uuid.UUID) (models.CronSchedule, error) {
		return models.CronSchedule{ID: got, Enabled: true}, nil
	}
	svc.disable = func(_ context.Context, got uuid.UUID) (models.CronSchedule, error) {
		return models.CronSchedule{ID: got, Enabled: false}, nil
	}

	rec := doJSON(t, schedulerEngine(svc), http.MethodPost, "/api/v1/scheduler/jobs/"+id.String()+"/enable", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("enable: expected %d, got %d", http.StatusOK, rec.Code)
	}
	var enabled struct {
		Data models.CronSchedule `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &enabled); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !enabled.Data.Enabled {
		t.Error("expected enabled=true")
	}

	rec = doJSON(t, schedulerEngine(svc), http.MethodPost, "/api/v1/scheduler/jobs/"+id.String()+"/disable", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("disable: expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestSchedulerRun(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	jobID := uuid.New()
	svc.runNow = func(context.Context, uuid.UUID) (uuid.UUID, error) { return jobID, nil }

	rec := doJSON(t, schedulerEngine(svc), http.MethodPost, "/api/v1/scheduler/jobs/"+uuid.NewString()+"/run", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	var body struct {
		Data struct {
			JobID string `json:"job_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.JobID != jobID.String() {
		t.Errorf("expected job id %s, got %s", jobID, body.Data.JobID)
	}
}

func TestSchedulerRunConflict(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	svc.runNow = func(context.Context, uuid.UUID) (uuid.UUID, error) { return uuid.Nil, jobs.ErrJobRunning }

	rec := doJSON(t, schedulerEngine(svc), http.MethodPost, "/api/v1/scheduler/jobs/"+uuid.NewString()+"/run", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected %d, got %d", http.StatusConflict, rec.Code)
	}
	if got := decodeError(t, rec).Error.Code; got != handler.CodeConflict {
		t.Errorf("expected conflict, got %q", got)
	}
}

func TestSchedulerRunNotFound(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	svc.runNow = func(context.Context, uuid.UUID) (uuid.UUID, error) { return uuid.Nil, scheduler.ErrNotFound }

	rec := doJSON(t, schedulerEngine(svc), http.MethodPost, "/api/v1/scheduler/jobs/"+uuid.NewString()+"/run", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestSchedulerInternalError(t *testing.T) {
	svc := newFakeSchedulerControl(t)
	svc.list = func(context.Context) ([]models.CronSchedule, error) { return nil, errors.New("db down") }

	rec := doJSON(t, schedulerEngine(svc), http.MethodGet, "/api/v1/scheduler/jobs", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
