package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/josebianco/go-vue-data-lab/internal/handler"
)

func TestPaginationNormalizedDefaults(t *testing.T) {
	cases := []struct {
		name        string
		in          handler.Pagination
		wantPage    int
		wantPerPage int
	}{
		{"zero value", handler.Pagination{}, 1, handler.DefaultPerPage},
		{"negative page", handler.Pagination{Page: -3}, 1, handler.DefaultPerPage},
		{"negative per_page", handler.Pagination{PerPage: -10}, 1, handler.DefaultPerPage},
		{"per_page above max", handler.Pagination{Page: 2, PerPage: 5000}, 2, handler.MaxPerPage},
		{"in range kept", handler.Pagination{Page: 3, PerPage: 50}, 3, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.Normalized()
			if got.Page != tc.wantPage {
				t.Errorf("Page = %d, want %d", got.Page, tc.wantPage)
			}
			if got.PerPage != tc.wantPerPage {
				t.Errorf("PerPage = %d, want %d", got.PerPage, tc.wantPerPage)
			}
		})
	}
}

func TestPaginationOffsetAndLimit(t *testing.T) {
	cases := []struct {
		in         handler.Pagination
		wantOffset int
		wantLimit  int
	}{
		{handler.Pagination{}, 0, handler.DefaultPerPage},
		{handler.Pagination{Page: 1, PerPage: 25}, 0, 25},
		{handler.Pagination{Page: 3, PerPage: 25}, 50, 25},
		{handler.Pagination{Page: 2}, handler.DefaultPerPage, handler.DefaultPerPage},
	}
	for _, tc := range cases {
		if got := tc.in.Offset(); got != tc.wantOffset {
			t.Errorf("Pagination%+v.Offset() = %d, want %d", tc.in, got, tc.wantOffset)
		}
		if got := tc.in.Limit(); got != tc.wantLimit {
			t.Errorf("Pagination%+v.Limit() = %d, want %d", tc.in, got, tc.wantLimit)
		}
	}
}

func TestPaginationTotalPages(t *testing.T) {
	cases := []struct {
		total     int64
		perPage   int
		wantPages int
	}{
		{0, 20, 0},
		{-5, 20, 0},
		{1, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{100, 25, 4},
		{101, 25, 5},
	}
	for _, tc := range cases {
		p := handler.Pagination{PerPage: tc.perPage}
		if got := p.TotalPages(tc.total); got != tc.wantPages {
			t.Errorf("TotalPages(total=%d, perPage=%d) = %d, want %d", tc.total, tc.perPage, got, tc.wantPages)
		}
	}
}

func TestPaginationBindingRejectsOutOfRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type query struct {
		handler.Pagination
	}

	cases := []struct {
		q       string
		wantErr bool
	}{
		{"", false},
		{"page=1&per_page=20", false},
		{"page=100&per_page=100", false},
		{"page=0", false}, // zero is indistinguishable from "omitted": default applies
		{"page=-1", true},
		{"per_page=0", false}, // omitted/zero falls back to the default
		{"per_page=101", true},
		{"per_page=abc", true},
	}
	for _, tc := range cases {
		t.Run(tc.q, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/?"+tc.q, nil)

			var q query
			err := c.ShouldBindQuery(&q)
			if tc.wantErr && err == nil {
				t.Fatalf("expected binding error for %q", tc.q)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected binding error for %q: %v", tc.q, err)
			}
		})
	}
}

func TestRespondPaginatedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler.RespondPaginated(c, http.StatusOK, []string{"a", "b"}, handler.Pagination{Page: 2, PerPage: 10}, 35)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body struct {
		Data []string `json:"data"`
		Meta struct {
			Page       int   `json:"page"`
			PerPage    int   `json:"per_page"`
			Total      int64 `json:"total"`
			TotalPages int   `json:"total_pages"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode envelope: %v (body: %s)", err, rec.Body.String())
	}
	if len(body.Data) != 2 || body.Data[0] != "a" {
		t.Errorf("unexpected data: %+v", body.Data)
	}
	if body.Meta.Page != 2 || body.Meta.PerPage != 10 {
		t.Errorf("unexpected meta paging: %+v", body.Meta)
	}
	if body.Meta.Total != 35 {
		t.Errorf("expected total 35, got %d", body.Meta.Total)
	}
	if body.Meta.TotalPages != 4 {
		t.Errorf("expected total_pages 4, got %d", body.Meta.TotalPages)
	}
}

func TestRespondPaginatedNormalizesMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	handler.RespondPaginated(c, http.StatusOK, []string{}, handler.Pagination{}, 0)

	var body handler.PaginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	meta := body.Meta
	if meta.Page != handler.DefaultPage || meta.PerPage != handler.DefaultPerPage {
		t.Errorf("expected normalized defaults in meta, got %+v", meta)
	}
	if meta.Total != 0 || meta.TotalPages != 0 {
		t.Errorf("expected zero totals, got %+v", meta)
	}
	if body.Data == nil {
		t.Error("expected data to serialize (empty slice, not null)")
	}
}
