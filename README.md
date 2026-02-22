# Anycast DNS Cloud Dashboard

Control plane and user portal for parking domains on Anycast DNS nodes.

This project is built with:

- Go + chi (`go-chi`) for HTTP routing
- templ for server-side HTML rendering
- Datastar for reactive UI updates
- Flowbite/Tailwind for UI styling
- CQRS-style single SSE read stream for realtime updates
- SQLite (pure Go driver, no cgo) + GORM for persistence
- Goose SQL migrations (no GORM auto-migrations)

## What It Does

- Manage multiple VPN-connected DNS servers (url, port, token, name)
- Park domains and sync DNS records across nodes
- Transfer parked domains between user accounts
- Multi-user auth with roles:
  - `admin`: server management, sync, user creation, full domain transfer
  - `user`: park domains, transfer own domains
- Realtime UI updates over a single SSE connection (`/any/cqrs`)

## Architecture Summary

### CQRS flow

- **Command side** (`/ui/*`): mutates state and returns `204 No Content`
- **Query side** (`/any/cqrs`): long-lived Datastar SSE stream that patches read-model HTML fragments

Patched UI nodes:

- `#flash`
- `#overview`
- `#servers`
- `#records`
- `#clock`

This avoids periodic polling and minimizes browser connection usage (important for HTTP/1.1 limits).

### Persistence

- Database: SQLite file
- Driver: `github.com/glebarez/sqlite` (pure-Go, modernc-based, no cgo)
- ORM: GORM (query/mapping only)
- Schema: Goose SQL migrations in `migrations/`

## Project Layout

- `cmd/gui/main.go` - app bootstrap, DB open, migrations, admin bootstrap
- `internal/dashboard/app.go` - app state, routes, read-model helpers
- `internal/dashboard/handlers.go` - command handlers
- `internal/dashboard/cqrs_stream.go` - single SSE read stream
- `internal/dashboard/auth.go` - user/session auth + role checks
- `internal/dashboard/auth_handlers.go` - login/logout handlers
- `internal/dashboard/client.go` - upstream `/v1/*` DNS API client logic
- `internal/dashboard/sync.go` - periodic reconciliation loop
- `internal/dashboard/views.templ` - templ page/fragment components
- `migrations/` - Goose SQL migrations

## Requirements

- Go 1.25+

No cgo requirement for SQLite in this setup.

## Environment Variables

- `GUI_ADDR` (default `:8090`) - HTTP listen address
- `GUI_DB` (default `gui.db`) - SQLite database path
- `GUI_ADMIN_EMAIL` (default `admin@local`) - bootstrap admin email
- `GUI_ADMIN_PASSWORD` (default `admin123`) - bootstrap admin password

## Local Run

1. Generate templ code:

```bash
go generate ./internal/dashboard
```

2. Start app:

```bash
go run ./cmd/gui
```

3. Open:

- `http://localhost:8090/login`

On first run, an admin is created if no admin exists.

## Migrations (Goose)

Migrations are in `migrations/` and are run automatically on startup via `goose.Up`.

Current migration set:

- `00001_create_users.sql`
- `00002_create_sessions.sql`
- `00003_create_domain_owners.sql`

Important policy:

- Always use Goose migrations for schema changes
- Do not use GORM auto-migrate

## Auth and Roles

### Login

- GET `/login`
- POST `/auth/login`
- GET `/auth/logout`

Session is stored server-side in `sessions` table and tracked via `session_token` cookie.

### Role permissions

- Admin-only:
  - add/remove servers
  - manual sync
  - create users
  - transfer any domain
- User:
  - park domains
  - transfer only domains they own

## DNS API Integration Notes

Upstream DNS control API behavior implemented in client code:

- sends `Authorization: Bearer <token>` and `X-API-Token`
- handles non-2xx `{ "error": "..." }`
- avoids unknown JSON fields with strict outbound structs
- accepts normalized FQDN values
- respects server-side defaults/inference (`ttl`, `zone`, `type`, `propagate`)

## Development Workflow

Recommended loop:

```bash
go generate ./internal/dashboard
gofmt -w cmd/gui/main.go internal/dashboard/*.go
go test ./...
go build ./...
```

## Testing

Test coverage includes:

- normalization and keying helpers
- auth/header/error behavior in client/auth logic
- sync reconciliation
- CQRS stream patch behavior
- ownership transfer behavior

Run:

```bash
go test ./...
```

## Notes

- Current CQRS notifier fan-out is in-process.
- If you need multi-instance distributed read-model notifications, wire CQRS notifier subjects via NATS next.
