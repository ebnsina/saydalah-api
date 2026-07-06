package catalog

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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

func (h *Handler) categories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.svc.Categories(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": cats})
}

// optionalStr returns a pointer to v, or nil when v is empty.
func optionalStr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
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
	q := r.URL.Query()
	filter := Filter{
		Search:   optionalStr(q.Get("search")),
		Category: optionalStr(q.Get("category")),
	}
	// active=true|false filters by status; absent means no filter.
	switch q.Get("active") {
	case "true":
		v := true
		filter.Active = &v
	case "false":
		v := false
		filter.Active = &v
	}
	// branch_id scopes the on-hand stock reported per product (POS availability).
	if raw := q.Get("branch_id"); raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			filter.BranchID = &id
		}
	}
	res, err := h.svc.List(r.Context(), filter, httpx.ParsePagination(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.NewPage(res.Items, res.Total, r))
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
