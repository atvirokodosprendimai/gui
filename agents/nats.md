# NATS Playbook

## Goals

- Keep UI notifications scoped and safe.
- Prevent cross-user/session leakage.
- Keep distributed notifier behavior observable.

## Subject Contract

- `fe.update.global`
- `fe.update.user.<user_id>`
- `fe.update.session.<session_id>`

You MUST NOT publish private updates to `fe.update.global`.

## Payload Contract

Minimum:

- `el`
- `scope`

Optional:

- `user_id`
- `session_id`
- `correlation_id`
- `emitted_at`

## Security Rules

- You MUST validate subject-derived scope and payload scope match.
- You MUST validate identity fields for user/session scopes.
- You MUST drop unknown scopes.
- You MUST drop empty/invalid target element identifiers.

## Reliability + Ops

- Core NATS is acceptable for transient UI notifications.
- If replay is required, you MUST move to JetStream explicitly.
- You MUST log publish failures and scope mismatch drops.
- You MUST include correlation IDs in command->notify->patch logs.

Minimum observability to keep (MUST):

- `ui_update_publish_total{scope}`
- `ui_update_drop_scope_mismatch_total`
- `ui_update_drop_invalid_payload_total`
- `ui_update_patch_total{el}`
- `ui_update_notify_to_patch_ms`

## Minimal Pattern

```go
subject := subjectForUpdate(upd) // global/user/session scoped subject
b, _ := json.Marshal(upd)
if err := nc.Publish(subject, b); err != nil {
    // log + metric increment
}
```

## Review Checklist

- [ ] Subject and payload validation enforced
- [ ] No private data on global subject
- [ ] Observability logs/metrics updated
- [ ] Tests cover scope routing and mismatch drops
