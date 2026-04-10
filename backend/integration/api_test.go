//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"taskflow/backend/internal/handlers"
	authmw "taskflow/backend/internal/middleware"
	"taskflow/backend/internal/store"
)

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "migrations")
}

func mustMigrate(t *testing.T, dsn string) {
	t.Helper()
	dir := migrationsDir(t)
	m, err := migrate.New("file://"+filepath.ToSlash(dir), dsn)
	require.NoError(t, err)
	defer m.Close()
	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		require.NoError(t, err)
	}
}

func TestAuthRegisterLoginFlow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run integration tests")
	}
	ctx := context.Background()
	mustMigrate(t, dsn)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	st := store.New(pool)
	ah := &handlers.AuthHandler{
		Store:      st,
		JWTSecret:  []byte("test-secret-key-for-integration-only-32b"),
		BcryptCost: 12,
	}

	r := chi.NewRouter()
	r.Post("/auth/register", ah.Register)
	r.Post("/auth/login", ah.Login)

	body := map[string]string{
		"name":     "Integration User",
		"email":    "integration-" + time.Now().Format("150405") + "@example.com",
		"password": "password123",
	}
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var reg map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &reg))
	require.Contains(t, reg, "token")

	loginBody := map[string]string{"email": body["email"], "password": "password123"}
	lb, err := json.Marshal(loginBody)
	require.NoError(t, err)
	req2 := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(lb))
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)
}

func TestProtectedRouteWithoutToken(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run integration tests")
	}
	ctx := context.Background()
	mustMigrate(t, dsn)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	st := store.New(pool)
	ph := &handlers.ProjectsHandler{Store: st}

	r := chi.NewRouter()
	r.Use(authmw.JWT([]byte("test-secret-key-for-integration-only-32b")))
	r.Get("/projects", ph.List)

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRegisterValidationErrorShape(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run integration tests")
	}
	ctx := context.Background()
	mustMigrate(t, dsn)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	st := store.New(pool)
	ah := &handlers.AuthHandler{
		Store:      st,
		JWTSecret:  []byte("test-secret-key-for-integration-only-32b"),
		BcryptCost: 12,
	}

	r := chi.NewRouter()
	r.Post("/auth/register", ah.Register)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte(`{"name":"","email":"bad","password":"short"}`)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var payload struct {
		Error  string            `json:"error"`
		Fields map[string]string `json:"fields"`
	}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&payload))
	require.Equal(t, "validation failed", payload.Error)
	require.NotEmpty(t, payload.Fields)
}
