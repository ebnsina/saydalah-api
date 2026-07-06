package purchasing

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/middleware"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the purchasing handler and route registration.
type Module struct {
	handler *Handler
}

// New wires the purchasing module: store → repository → service → handler.
func New(s *store.Store) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(s)))}
}

// Mount registers purchasing routes. Ordering and receiving stock is a
// pharmacist/manager responsibility; cashiers are excluded.
func (m *Module) Mount(r chi.Router) {
	r.Route("/purchase-orders", func(r chi.Router) {
		r.Use(middleware.RequireRole(store.UserRoleManager, store.UserRolePharmacist))
		r.Post("/", m.handler.create)
		r.Get("/", m.handler.list)
		r.Get("/{id}", m.handler.get)
		r.Post("/{id}/receive", m.handler.receive)
	})
}
