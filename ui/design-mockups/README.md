# Spectre UI redesign: mockups

Two different directions for the dark redesign. Both keep the same page structure: navigation on the left and a large detail area on the right. They are static HTML files with no build step. Open `approach-a.html` or `approach-b.html` and add `#timeline`, `#graph` or `#settings` to the URL. Screenshots (1600×960 @2x) are in `screenshots/`.

## A: Graphite (a dense, professional tool)

- **Palette:** cool graphite neutrals (`#0d0e10` / `#111214` / `#17181b`) with one cobalt accent, `#5b8def`.
- **Nav:** a 56px icon rail with no hover-expand. A thin accent tick marks the active page.
- **Top bar:** 52px tall. Breadcrumb (`Timeline / prod-eu-1`), then a single query bar that holds removable filter tokens (`ns:`, `kind:`, `status:`, opened with ⌘K), then a segmented time control (15m / 1h / 6h / 24h / custom).
- **Separation:** flat panes divided by 1px hairlines; no cards or shadows. A status bar at the bottom shows counts.
- **Timeline:** rows grouped by kind. Bars are thin (8px) and solid, with events as dots. A crash-loop is shown as one "24 restarts" marker instead of dozens of overlapping dots. The detail pane is docked flush on the right.
- **Graph:** layered left-to-right layout with labelled node cards and the causal path highlighted in the accent color. Causal paths are in a docked pane.
- **Settings:** a sub-nav column next to settings rows, in the style of Linear or Vercel.

## B: Warm Ink (calm and roomy, close to claude.ai/code)

- **Palette:** warm neutrals (`#151412` / `#1c1b19` / `#23221f`) with one ember accent, `#e07a47`.
- **Nav:** a 236px sidebar with labels, a cluster switcher, a "Recently viewed" list (new idea) and Settings at the bottom.
- **Canvas:** the content sits on an inset canvas with rounded corners, so the frame separates it from the sidebar.
- **Header and filters:** a page title with a summary line, then a row of soft filter chips (`Namespace 4 selected ▾`) and a search box opened with `/`. Controls are 30–32px tall and have no heavy borders.
- **Timeline:** taller rows (46px) with translucent tinted bars and event ticks. The detail view is a floating sheet, and the tracks shrink to make room for it.
- **Graph:** round nodes with labels, the causal path drawn in ember, and a causal-path sheet with a step-by-step chain.
- **Settings:** one centered column of grouped cards, in the style of macOS or claude.ai settings.

## Notes

- Both palettes avoid purple and teal, and the ghost tile gradient uses each palette's accent color.
- In B the ember accent sits close to the warning color, which is amber. To keep them apart, the accent is only used for the brand, the active nav item, primary buttons, toggles and the causal path. Row selection uses a neutral highlight, and warnings use a yellow-sand tone, `#d8b150`.
- The two directions can be mixed. For example, A's palette and query bar could go into B's sidebar and canvas layout.
