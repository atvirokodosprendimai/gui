# CQRS Playbook

## Goals

- Keep a single write path for mutations.
- Keep read models optimized for UI queries.
- Notify and patch only what changed.

## Core Flow

1. Command received and validated.
2. Write mutation committed.
3. Read model updated (sync/async depending on path).
4. UI update intent emitted.
5. SSE stream applies targeted fragment patch.

## Rules

- You MUST separate command handlers from read-model rendering concerns.
- You MUST keep read filters role-aware and ownership-aware.
- You SHOULD patch by element key (`el`) instead of global fanout when possible.
- You SHOULD make updates idempotent where possible.

## Operability Requirements

- You MUST emit correlation IDs from command -> notification -> patch logs.
- You MUST track per-element patch counts and patch latency.
- You MUST track and alert on dropped scoped updates.

## Scope + Security

- Global events: patch all streams.
- User events: patch only matching authenticated user.
- Session events: patch only matching session.
- Unknown/mismatched scope: drop.

## Anti-Patterns (Avoid)

- Full-page patch fanout for small read-model changes.
- Publishing user-private updates to global channels.
- Rendering privileged data based only on frontend visibility.
- Querying write model directly inside stream patch loop.

## Testing Focus

- Command validation errors and happy paths.
- Scoped stream filtering behavior.
- Read-model visibility for admin vs user.
- Graceful shutdown with in-flight streams and sync activity.

## Review Checklist

- [ ] Write path validates input and auth
- [ ] Read path enforces role/ownership filters
- [ ] Notification scope is correct
- [ ] Patches are targeted and stable by element ID
- [ ] Tests updated for changed behavior
