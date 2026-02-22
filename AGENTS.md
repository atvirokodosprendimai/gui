# AGENTS.md

Repository-local operating guide for coding agents working on this project.

## Purpose

This project is a Go-based Anycast DNS dashboard with Datastar + templ frontend, CQRS read updates over SSE, optional NATS-based distributed UI notifications, and SQLite persistence via GORM + Goose.

Agents must prioritize correctness, security, and operability over novelty.

Detailed domain playbooks are available under `agents/`:

- `agents/golang.md`
- `agents/nats.md`
- `agents/datastar.md`
- `agents/flowbite.md`
- `agents/cqrs.md`

## Instruction Precedence

When instructions conflict, apply this order (highest first):

1. Explicit user request for current task
2. Root `AGENTS.md`
3. Relevant `agents/*.md` playbooks
4. General defaults/tooling conventions

If a lower-priority guide conflicts with a higher-priority one, follow the higher-priority rule and note the tradeoff in your response.

## Stack

- Language: Go
- Router: `github.com/go-chi/chi/v5`
- UI: `templ` + Datastar + Flowbite/Tailwind
- Realtime: Datastar SSE (`datastar-go`) with targeted element patches
- Messaging: NATS (Core; JetStream optional by requirement)
- DB: SQLite (modernc path via `github.com/glebarez/sqlite`) + GORM
- Migrations: Goose SQL migrations only

## Non-Negotiable Rules

1. Use Datastar Go SDK helpers:
   - Parse signals with `datastar.ReadSignals`.
   - Build SSE with `datastar.NewSSE`.
   - Patch templ fragments with `PatchElementTempl`.
2. Use `@post(...)` for state-changing actions. Never send secrets in URL query strings.
3. Keep CQRS patches targeted by stable element IDs (`el`) whenever possible.
4. Keep authorization checks server-side for every privileged mutation and privileged fragment render.
5. Use Goose migrations only. Do not add GORM auto-migrate behavior.
6. Preserve role model semantics (`admin`, `user`) and domain ownership constraints.

## CQRS + NATS Contract

### Subjects

- `fe.update.global`
- `fe.update.user.<user_id>`
- `fe.update.session.<session_id>`

Never publish user-private updates to global subject.

### Scope Enforcement (must)

Before patching any stream update:

- `global`: allow
- `user`: stream user must match payload user id
- `session`: stream session must match payload session id
- unknown scope: drop

### Payload minimum

- `el` (target element id)
- `scope`

Optional: `user_id`, `session_id`, `correlation_id`, `emitted_at`.

## Security Baseline

- Keep secure auth cookies (`HttpOnly`, `SameSite=Lax`, `Secure` on HTTPS).
- Keep HTTP server timeouts and body-size limits for mutation endpoints.
- Keep CSP and other security headers in middleware.
- Any new endpoint must include auth/role checks where required.
- Validate/normalize DNS inputs before write operations.

### Security Exception Policy

If a change requires weakening a control (for example CSP `unsafe-eval` for Datastar expression runtime), the change must include:

1. Explicit rationale in commit/PR notes
2. Narrowest possible scope of exception
3. Follow-up hardening task (pin/self-host assets, SRI, stricter policy where feasible)

## Context and Shutdown

- Propagate `context.Context` through long-running and outbound operations.
- Use request context for outbound HTTP calls (`NewRequestWithContext`).
- Ensure loops and stream handlers exit on app lifecycle cancellation.
- Ctrl+C (SIGINT/SIGTERM) shutdown path must be graceful and bounded.

### Shutdown Acceptance Criteria

- App exits within bounded timeout under active SSE streams and in-flight sync/network calls.
- Background loops stop on lifecycle cancellation.
- Shutdown does not wait indefinitely on external services.

## Testing Policy

For behavior changes, add/adjust tests first when practical.

Minimum validation before finishing:

```bash
go test ./...
go test -race ./...
```

These are mandatory merge gates for behavior-changing work unless the user explicitly asks to skip.

Prioritize tests for:

- auth/session behavior
- scoped CQRS/NATS delivery behavior
- graceful shutdown and cancellation
- input validation and security-sensitive handlers

## Coding Style

- Follow Effective Go and `gofmt` strictly.
- Keep functions focused; avoid hidden side effects.
- Prefer small typed structs over `map[string]any` for request payloads.
- Do not introduce global mutable state unless explicitly required.

## Documentation Expectations

When changing behavior/architecture/security posture, update relevant docs:

- `README.md`
- `DESIGN.md`
- `SECURITY.md`

## Migration Policy

- Schema or persistence-affecting changes must include Goose migrations.
- Security-sensitive persistence changes (sessions/auth/secrets) must include migration notes and rollback considerations.

## Commit Guidance

- Keep commits focused and coherent.
- Commit message should explain why (not only what).
- Do not include secrets.

## Definition of Done

A task is done when all are true:

1. Implementation follows rules above.
2. Tests pass (`go test ./...`, `go test -race ./...`).
3. Security/cqrs scope checks are preserved.
4. Relevant docs are updated when needed.
