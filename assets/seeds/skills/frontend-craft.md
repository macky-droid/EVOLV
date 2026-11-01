---
name: frontend-craft
description: Use when growing a component or restyling the EVOLV shell; covers the house visual language and UI quality bar.
---
# Frontend Craft

The UI is your body, not a demo. It should feel calm, deliberate, and fast.

## House tokens

- Background `#262624`, text `#e8e6e1`, accent `#d97757`, panels `#30302e`.
- Use the existing CSS variables (`--bg`, `--surface`, `--text`, `--muted`,
  `--line`, `--accent`) when editing the shell; do not hard-code a second theme.
- No neon glow, no gradient for its own sake, no external fonts or CDN scripts.
- Dark-first, readable at 14–16px body text, generous spacing.

## Components

A component is one self-contained `.html` file:

```html
<style>
  #c-clock-root { ... }
</style>
<div id="c-clock-root">...</div>
<script>
  (() => { ... })();
</script>
```

Rules:
- Every id starts with `c-<name>-` to avoid collisions on the shared canvas.
- Mount into the element you own; never query or mutate host elements.
- Clean up timers and listeners when the component is removed or re-rendered.
- Keep it under ~150 lines. If it needs a library, it is a tool, not a component.
- Interactivity must have a keyboard path and visible focus.
- Respect `prefers-reduced-motion`; animate transform/opacity, not layout.
- No fixed widths that break the canvas; use `min(100%, ...)` and flex/grid.
- Label every input. Use `aria-live` sparingly for status, not for decoration.

## Shell edits

- `style.css` owns the skin, including TOOL CALL CARDS. Find that section before
  editing; preserve the variables and dark palette unless asked otherwise.
- `app.js` owns rendering. Keep changes small and test the chat + tool cards.
- `index.html` owns structure. Do not remove hooks that app.js queries.
- After an edit the watcher hot-reloads; call reload_shell only if it did not.

## Quality bar

- First paint is useful: no blank box waiting for a request.
- Empty, loading, and error states are designed, not accidental.
- Long text wraps; numbers align; buttons look clickable and disabled states read
  as disabled.
- Motion has a purpose and can be turned off.
- Check at narrow width before calling it done.

## Done when

The panel mounts, survives a shell reload, looks intentional at desktop and
narrow widths, and its interaction works with keyboard and pointer.
