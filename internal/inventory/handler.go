package inventory

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
)

// Handler adapts HTTP requests to the inventory Service.
type Handler struct {
	svc *Service
}

// NewHandler constructs an inventory Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) batches(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	res, err := h.svc.Batches(r.Context(), id, optionalBranch(r), httpx.ParsePagination(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.NewPage(res.Items, res.Total, r))
}

func (h *Handler) nearExpiry(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	within := int32(httpx.QueryInt(r, "within_days", 30))
	items, err := h.svc.NearExpiry(r.Context(), id, optionalBranch(r), within)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) lowStock(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	items, err := h.svc.LowStock(r.Context(), id, optionalBranch(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) onHand(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	productID, err := httpx.URLParamUUID(r, "productID")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.svc.OnHand(r.Context(), id, optionalBranch(r), productID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// optionalBranch reads an optional ?branch_id filter (admins scope views to a
// specific branch; branch staff are pinned to their own branch by the service).
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
