package prescription

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/middleware"
	"github.com/ebnsina/saydalah-api/internal/sales"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the prescription handler and route registration.
type Module struct {
	handler *Handler
}

// New wires the prescription module: store → repository → service → handler. It
// takes the sales service so dispensing reuses FEFO checkout.
func New(s *store.Store, salesSvc *sales.Service) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(s), salesSvc))}
}

// Mount registers prescription routes. Recording and dispensing prescriptions is
// a pharmacist/manager responsibility.
func (m *Module) Mount(r chi.Router) {
	r.Route("/prescriptions", func(r chi.Router) {
		r.Use(middleware.RequireRole(store.UserRolePharmacist, store.UserRoleManager))
		r.Post("/", m.handler.create)
		r.Get("/", m.handler.list)
		r.Get("/{id}", m.handler.get)
		r.Post("/{id}/dispense", m.handler.dispense)
	})
}
