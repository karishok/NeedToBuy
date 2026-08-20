# Frontend Foundation + OTP Login Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the React+TypeScript SPA (`frontend/`) with the Organic design system ported in, and wire a fully working OTP login (email → code → session, restored across page reload) to the real Go backend.

**Architecture:** A Vite-scaffolded React app renders the Organic design system's CSS classes through small typed wrapper components (Button, Card, Field), a thin `fetch`-based API client talks to the existing `/api/auth/*` endpoints, and a React context (`AuthProvider`) holds session state — restored on load via one new backend endpoint, `GET /api/auth/me`, since the session cookie is `HttpOnly` and invisible to JS. A route guard shows `/login` to anonymous visitors and a placeholder authenticated screen to everyone else, standing in for the wishlist screen a later slice will build.

**Tech Stack:** Vite, React, TypeScript, react-router-dom, Vitest, React Testing Library, `@testing-library/user-event`. Backend addition: Go, `chi`, `sqlx` (matches the existing `backend/internal/auth` package).

## Global Constraints

- Routing: `react-router-dom`. No other routing library.
- API client: a hand-written `fetch` wrapper in `frontend/src/api/client.ts`, `credentials: 'include'` on every request. No TanStack Query/SWR/axios — YAGNI for four endpoints.
- `frontend/src/styles/organic.css` is `styles.css` from the mockup, copied byte-for-byte, never hand-edited. Any app-specific CSS this slice needs (there is exactly one rule) goes in a separate `frontend/src/styles/app.css`.
- Error text color: `var(--color-accent-700)` — Organic has no dedicated error/red token; do not invent one.
- Testing: Vitest + React Testing Library for the frontend; the existing `httptest` + `dbtest.Tx` pattern for the one backend change.
- `GET /api/auth/me` is the only backend change in this plan: `200 {"email": "..."}` for a valid session, `401` via the existing `apierr` envelope otherwise. Registered behind the existing `authHandler.Middleware` + `auth.RequireAuth`.
- Every new/changed Go file keeps the existing repo convention: doc comment on the package and every exported identifier, errors wrapped with `fmt.Errorf("<pkg>: <verb>: %w", err)`.
- Source spec: [[docs/superpowers/specs/2026-08-21-frontend-foundation-auth-design.md]] (addendum to [[docs/superpowers/specs/2026-08-20-system-architecture-design.md]]).

---

## Task 1: Backend — `GET /api/auth/me`

**Files:**
- Modify: `backend/internal/auth/user.go`
- Modify: `backend/internal/auth/handler.go`
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/internal/auth/handler_test.go`
- Modify: `backend/internal/httpapi/auth_flow_test.go`

**Interfaces:**
- Consumes: `Querier`, `UserID(ctx) (int64, bool)`, `RequireAuth`, `apierr.WriteJSON`/`WriteError`/`Internal` — all already defined in the package.
- Produces: `emailByUserID(ctx, db Querier, userID int64) (string, error)`; `(*Handler).Me(w, r)`; route `GET /api/auth/me`. Later frontend tasks call this route directly (not the Go symbols) via `fetch`.

- [ ] **Step 1: Write the failing tests**

Append to `backend/internal/auth/handler_test.go`:

```go
func TestMe_NoCookie_Unauthorized(t *testing.T) {
	h, _ := newTestHandler(t)
	protected := h.Middleware(RequireAuth(http.HandlerFunc(h.Me)))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMe_ValidSession_ReturnsEmail(t *testing.T) {
	h, mailer := newTestHandler(t)
	protected := h.Middleware(RequireAuth(http.HandlerFunc(h.Me)))

	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"parent@example.com"}`))
	h.RequestOTP(httptest.NewRecorder(), reqReq)
	verifyBody := `{"email":"parent@example.com","code":"` + mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	h.VerifyOTP(verifyRec, verifyReq)
	sessionCookieValue := verifyRec.Result().Cookies()[0]

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(sessionCookieValue)
	meRec := httptest.NewRecorder()
	protected.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", meRec.Code, http.StatusOK, meRec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(meRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["email"] != "parent@example.com" {
		t.Fatalf(`email = %q, want "parent@example.com"`, body["email"])
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/auth/... -run TestMe -v`
Expected: FAIL — `h.Me undefined (type *Handler has no field or method Me)`

- [ ] **Step 3: Implement `emailByUserID` and `Me`**

Append to `backend/internal/auth/user.go`:

```go
// emailByUserID returns the email for a user id, used by GET /api/auth/me
// to report who's logged in.
func emailByUserID(ctx context.Context, db Querier, userID int64) (string, error) {
	var email string
	if err := db.GetContext(ctx, &email, `SELECT email FROM users WHERE id = $1`, userID); err != nil {
		return "", fmt.Errorf("auth: lookup email for user %d: %w", userID, err)
	}
	return email, nil
}
```

Append to `backend/internal/auth/handler.go` (after `Logout`, before `Middleware`):

```go
// Me handles GET /api/auth/me. It must be registered behind RequireAuth —
// it reports 401 defensively if that invariant is ever violated.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserID(r.Context())
	if !ok {
		apierr.WriteError(w, unauthorized("login required"))
		return
	}
	email, err := emailByUserID(r.Context(), h.db, userID)
	if err != nil {
		log.Printf("auth: lookup email for user %d: %v", userID, err)
		apierr.WriteError(w, apierr.Internal("could not load account"))
		return
	}
	apierr.WriteJSON(w, http.StatusOK, map[string]string{"email": email})
}
```

In `backend/internal/httpapi/router.go`, add the route registration inside `NewRouter`, right after the three existing `r.Post("/api/auth/...")` lines:

```go
	r.With(auth.RequireAuth).Get("/api/auth/me", authHandler.Me)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/auth/... -run TestMe -v`
Expected: PASS — `TestMe_NoCookie_Unauthorized`, `TestMe_ValidSession_ReturnsEmail`

- [ ] **Step 5: Replace the throwaway `/test/whoami` probe with the real endpoint in the end-to-end test**

`backend/internal/httpapi/auth_flow_test.go` currently registers a temporary `/test/whoami` route (type-asserting the router to `chi.Router`) to prove the session middleware works, because no real protected route existed yet. Now one does — replace the whole file with:

```go
package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"needtobuy/internal/auth"
	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

// capturingMailer records the last code it was asked to send.
//
// This test writes one real, non-transactional user/session row (unlike
// the rest of the suite, which uses dbtest.Tx) because httpapi.NewRouter
// needs a live *sqlx.DB for its health check, not a transaction. The email
// is randomized per run so repeat runs don't collide; the local dev
// database is disposable (`docker compose down -v` resets it).
type capturingMailer struct {
	lastCode string
}

func (m *capturingMailer) SendOTP(_ context.Context, _ string, code string) error {
	m.lastCode = code
	return nil
}

func TestAuthFlow_RequestVerifyLogout_EndToEnd(t *testing.T) {
	dsn := dbtest.DSN(t)
	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	mailer := &capturingMailer{}
	authHandler := auth.NewHandler(conn, mailer, "pepper")
	router := httpapi.NewRouter(conn, authHandler)
	email := fmt.Sprintf("e2e-%d@example.com", time.Now().UnixNano())

	reqBody := fmt.Sprintf(`{"email":%q}`, email)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("request: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	verifyBody := fmt.Sprintf(`{"email":%q,"code":%q}`, email, mailer.lastCode)
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	router.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify: status = %d, body = %s", verifyRec.Code, verifyRec.Body.String())
	}
	cookies := verifyRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("verify: no session cookie set")
	}
	sessionCookie := cookies[0]

	// No cookie at all: /api/auth/me must reject.
	noCookieReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	noCookieRec := httptest.NewRecorder()
	router.ServeHTTP(noCookieRec, noCookieReq)
	if noCookieRec.Code != http.StatusUnauthorized {
		t.Fatalf("me without cookie: status = %d, want %d", noCookieRec.Code, http.StatusUnauthorized)
	}

	// With the real session cookie: /api/auth/me must report the email.
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me with cookie: status = %d, body = %s", meRec.Code, meRec.Body.String())
	}
	var me map[string]string
	if err := json.Unmarshal(meRec.Body.Bytes(), &me); err != nil {
		t.Fatalf("unmarshal me body: %v", err)
	}
	if me["email"] != email {
		t.Fatalf("me email = %q, want %q", me["email"], email)
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthReq.AddCookie(sessionCookie)
	healthRec := httptest.NewRecorder()
	router.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("healthz with session cookie: status = %d, body = %s", healthRec.Code, healthRec.Body.String())
	}
	var health map[string]string
	if err := json.Unmarshal(healthRec.Body.Bytes(), &health); err != nil {
		t.Fatalf("unmarshal healthz body: %v", err)
	}
	if health["status"] != "ok" {
		t.Fatalf(`healthz status = %q, want "ok"`, health["status"])
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout: status = %d, body = %s", logoutRec.Code, logoutRec.Body.String())
	}

	// Same session cookie after logout: the session must be revoked.
	postLogoutReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	postLogoutReq.AddCookie(sessionCookie)
	postLogoutRec := httptest.NewRecorder()
	router.ServeHTTP(postLogoutRec, postLogoutReq)
	if postLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: status = %d, want %d", postLogoutRec.Code, http.StatusUnauthorized)
	}
}
```

(This drops the `github.com/go-chi/chi/v5` import entirely — no more type-asserting the router.)

- [ ] **Step 6: Run the full backend suite**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./... -v`
Expected: PASS — every package.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/auth backend/internal/httpapi
git commit -m "Add GET /api/auth/me"
```

---

## Task 2: Scaffold the Vite + React + TypeScript app

**Files:**
- Create: `frontend/` (via `npm create vite@latest`)
- Modify: `frontend/vite.config.ts`
- Create: `frontend/src/setupTests.ts`
- Create: `frontend/src/App.test.tsx`
- Modify: `frontend/package.json` (`test` script)

**Interfaces:**
- Produces: a buildable, testable `frontend/` app with `npm run build` and `npm test` both green. Every later task's dev loop runs inside this scaffold.

- [ ] **Step 1: Scaffold the app**

Run from the repo root:
```bash
npm create vite@latest frontend -- --template react-ts
cd frontend
npm install
npm install react-router-dom
npm install -D vitest @testing-library/react @testing-library/jest-dom @testing-library/user-event jsdom
```

- [ ] **Step 2: Configure Vite for the API proxy and Vitest**

Replace `frontend/vite.config.ts` with:

```ts
/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/setupTests.ts',
  },
})
```

`frontend/src/setupTests.ts`:

```ts
import '@testing-library/jest-dom/vitest'
```

Add the test script:
```bash
npm pkg set scripts.test="vitest run"
```

- [ ] **Step 3: Write a smoke test proving the toolchain works end to end**

`frontend/src/App.test.tsx`:

```tsx
import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import App from './App'

describe('App', () => {
  it('renders without crashing', () => {
    const { container } = render(<App />)
    expect(container).not.toBeEmptyDOMElement()
  })
})
```

(This test is intentionally content-agnostic — it survives Task 7 replacing `App`'s contents. Task 7 replaces this file with real routing assertions once there's routing to assert on.)

- [ ] **Step 4: Run the test and the build**

Run (from `frontend/`): `npm test`
Expected: PASS — 1 test.

Run: `npm run build`
Expected: exits 0, produces `dist/`.

- [ ] **Step 5: Commit**

```bash
git add frontend
git commit -m "Scaffold Vite + React + TypeScript frontend"
```

---

## Task 3: Port the Organic design tokens and base components

**Files:**
- Create: `frontend/src/styles/organic.css`
- Modify: `frontend/src/main.tsx`
- Modify: `frontend/src/App.tsx`
- Delete: `frontend/src/index.css`, `frontend/src/App.css`
- Create: `frontend/src/components/Button.tsx`, `frontend/src/components/Button.test.tsx`
- Create: `frontend/src/components/Card.tsx`, `frontend/src/components/Card.test.tsx`
- Create: `frontend/src/components/Field.tsx`, `frontend/src/components/Field.test.tsx`

**Interfaces:**
- Produces: `<Button variant="primary"|"secondary"|"ghost" block? …>`, `<Card kicker?: string>`, `<Field label: string, id: string, ...input props>` — Task 6 (`LoginPage`) and Task 7 (`PlaceholderHome`) build their screens out of these three.

- [ ] **Step 1: Extract the design system's stylesheet from the mockup archive**

The mockup archive is already committed at the repo root. Run from the repo root:
```bash
mkdir -p frontend/src/styles
unzip -p "Scope clarification needed.zip" "_ds/organic-6416d56d-c2d8-4c20-aeca-ca9373d02961/styles.css" > frontend/src/styles/organic.css
```
Verify: `head -5 frontend/src/styles/organic.css` should show the `Organic — design-system tokens...` comment header.

- [ ] **Step 2: Wire the stylesheet in and drop the default Vite styles**

```bash
rm frontend/src/index.css frontend/src/App.css
```

In `frontend/src/App.tsx`, delete the line `import './App.css'` (leave the rest of the scaffolded content untouched — Task 7 replaces it wholesale).

Replace `frontend/src/main.tsx` with:

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/organic.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

- [ ] **Step 3: Run the existing tests to confirm nothing broke**

Run (from `frontend/`): `npm test`
Expected: PASS — the Task 2 smoke test still passes (it only checks the container isn't empty).

- [ ] **Step 4: Write the failing component tests**

`frontend/src/components/Button.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Button } from './Button'

describe('Button', () => {
  it('applies the primary variant class by default', () => {
    render(<Button>Click me</Button>)
    const button = screen.getByRole('button', { name: 'Click me' })
    expect(button.className).toContain('btn-primary')
  })

  it('applies the requested variant and block modifier', () => {
    render(
      <Button variant="ghost" block>
        Cancel
      </Button>,
    )
    const button = screen.getByRole('button', { name: 'Cancel' })
    expect(button.className).toContain('btn-ghost')
    expect(button.className).toContain('btn-block')
  })
})
```

`frontend/src/components/Card.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Card } from './Card'

describe('Card', () => {
  it('renders the kicker and children', () => {
    render(<Card kicker="Вход">Hello</Card>)
    expect(screen.getByText('Вход')).toHaveClass('card-kicker')
    expect(screen.getByText('Hello')).toBeInTheDocument()
  })
})
```

`frontend/src/components/Field.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Field } from './Field'

describe('Field', () => {
  it('associates the label with the input via id', () => {
    render(<Field id="email" label="Email" />)
    const input = screen.getByLabelText('Email')
    expect(input).toHaveClass('input')
  })
})
```

- [ ] **Step 5: Run to verify they fail**

Run (from `frontend/`): `npm test`
Expected: FAIL — `Failed to resolve import "./Button"` (and `./Card`, `./Field`)

- [ ] **Step 6: Implement the components**

`frontend/src/components/Button.tsx`:

```tsx
import type { ButtonHTMLAttributes } from 'react'

type Variant = 'primary' | 'secondary' | 'ghost'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  block?: boolean
}

export function Button({ variant = 'primary', block = false, className, ...rest }: ButtonProps) {
  const classes = ['btn', `btn-${variant}`, block ? 'btn-block' : '', className].filter(Boolean).join(' ')
  return <button className={classes} {...rest} />
}
```

`frontend/src/components/Card.tsx`:

```tsx
import type { ReactNode } from 'react'

interface CardProps {
  kicker?: string
  children: ReactNode
  className?: string
}

export function Card({ kicker, children, className }: CardProps) {
  const classes = ['card', 'elev-md', className].filter(Boolean).join(' ')
  return (
    <div className={classes}>
      {kicker ? <div className="card-kicker">{kicker}</div> : null}
      {children}
    </div>
  )
}
```

`frontend/src/components/Field.tsx`:

```tsx
import type { InputHTMLAttributes } from 'react'

interface FieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string
}

export function Field({ label, id, ...rest }: FieldProps) {
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      <input id={id} className="input" {...rest} />
    </div>
  )
}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run (from `frontend/`): `npm test`
Expected: PASS — all tests, including Task 2's smoke test.

- [ ] **Step 8: Commit**

```bash
git add frontend
git commit -m "Port Organic design tokens and add base components"
```

---

## Task 4: API client

**Files:**
- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/api/client.test.ts`

**Interfaces:**
- Produces: `class ApiError extends Error { code: string; status: number }`; `requestOtp(email: string): Promise<{status: string}>`; `verifyOtp(email: string, code: string): Promise<{status: string}>`; `logout(): Promise<{status: string}>`; `me(): Promise<{email: string}>`. Tasks 5 and 6 import all of these.

- [ ] **Step 1: Write the failing tests**

`frontend/src/api/client.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest'
import { requestOtp, verifyOtp, me } from './client'

afterEach(() => {
  vi.unstubAllGlobals()
})

function mockFetch(status: number, body: unknown) {
  const fetchMock = vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

describe('requestOtp', () => {
  it('posts the email and includes credentials', async () => {
    const fetchMock = mockFetch(200, { status: 'sent' })
    await requestOtp('parent@example.com')

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/auth/otp/request',
      expect.objectContaining({
        method: 'POST',
        credentials: 'include',
        body: JSON.stringify({ email: 'parent@example.com' }),
      }),
    )
  })
})

describe('verifyOtp', () => {
  it('throws an ApiError with the server message on a 400', async () => {
    mockFetch(400, { error: { code: 'bad_request', message: 'invalid or expired code' } })

    await expect(verifyOtp('parent@example.com', '000000')).rejects.toMatchObject({
      code: 'bad_request',
      message: 'invalid or expired code',
    })
  })
})

describe('me', () => {
  it('resolves with the email on success', async () => {
    mockFetch(200, { email: 'parent@example.com' })
    await expect(me()).resolves.toEqual({ email: 'parent@example.com' })
  })

  it('throws on a 401', async () => {
    mockFetch(401, { error: { code: 'unauthorized', message: 'login required' } })
    await expect(me()).rejects.toMatchObject({ status: 401 })
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `npm test`
Expected: FAIL — `Failed to resolve import "./client"`

- [ ] **Step 3: Implement the client**

`frontend/src/api/client.ts`:

```ts
export class ApiError extends Error {
  code: string
  status: number

  constructor(code: string, message: string, status: number) {
    super(message)
    this.code = code
    this.status = status
  }
}

interface ErrorEnvelope {
  error?: { code: string; message: string }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })

  if (!response.ok) {
    let envelope: ErrorEnvelope = {}
    try {
      envelope = (await response.json()) as ErrorEnvelope
    } catch {
      // Response body wasn't JSON — fall through to the generic error below.
    }
    throw new ApiError(
      envelope.error?.code ?? 'unknown',
      envelope.error?.message ?? `Request failed with status ${response.status}`,
      response.status,
    )
  }

  return (await response.json()) as T
}

export function requestOtp(email: string): Promise<{ status: string }> {
  return request('/api/auth/otp/request', {
    method: 'POST',
    body: JSON.stringify({ email }),
  })
}

export function verifyOtp(email: string, code: string): Promise<{ status: string }> {
  return request('/api/auth/otp/verify', {
    method: 'POST',
    body: JSON.stringify({ email, code }),
  })
}

export function logout(): Promise<{ status: string }> {
  return request('/api/auth/logout', { method: 'POST' })
}

export function me(): Promise<{ email: string }> {
  return request('/api/auth/me')
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `frontend/`): `npm test`
Expected: PASS — all tests.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api
git commit -m "Add API client for auth endpoints"
```

---

## Task 5: Session context (`AuthProvider`/`useAuth`)

**Files:**
- Create: `frontend/src/auth/AuthContext.tsx`
- Create: `frontend/src/auth/useAuth.ts`
- Create: `frontend/src/auth/AuthContext.test.tsx`

**Interfaces:**
- Consumes: `me` from `frontend/src/api/client.ts` (Task 4).
- Produces: `interface Session { email: string }`; `type SessionState = 'loading' | Session | null`; `<AuthProvider>`; `useAuth(): { session: SessionState; setSession: (s: Session | null) => void }`. Tasks 6 and 7 consume `useAuth`.

- [ ] **Step 1: Write the failing tests**

`frontend/src/auth/AuthContext.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from './AuthContext'
import { useAuth } from './useAuth'
import * as client from '../api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

function Probe() {
  const { session } = useAuth()
  if (session === 'loading') return <p>loading</p>
  if (session === null) return <p>logged out</p>
  return <p>logged in as {session.email}</p>
}

describe('AuthProvider', () => {
  it('restores the session when GET /api/auth/me succeeds', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com' })

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    )

    expect(screen.getByText('loading')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByText('logged in as parent@example.com')).toBeInTheDocument())
  })

  it('treats a failed /api/auth/me as logged out', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('unauthorized'))

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>,
    )

    await waitFor(() => expect(screen.getByText('logged out')).toBeInTheDocument())
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `npm test`
Expected: FAIL — `Failed to resolve import "./AuthContext"`

- [ ] **Step 3: Implement `AuthContext` and `useAuth`**

`frontend/src/auth/AuthContext.tsx`:

```tsx
import { createContext, useEffect, useState, type ReactNode } from 'react'
import { me as fetchMe } from '../api/client'

export interface Session {
  email: string
}

export type SessionState = 'loading' | Session | null

interface AuthContextValue {
  session: SessionState
  setSession: (session: Session | null) => void
}

export const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionState>('loading')

  useEffect(() => {
    fetchMe()
      .then((result) => setSession({ email: result.email }))
      .catch(() => setSession(null))
  }, [])

  return <AuthContext.Provider value={{ session, setSession }}>{children}</AuthContext.Provider>
}
```

`frontend/src/auth/useAuth.ts`:

```ts
import { useContext } from 'react'
import { AuthContext } from './AuthContext'

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `frontend/`): `npm test`
Expected: PASS — all tests.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/auth/AuthContext.tsx frontend/src/auth/useAuth.ts frontend/src/auth/AuthContext.test.tsx
git commit -m "Add AuthProvider/useAuth session context"
```

---

## Task 6: Login page (email + code stages)

**Files:**
- Create: `frontend/src/auth/LoginPage.tsx`
- Create: `frontend/src/auth/LoginPage.test.tsx`
- Create: `frontend/src/styles/app.css`
- Modify: `frontend/src/main.tsx`

**Interfaces:**
- Consumes: `requestOtp`, `verifyOtp`, `ApiError` (Task 4); `useAuth` (Task 5); `Button`, `Card`, `Field` (Task 3).
- Produces: `<LoginPage>` — Task 7 routes `/login` to it.

- [ ] **Step 1: Add the error-text style**

`frontend/src/styles/app.css`:

```css
.error-text {
  color: var(--color-accent-700);
  font-size: 13px;
  margin: 0 0 var(--space-2);
}
```

Add the import to `frontend/src/main.tsx`, right after the `organic.css` import:

```tsx
import './styles/organic.css'
import './styles/app.css'
```

- [ ] **Step 2: Write the failing tests**

`frontend/src/auth/LoginPage.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { LoginPage } from './LoginPage'
import { AuthProvider } from './AuthContext'
import * as client from '../api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

function renderLoginPage() {
  vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))
  return render(
    <MemoryRouter>
      <AuthProvider>
        <LoginPage />
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('LoginPage', () => {
  it('requests a code, then verifies it', async () => {
    const requestOtp = vi.spyOn(client, 'requestOtp').mockResolvedValue({ status: 'sent' })
    const verifyOtp = vi.spyOn(client, 'verifyOtp').mockResolvedValue({ status: 'ok' })
    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText('Email'), 'parent@example.com')
    await user.click(screen.getByRole('button', { name: 'Получить код' }))

    await waitFor(() => expect(requestOtp).toHaveBeenCalledWith('parent@example.com'))
    expect(await screen.findByLabelText('Код из письма')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Код из письма'), '123456')
    await user.click(screen.getByRole('button', { name: 'Подтвердить' }))

    await waitFor(() => expect(verifyOtp).toHaveBeenCalledWith('parent@example.com', '123456'))
  })

  it('shows the server error message when the code is rejected', async () => {
    vi.spyOn(client, 'requestOtp').mockResolvedValue({ status: 'sent' })
    vi.spyOn(client, 'verifyOtp').mockRejectedValue(
      new client.ApiError('bad_request', 'invalid or expired code', 400),
    )
    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText('Email'), 'parent@example.com')
    await user.click(screen.getByRole('button', { name: 'Получить код' }))
    await screen.findByLabelText('Код из письма')

    await user.type(screen.getByLabelText('Код из письма'), '000000')
    await user.click(screen.getByRole('button', { name: 'Подтвердить' }))

    expect(await screen.findByText('invalid or expired code')).toBeInTheDocument()
  })

  it('shows the cooldown message on a 429 from requesting a code', async () => {
    vi.spyOn(client, 'requestOtp').mockRejectedValue(
      new client.ApiError('too_many_requests', 'code already sent, try again shortly', 429),
    )
    const user = userEvent.setup()
    renderLoginPage()

    await user.type(screen.getByLabelText('Email'), 'parent@example.com')
    await user.click(screen.getByRole('button', { name: 'Получить код' }))

    expect(await screen.findByText('code already sent, try again shortly')).toBeInTheDocument()
  })
})
```

- [ ] **Step 3: Run to verify it fails**

Run (from `frontend/`): `npm test`
Expected: FAIL — `Failed to resolve import "./LoginPage"`

- [ ] **Step 4: Implement `LoginPage`**

`frontend/src/auth/LoginPage.tsx`:

```tsx
import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { Field } from '../components/Field'
import { requestOtp, verifyOtp, ApiError } from '../api/client'
import { useAuth } from './useAuth'

type Stage = 'email' | 'code'

const GENERIC_ERROR = 'Что-то пошло не так, попробуйте ещё раз'

export function LoginPage() {
  const [stage, setStage] = useState<Stage>('email')
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const { setSession } = useAuth()
  const navigate = useNavigate()

  async function handleRequestCode(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await requestOtp(email)
      setStage('code')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : GENERIC_ERROR)
    } finally {
      setSubmitting(false)
    }
  }

  async function handleVerifyCode(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await verifyOtp(email, code)
      setSession({ email })
      navigate('/')
    } catch (err) {
      setCode('')
      setError(err instanceof ApiError ? err.message : GENERIC_ERROR)
    } finally {
      setSubmitting(false)
    }
  }

  if (stage === 'code') {
    return (
      <Card kicker="Вход">
        <h3>Введите код</h3>
        <p>
          Отправили 6-значный код на <strong>{email}</strong>.
        </p>
        <form onSubmit={handleVerifyCode}>
          <Field
            id="code"
            label="Код из письма"
            value={code}
            onChange={(event) => setCode(event.target.value)}
            placeholder="000000"
          />
          {error ? <p className="error-text">{error}</p> : null}
          <Button type="submit" block disabled={submitting}>
            Подтвердить
          </Button>
          <Button type="button" variant="ghost" block onClick={() => setStage('email')}>
            Изменить почту
          </Button>
        </form>
      </Card>
    )
  }

  return (
    <Card kicker="Вход">
      <h3>Введите почту</h3>
      <p>Пришлём одноразовый код подтверждения — пароль не нужен.</p>
      <form onSubmit={handleRequestCode}>
        <Field
          id="email"
          label="Email"
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          placeholder="you@example.com"
        />
        {error ? <p className="error-text">{error}</p> : null}
        <Button type="submit" block disabled={submitting}>
          Получить код
        </Button>
      </form>
    </Card>
  )
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run (from `frontend/`): `npm test`
Expected: PASS — all tests.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/auth/LoginPage.tsx frontend/src/auth/LoginPage.test.tsx frontend/src/styles/app.css frontend/src/main.tsx
git commit -m "Add LoginPage: email and code stages"
```

---

## Task 7: Route guard, placeholder home, and app wiring

**Files:**
- Create: `frontend/src/auth/RequireAuth.tsx`
- Create: `frontend/src/auth/PlaceholderHome.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/App.test.tsx`
- Modify: `frontend/src/main.tsx`

**Interfaces:**
- Consumes: `useAuth` (Task 5), `LoginPage` (Task 6), `logout` (Task 4), `Card`/`Button` (Task 3).
- Produces: the final `App` — nothing later in this plan depends on it; it's the plan's deliverable.

- [ ] **Step 1: Write the failing tests**

Replace `frontend/src/App.test.tsx` (this supersedes Task 2's content-agnostic smoke test now that there's real routing to assert on):

```tsx
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from './App'
import * as client from './api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('App routing', () => {
  it('redirects to /login when not authenticated', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByLabelText('Email')).toBeInTheDocument())
  })

  it('shows the placeholder home when authenticated', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com' })

    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByText('Вы вошли как parent@example.com')).toBeInTheDocument())
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `npm test`
Expected: FAIL — the placeholder-home test fails because `App` still renders the scaffolded Vite counter demo, not routes.

- [ ] **Step 3: Implement `RequireAuth`, `PlaceholderHome`, and the final `App`**

`frontend/src/auth/RequireAuth.tsx`:

```tsx
import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from './useAuth'

export function RequireAuth({ children }: { children: ReactNode }) {
  const { session } = useAuth()
  if (session === 'loading') return null
  if (session === null) return <Navigate to="/login" replace />
  return <>{children}</>
}
```

`frontend/src/auth/PlaceholderHome.tsx`:

```tsx
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { logout } from '../api/client'
import { useAuth } from './useAuth'

export function PlaceholderHome() {
  const { session, setSession } = useAuth()
  const email = session && session !== 'loading' ? session.email : ''

  async function handleLogout() {
    await logout()
    setSession(null)
  }

  return (
    <Card kicker="Нужняшки">
      <h3>Вы вошли как {email}</h3>
      <p>Вишлист и профиль ребёнка появятся здесь в следующих срезах.</p>
      <Button variant="secondary" onClick={handleLogout}>
        Выйти
      </Button>
    </Card>
  )
}
```

Replace `frontend/src/App.tsx`:

```tsx
import { Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { useAuth } from './auth/useAuth'
import { LoginPage } from './auth/LoginPage'
import { PlaceholderHome } from './auth/PlaceholderHome'
import { RequireAuth } from './auth/RequireAuth'

function LoginRoute() {
  const { session } = useAuth()
  if (session === 'loading') return null
  if (session !== null) return <Navigate to="/" replace />
  return <LoginPage />
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginRoute />} />
      <Route
        path="/"
        element={
          <RequireAuth>
            <PlaceholderHome />
          </RequireAuth>
        }
      />
    </Routes>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  )
}
```

Replace `frontend/src/main.tsx` (adds `BrowserRouter`, which `App` itself deliberately does not include, so tests can wrap it in `MemoryRouter` instead):

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import './styles/organic.css'
import './styles/app.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>,
)
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `frontend/`): `npm test`
Expected: PASS — all tests.

Run: `npm run build`
Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "Wire routing: RequireAuth guard, placeholder home, login redirect"
```

- [ ] **Step 6: Manual end-to-end smoke test against the real backend**

From the repo root:
```bash
docker compose up -d
cd backend && go run ./cmd/server &
cd ../frontend && npm run dev
```
Open `http://localhost:5173` (Vite's default port). Expected: redirected to `/login`. Enter an email, click «Получить код», open `http://localhost:1080` (mailcatcher) and read the 6-digit code, enter it and click «Подтвердить». Expected: redirected to `/`, showing «Вы вошли как …» with the email you entered. Reload the page: expected to land back on `/` (not `/login`) — proves session restore via `GET /api/auth/me` works. Click «Выйти»: expected redirect to `/login`. Stop both background processes afterward.

