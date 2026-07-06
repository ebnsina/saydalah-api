package user

import (
	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/middleware"
	"github.com/ebnsina/saydalah-api/internal/store"
)

// Module bundles the user handler and route registration.
type Module struct {
	handler *Handler
	svc     *Service
}

// New wires the user module: store → repository → service → handler.
func New(s *store.Store) *Module {
	svc := NewService(NewRepository(s))
	return &Module{handler: NewHandler(svc), svc: svc}
}

// Service exposes the user service so the composition root can reuse it (e.g.
// the auth login flow and first-admin bootstrap need user lookups/creation).
func (m *Module) Service() *Service { return m.svc }

// Mount registers user routes. Managing staff accounts is restricted to
// managers and admins.
func (m *Module) Mount(r chi.Router) {
	r.Route("/users", func(r chi.Router) {
		r.Use(middleware.RequireRole(store.UserRoleManager))
		r.Get("/", m.handler.list)
		r.Post("/", m.handler.create)
		r.Get("/{id}", m.handler.get)
		r.Put("/{id}", m.handler.update)
		r.Put("/{id}/password", m.handler.setPassword)
	})
}
