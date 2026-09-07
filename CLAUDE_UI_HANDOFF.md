# chinam0k handoff: ron1n operator UI prototype

Owner assignment: `CHINAM0K-RON1N-UI-001`

## Before editing

1. Read `README.md`, `AGENTS.md`, `DECISION.md`, `ARCHITECTURE.md`, `BUILD_STATUS.md`, `TESTING.md`, and `DESIGN.md` in full.
2. Inspect `git status` and preserve every existing dirty path.
3. Confirm the claim in `DECISION.md`. Do not broaden it.
4. Send lukk4n a short AgentMemory/inbox acknowledgement with the exact files you will create.

## Your lane

Create only:

```text
ui/prototype/index.html
ui/prototype/styles.css
ui/prototype/fixtures.js
ui/prototype/app.js
ui/prototype/HANDOFF.md
```

Do not edit any existing file. Do not touch Go, installers, release scripts/assets, existing Markdown, the imported PSFree tree, or any PS4-facing route. If one of the allowed files already exists with uncommitted work, stop and report the collision.

## Build brief

Implement a self-contained static prototype of the `DESIGN.md` Control Deck. Use semantic HTML, native CSS, and dependency-free JavaScript. It must open from a static server and make zero network requests. `fixtures.js` supplies clearly labelled prototype data; no fixture may contain a real token, capability, private key, public IP, or user path.

Required views:

- Overview
- Local Host
- Remote Relay
- Integrity
- Activity
- Sources
- Settings

Required interactions:

- responsive command rail/drawer navigation;
- light/dark theme control with local preference;
- URL and ID copy feedback;
- activity-stage filtering;
- a remote-session creation dialog with TTL choices and one-time masked capability behavior;
- a separate revoke confirmation using a non-secret session ID;
- disabled/loading/error states driven only by fixtures;
- a persistent statement that successful transfer does not prove console execution.

Use a tiny adapter boundary in `app.js` so lukk4n can replace fixtures with the Go API. Name intent methods without implementing requests: `getStatus`, `getActivity`, `getSources`, `verifyBundle`, `createSession`, and `revokeSession`. No `fetch`, WebSocket, form action, or external link should execute during the prototype.

## Quality gate

- Use only tokens copied from `DESIGN.md`; no ad-hoc raw visual values in components.
- No nested cards, rounded-everything, glass, blue/purple gradient, glow, emoji, generic three-column feature grid, decorative status dots, fake terminal rain, or perpetual animation.
- Every interactive element has hover, active, focus-visible, and disabled treatment.
- Verify at 390x844, 768x1024, and 1440x900; no horizontal overflow.
- Verify keyboard-only navigation, dialog focus/escape, reduced motion, theme parity, and a clean browser console.
- Capture the exact commands and results in `ui/prototype/HANDOFF.md` and note any limitation honestly.

## Handoff

Commit only the five allowed paths as `0xb0rn3 <154826956+0xb0rn3@users.noreply.github.com>` and push. Report the commit hash, screenshots/checks, and exact files to lukk4n through AgentMemory/inbox. Stop after the prototype handoff. lukk4n owns API design, Go embedding, security review, integration, final browser tests, documentation, and release publication.
