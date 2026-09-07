# DESIGN.md — ron1n Control Deck

This is the visual contract for every ron1n operator surface. The PS4-facing imported pages are external byte-exact content and are outside this design system.

## Foundations

ron1n is an operations instrument for authorized PS4 firmware-9.00 owners. The interface should feel like a precise workshop control deck: industrial, calm, information-dense where evidence matters, and quiet everywhere else. Japanese-inspired restraint comes from proportion, alignment, and negative space rather than decorative samurai imagery. The page job is to operate and verify. The voice is direct, technical, and evidence-led.

- One primary action per view.
- Structure comes from type, spacing, and rules rather than stacked cards.
- Dark mode is the default; light mode is a first-class paper-and-ink counterpart.
- Never claim an exploit, kernel stage, patch, or payload executed from HTTP telemetry.

## Color tokens

Every component uses these roles. Raw color values are forbidden outside the token declarations.

```css
:root {
  color-scheme: light;
  --bg: #f1efe8;
  --surface: #fbfaf6;
  --surface-raised: #ffffff;
  --surface-pressed: #e5e1d7;
  --text: #171819;
  --muted: #55595d;
  --dim: #73777a;
  --border: #c7c2b7;
  --border-strong: #858178;
  --accent: #c83b29;
  --accent-hi: #a92e20;
  --on-accent: #ffffff;
  --ok: #26754e;
  --warn: #9a650d;
  --bad: #b52d2d;
  --focus: #171819;
  --scrim: rgba(10, 11, 12, .66);
}

:root[data-theme="dark"] {
  color-scheme: dark;
  --bg: #0b0d0f;
  --surface: #121519;
  --surface-raised: #191d22;
  --surface-pressed: #242a31;
  --text: #f3f1eb;
  --muted: #aaa8a1;
  --dim: #7d8186;
  --border: #30353b;
  --border-strong: #60666d;
  --accent: #ff5a3d;
  --accent-hi: #ff765f;
  --on-accent: #1b0804;
  --ok: #65d49a;
  --warn: #f0b85f;
  --bad: #ff746b;
  --focus: #f3f1eb;
  --scrim: rgba(0, 0, 0, .74);
}
```

Vermilion is the only brand accent and is reserved for the current view, the primary action, and high-salience focus. State colors communicate real state only. No purple/blue gradients, aurora glow, glassmorphism, or decorative neon.

## Typography

- Display: `Inter, Arial, Helvetica, system-ui, sans-serif`; weights 750–900, tight tracking, sentence case except the `RON1N` wordmark.
- Body: `Inter, system-ui, -apple-system, "Segoe UI", sans-serif`; weights 400/500/650.
- Data: `"JetBrains Mono", "Cascadia Mono", "SFMono-Regular", Consolas, monospace`.
- Scale: `--fs-xs: .75rem`, `--fs-sm: .875rem`, `--fs-base: 1rem`, `--fs-lg: 1.25rem`, `--fs-xl: 1.625rem`, `--fs-2xl: clamp(2rem, 4vw, 3.5rem)`.
- Body leading is 1.5; compact telemetry leading is 1.35; display leading is .95–1.08.
- Hashes, versions, ports, URLs, timestamps, session IDs, and transfer sizes use the data face.

No remote font request is allowed. The system stacks must render cleanly offline.

## Spacing, geometry, and elevation

- Spacing: `--s1: 4px`, `--s2: 8px`, `--s3: 12px`, `--s4: 16px`, `--s6: 24px`, `--s8: 32px`, `--s12: 48px`, `--s16: 64px`.
- Corners: `--r-control: 2px`, `--r-panel: 0`; panels and major controls remain mechanically square.
- Rules: `--line: 1px`; major section boundaries may use `2px` with `--border-strong`.
- Elevation: use borders and surface shifts. Only dialogs may use `--shadow-dialog: 0 24px 70px rgba(0,0,0,.34)`.
- Touch targets are at least 44px. Desktop content width is capped at 1600px.

The desktop shell uses a 240px command rail, a compact top status strip, and one main stage. At 800px and below, the rail becomes a labelled bottom navigation or drawer and the stage becomes a single column. At 480px, long values wrap or scroll inside their own data row; the document must never overflow horizontally.

## Motion

```css
:root {
  --dur-fast: 120ms;
  --dur: 180ms;
  --dur-panel: 260ms;
  --ease-out: cubic-bezier(.2,.7,.2,1);
  --ease-in: cubic-bezier(.4,0,1,1);
}
```

Animate only user-caused state changes: active navigation, disclosure, dialog, copy confirmation, and newly arrived activity. Use opacity plus at most 6px of transform. No pulsing indicators, scroll reveals, typing effects, ambient loops, or fake progress. `prefers-reduced-motion: reduce` makes every transition immediate.

## Application shell

- Wordmark: `RON1N` with `0.0.1zoro` and the distribution revision shown as separate data.
- Command rail: Overview, Local Host, Remote Relay, Integrity, Activity, Sources, Settings.
- Status strip: service state, local listener, active bundle short ID, and relay state. Every state marker includes a text label.
- Main-stage header: eyebrow, exact view title, one sentence of operational context, and at most one primary action.
- Panels are flat grid compartments separated by rules. Never nest cards.

## Components

- Primary button: solid accent, on-accent text, square 44px minimum height. Hover changes to `--accent-hi`; active translates 1px; focus uses a 2px outer focus rule.
- Secondary button: transparent or surface fill with strong border. Destructive actions use `--bad` only after a confirmation step.
- Inputs: persistent labels above fields, visible help/error text, 44px height, data face for URLs/tokens/ports. Placeholder text never carries required instructions.
- Status: `OK`, `BUSY`, `OFF`, `ERROR`, or an exact transfer stage in text. A small square marker may accompany the word; color never acts alone.
- Data row: label, value, optional copy control. Sensitive values default to masked and are removed from the DOM after their one-time display expires.
- Activity row: timestamp, stage, route/artifact, byte count, transport, and result. Stage names come from the Go contract unchanged.
- Dialog: reserved for remote-session creation, revocation, and destructive changes. Focus is trapped and returned to the invoking control.
- Toast: short action confirmation only. Errors remain visible inline until resolved.

## View requirements

1. Overview: service health, active bundle, local connection URLs, relay readiness, and recent activity. The primary action reflects the real next step.
2. Local Host: start/restart guidance, listener, root/cache URLs, copy controls, content state, and firewall guidance without automatic mutation.
3. Remote Relay: configure an HTTPS relay, connect outbound workers, create an expiring session, display its URL once, retain the non-secret session ID, and revoke explicitly.
4. Integrity: pinned source/revision, bundle ID, signature state, required-file checks, GoldHEN hash, and a named verify action.
5. Activity: filterable truthful stages: `artifact-requested`, `head-complete`, `artifact-partial`, `artifact-delivered`, and `response-failed`. Include a permanent note that delivery is not execution.
6. Sources: attribution, license status, role, inclusion policy, and links from the audited catalog.
7. Settings: port, recent-event window, content update controls, autostart state, theme, diagnostics, and preserved data locations. Application update and content update are distinct.

## Security and copy rules

- The production dashboard is loopback-only by default and never travels through browser capability routes.
- Never render signing private keys or saved relay bearer tokens.
- Never add arbitrary shell, command, URL proxy, filesystem browser, upload, or payload-execution controls.
- Capability URLs are secrets: show once, copy deliberately, auto-mask, never place in logs or fixtures.
- Use specific verbs: `Verify bundle`, `Start local host`, `Create 30-minute session`, `Revoke session`.
- Avoid hype, vague CTAs, samurai clichés, emoji, fake terminal output, and fake online status.

## Acceptance

- 390px, 768px, and 1440px layouts have no document-level horizontal overflow.
- Light and dark themes have hierarchy and WCAG AA contrast parity.
- Keyboard navigation, focus-visible, labels, dialog focus, and live action feedback work without a mouse.
- Reduced motion disables non-essential transitions.
- Browser console is clean; no CDN, analytics, or runtime dependency is loaded.
- Raw color, spacing, radius, and duration values appear only in this token contract or the prototype token block copied from it.
