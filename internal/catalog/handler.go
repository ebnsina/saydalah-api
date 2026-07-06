package catalog

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/httpx"
)

// Handler adapts HTTP requests to the catalog Service.
type Handler struct {
	svc *Service
}

// NewHandler constructs a catalog Handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in CreateRequest
	if err := httpx.Decode(w, r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	p, err := h.svc.Create(r.Context(), in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toResponse(p))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.URLParamUUID(r, "id")
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	p, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(p))
}

func (h *Handler) getByBarcode(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		httpx.Error(w, r, httpx.NewError(http.StatusBadRequest, "barcode is required"))
		return
	}
	p, err := h.svc.GetByBarcode(r.Context(), code)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(p))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	var search *string
	if q := r.URL.Query().Get("search"); q != "" {
		search = &q
	}
	res, err := h.svc.List(r.Context(), search, httpx.ParsePagination(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items := make([]Response, len(res.Items))
	for i, p := range res.Items {
		items[i] = toResponse(p)
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
	p, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(p))
}
