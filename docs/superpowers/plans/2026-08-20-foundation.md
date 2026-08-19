# Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the backend skeleton — repo layout, local Postgres, config loading, DB connection, migration tooling, JSON error envelope, and a router with a working `/healthz` endpoint — so later plans (Auth, Child & Wishlist, Catalog & Admin) have solid ground to build domain features on.

**Architecture:** A Go module at `backend/` following the flat `internal/` package layout from the design spec (`config`, `db`, `httpapi`), backed by Postgres running via Docker Compose. Migrations are plain SQL files embedded into the binary via `embed.FS` and applied with golang-migrate. No domain logic (auth, children, wishlist) is implemented in this plan — only the plumbing every later plan depends on.

**Tech Stack:** Go 1.22+, chi (`github.com/go-chi/chi/v5`), sqlx (`github.com/jmoiron/sqlx`), pgx (`github.com/jackc/pgx/v5`, stdlib driver), golang-migrate (`github.com/golang-migrate/migrate/v4`), Postgres 16 via Docker Compose.

## Global Constraints

- Monorepo: `backend/` (Go) and `frontend/` (React/TS) as top-level dirs, `docker-compose.yml` at repo root. (spec §1)
- Backend: Go, JSON REST API via `net/http` + chi, no heavy framework. (spec §1, `docs/mvp-decisions.md`)
- DB access: sqlx, no ORM. (spec §1)
- Migrations: golang-migrate, plain up/down `.sql` files. (spec §1)
- Local dev DB: Postgres via Docker Compose. (spec §1)
- Backend package layout is flat: `internal/httpapi`, `internal/auth`, `internal/child`, `internal/wishlist`, `internal/catalog`, `internal/db`, `internal/config` — this plan only creates `httpapi`, `db`, `config`. (spec §2)
- Error responses use a uniform JSON envelope: `{"error": {"code": "...", "message": "..."}}`. (spec §5)
- Repository-layer tests run against a real Postgres (via Docker Compose), not mocks. (spec §6)

---

### Task 1: Repo scaffold + Postgres via Docker Compose

**Files:**
- Create: `docker-compose.yml`
- Create: `.gitignore`
- Create: `backend/go.mod`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: a `needtobuy` Go module at `backend/`; a Postgres instance reachable at `postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable` once `docker compose up -d` is run.

- [ ] **Step 1: Create `docker-compose.yml` at the repo root**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: needtobuy
      POSTGRES_PASSWORD: needtobuy
      POSTGRES_DB: needtobuy
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U needtobuy"]
      interval: 2s
      timeout: 2s
      retries: 15

volumes:
  pgdata:
```

- [ ] **Step 2: Create `.gitignore` at the repo root**

```
/backend/needtobuy
/backend/tmp/
*.log
```

- [ ] **Step 3: Initialize the Go module**

Run: `cd backend && go mod init needtobuy`
Expected: creates `backend/go.mod` containing `module needtobuy` and a `go` version line.

- [ ] **Step 4: Bring up Postgres and verify it's reachable**

Run: `docker compose up -d && docker compose exec postgres pg_isready -U needtobuy`
Expected: `postgres:5432 - accepting connections`

- [ ] **Step 5: Commit**

```bash
git add docker-compose.yml .gitignore backend/go.mod
git commit -m "Scaffold repo: docker-compose Postgres + backend Go module"
```

---

### Task 2: Config loading

**Files:**
- Create: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`

**Interfaces:**
- Consumes: nothing beyond stdlib `os`.
- Produces: `config.Config{DatabaseURL string, Port string}` and `config.Load() Config`, used by `cmd/server/main.go` in Task 6.

- [ ] **Step 1: Write the failing tests**

`backend/internal/config/config_test.go`:

```go
package config_test

import (
	"testing"

	"needtobuy/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	cfg := config.Load()

	want := "postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable"
	if cfg.DatabaseURL != want {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8080")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://custom/db")
	t.Setenv("PORT", "9090")

	cfg := config.Load()

	if cfg.DatabaseURL != "postgres://custom/db" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://custom/db")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/config/... -v`
Expected: FAIL — `package needtobuy/internal/config is not in std` / no `config.go` file yet (build failure).

- [ ] **Step 3: Write the implementation**

`backend/internal/config/config.go`:

```go
// Package config loads process-wide settings from environment variables.
package config

import "os"

// Config holds settings for the running process.
type Config struct {
	DatabaseURL string
	Port        string
}

// Load reads Config from environment variables, falling back to
// local-development defaults for anything unset.
func Load() Config {
	return Config{
		DatabaseURL: getenv("DATABASE_URL", "postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable"),
		Port:        getenv("PORT", "8080"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/config/... -v`
Expected: PASS — both `TestLoad_Defaults` and `TestLoad_FromEnv`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/config
git commit -m "Add config loading from environment variables"
```

---

### Task 3: DB connection wrapper + shared test helper

**Files:**
- Create: `backend/internal/db/db.go`
- Create: `backend/internal/dbtest/dbtest.go`
- Test: `backend/internal/db/db_test.go`

**Interfaces:**
- Consumes: a Postgres DSN string (from `config.Config.DatabaseURL` in later wiring).
- Produces: `db.Connect(dsn string) (*sqlx.DB, error)`; `dbtest.DSN(t *testing.T) string` — reused by Task 4 and Task 6 tests.

- [ ] **Step 1: Add dependencies**

Run: `cd backend && go get github.com/jmoiron/sqlx github.com/jackc/pgx/v5`
Expected: `go.mod`/`go.sum` updated with both modules.

- [ ] **Step 2: Create the shared test-DSN helper**

`backend/internal/dbtest/dbtest.go`:

```go
// Package dbtest provides shared helpers for tests that need a real
// Postgres connection (started via `docker compose up -d`).
package dbtest

import (
	"os"
	"testing"
)

// DSN returns the TEST_DATABASE_URL environment variable, skipping the
// calling test if it isn't set.
func DSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; run `docker compose up -d` and export TEST_DATABASE_URL=postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable to run this test")
	}
	return dsn
}
```

- [ ] **Step 3: Write the failing tests**

`backend/internal/db/db_test.go`:

```go
package db_test

import (
	"testing"

	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
)

func TestConnect_InvalidDSN(t *testing.T) {
	_, err := db.Connect("not-a-valid-dsn")
	if err == nil {
		t.Fatal("expected error for invalid DSN, got nil")
	}
}

func TestConnect_Success(t *testing.T) {
	dsn := dbtest.DSN(t)

	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd backend && go test ./internal/db/... -v`
Expected: FAIL — `db.Connect` undefined (build failure).

- [ ] **Step 5: Write the implementation**

`backend/internal/db/db.go`:

```go
// Package db provides the Postgres connection and migration tooling
// shared by every domain package.
package db

import (
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// Connect opens a connection pool to Postgres at dsn and verifies
// connectivity with a ping.
func Connect(dsn string) (*sqlx.DB, error) {
	conn, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: connect: %w", err)
	}
	return conn, nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `docker compose up -d && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/db/... -v` (from `backend/`)
Expected: PASS — `TestConnect_InvalidDSN` and `TestConnect_Success`.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/db backend/internal/dbtest backend/go.mod backend/go.sum
git commit -m "Add Postgres connection wrapper and shared DB test helper"
```

---

### Task 4: Migration tooling + first migration (users table)

**Files:**
- Create: `backend/migrations/migrations.go`
- Create: `backend/migrations/000001_create_users.up.sql`
- Create: `backend/migrations/000001_create_users.down.sql`
- Create: `backend/internal/db/migrate.go`
- Test: `backend/internal/db/migrate_test.go`

**Interfaces:**
- Consumes: `dbtest.DSN(t *testing.T) string` (Task 3).
- Produces: `db.Migrate(dsn string) error`; a `users` table (`id`, `email`, `created_at`) in Postgres once run. `db.Migrate` is called by `cmd/server/main.go` in Task 6, and will be extended with further `.sql` files by the Auth plan.

- [ ] **Step 1: Add the golang-migrate dependency**

Run: `cd backend && go get github.com/golang-migrate/migrate/v4`
Expected: `go.mod`/`go.sum` updated.

- [ ] **Step 2: Create the embedded migrations package**

`backend/migrations/migrations.go`:

```go
// Package migrations embeds the golang-migrate SQL files so they ship
// inside the compiled binary.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

- [ ] **Step 3: Write the first migration**

`backend/migrations/000001_create_users.up.sql`:

```sql
CREATE TABLE users (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`backend/migrations/000001_create_users.down.sql`:

```sql
DROP TABLE users;
```

- [ ] **Step 4: Write the failing tests**

`backend/internal/db/migrate_test.go`:

```go
package db_test

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
)

func TestMigrate_CreatesUsersTable(t *testing.T) {
	dsn := dbtest.DSN(t)

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer conn.Close()

	var tableName string
	if err := conn.QueryRow("SELECT to_regclass('public.users')::text").Scan(&tableName); err != nil {
		t.Fatalf("query error = %v", err)
	}
	if tableName != "users" {
		t.Fatalf("expected users table to exist, got %q", tableName)
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {
	dsn := dbtest.DSN(t)

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
}
```

- [ ] **Step 5: Run tests to verify they fail**

Run: `cd backend && go test ./internal/db/... -run TestMigrate -v`
Expected: FAIL — `db.Migrate` undefined (build failure).

- [ ] **Step 6: Write the implementation**

`backend/internal/db/migrate.go`:

```go
package db

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"needtobuy/migrations"
)

// Migrate applies all pending up migrations to the database at dsn. It
// returns nil if the schema is already up to date.
func Migrate(dsn string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("migrate: load source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: up: %w", err)
	}
	return nil
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `docker compose up -d && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/db/... -v` (from `backend/`)
Expected: PASS — all tests in `internal/db`, including `TestMigrate_CreatesUsersTable` and `TestMigrate_IsIdempotent`.

- [ ] **Step 8: Commit**

```bash
git add backend/migrations backend/internal/db backend/go.mod backend/go.sum
git commit -m "Add migration tooling and initial users table migration"
```

---

### Task 5: JSON error envelope + response helpers

**Files:**
- Create: `backend/internal/httpapi/errors.go`
- Create: `backend/internal/httpapi/response.go`
- Test: `backend/internal/httpapi/errors_test.go`
- Test: `backend/internal/httpapi/response_test.go`

**Interfaces:**
- Consumes: nothing beyond stdlib.
- Produces: `httpapi.Error{Code, Message string; HTTPStatus int}`; `httpapi.NotFound(what string) *Error`; `httpapi.BadRequest(message string) *Error`; `httpapi.Internal(message string) *Error`; `httpapi.WriteJSON(w http.ResponseWriter, status int, v any)`; `httpapi.WriteError(w http.ResponseWriter, err *Error)`. Used by the health handler in Task 6 and by every domain handler in later plans.

- [ ] **Step 1: Write the failing tests**

`backend/internal/httpapi/errors_test.go`:

```go
package httpapi_test

import (
	"net/http"
	"testing"

	"needtobuy/internal/httpapi"
)

func TestNotFound(t *testing.T) {
	err := httpapi.NotFound("child")

	if err.Code != "not_found" {
		t.Errorf("Code = %q, want %q", err.Code, "not_found")
	}
	if err.HTTPStatus != http.StatusNotFound {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusNotFound)
	}
	if err.Message != "child not found" {
		t.Errorf("Message = %q, want %q", err.Message, "child not found")
	}
}

func TestBadRequest(t *testing.T) {
	err := httpapi.BadRequest("email is required")

	if err.Code != "bad_request" {
		t.Errorf("Code = %q, want %q", err.Code, "bad_request")
	}
	if err.HTTPStatus != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusBadRequest)
	}
}

func TestInternal(t *testing.T) {
	err := httpapi.Internal("database unavailable")

	if err.Code != "internal" {
		t.Errorf("Code = %q, want %q", err.Code, "internal")
	}
	if err.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusInternalServerError)
	}
}
```

`backend/internal/httpapi/response_test.go`:

```go
package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"needtobuy/internal/httpapi"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	httpapi.WriteJSON(rec, http.StatusCreated, map[string]string{"status": "ok"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`body["status"] = %q, want "ok"`, body["status"])
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()

	httpapi.WriteError(rec, httpapi.NotFound("child"))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("error.code = %q, want not_found", body.Error.Code)
	}
	if body.Error.Message != "child not found" {
		t.Errorf("error.message = %q, want %q", body.Error.Message, "child not found")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/httpapi/... -v`
Expected: FAIL — package `needtobuy/internal/httpapi` has no Go files yet (build failure).

- [ ] **Step 3: Write the implementation**

`backend/internal/httpapi/errors.go`:

```go
// Package httpapi holds the HTTP router, middleware, and response
// helpers shared by every domain handler.
package httpapi

import "net/http"

// Error is an application error that maps directly to an HTTP response.
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *Error) Error() string { return e.Message }

// NotFound builds a 404 error for a missing resource named what
// (e.g. "child").
func NotFound(what string) *Error {
	return &Error{Code: "not_found", Message: what + " not found", HTTPStatus: http.StatusNotFound}
}

// BadRequest builds a 400 error with the given human-readable message.
func BadRequest(message string) *Error {
	return &Error{Code: "bad_request", Message: message, HTTPStatus: http.StatusBadRequest}
}

// Internal builds a 500 error with the given human-readable message.
func Internal(message string) *Error {
	return &Error{Code: "internal", Message: message, HTTPStatus: http.StatusInternalServerError}
}
```

`backend/internal/httpapi/response.go`:

```go
package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON writes v as a JSON response body with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httpapi: write json: %v", err)
	}
}

type errorEnvelope struct {
	Error *Error `json:"error"`
}

// WriteError writes err as the standard {"error": {...}} JSON envelope.
func WriteError(w http.ResponseWriter, err *Error) {
	WriteJSON(w, err.HTTPStatus, errorEnvelope{Error: err})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/httpapi/... -v`
Expected: PASS — `TestNotFound`, `TestBadRequest`, `TestInternal`, `TestWriteJSON`, `TestWriteError`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/httpapi
git commit -m "Add JSON error envelope and response helpers"
```

---

### Task 6: Router, health endpoint, and server entrypoint

**Files:**
- Create: `backend/internal/httpapi/router.go`
- Create: `backend/cmd/server/main.go`
- Test: `backend/internal/httpapi/router_test.go`

**Interfaces:**
- Consumes: `db.Connect(dsn string) (*sqlx.DB, error)` (Task 3), `db.Migrate(dsn string) error` (Task 4), `config.Load() config.Config` (Task 2), `httpapi.WriteJSON`/`httpapi.WriteError`/`httpapi.Internal` (Task 5), `dbtest.DSN(t *testing.T) string` (Task 3).
- Produces: `httpapi.NewRouter(database *sqlx.DB) http.Handler` — the router later plans register their domain routes on; a running server binary via `cmd/server/main.go`; `GET /healthz`.

- [ ] **Step 1: Add the chi dependency**

Run: `cd backend && go get github.com/go-chi/chi/v5`
Expected: `go.mod`/`go.sum` updated.

- [ ] **Step 2: Write the failing tests**

`backend/internal/httpapi/router_test.go`:

```go
package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

func TestHealthz_OK(t *testing.T) {
	dsn := dbtest.DSN(t)

	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	router := httpapi.NewRouter(conn)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHealthz_DatabaseDown(t *testing.T) {
	dsn := dbtest.DSN(t)

	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	conn.Close() // force the ping in the handler to fail

	router := httpapi.NewRouter(conn)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd backend && go test ./internal/httpapi/... -run TestHealthz -v`
Expected: FAIL — `httpapi.NewRouter` undefined (build failure).

- [ ] **Step 4: Write the router implementation**

`backend/internal/httpapi/router.go`:

```go
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
)

// NewRouter builds the top-level chi router. database is used by the
// health check to verify connectivity; domain packages register their
// own routes on the returned handler in later plans.
func NewRouter(database *sqlx.DB) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/healthz", healthHandler(database))

	return r
}

func healthHandler(database *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := database.PingContext(r.Context()); err != nil {
			WriteError(w, Internal("database unavailable"))
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `docker compose up -d && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/httpapi/... -v` (from `backend/`)
Expected: PASS — all tests in `internal/httpapi`, including `TestHealthz_OK` and `TestHealthz_DatabaseDown`.

- [ ] **Step 6: Write the server entrypoint**

`backend/cmd/server/main.go`:

```go
// Command server runs the NeedToBuy HTTP API.
package main

import (
	"log"
	"net/http"

	"needtobuy/internal/config"
	"needtobuy/internal/db"
	"needtobuy/internal/httpapi"
)

func main() {
	cfg := config.Load()

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer database.Close()

	router := httpapi.NewRouter(database)

	log.Printf("listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
```

- [ ] **Step 7: Verify the server runs end-to-end**

Run (from `backend/`): `docker compose up -d && go run ./cmd/server &` then `curl -i http://localhost:8080/healthz`, then stop the background process.
Expected: `HTTP/1.1 200 OK` with body `{"status":"ok"}`.

- [ ] **Step 8: Run the full backend test suite**

Run (from `backend/`): `docker compose up -d && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./... -v`
Expected: PASS — every test across `internal/config`, `internal/db`, `internal/httpapi`.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/httpapi backend/cmd backend/go.mod backend/go.sum
git commit -m "Add router, health endpoint, and server entrypoint"
```
