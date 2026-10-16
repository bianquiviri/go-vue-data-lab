package handler

import "github.com/gin-gonic/gin"

// Pagination defaults and limits for list endpoints.
const (
	DefaultPage    = 1
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// Pagination is the reusable query-string paging contract of every list
// endpoint. It binds `page` and `per_page` from the query string; omitted or
// zero values fall back to the defaults, out-of-range values are rejected by
// the binding tags (400 validation_failed).
type Pagination struct {
	Page    int `form:"page" json:"page" binding:"omitempty,min=1"`
	PerPage int `form:"per_page" json:"per_page" binding:"omitempty,min=1,max=100"`
}

// Normalized returns a copy of p with the defaults applied and per_page
// clamped to MaxPerPage.
func (p Pagination) Normalized() Pagination {
	if p.Page < 1 {
		p.Page = DefaultPage
	}
	switch {
	case p.PerPage < 1:
		p.PerPage = DefaultPerPage
	case p.PerPage > MaxPerPage:
		p.PerPage = MaxPerPage
	}
	return p
}

// Offset returns the SQL offset for the normalized page.
func (p Pagination) Offset() int {
	n := p.Normalized()
	return (n.Page - 1) * n.PerPage
}

// Limit returns the SQL limit for the normalized per_page.
func (p Pagination) Limit() int {
	return p.Normalized().PerPage
}

// TotalPages returns the number of pages for total items, or 0 when empty.
func (p Pagination) TotalPages(total int64) int {
	perPage := int64(p.Normalized().PerPage)
	if total <= 0 || perPage <= 0 {
		return 0
	}
	return int((total + perPage - 1) / perPage)
}

// PageMeta is the metadata block of a paginated response.
type PageMeta struct {
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// PaginatedResponse is the stable envelope of every list endpoint:
//
//	{"data": [...], "meta": {"page":1,"per_page":20,"total":123,"total_pages":7}}
type PaginatedResponse struct {
	Data any      `json:"data"`
	Meta PageMeta `json:"meta"`
}

// RespondPaginated writes a paginated envelope with the given status code.
// p is normalized so meta always reflects the paging actually applied.
func RespondPaginated(c *gin.Context, status int, data any, p Pagination, total int64) {
	p = p.Normalized()
	RespondJSON(c, status, PaginatedResponse{
		Data: data,
		Meta: PageMeta{
			Page:       p.Page,
			PerPage:    p.PerPage,
			Total:      total,
			TotalPages: p.TotalPages(total),
		},
	})
}
