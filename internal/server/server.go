// Package server assembles the HTTP router: global middleware, health probes,
// and the versioned /api/v1 group that feature modules mount their routes on.
// It knows nothing about individual domains — modules register themselves,
// keeping the composition root (cmd/api) the only place that wires everything.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ebnsina/saydalah-api/internal/config"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/middleware"
)

// Server wraps the root router and exposes the API group for module mounting.
type Server struct {
	mux  *chi.Mux
	api  chi.Router
	pool *pgxpool.Pool
}

// New builds the router with global middleware and health endpoints. Call API()
// to mount module routes, then Handler() to obtain the http.Handler to serve.
func New(cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool) *Server {
	mux := chi.NewRouter()

	mux.Use(middleware.RequestID)
	mux.Use(middleware.Recoverer(logger))
	mux.Use(middleware.Logger(logger))
	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Consistent JSON responses for unmatched routes/methods.
	mux.NotFound(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, httpx.ErrNotFound)
	})
	mux.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, r, httpx.NewError(http.StatusMethodNotAllowed, "method not allowed"))
	})

	s := &Server{mux: mux, pool: pool}
	s.registerHealth()

	// Feature modules mount their subrouters under this versioned group.
	mux.Route("/api/v1", func(r chi.Router) {
		s.api = r
	})
	return s
}

// API returns the /api/v1 router group for modules to register their routes.
func (s *Server) API() chi.Router { return s.api }

// Handler returns the fully assembled http.Handler.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) registerHealth() {
	// Liveness: the process is up.
	s.mux.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Readiness: dependencies (the database) are reachable.
	s.mux.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.pool.Ping(ctx); err != nil {
			httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}
