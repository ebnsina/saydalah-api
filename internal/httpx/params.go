package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Pagination holds normalized list parameters parsed from the query string.
type Pagination struct {
	Limit  int32
	Offset int32
}

// ParsePagination reads ?page and ?page_size, clamps them to sane bounds, and
// returns the corresponding LIMIT/OFFSET for a SQL query. Defaults: page 1,
// page_size 20; page_size is capped at 100.
func ParsePagination(r *http.Request) Pagination {
	page, size := pageParams(r)
	return Pagination{Limit: int32(size), Offset: int32((page - 1) * size)}
}

// pageParams reads and clamps ?page and ?page_size shared by ParsePagination
// and NewPage so both stay consistent.
func pageParams(r *http.Request) (page, size int) {
	page = max(QueryInt(r, "page", 1), 1)
	size = QueryInt(r, "page_size", 20)
	if size < 1 {
		size = 20
	}
	size = min(size, 100)
	return page, size
}

// Page is the envelope for paginated list responses: the items plus metadata a
// client needs to render pagination controls.
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// NewPage builds a Page envelope, echoing the request's page/page_size.
func NewPage[T any](items []T, total int64, r *http.Request) Page[T] {
	page, size := pageParams(r)
	return Page[T]{Items: items, Total: total, Page: page, PageSize: size}
}

// URLParamUUID parses a named chi URL parameter as a UUID, returning a
// client-facing 400 error when it is missing or malformed.
func URLParamUUID(r *http.Request, name string) (uuid.UUID, error) {
	raw := chi.URLParam(r, name)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, &APIError{Status: http.StatusBadRequest, Message: "invalid " + name}
	}
	return id, nil
}
