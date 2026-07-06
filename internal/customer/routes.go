package customer

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the customer handler and route registration.
type Module struct {
	handler *Handler
	svc     *Service
}

// New wires the customer module: store → repository → service → handler.
func New(s *store.Store) *Module {
	svc := NewService(NewRepository(s))
	return &Module{handler: NewHandler(svc), svc: svc}
}

// Service exposes the customer service (unused externally for now, provided for
// symmetry with other modules that are reused).
func (m *Module) Service() *Service { return m.svc }

// Mount registers customer routes, available to any authenticated staff.
func (m *Module) Mount(r chi.Router) {
	r.Route("/customers", func(r chi.Router) {
		r.Get("/", m.handler.list)
		r.Post("/", m.handler.create)
		r.Get("/{id}", m.handler.get)
		r.Put("/{id}", m.handler.update)
	})
}
