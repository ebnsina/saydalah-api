package sales

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/middleware"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the sales handler and route registration.
type Module struct {
	handler *Handler
	svc     *Service
}

// New wires the sales module: store → repository → service → handler.
func New(s *store.Store) *Module {
	svc := NewService(NewRepository(s))
	return &Module{handler: NewHandler(svc), svc: svc}
}

// Service exposes the sales service so other modules (prescription dispensing)
// can reuse FEFO checkout.
func (m *Module) Service() *Service { return m.svc }

// Mount registers point-of-sale routes. Cashiers, pharmacists, and managers can
// all ring up sales.
func (m *Module) Mount(r chi.Router) {
	r.Route("/sales", func(r chi.Router) {
		r.Use(middleware.RequireRole(store.UserRoleCashier, store.UserRolePharmacist, store.UserRoleManager))
		r.Post("/", m.handler.create)
		r.Get("/", m.handler.list)
		r.Get("/{id}", m.handler.get)
	})
}
