package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskflow/backend/internal/config"
	httperr "taskflow/backend/internal/errors"
	"taskflow/backend/internal/handlers"
	authmw "taskflow/backend/internal/middleware"
	"taskflow/backend/internal/realtime"
	"taskflow/backend/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	if err := runMigrations(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		slog.Error("migrations", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db pool", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		slog.Error("db ping", "err", err)
		os.Exit(1)
	}

	st := store.New(pool)
	ah := &handlers.AuthHandler{Store: st, JWTSecret: []byte(cfg.JWTSecret), BcryptCost: cfg.BcryptCost}
	hub := realtime.NewHub()
	ph := &handlers.ProjectsHandler{Store: st}
	th := &handlers.TasksHandler{Store: st, Bus: hub}
	eh := &handlers.EventsHandler{Store: st, Hub: hub, JWTSecret: []byte(cfg.JWTSecret)}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	frontend := os.Getenv("FRONTEND_ORIGIN")
	if frontend == "" {
		frontend = "http://localhost:3000"
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{frontend},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		httperr.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// EventSource can't send Authorization; ?token= and no request timeout on this route.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(0))
		r.Get("/projects/{id}/events", eh.Stream)
	})

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", ah.Register)
		r.Post("/login", ah.Login)
	})

	r.Group(func(r chi.Router) {
		r.Use(authmw.JWT([]byte(cfg.JWTSecret)))
		r.Use(middleware.Timeout(60 * time.Second))
		r.Get("/projects", ph.List)
		r.Post("/projects", ph.Create)
		r.Get("/projects/{id}/tasks", th.List)
		r.Post("/projects/{id}/tasks/reorder", th.Reorder)
		r.Post("/projects/{id}/tasks", th.Create)
		r.Get("/projects/{id}/stats", ph.Stats)
		r.Get("/projects/{id}/members", ph.Members)
		r.Get("/projects/{id}", ph.Get)
		r.Patch("/projects/{id}", ph.Patch)
		r.Delete("/projects/{id}", ph.Delete)
		r.Patch("/tasks/{id}", th.Patch)
		r.Delete("/tasks/{id}", th.Delete)
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
	slog.Info("stopped")
}

func runMigrations(databaseURL, migrationsPath string) error {
	abs, err := filepath.Abs(migrationsPath)
	if err != nil {
		return err
	}
	sourceURL := "file://" + filepath.ToSlash(abs)
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
