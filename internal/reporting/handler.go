package reporting

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/cache"
	"github.com/ebnsina/saydalah-api/internal/httpx"
)

// reportTTL is how long report responses stay cached. Reports tolerate slight
// staleness, so a short window absorbs dashboard/report traffic without a
// noticeable lag behind new sales.
const reportTTL = 60 * time.Second

// Handler adapts HTTP requests to the reporting Service, with an optional cache.
type Handler struct {
	svc   *Service
	cache *cache.Cache
}

// NewHandler constructs a reporting Handler.
func NewHandler(svc *Service, c *cache.Cache) *Handler { return &Handler{svc: svc, cache: c} }

// cached resolves the caller, serves the report from cache when present, or
// computes it and stores it. compute returns the exact JSON shape to send.
func (h *Handler) cached(w http.ResponseWriter, r *http.Request, name, extra string, compute func(auth.Identity) (any, error)) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	body, err := h.cache.GetOrSet(r.Context(), reportKey(name, id, r, extra), reportTTL, func() (any, error) {
		return compute(id)
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(body); err != nil {
		httpx.Error(w, r, err)
	}
}

// reportKey namespaces a cache entry by report, the caller's own branch scope
// (so a cashier's "my branch" can't read another branch's cache), the requested
// branch, the date window, and any extra discriminator (e.g. top-N limit).
func reportKey(name string, id auth.Identity, r *http.Request, extra string) string {
	scope := "chain"
	if id.BranchID != nil {
		scope = id.BranchID.String()
	}
	req := "all"
	if b := optionalBranch(r); b != nil {
		req = b.String()
	}
	rng := parseRange(r)
	return fmt.Sprintf("rpt:%s:%s:%s:%d:%d:%s", name, scope, req, rng.From.Unix(), rng.To.Unix(), extra)
}

func (h *Handler) salesSummary(w http.ResponseWriter, r *http.Request) {
	h.cached(w, r, "sales-summary", "", func(id auth.Identity) (any, error) {
		return h.svc.SalesSummary(r.Context(), id, optionalBranch(r), parseRange(r))
	})
}

func (h *Handler) salesDaily(w http.ResponseWriter, r *http.Request) {
	h.cached(w, r, "sales-daily", "", func(id auth.Identity) (any, error) {
		items, err := h.svc.SalesDaily(r.Context(), id, optionalBranch(r), parseRange(r))
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	})
}

func (h *Handler) salesByPayment(w http.ResponseWriter, r *http.Request) {
	h.cached(w, r, "sales-by-payment", "", func(id auth.Identity) (any, error) {
		items, err := h.svc.SalesByPayment(r.Context(), id, optionalBranch(r), parseRange(r))
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	})
}

func (h *Handler) inventoryValuation(w http.ResponseWriter, r *http.Request) {
	h.cached(w, r, "inventory-valuation", "", func(id auth.Identity) (any, error) {
		return h.svc.InventoryValuation(r.Context(), id, optionalBranch(r))
	})
}

func (h *Handler) topProducts(w http.ResponseWriter, r *http.Request) {
	limit := int32(httpx.QueryInt(r, "limit", 10))
	h.cached(w, r, "top-products", fmt.Sprintf("l%d", limit), func(id auth.Identity) (any, error) {
		items, err := h.svc.TopProducts(r.Context(), id, optionalBranch(r), parseRange(r), limit)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	})
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
