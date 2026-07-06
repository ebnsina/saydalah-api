package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/ebnsina/saydalah-api/internal/auth"
	"github.com/ebnsina/saydalah-api/internal/branch"
	"github.com/ebnsina/saydalah-api/internal/catalog"
	"github.com/ebnsina/saydalah-api/internal/config"
	"github.com/ebnsina/saydalah-api/internal/customer"
	"github.com/ebnsina/saydalah-api/internal/httpx"
	"github.com/ebnsina/saydalah-api/internal/inventory"
	"github.com/ebnsina/saydalah-api/internal/middleware"
	"github.com/ebnsina/saydalah-api/internal/prescription"
	"github.com/ebnsina/saydalah-api/internal/purchasing"
	"github.com/ebnsina/saydalah-api/internal/reporting"
	"github.com/ebnsina/saydalah-api/internal/sales"
	"github.com/ebnsina/saydalah-api/internal/server"
	"github.com/ebnsina/saydalah-api/internal/stock"
	"github.com/ebnsina/saydalah-api/internal/store"
	"github.com/ebnsina/saydalah-api/internal/supplier"
	"github.com/ebnsina/saydalah-api/internal/user"
)

// registerModules constructs each feature module (repository → service →
// handler) and mounts its routes. This is the single wiring seam: dependencies
// are built here and flow in one direction. Public routes (login) sit outside
// the auth gate; everything else runs behind Authenticate.
func registerModules(srv *server.Server, st *store.Store, tm *auth.TokenManager, cfg config.Config) {
	api := srv.API()

	authMod := auth.New(st, tm)
	// Login gets a stricter, dedicated rate limit on top of the global one to
	// blunt credential brute-forcing.
	api.Group(func(r chi.Router) {
		r.Use(middleware.RateLimit(cfg.LoginRateRPS, cfg.LoginRateBurst))
		authMod.MountPublic(r)
	})

	api.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(tm))

		authMod.MountProtected(r)
		branch.New(st).Mount(r)
		user.New(st).Mount(r)
		catalog.New(st).Mount(r)
		supplier.New(st).Mount(r)
		purchasing.New(st).Mount(r)
		inventory.New(st).Mount(r)
		stock.New(st).Mount(r)

		salesMod := sales.New(st)
		salesMod.Mount(r)

		customer.New(st).Mount(r)
		prescription.New(st, salesMod.Service()).Mount(r)
		reporting.New(st).Mount(r)
	})
}

// bootstrapAdmin creates the first admin account from configuration when the
// users table is empty, so a freshly migrated database is immediately usable.
// It is a no-op when credentials are unset or any user already exists.
func bootstrapAdmin(ctx context.Context, st *store.Store, cfg config.Config, logger *slog.Logger) error {
	if cfg.AdminEmail == "" || cfg.AdminPassword == "" {
		return nil
	}
	count, err := st.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap admin: count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	svc := user.New(st).Service()
	if _, err := svc.Create(ctx, user.CreateRequest{
		Email:    cfg.AdminEmail,
		Password: cfg.AdminPassword,
		FullName: "Administrator",
		Role:     store.UserRoleAdmin,
	}); err != nil {
		// A concurrent instance may have created it first; treat conflict as success.
		if errors.Is(err, httpx.ErrConflict) {
			return nil
		}
		return fmt.Errorf("bootstrap admin: create: %w", err)
	}
	logger.Info("bootstrap: created initial admin", "email", cfg.AdminEmail)
	return nil
}
