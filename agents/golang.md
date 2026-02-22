# Go Playbook

## Goals

- Keep code idiomatic, small, and testable.
- Prefer explicitness over cleverness.
- Keep cancellation and shutdown behavior correct.

## Implementation Rules

- You MUST run `gofmt` on all changed Go files.
- You MUST keep functions focused and side effects obvious.
- You MUST check all errors and SHOULD return wrapped context where useful.
- You SHOULD accept interfaces where needed and return concrete types.
- You MUST avoid package-level mutable globals unless explicitly required.
- You MUST propagate `context.Context` through long-running and outbound operations.

## HTTP + Concurrency

- You MUST use `http.NewRequestWithContext` for outbound calls.
- You MUST set server/client timeouts intentionally.
- You MUST guard shared maps with mutexes.
- You SHOULD prefer bounded channels and MUST define drop behavior explicitly.

## Graceful Shutdown Criteria

- SIGINT/SIGTERM MUST stop loops and stream handlers quickly.
- In-flight outbound calls MUST be cancelable via context.
- Shutdown MUST complete within bounded timeout under load.

## Data + Validation

- You MUST use typed structs for request payloads.
- You MUST validate input before mutation.
- You MUST normalize DNS/domain values consistently.

## Testing Checklist

- Unit tests for new logic and edge cases.
- Integration tests for auth/cqrs/shutdown paths when behavior changes.
- Always run:

```bash
go test ./...
go test -race ./...
```
