package supplier

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/middleware"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the supplier handler and route registration.
type Module struct {
	handler *Handler
}

// New wires the supplier module: store → repository → service → handler.
func New(s *store.Store) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(s)))}
}

// Mount registers supplier routes. Suppliers are maintained by managers; any
// authenticated staff may read them (needed when creating purchase orders).
func (m *Module) Mount(r chi.Router) {
	r.Route("/suppliers", func(r chi.Router) {
		r.Get("/", m.handler.list)
		r.Get("/{id}", m.handler.get)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(store.UserRoleManager))
			r.Post("/", m.handler.create)
			r.Put("/{id}", m.handler.update)
		})
	})
}
