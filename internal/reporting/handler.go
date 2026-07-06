package reporting

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
)

// Handler adapts HTTP requests to the reporting Service.
type Handler struct {
	svc *Service
}

// NewHandler constructs a reporting Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) salesSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	res, err := h.svc.SalesSummary(r.Context(), id, optionalBranch(r), parseRange(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) salesDaily(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	items, err := h.svc.SalesDaily(r.Context(), id, optionalBranch(r), parseRange(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) inventoryValuation(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	res, err := h.svc.InventoryValuation(r.Context(), id, optionalBranch(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) topProducts(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	limit := int32(httpx.QueryInt(r, "limit", 10))
	items, err := h.svc.TopProducts(r.Context(), id, optionalBranch(r), parseRange(r), limit)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// parseRange reads ?from and ?to as YYYY-MM-DD dates. Defaults to the last 30
// days. The window is half-open [from, to): to is advanced by one day so the
// end date is inclusive.
func parseRange(r *http.Request) DateRange {
	const layout = "2006-01-02"
	now := time.Now()
	from := now.AddDate(0, 0, -30)
	to := now

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(layout, v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(layout, v); err == nil {
			to = t
		}
	}
	// Make the range half-open and end-inclusive.
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	to = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	return DateRange{From: from, To: to}
}

func optionalBranch(r *http.Request) *uuid.UUID {
	raw := r.URL.Query().Get("branch_id")
	if raw == "" {
		return nil
	}
	if id, err := uuid.Parse(raw); err == nil {
		return &id
	}
	return nil
}
