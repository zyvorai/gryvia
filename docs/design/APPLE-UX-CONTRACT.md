# Gryvia UX contract

The dashboard (`web-ui/`) and docs site (`website/`) share one apple.com-style
design system. Tokens live in `web-ui/src/styles/tokens.css` and are mirrored
in `website/src/css/custom.css`. The system is adapted from the sibling
project netra.

## Laws
1. **Elevation runs up.** Dark: page `#000`, panel `#1d1d1f`, lighter cards. Light: white page, hairline cards.
2. **Color signals deviation.** Nominal values are graphite. Blue means intent (the primary action). Red, amber and green appear only for a real state (`.pill.ok/.warn/.bad`, `.dot`, `.progress.*`).
3. **One primary action per view** (`button.primary` / `.buttonlike.primary`). Secondary is `btn-secondary`, destructive is `danger`.
4. **No hard-coded hex/rgba in components.** Use tokens (`var(--text-secondary)`). Tailwind is for layout utilities only (flex, grid, gap, spacing), never color.
5. **Read-only surfaces stay read-only.**

## Structure
| Tier | Pages | Building blocks |
|---|---|---|
| Story | Login, Dashboard | `PageHero`, `.apple-metric-band`, `Reveal` |
| Browse | Jobs, Nodes, Flows, Workspaces, Models | `PageHero`, `.toolbar-pill`, `.table-wrap` |
| Work | Quotas, Policies, Tuner, Workflows, Submit Job | `.card` panels, `.card-grid`, `.modal-card` |

## Tokens
- Fonts: system stack only (`-apple-system, "SF Pro Text/Display"`, `ui-monospace` for code). No webfonts.
- Type scale: `--fs-h1` … `--fs-body`; no raw sizes above 17px.
- Radii: `--radius-xs` (6px) to `--radius-3xl` (28px), `--radius-pill` for buttons.
- Motion: `--ease-apple`, 0.28s transitions; honor `prefers-reduced-motion`.
- Hit target: `--hit-min` (44px). Focus: 2px `--apple-blue` outline.
- Glass: nav uses `--nav-bg` with `backdrop-filter: saturate(180%) blur(20px)`.

## Theme
`data-theme` on `<html>`; default `light`. The stored `gryvia-theme` value wins and
is applied by an inline script in `index.html` before first paint.

## Checks
Every page must be checked in light and dark at 1440px and 390px, with no
horizontal scroll at 390px.
