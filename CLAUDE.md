# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**folixenda** is an events discovery platform (events in Asturias, Spain) — a monorepo with:
- `frontend/` — Next.js 16 app (TypeScript, Tailwind v4, pnpm, i18next es/en, Leaflet, Better Auth)
- `backend/` — Go REST API (stdlib `net/http`, Postgres)
- `scraper/` — Python (poetry) scrapers that load events into Postgres (feteas, asturiasdefiesta)
- `db_queries/` — local scratch SQL queries and CSVs (gitignored); not the schema

The design system uses orange `#ec5b13` as primary color, `#f8f6f6` light background, `#221610` dark background, and Plus Jakarta Sans font.

## Auth architecture

Better Auth runs inside Next.js (tables in Postgres schema `auth`) and issues short-lived JWTs (15 min, EdDSA). The Go API never sees passwords: it verifies the JWT against `/api/auth/jwks` and reads the user from `sub`. Roles (`admin`, `consumer`, `publisher`) live in Better Auth's `auth."user".role` and travel in the JWT. Details: `frontend/docs/autenticacion.md`.

## Frontend (`frontend/`)

**Package manager:** pnpm (not npm/yarn). Next 16 has breaking changes — see `frontend/AGENTS.md`.

```bash
pnpm dev          # Dev server on http://localhost:3000
pnpm build        # Production build (also runs type-check)
pnpm lint         # ESLint
```

**Pages:**
- `/` — Mini calendar + event list for the selected day (+ map)
- `/calendar` — Monthly calendar grid with map toggle
- `/events/[id]` — Event detail with save/share and related events
- `/my-events` — User's saved events (requires session)
- `/login`, `/signup` — Better Auth email/password
- `/admin` — Admin panel (requires role `admin`)

**Key conventions:**
- Tailwind v4: theme tokens live in `app/globals.css` inside `@theme {}` (no `tailwind.config.ts`)
- Primary color via `text-primary`/`bg-primary` or inline `#ec5b13`
- Category colors: only from `lib/categories.ts`
- Dates: use `lib/dates.ts` (local-date `YYYY-MM-DD`, Europe/Madrid for "has ended"); never `toISOString()` for calendar days
- API client: `lib/api/api.ts` (axios, base `${NEXT_PUBLIC_API_URL}/api`, defaults to `http://localhost:8080`); adds the JWT in the browser. Server components pass `getServerApiToken()` from `lib/better-auth/server-token.ts`
- All UI strings in `i18n/locales/{es,en}.json`
- Feature docs (Spanish): `frontend/docs/eventos.md`, `frontend/docs/autenticacion.md`

## Backend (`backend/`)

```bash
air                          # Dev server with hot reload on :8080 (requires air)
go run .                     # Start server without hot reload
go build ./...               # Build
go test ./... -race -cover   # Unit tests (no DB needed: fakes + sqlmock)
go vet ./...                 # Static analysis
go run ./cmd/migrate status  # Migrations: up (default) | status | down
```

Env: DB settings in `.env`, plus `AUTH_JWKS_URL`, `AUTH_ISSUER`, `AUTH_AUDIENCE`.

**API endpoints** ("auth" = valid JWT):
| Method | Path | Access |
|--------|------|--------|
| `GET` | `/api/events` | public; `?date=YYYY-MM-DD`, `?from=&to=` (inclusive), `?category=Music` |
| `GET` | `/api/events/{id}` | public |
| `POST` / `PUT` / `DELETE` | `/api/events[/{id}]` | admin, publisher |
| `GET` | `/api/users` | admin (no DELETE route: users are deleted in Better Auth from `/admin`, the profile cascades) |
| `GET` / `PUT` | `/api/users/{id}` | admin or self (non-admins can't change `role`); PUT is partial and also syncs `role`/`name` to `auth."user"` |
| `GET` / `POST` | `/api/savedEvents` | auth (user from token) |
| `GET` / `DELETE` | `/api/savedEvents/{eventId}` | auth; GET returns `{"saved": bool}` |
| `GET` | `/healthz` | health check |

**Package structure:**
- `main.go` — entry point, stores, verifier, `protect = authenticate(provision(h))`, CORS + logging
- `auth/` — JWT verifier (JWKS), claims, context helpers, role constants
- `middleware/` — `Authenticate`, `Provision` (upserts the user profile from token claims), `RequireRole`, `RequireSelfOrRole`, CORS, logging
- `handlers/` — HTTP handlers, registered via `handler.Register(mux, protect)`; each adds its role middleware
- `store/` — Postgres stores (`EventFilter`, error translation to `ErrNotFound`/`ErrConflict`)
- `models/` — `Event`, `User`, `SavedEvent`
- `migrations/` — goose SQL migrations (embedded); applied on server startup. Add a new `0000N_name.sql`, never edit applied ones

Route pattern matching uses Go 1.22 `http.ServeMux` method+path syntax (`GET /api/events/{id}`).

## Database

`public.users.id` is a uuid FK to `auth."user"(id) ON DELETE CASCADE`; saved events cascade too. Setup order: `db_queries/better_auth.sql` (auth schema + role) → Better Auth migrations (`auth."user"`) → Go migrations in `backend/migrations/` (run automatically on startup). Users are deleted via Better Auth (`frontend/app/admin/actions.ts`), and the cascade cleans the Go side. The Go DB role needs `USAGE ON SCHEMA auth` and `SELECT, REFERENCES, UPDATE (role, name, "updatedAt") ON auth."user"`.
