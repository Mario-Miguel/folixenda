# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

**folixenda** is an events discovery platform — a monorepo with:
- `frontend/` — Next.js 16 app (TypeScript, Tailwind v4, pnpm)
- `backend/` — Go REST API (stdlib `net/http`, in-memory store)

The design system uses orange `#ec5b13` as primary color, `#f8f6f6` light background, `#221610` dark background, and Plus Jakarta Sans font.

## Frontend (`frontend/`)

**Package manager:** pnpm (not npm/yarn)

```bash
pnpm dev          # Dev server on http://localhost:3000
pnpm build        # Production build (also runs type-check)
pnpm lint         # ESLint
```

**Pages:**
- `/` — Main dashboard: mini calendar + event list for selected day
- `/calendar` — Full monthly calendar grid with map toggle
- `/events/[id]` — Event detail with ticket sidebar and related events

**Key conventions:**
- Tailwind v4: theme tokens live in `app/globals.css` inside `@theme {}` (no `tailwind.config.ts`)
- Primary color is always applied via `style={{ color/backgroundColor: "#ec5b13" }}` inline or via the `text-primary`/`bg-primary` classes defined in `@theme`
- API base URL comes from `NEXT_PUBLIC_API_URL` env var (defaults to `http://localhost:8080`)
- Mock data in `lib/mock-data.ts` is used while the backend is not wired up

## Backend (`backend/`)

```bash
air                          # Dev server with hot reload on :8080 (requires air)
go run ./cmd/server          # Start server without hot reload
go build ./...               # Build
go test ./...                # Run tests
go vet ./...                 # Static analysis
```

**API endpoints:**
| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/events` | List events; optional `?date=YYYY-MM-DD&category=Music` |
| `GET` | `/api/events/{id}` | Get single event |
| `POST` | `/api/events` | Create event (JSON body) |
| `GET` | `/healthz` | Health check |

**Package structure:**
- `main.go` — entry point, HTTP server setup, CORS + logging middleware
- `models/` — `Event` struct and `EventCategory` type
- `store/` — thread-safe in-memory store with seed data
- `handlers/` — HTTP handlers, registered via `handler.Register(mux)`

The store uses `sync.RWMutex` and preserves insertion order via a separate `order []string` slice. Route pattern matching uses Go 1.22 `http.ServeMux` method+path syntax (`GET /api/events/{id}`).
