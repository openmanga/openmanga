# Storyboarder Next — Roadmap

A macOS tool for storyboarding manga first (the rough page draft, "name"), film later. Go CLI (`sb`) is the engine and the API for Claude; the app is Tauri with a UI that looks and behaves like a native SwiftUI Mac app.

Decided 2026-09-24. Full feature inventory of the original app: `docs/SCOPE.md` (314 features after exclusions).

## Architecture

- **`sb` (Go)**: every data and 2D-render operation. JSON output with `--json`. Owns the project files.
- **App (Tauri 2)**: macOS window, native menus, pen input, live drawing canvas (PIXI/WebGL), calls `sb` as a sidecar. Visual language: SwiftUI on macOS (sidebar + canvas + inspector, SF Pro, SF Symbols-style icons, system accent, light and dark).
- **Claude skill** (later): wraps `sb` so Claude can create pages, draw rough panels, render them, look at the result and iterate.

## v0.1 — Engine (in progress)

- Project, board, layer, audio metadata, prefs, keymap, language, recent projects, Fountain / Final Draft parsing.
- 2D rendering in Go: flatten layers, thumbnails, posterframes, image import, flip, clear, merge, GIF and PNG export.
- Claude drawing loop: `sb draw svg`, `sb draw strokes` (pressure taper), `sb draw text`, `sb draw erase`, board/layer renders with an optional coordinate grid, contact sheet, per-layer undo history.
- Stays compatible with existing `.storyboarder` projects.

## v1 — Manga name (first release)

The core loop: draw freely on boards, lay out pages, place boards into panels, add balloons, read the page, adjust.

- **Boards (drawings)**: free drawing sheets, independent of pages, with the six drawing layers (reference, fill, tone, pencil, ink, notes). A focused drawing screen for one board. Boards stay in the `.storyboarder` `boards[]` list, so the old app still opens them.
- **Pages**: stored next to the boards in a manga-mode `.storyboarder` (`pages[]`). Default page ratio 1:1.414, reading direction right-to-left (left-to-right option). Page 1 stands alone by default (project setting), then spreads [3|2], [5|4].
- **Panels (koma)**: vector frames on the page. Templates (splash, 2-tier, 3-tier, 4-koma, 1 big + 2, grid), split horizontal / vertical / diagonal with a gutter, merge, resize, delete, border width, bleed to page edge. Automatic reading order (right-to-left, top-to-bottom), manual override.
- **Placing drawings**: put a board into a panel (drag from the boards list), then position, scale, rotate and crop it inside the panel. The same board can be used in several panels. A panel focus view zooms into one panel to adjust the placement or draw directly for that panel.
- **Page-level drawing**: optional page layers for art that crosses panel borders.
- **Balloons and text**: speech, thought, shout, whisper, narration box, SFX; tail pointing at a speaker; horizontal or vertical text.
- **Page views**: single page, two-page spread (in reading order), all pages as a thumbnail grid.
- **Import**: images as a page or into a panel's reference layer.
- **Export**: PDF (one page per sheet), PNG per page, contact sheet.
- **Claude**: panel-aware drawing (`--panel` with panel-local coordinates), page and spread renders for review.
- **App**: Tauri UI for all of the above, pen pressure, keyboard shortcuts, light and dark.
- Hidden in v1 (kept in the engine): durations, audio, timeline, playback, video and GIF export.

## v1.x — Writing to pages

- Script to pages: import a Fountain script and distribute scenes and dialogue across pages and panels; dialogue becomes balloon text.
- Story notes per page and per panel; page count and pacing overview.
- Claude skill: "rough out chapter 1 from this script" end to end through `sb`.

## v2 — Film storyboard mode

Everything film-specific from the original, re-enabled in the app: boards in a row with durations, dialogue/action/notes, audio attach and record, timeline and playback, GIF / MP4 export, film PDF presets, fps and aspect ratio presets.

## v3 — Shot Generator (3D)

3D posing and camera blocking per board or panel: characters, poses, hand poses, props, lights, cameras, shot size and angle framing, Shot Explorer. The UI renders (three.js); `sb` owns the scene data. For manga it mostly serves as pose and perspective reference.

## Not planned

Excluded by the user: PSD import/export and linked PSD, external image editor, phone upload, QR worksheets, Final Cut / Premiere export, web bundle, watermark, VR / AR, storyboarders.com, wonderunit licensing, analytics, auto-updater.
