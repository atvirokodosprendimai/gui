# Anycast DNS Cloud Dashboard Design

## Goals

- Provide a clean dashboard and user area for parking domains to Anycast DNS.
- Manage N VPN-connected DNS servers from one place.
- Keep dashboard state and all servers converged (bi-directional reconciliation).
- Use Datastar on the frontend and `go-chi` on the backend.
- Keep UI reactive with near-real-time updates.

## Stack

- Backend: Go + `github.com/go-chi/chi/v5`
- Frontend: Datastar (CDN), server-rendered fragments, reactive `data-*` attributes
- HTML rendering: `templ` components (`github.com/a-h/templ`)
- State storage: in-memory maps (servers and normalized DNS records)

## Project structure (modular)

- `cmd/gui/main.go`
  - composition root, env config, HTTP server bootstrap
- `internal/dashboard/app.go`
  - app state container, router wiring, fragment rendering helpers
- `internal/dashboard/handlers.go`
  - HTTP handlers only (parse request, call app logic, return fragments)
- `internal/dashboard/client.go`
  - upstream `/v1/*` client logic, auth headers, strict JSON bodies, error decoding
- `internal/dashboard/sync.go`
  - periodic reconciliation loop and sync algorithm implementation
- `internal/dashboard/types.go`
  - domain models and normalization helpers
- `internal/dashboard/views.templ`
  - templ source for full page and fragment components
- `internal/dashboard/views_templ.go`
  - generated Go code from `templ generate`, compiled as normal package code
- `internal/dashboard/templ_render.go`
  - shared response rendering helpers that write templ output directly to `http.ResponseWriter`
- `internal/dashboard/cqrs_stream.go`
  - single long-lived CQRS SSE endpoint for read-model and clock Datastar patches

This split keeps responsibilities focused and follows Go best-practice boundaries: thin handlers, explicit data structs, and isolated transport/client code.

## Build step notes

- templ sources are committed as `*.templ` and generated to `*_templ.go`.
- required generation step before build:
  - `templ generate ./internal/dashboard`
- shortcut:
  - `go generate ./internal/dashboard`

## Data model

- **Server node**
  - `id`, `name`, `url`, `port`, `token`
  - health fields: `online`, `last_sync_at`, `last_error`, `record_count`
- **DNS record**
  - normalized key by `(fqdn, type, value)`
  - stores API fields (`name`, `type`, `ip`, `text`, `target`, `priority`, `ttl`, `zone`)
- **Domain ownership**
  - dashboard-local `domain -> account` mapping
  - used for account visibility in records and parked-domain transfer flow

## API contract handling (required constraints)

1. **Auth for `/v1/*`**
   - Send `Authorization: Bearer <token>`
   - Also sends `X-API-Token` for compatibility.

2. **Error handling for non-2xx**
   - Parse `{ "error": "..." }` and surface that message in node status/flash.
   - Fallback to response text/status phrase when `error` is absent.

3. **Unknown request JSON fields**
   - Outbound JSON uses strict typed structs with only known properties.
   - No dynamic maps for record writes.

4. **Normalized FQDN acceptance**
   - Normalize names/zones/targets to lowercase trailing-dot format for comparisons.
   - Accept upstream already-normalized values unchanged.

5. **Defaults/inference support**
   - Request writer omits optional fields when not explicitly set.
   - Leaves room for server-side defaults/inference (`ttl`, `zone`, `type`, `propagate`).

## Sync strategy

Sync loop runs every 15 seconds and can also be triggered manually.

### Reconciliation algorithm

1. Pull records from each configured server (`GET /v1/records`).
2. Build a union set from:
   - current dashboard records
   - all records fetched from all servers
3. Update dashboard record map to the union.
4. For each server, push missing union records with `POST /v1/records/{name}/add`.

Outcome: if dashboard lacks records, they are imported; if a server lacks records, they are backfilled.

## Frontend behavior (Datastar)

- Datastar signals back form inputs (`data-bind:*`).
- Actions use Datastar `@get(...)` for:
  - adding/removing servers
  - parking domains
  - transferring parked domains between accounts
  - manual sync
- Single CQRS read stream uses one long-lived Datastar SSE connection:
  - `@get('/any/cqrs', {openWhenHidden: true, requestCancellation: 'disabled'})`
  - stream patches `#flash`, `#overview`, `#servers`, `#records`, and `#clock`
- No periodic polling is used.
- Search is client-side reactive (`data-bind:filter`) over streamed record rows.

This provides a reactive operator experience without full SPA complexity.

### Core UI nodes

- `#flash` for server-side action feedback messages
- `#clock` for live UTC server clock updates from SSE
- `#overview` for aggregate node/record counters
- `#servers` for Anycast server inventory and health
- `#records` for parked domains and DNS records table

## CQRS flow

- **Command side (writes)**
  - `/ui/server/add`, `/ui/server/delete/{id}`, `/ui/domain/park`, `/ui/domain/transfer`, `/ui/sync/now`
  - handlers mutate state and return `204 No Content`
- **Query side (reads)**
  - `/any/cqrs` keeps one SSE connection per UI session
  - server pushes read-model patches on state-change notifications and clock ticks

This avoids browser connection pressure on HTTP/1.1 and keeps update traffic on a single stream.

## Account transfer behavior

- Park flow accepts optional owner account (`account` query parameter).
- Transfer endpoint updates ownership mapping only (DNS nodes do not store account ownership).
- Transfer route:
  - `GET /ui/domain/transfer?domain=<fqdn>&to_account=<account>`
- Records table includes an account column sourced from ownership mapping.

## Routing

- Page: `/`
- Health: `/healthz`
- UI fragments:
  - `/fragments/overview`
  - `/fragments/servers`
  - `/fragments/records`
- UI actions:
  - `/ui/server/add`
  - `/ui/server/delete/{id}`
  - `/ui/domain/park`
  - `/ui/domain/transfer`
  - `/ui/sync/now`
- SSE:
  - `/any/cqrs` long-lived Datastar SSE stream for full read-model updates and live clock patches

## Security and operational notes

- Server tokens are stored in process memory only.
- Dashboard currently has no auth layer (intended for internal/VPN use).
- Since persistence is in-memory, restart clears dashboard state.
- Recommended next step in production: persist servers/records in SQLite/Postgres and add role-based auth.

## Testing policy

- Every behavior change should include unit tests.
- At minimum, tests must cover:
  - normalization and keying logic,
  - upstream API contract behavior (auth headers, error decoding, request shape),
  - sync reconciliation behavior.
- Run `go test ./...` before merging changes.
