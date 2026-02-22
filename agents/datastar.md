# Datastar Playbook

## Goals

- Keep command transport safe and typed.
- Keep SSE patching targeted and efficient.
- Preserve server-side auth and role enforcement.

## Backend Rules (Go)

- You MUST parse command signals with `datastar.ReadSignals` into typed structs.
- For mutations, you MUST use `@post(...)` in UI actions.
- You MUST NOT send secrets in query strings.
- You MUST build SSE streams with `datastar.NewSSE`.
- You MUST patch fragments with `PatchElementTempl`.

## Patch Strategy

- You MUST patch by stable element IDs (`el`) such as `flash`, `overview`, `records`.
- You SHOULD avoid full-page re-renders for minor updates.
- You MUST re-query read model using current auth context before rendering fragment.

## Security Rules

- You MUST enforce auth/role checks server-side in handlers and fragment data selection.
- You MUST NOT rely on frontend visibility rules for privilege boundaries.

## CSP Note

- Datastar expression evaluation may require CSP allowances.
- You MUST keep CSP as tight as possible otherwise and SHOULD prefer pinned/self-hosted assets where practical.

If CSP is weakened (for example adding `unsafe-eval`), include:

- rationale in commit/PR notes
- narrowest possible source scope
- follow-up hardening task and owner

## Minimal Patterns

Typed signal parsing:

```go
var sig createUserSignals
if err := datastar.ReadSignals(r, &sig); err != nil {
    http.Error(w, "invalid payload", http.StatusBadRequest)
    return
}
```

SSE + targeted templ patch:

```go
sse := datastar.NewSSE(w, r)
if err := sse.PatchElementTempl(OverviewFragment(nodeCount, onlineCount, recordCount)); err != nil {
    return
}
```

## Review Checklist

- [ ] Datastar SDK helpers used (no custom wire protocol)
- [ ] Mutations use POST actions
- [ ] Scoped patches enforced before patching stream
- [ ] Fragment IDs are stable and targeted
