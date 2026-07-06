package user

import (
	"net/http"

	"github.com/ebnsina/saydalah-api/internal/httpx"
)

// Handler adapts HTTP requests to the user Service.
type Handler struct {
	svc *Service
}

// NewHandler constructs a user Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateRequest
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	u, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(u))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	u, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(u))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.List(r.Context(), httpx.ParsePagination(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items := make([]Response, len(res.Items))
	for i, u := range res.Items {
		items[i] = toResponse(u)
	}
	httpx.JSON(w, http.StatusOK, httpx.NewPage(items, res.Total, r))
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in UpdateRequest
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	u, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(u))
}

func (h *Handler) setPassword(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var in SetPasswordRequest
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.ResetPassword(r.Context(), id, in.Password); err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
