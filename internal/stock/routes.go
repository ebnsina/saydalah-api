package stock

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/middleware"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the stock handler and route registration.
type Module struct {
	handler *Handler
}

// New wires the stock module: store → repository → service → handler.
func New(s *store.Store) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(s)))}
}

// Mount registers manual stock-operation routes under /stock. Adjustments and
// returns are a pharmacist/manager responsibility; the ledger is readable by
// any authenticated staff.
func (m *Module) Mount(r chi.Router) {
	r.Route("/stock", func(r chi.Router) {
		r.Get("/movements", m.handler.movements)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(store.UserRolePharmacist, store.UserRoleManager))
			r.Post("/adjustments", m.handler.adjust)
			r.Post("/returns", m.handler.returnStock)
		})
	})
}
