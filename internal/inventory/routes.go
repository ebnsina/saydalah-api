package inventory

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the inventory handler and route registration.
type Module struct {
	handler *Handler
}

// New wires the inventory module: store → repository → service → handler.
func New(s *store.Store) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(s)))}
}

// Mount registers inventory read routes, available to any authenticated staff.
func (m *Module) Mount(r chi.Router) {
	r.Route("/inventory", func(r chi.Router) {
		r.Get("/batches", m.handler.batches)
		r.Get("/near-expiry", m.handler.nearExpiry)
		r.Get("/low-stock", m.handler.lowStock)
		r.Get("/on-hand/{productID}", m.handler.onHand)
	})
}
