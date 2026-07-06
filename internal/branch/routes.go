package branch

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/middleware"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the branch handler and exposes route registration. Construct
// it with New and mount it onto an authenticated router in the composition root.
type Module struct {
	handler *Handler
}

// New wires the branch module: store → repository → service → handler.
func New(s *store.Store) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(s)))}
}

// Mount registers branch routes. Reads are open to any authenticated staff;
// writes require manager (admins are always allowed by RequireRole).
func (m *Module) Mount(r chi.Router) {
	r.Route("/branches", func(r chi.Router) {
		r.Get("/", m.handler.list)
		r.Get("/{id}", m.handler.get)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(store.UserRoleManager))
			r.Post("/", m.handler.create)
			r.Put("/{id}", m.handler.update)
		})
	})
}
