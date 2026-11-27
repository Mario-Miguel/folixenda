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

**Account types:** at signup users pick Persona (`consumer`: browses and saves events) or Empresa/plataforma (`publisher`: publishes and manages its own events). The role is set server-side by `databaseHooks.user.create.before` in `lib/better-auth/server.ts` from the signup body's `accountType` (only `consumer`/`publisher`; `role` itself has `input: false`, so nobody can sign up as admin). The account type is permanent (switching needs a new account); only admin can be granted/removed. Admins are assigned by hand. Publishing will become paid: the single gate is `auth.CanPublish` in Go. Details: `frontend/docs/perfiles.md`.

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
- `/events/[id]` — Event detail: sessions list, gallery lightbox (`EventGallery.tsx`), save/share, related events
- `/my-events` — Consumer's saved events (requires session)
- `/publisher` — Publisher dashboard: own events (upcoming/past, saves count), links to the editor, delete
- `/publisher/events/new`, `/publisher/events/[id]/edit` — Full-page event editor (publishers: own events; admins: any)
- `/profile` — Account, organization (publishers), notification preferences, password, delete account
- `/login`, `/signup` — Better Auth email/password
- `/admin` — Admin panel (requires role `admin`); creates/edits events via the editor pages

**Key conventions:**
- Tailwind v4: theme tokens live in `app/globals.css` inside `@theme {}` (no `tailwind.config.ts`)
- Primary color via `text-primary`/`bg-primary` or inline `#ec5b13`
- Category colors: only from `lib/categories.ts`
- Event editor: `components/event-editor/*` (sections Info, Schedule single/multi-day, Place with `AddressSearch` + Leaflet `LocationPicker`, Images; validation in `model.ts` mirrors Go). `createEvent`/`updateEvent` take `EventInput` (sends `sessions`, not `date`/`startTime`/...) and PUT is a full replace; images go through `uploadEventImage`/`deleteEventImage`/`reorderEventImages`
- Multi-day events: `Event.sessions` (≥1, sorted) is authoritative; `date`/`startTime`/`endTime` = first session, `endDate` = last. Use the session helpers in `lib/dates.ts` (`sessionOn`, `nextSession`, `hasEventEnded`, `isMultiDay`, `formatEventDateRange`, `formatScheduleSummary`) and `components/EventSchedule.tsx` (`useEventWhen(event, day)`); day views pass `day`
- Geocoding: `lib/api/geocode.ts` (via the Go proxy, never call CartoCiudad/Nominatim from the browser)
- Current user's data: `lib/api/me.ts` (types in `lib/types.ts`)
- Dates: use `lib/dates.ts` (local-date `YYYY-MM-DD`, Europe/Madrid for "has ended", by the last session); never `toISOString()` for calendar days
- API client: `lib/api/api.ts` (axios, base `${NEXT_PUBLIC_API_URL}/api`, defaults to `http://localhost:8080`); adds the JWT in the browser. Server components pass `getServerApiToken()` from `lib/better-auth/server-token.ts`
- All UI strings in `i18n/locales/{es,en}.json`
- Feature docs (Spanish): `frontend/docs/eventos.md`, `frontend/docs/perfiles.md`, `frontend/docs/autenticacion.md`

## Backend (`backend/`)

```bash
air                          # Dev server with hot reload on :8080 (requires air)
go run .                     # Start server without hot reload
go build ./...               # Build
go test ./... -race -cover   # Unit tests (no DB needed: fakes + sqlmock)
go vet ./...                 # Static analysis
go run ./cmd/migrate status  # Migrations: up (default) | status | down
```

Env: see `backend/.env.example` (copy to `.env`): `POSTGRES_*`, `AUTH_JWKS_URL`, `AUTH_ISSUER`, `AUTH_AUDIENCE`, and image storage. `STORAGE_DRIVER=local` (default, dev) writes to `backend/uploads/` (gitignored) and serves it at `GET /uploads/...`; `STORAGE_DRIVER=s3` uses any S3-compatible service (`S3_*` vars, minio-go). Local disk is the dev default because the official `minio/minio` image is no longer on Docker Hub; there is no docker-compose.

**API endpoints** ("auth" = valid JWT):
| Method | Path | Access |
|--------|------|--------|
| `GET` | `/api/events` | public; `?date=D` (any session starting on D), `?from=&to=` (any session start in range, inclusive), `?category=Music` |
| `GET` | `/api/events/{id}` | public; includes `publisherId` and `organizer` when owned by a publisher |
| `POST` | `/api/events` | `CanPublish` (admin, publisher); a publisher always becomes the owner (`publisherId`). Body needs `sessions` (1–60, no duplicate date+startTime), title ≤200, description ≤5000, perks ≤20×80, venue, price ≥0 |
| `PUT` / `DELETE` | `/api/events/{id}` | admin any event; publisher only its own (403 otherwise). PUT = same rules as POST; DELETE also removes the image files |
| `POST` | `/api/events/{id}/images` | owner or admin; multipart `file` + `kind` (`cover`\|`gallery`); jpg/png/webp by magic bytes, ≤5 MB (413), gallery ≤10 (409); a new cover replaces the old one |
| `DELETE` | `/api/events/{id}/images/{imageId}` | owner or admin |
| `PUT` | `/api/events/{id}/images/order` | owner or admin; `{"imageIds": [...]}` = every gallery image once |
| `GET` | `/api/geocode/search?q=&limit=`, `/api/geocode/reverse?lat=&lon=` | `CanPublish`; proxy CartoCiudad → Nominatim fallback, 502 if both fail |
| `GET` | `/api/users` | admin (no DELETE route: users are deleted in Better Auth from `/admin`, the profile cascades) |
| `GET` / `PUT` | `/api/users/{id}` | admin or self (non-admins can't change `role`; nobody can switch consumer↔publisher: 409, see `auth.AccountTypeChangeAllowed`); PUT is partial and also syncs `role`/`name` to `auth."user"` |
| `GET` / `POST` | `/api/savedEvents` | auth (user from token) |
| `GET` / `DELETE` | `/api/savedEvents/{eventId}` | auth; GET returns `{"saved": bool}` |
| `GET` | `/api/me` | auth; `{user, publisherProfile, notifications}` |
| `PUT` | `/api/me/notifications` | auth; stored preferences only (nothing is sent yet) |
| `GET` / `PUT` | `/api/me/publisher-profile` | publisher |
| `GET` | `/api/me/events` | publisher; own events with `savesCount` |
| `GET` | `/uploads/...` | local storage only: uploaded images (no listing, immutable cache) |
| `GET` | `/healthz` | health check |

**Package structure:**
- `main.go` — entry point, stores, verifier, `protect = authenticate(provision(h))`, CORS + logging
- `auth/` — JWT verifier (JWKS), claims, context helpers, role constants
- `middleware/` — `Authenticate`, `Provision` (upserts the user profile from token claims), `RequireRole`, `RequireSelfOrRole`, CORS, logging
- `handlers/` — HTTP handlers (`me.go` for `/api/me/*`), registered via `handler.Register(mux, protect)`; each adds its role middleware
- `handlers/event_images.go` (image upload/delete/reorder), `handlers/geocode.go` (geocoding proxy)
- `store/` — Postgres stores (`EventFilter`, sessions written in a transaction, `event_images_store.go`, error translation to `ErrNotFound`/`ErrConflict`)
- `storage/` — `Storage` interface (`Put`/`Delete`/`URL`): `Local` (disk + `/uploads` handler) and `MinIO` (S3-compatible)
- `geocode/` — CartoCiudad first, Nominatim fallback (≤1 req/1.1 s, generic User-Agent), 10-min in-memory cache
- `models/` — `Event` (+ `EventSession`, `EventImage`, `Organizer`, `PublisherEvent`), `User`, `SavedEvent`, `PublisherProfile`, `NotificationPreferences`
- `migrations/` — goose SQL migrations (embedded); applied on server startup. Add a new `0000N_name.sql`, never edit applied ones

Route pattern matching uses Go 1.22 `http.ServeMux` method+path syntax (`GET /api/events/{id}`).

## Database

`public.users.id` is a uuid FK to `auth."user"(id) ON DELETE CASCADE`; saved events cascade too. Setup order: `db_queries/better_auth.sql` (auth schema + role) → Better Auth migrations (`auth."user"`) → Go migrations in `backend/migrations/` (run automatically on startup). Users are deleted via Better Auth (`frontend/app/admin/actions.ts`), and the cascade cleans the Go side. Migration `00004` adds `event_sessions` (one row per day; an `AFTER INSERT` trigger `events_default_session` gives the scraper's plain `events` inserts one session; the API replaces sessions on every write; `events.start_time/end_time` keep the overall span) and `event_images` (≤1 cover per event). Image rows cascade on event/account deletion but files don't: deleting via the API removes them, account deletion leaves orphans (TODO in `handlers/event_images.go`). The Go DB role needs `USAGE ON SCHEMA auth` and `SELECT, REFERENCES, UPDATE (role, name, "updatedAt") ON auth."user"`.
