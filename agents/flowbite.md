# Flowbite + UI Playbook

## Goals

- Keep UI consistent, readable, and intentional.
- Follow existing visual language in this repository.
- Preserve accessibility and responsive behavior.

## Color + Tokens

Use semantic color intent instead of ad-hoc per-page palettes:

- Primary: action emphasis (buttons/interactive highlights)
- Neutral: surfaces, borders, body text
- Success: healthy/online status
- Warning: degraded/incomplete status
- Error: failures/destructive actions

Rules:

- You MUST keep contrast at WCAG AA minimum for text and controls.
- You MUST avoid low-contrast muted-on-muted combinations.
- You MUST keep status colors consistent across overview, tables, and alerts.

## Component Guidance

- You SHOULD use Flowbite/Tailwind utility patterns already present in views.
- You MUST keep forms and tables predictable and scannable.
- You SHOULD use clear hierarchy: title, helper text, action controls.
- You MUST avoid style drift between fragments and full page sections.

## Accessibility

- You MUST ensure labels for all form controls.
- You MUST keep visible focus states on interactive controls.
- You MUST maintain sufficient color contrast for text and status indicators.
- You MUST use semantic elements (`section`, `table`, `button`, `form`) correctly.

## Interaction + Realtime

- You SHOULD keep Datastar bindings explicit and readable.
- You SHOULD avoid heavy inline logic in attribute expressions.
- You MUST keep partial fragment updates visually stable (no flicker/jumps).

## Review Checklist

- [ ] Mobile and desktop layout remain usable
- [ ] Form semantics and labels intact
- [ ] Focus/hover/disabled states visible
- [ ] Fragment updates preserve layout consistency
