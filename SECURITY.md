# SECURITY

This document defines threat scenarios, security controls, and hard rules for this project.

## Security model summary

- Auth: cookie-based server-side sessions (`session_token`)
- Roles: `admin`, `user`
- Transport: HTTP + Datastar SSE (`/any/cqrs`)
- Write API: Datastar `@post` command endpoints (`/ui/*`)
- Persistence: SQLite + Goose migrations

## Critical rules (must follow)

### 1) Do not use GET for state-changing commands

- `POST` only for `/ui/*` mutations.
- Never place secrets in URLs/query strings.

### 2) Always parse Datastar signals with Datastar Go SDK

- Use `datastar.ReadSignals(r, &typedStruct)`.
- Avoid custom signal wire parsing assumptions.

### 3) Scope UI notifications by user/session when needed

- Global UI updates are acceptable only for global read-model fragments.
- Per-user feedback (flash/errors) must be targeted by session token.

### 4) Use role checks server-side for every command

- UI visibility is not authorization.
- Enforce admin/user permissions in handlers.

### 5) Use migrations only for schema changes

- Goose SQL migrations only.
- Do not use GORM auto-migrations.

## Threat scenarios and controls

### A. Cross-user update leakage in CQRS stream

Risk:

- Single `fe.update` stream fanout can leak user-specific updates if not filtered.

Controls:

- Use structured update payloads (`subject`, `el`, optional `session_token`).
- Stream consumer must ignore events with non-matching `session_token`.
- Render user-scoped fragments with current authenticated user context.

Current status:

- Implemented session-targeted UI updates for flash.
- Global fragments are rendered per-request with role-aware filtering.

### B. Secrets leakage in frontend commands

Risk:

- Tokens/passwords in query strings leak via browser history, logs, proxies, referrers.

Controls:

- Datastar `@post` + SDK signal extraction.
- No command secrets in URL.

### C. Session theft / weak cookie settings

Risk:

- Cookie replay over insecure links.

Controls:

- `HttpOnly`, `SameSite=Lax`, `Secure` when HTTPS/forwarded HTTPS.
- Prefer TLS termination with trusted `X-Forwarded-Proto`.

### D. Privilege bypass via client manipulation

Risk:

- Hidden admin UI may be called directly.

Controls:

- Server-side role checks in each handler.

### E. User enumeration / account transfer abuse

Risk:

- Invalid transfer target probing.

Controls:

- Validate target account exists (when DB auth enabled).
- Non-admin users can transfer only domains they own.

### F. Persistence drift (in-memory vs DB)

Risk:

- Ignore DB write errors and keep stale memory state.

Controls:

- Treat persistence errors as command failures.
- Return error status + targeted flash.

### G. SSE abuse / resource exhaustion

Risk:

- Many long-lived connections, unbounded memory/channels.

Controls:

- Bounded watcher channel buffers.
- Remove watcher on disconnect.
- Consider connection limits/rate-limits at reverse proxy.

### H. NATS integration future risk

Risk:

- If moving CQRS notifications to NATS without tenant/user scoping, cross-tenant leaks can occur.

Controls:

- Use scoped subjects and payload fields:
  - `fe.update.global`
  - `fe.update.user.<user_id>`
  - `fe.update.session.<session_id>`
- Validate subject routing and payload authorization before patching.

Current status:

- NATS-backed updater is implemented and optional (`GUI_NATS_URL`).
- Scoped subjects are supported:
  - `fe.update.global`
  - `fe.update.user.<user_id>`
  - `fe.update.session.<session_id>`
- SSE handler enforces session/user scope before patching.

## What not to do

- Do not parse Datastar signals using ad-hoc JSON maps for production paths.
- Do not broadcast user-private UI events globally.
- Do not keep default admin credentials in code.
- Do not return detailed internals in flash/errors.
- Do not skip migrations for schema updates.

## What to do for max security

- Require explicit admin bootstrap env vars on first run.
- Add session expiry cleanup task (delete expired sessions).
- Add request rate limiting for auth endpoints.
- Add account lockout/backoff for repeated login failures.
- Add audit log table for admin commands and domain transfers.
- Add CSP and strict transport headers at reverse proxy.
- Add optional 2FA for admin accounts.

## Datastar skill improvements to add

Suggested additions for the Datastar skill document:

1. **Signal parsing hard rule**
   - "On Go backends, use `datastar.ReadSignals` with typed structs for command endpoints."

2. **Security routing rule for CQRS/SSE**
   - "When using shared notification channels, include target scope (`global`, `user_id`, `session_id`) and enforce it before patching."

3. **Command transport rule**
   - "Use `@post` for mutations; avoid sending secrets in URLs."

4. **Patch minimization rule**
   - "Prefer targeted fragment patches by element id (`el`) over full-page fragment fanout."

5. **Role-aware rendering reminder**
   - "All server-rendered read models used for patches must be filtered by authenticated user role/context."
