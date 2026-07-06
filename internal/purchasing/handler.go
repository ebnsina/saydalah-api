package purchasing

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/httpx"
)

// Handler adapts HTTP requests to the purchasing Service.
type Handler struct {
	svc *Service
}

// NewHandler constructs a purchasing Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	var in CreateRequest
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	po, err := h.svc.Create(r.Context(), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, po)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	poID, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	po, err := h.svc.Get(r.Context(), id, poID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, po)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	branch := optionalBranch(r)
	res, err := h.svc.List(r.Context(), id, branch, httpx.ParsePagination(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.NewPage(res.Items, res.Total, r))
}

func (h *Handler) receive(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFrom(r.Context())
	if !ok {
		httpx.Error(w, r, httpx.ErrUnauthorized)
		return
	}
	poID, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in ReceiveRequest
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	po, err := h.svc.Receive(r.Context(), id, poID, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, po)
}

// optionalBranch reads an optional ?branch_id filter (used by admins to scope a
// listing to a specific branch).
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
