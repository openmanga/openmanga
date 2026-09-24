# v1 polish backlog

State on 2026-09-24: v1 features are built (see ROADMAP.md). The app is installed at
`/Applications/Storyboarder Next.app`. This page lists what is left, so you can pick what comes next.
Tick an item to schedule it; strike it to drop it.

Size: S = under an hour of agent work, M = a few hours, L = a day or more. "Where" says which part changes.

## Needs you (can't be tested by an agent)

- [ ] **Real pen drawing**: pressure, lag, taper, eraser end. All agent checks used synthetic pointer events.
- [ ] **Trackpad pinch-zoom** on the canvas.
- [ ] **Drag images from Finder** into the Import sheet.

## Missing features

| | Item | Why | Where | Size |
|---|---|---|---|---|
| [ ] | Duplicate a page | No way to copy a page layout with its panels and balloons | CLI + app | S |
| [ ] | Balloon font and speaker fields | Inspector shows them in the design; not wired | CLI + app | S |
| [ ] | Lasso on pages | Lasso fill/erase/move works on boards only; the CLI region commands are board-only | CLI + app | M |
| [ ] | Undo for lasso edits | Lasso changes several layers at once; undo works per layer | CLI + app | M |
| [ ] | Onion skin for pages | Show neighbouring pages faintly while drawing on page layers | app | S |
| [ ] | Pen tilt | Stroke format has no tilt, so the Settings switch does nothing yet | CLI + app | M |
| [ ] | Native app menu | File / Edit / View / Window menus; Settings… only opens with ⌘, today | app | S |

## Known issues

| | Item | Detail | Where | Size |
|---|---|---|---|---|
| [ ] | Small SFX hidden by the reading-order badge | Default SFX size (32 px) is tiny on a 2000 px page, and the badge sits on top of it | CLI default + app badge placement | S |
| [ ] | Dev test hook ships in release builds | `SB_UI_SCRIPT` runs a JS file when set; inert otherwise. Remove or compile out of release | app (Rust + main.ts) | S |
| [ ] | `page info` returns `panels: null` for an empty page | Should be `[]`; the app works around it | CLI | S |
| [ ] | Window blank after a reload while it's in the background | WKWebView pauses rendering for hidden windows; only seen on dev hot-reload | app | S |
| [ ] | A `sb recent list` process outlived the app once | Seen after quitting the installed app on first launch; could not reproduce by hand (the command returns in 10 ms). Worth a timeout on sidecar calls | app (Rust) | S |

## Shipping

| | Item | Detail | Size |
|---|---|---|---|
| [ ] | Code signing | The app is unsigned; installed with `xattr -cr`. Needed only to share it with others | M |
| [ ] | DMG | Configured but not built (building it drives Finder) | S |
| [ ] | App icon | Check it's the Storyboarder Next icon, not the old one | S |

## Next big steps (from the roadmap)

| | Item | Detail | Size |
|---|---|---|---|
| [ ] | Claude skill for `sb` | Wrap the CLI and `cli/docs/claude-drawing-loop.md` in a skill so Claude can rough out pages end to end | M |
| [ ] | Script to pages (v1.x) | Import a Fountain script and distribute dialogue into pages, panels and balloons | L |
| [ ] | GitHub remote | Push this repo to a new private repo under your account | S |
