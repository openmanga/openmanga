# sb — Storyboarder Next backend

`sb` is a Go command-line tool that reads and writes Storyboarder projects
(`.storyboarder` scenes and `.fountain`/`.fdx` script projects) so the original
app can still open them. The desktop UI and Claude both call it as a subprocess:
every feature is `sb <noun> <verb> [args] [flags]`, and `--json` prints exactly one
JSON object (the result, or `{"error":{"code","message"}}` with a non-zero exit).

## Build and run

```sh
go build -o sb .                                  # debug build (~19 MB)
go build -trimpath -ldflags="-s -w" -o sb .       # release build (~14 MB)
go test ./... && go vet ./...

./sb help                     # every command (this text is the skill reference)
./sb board --help             # one group, with flags and details
./sb project new ~/Boards/Heist --aspect 2.39
cd ~/Boards/Heist && ../sb board add --json
```

Requirements at runtime: none for editing; `ffmpeg` on `PATH` (or `SB_FFMPEG`) for
`export video`; `ffprobe` for non-WAV audio durations; `lpr` (macOS) / `lp` (Linux)
for `print pdf`. `sb doctor` checks them.

Environment:

| Variable | Meaning |
|---|---|
| `SB_USER_DATA` | user-data folder (default: the app's, e.g. `~/Library/Application Support/Storyboarder`) holding `pref.json`, `keymap.json`, `locales/`, `presets/` |
| `SB_FFMPEG` | ffmpeg binary to use |
| `SB_FALLBACK_FONT` | TTF used in PDFs for text THICCCBOI cannot show (default: Arial Unicode on macOS, DejaVu Sans on Linux) |

## Conventions

- `--project <file|folder>` on every command; otherwise the `.storyboarder` (or the
  script with `storyboards/storyboard.settings`) in the current folder.
- Script projects: board/scene commands act on `--scene <n>` (default:
  `storyboard.settings` `lastScene + 1`; `sb scene open <n>` also remembers it).
- Boards: 1-based numbers or 5-char uids; lists `1,3`, ranges `2-4`, `all`, `last`.
- 3D objects: full id, 4+ char id prefix, or display name (`"Camera 1"`).
  3D commands act on `--board <i>` or the board chosen with `sb sg load <i>`.
- Writes keep every JSON key the CLI does not model (an order-preserving JSON tree
  is used end to end), print with 2-space indentation like `JSON.stringify(x, null, 2)`,
  and go through `<file>.backup-<ms>` renamed over the original. `version` is set
  to `3.0.0` on save, shot labels/numbers/times are recomputed, string durations
  from old projects are accepted everywhere.

## Command reference

Run `sb help` for the full list (232 commands) and `sb <noun> --help` for flags.
**film** marks commands that only matter in film mode (hidden in the v1 manga
app, kept in the engine); **3D** is Shot Generator data, postponed to v3 (it
exists and is tested but is not being extended).

| Group | Commands | Mode |
|---|---|---|
| project | new (`--manga`), set-mode, open, info, stats, migrate, verify [--fix], files, copy, zip, cleanup [--dry-run], contact-sheet | both |
| recent | list, add, prune | both |
| page / pages | page setup, add, list, info, delete, move, template, render [--grid]; pages contact-sheet | manga |
| panel | add, split, merge, set, delete, list, order, place (`--fit fill|fit|none`), clear-content, map | manga |
| balloon | add, set, delete, list | manga |
| spread | render | manga |
| draw | svg, strokes, path (`--taper both|start|end|none`, `--min-pressure`), tone (`--rect|--polygon --pattern dots|lines|crosshatch --spacing --density --angle [--gradient x1,y1,x2,y2 --density-to]`), text, erase (board, or `--page` [`--panel`]) | both |
| layer | list, replace, clear, merge, set-opacity (any drawing layer), history, undo, redo | both |
| board | list, get, info (`usedIn`), add (`--size WxH`, `--like-panel p:K [--place]`), set-name, set-description, delete, duplicate, move, flip, transform, erase-region, fill-region, move-region, render (`--layer`, `--grid`), path, export-clip, cut, paste, replace, set-dialogue/action/notes | both |
| board | set-new-shot, set-duration, suggest-duration, set-from-script | film |
| import | images (boards; `--pages [--fit]`; `--page --panel`) | both / manga |
| export | pages (`--png`/`--pdf`, `--include-notes`, `--page-numbers`, `--dpi`, `--crop-marks`), images | manga / both |
| export | pdf, pdf-presets, gif, video | film |
| print | pdf | film |
| scene | list, open, script, renumber, set-fps, set-default-duration | film (script projects) |
| script | parse, info, locations, characters, add-scene-ids, init, watch | film (v1.x script-to-pages later) |
| audio | set, clear, refresh | film |
| shotlist, sg … | Shot Generator data, explore, hand-off | 3D |
| prefs / keymap / lang | prefs get/set/set-tool/migrate/path, keymap list/set/migrate, lang list/set/copy/edit/import/export/remove | both |
| misc | doctor, app log-path, help urls, timelapse list, tip | both |

Drawing and review loop for agents: [docs/claude-drawing-loop.md](docs/claude-drawing-loop.md).
Manga file format: [docs/manga-format.md](docs/manga-format.md).

### 3D hand-off (ui-3d)

Everything that needs WebGL stays in the UI. The UI renders the active camera and
the 900x900 top-down plot and calls:

```sh
sb sg save --board 3 --camera-image cam.png --plot-image plot.png [--json scene.json]
sb sg insert-board --board 3 --camera-image cam.png --plot-image plot.png
```

This writes `board-N-UID-shot-generator.png` (fit to the board, 3 px padding as the
original), `-shot-generator-thumbnail.jpg`, `-camera-plot.png`, the board thumbnail
and posterframe, sets the `shot-generator` layer (opacity 1, thumbnail) and stores
`board.sg = {version, data}` — the same result as `saveToBoardFromShotGenerator`.
`sb sg status` then reports whether the scene changed since that save (SHA-1 of the
serialized scene, same function as the app's `getHash`).

## Dependencies

| Module | Why |
|---|---|
| `golang.org/x/image` | image scaling (CatmullRom), TrueType text for GIF captions and PDF previews, polygon rasterization for lasso tools; stdlib has none of these |
| `github.com/signintech/gopdf` | maintained pure-Go PDF writer with TrueType embedding/subsetting, JPEG embedding, transparency and clipping, needed to reproduce the pdfkit layout and write page PDFs |
| `github.com/tdewolff/canvas` | maintained pure-Go SVG parser + rasterizer (paths with arcs/beziers, shapes, strokes with caps/joins, fills, opacity, groups/transforms, text) for `sb draw svg`; the alternative `srwiley/oksvg` is unmaintained since 2022 and has no text. It adds about 6 MB to the binary. |

(Everything else in `go.mod` is indirect, pulled in by the three above; only
the packages actually imported are compiled into the binary.)
The CLI dispatcher, JSON handling, ZIP, GIF encoding and 3D math are stdlib-only.

Embedded data (copied from the original app): the four THICCCBOI TTF weights,
the three built-in locales, 89 default key bindings (the original 87 plus `menu:tools:panel` P and `menu:tools:balloon` B), story tips, 342 poses,
32 hand poses, 6 emotions, model metadata, the 15 PDF presets, and
`internal/sg/data/skeletons.json`, which is generated from the character `.glb`
files with `go run ./internal/sg/gen <storyboarder>/src/data/shot-generator/dummies/gltf`.

## Tests

`go test ./...` covers: byte-identical JSON round trip of every fixture; scene
save keeps unknown fields; shot numbering (1A, 1B, …, 2Z, 2A2) and timing with
string durations; Fountain and FDX parsing compared with output of the original
JS parsers (`testdata/oracle/*.json`, generated once with Node); flatten pixel
values; cleanup rename/trash plan on the `ducks` fixture; pre-1.6 migration;
clipboard round trip; GIF frames/delays; ffmpeg arguments (and a real encode when
ffmpeg is installed); a PDF for each of the 15 presets; page preview size; 3D
object defaults, update rules, mirror, groups, framing, deterministic explorer,
hash equal to the original's; presets and undo history; CLI end-to-end and help
completeness. Drawing/manga: SVG rasterization pixels (plain and panel
offset), stroke taper width, panel clipping leaves outside pixels untouched,
layer undo restores the exact previous bytes and history is capped at 20,
contact sheet size, templates, split with gutter, rtl/ltr reading order,
balloon pixels stay inside their box, spreads [3|2] vs [2|3], and a CLI test
drawing into a panel of a manga page.

## Known gaps and deliberate differences

- Drawing: strokes are rasterized as tapered ribbons with round joins (no brush
  textures other than a pencil grain); SVG text uses system fonts found by the
  SVG renderer (a `font-family` with no installed match is a `font_not_found`
  error, not a fallback, because that renderer cannot load the embedded fonts),
  `draw text` and balloons use THICCCBOI with a system fallback.
- Layer history keeps 20 versions per layer file in `images/.history/` (ignored
  by cleanup, copy and zip); redo lasts until the next edit.
- Manga: panels are polygons (no curved panels); balloon tails are straight
  wedges; panel content is cropped only by the panel shape; GIF/video scale
  boards with their own `size` to the project size; `export
  pages --pdf` sizes each sheet to the page ratio with the long side of A4.

- `sg render`, explorer thumbnails, pose thumbnails and emotion thumbnails are ui-3d;
  `sg preset emotion create` stores the texture and preset without a thumbnail.
- The raw scene setter is `sg replace --json` (SCOPE's `sg set <i> --json` would
  clash with `sg set <id> key=value`).
- PDF: text is laid out with the fonts' kerning and a simplified line breaker; the
  footer reads "Storyboarder by wonder unit" (the original relies on a font ligature
  for "\\"). pdfkit adds its ellipsis even when text just fits the last line; here
  it only appears when text is cut. Unicode fallback uses a system font (the app's
  10 MB unicore.ttf is not embedded).
- GIF palette: popularity quantizer + Floyd–Steinberg instead of NeuQuant.
- Camera framing, drop-to-floor and establishing shots use the bind-pose mesh
  bounds (box corners) and the extracted skeletons; drop-to-floor only treats the
  ground and box objects as surfaces. The establishing-shot direction uses the
  camera height where the original reads an undefined value.
- Character preset attachables are recreated with their stored bone-space offsets
  (the original re-derives them from IK bone data that is not saved).
- Bugs not reproduced: `UPDATE_OBJECTS` ignoring 0 values (`sg move` accepts 0);
  the hand-pose preset check looking in pose presets; `"\nundefined"` appended to
  slugline notes; string durations written by "copy script line" (numbers are written).
- `project new --force`, `project copy --force` and `project cleanup` move files to
  the user's trash (macOS `~/.Trash`, Linux XDG trash); Windows is not supported for those.
- The slow-CPU default check uses the core count only (clock speed is not available
  from the Go stdlib).
- `script watch` polls the file (no fsnotify dependency).
