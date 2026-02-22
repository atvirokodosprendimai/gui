# Agents Playbooks

This folder contains human-readable playbooks for AI/code agents working in this repository.

Use these together with the root `AGENTS.md`:

- `../AGENTS.md` = project-wide hard rules and definition of done.
- `agents/*.md` = domain-specific implementation guidance and review checklists.

## Playbooks

- `golang.md` - idiomatic Go, error handling, concurrency, and testing.
- `nats.md` - subjects, scoped UI update routing, reliability, and ops.
- `datastar.md` - signals, SSE patching, and safe command transport.
- `flowbite.md` - UI composition, accessibility, and consistency guidance.
- `cqrs.md` - command/read split, notification flow, and patch strategy.

## How To Use

1. You MUST start with `AGENTS.md` and confirm constraints.
2. You MUST load only the playbooks relevant to the task.
3. You SHOULD follow checklists while implementing and reviewing.
4. You MUST run required tests before finalizing.

## Playbook Selection Matrix

- Go backend logic: `golang.md`
- Datastar handlers/signals/SSE: `datastar.md` + `golang.md`
- NATS publish/subscribe/routing: `nats.md` + `cqrs.md`
- CQRS flow/read model/patch targeting: `cqrs.md` + `datastar.md`
- UI/layout/accessibility updates: `flowbite.md` (+ `datastar.md` if bindings/events change)
- Auth/security-sensitive changes: relevant domain playbook + root `AGENTS.md` security sections
