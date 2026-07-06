package catalog

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/middleware"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the catalog handler and route registration.
type Module struct {
	handler *Handler
}

// New wires the catalog module: store → repository → service → handler.
func New(s *store.Store) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(s)))}
}

// Mount registers product routes. Any authenticated staff may read the catalog
// (needed at the point of sale); managers maintain it.
func (m *Module) Mount(r chi.Router) {
	r.Route("/products", func(r chi.Router) {
		r.Get("/", m.handler.list)
		r.Get("/categories", m.handler.categories)
		r.Get("/barcode/{code}", m.handler.getByBarcode)
		r.Get("/{id}", m.handler.get)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(store.UserRoleManager))
			r.Post("/", m.handler.create)
			r.Put("/{id}", m.handler.update)
		})
	})
}
