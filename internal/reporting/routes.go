package reporting

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/middleware"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the reporting handler and route registration.
type Module struct {
	handler *Handler
}

// New wires the reporting module: store → repository → service → handler.
func New(s *store.Store) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(s)))}
}

// Mount registers reporting routes, restricted to managers (and admins).
func (m *Module) Mount(r chi.Router) {
	r.Route("/reports", func(r chi.Router) {
		r.Use(middleware.RequireRole(store.UserRoleManager))
		r.Get("/sales-summary", m.handler.salesSummary)
		r.Get("/sales-daily", m.handler.salesDaily)
		r.Get("/inventory-valuation", m.handler.inventoryValuation)
		r.Get("/top-products", m.handler.topProducts)
	})
}
