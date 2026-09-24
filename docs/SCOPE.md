# Storyboarder Next — feature scope

Derived from the full inventory of Storyboarder 3.0.0 (354 features) with the user's exclusions applied on 2026-09-24:
no PSD, phone, QR/worksheet import; no PSD / Final Cut / Premiere / web-bundle export; no watermark; no linked PSD or external editor; no VR/AR/remote; no dead online services (storyboarders.com, wonderunit licensing, Google Analytics, ads, auto-updater).

Owner column: `cli` = Go backend (`sb` binary, JSON output). `ui` = interactive, lives in the app. `ui-3d` = needs the three.js renderer, so the UI renders and hands images to `sb`.


## Project and files

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| New blank project | Creates `<dir>/<name>.storyboarder` and `images/` with aspect ratio, fps, default timing | DIRECT | cli | `project new <dir> --aspect 2.39 --fps 24` |
| Aspect ratio presets | 2.39, 2.0, 1.85, 16:9, 9:16, 1:1, 4:3 | DIRECT | cli | `project new --aspect` |
| Overwrite existing folder | Moves an existing target folder to the Trash before creating | DIRECT | cli | `project new --force` |
| Open project | Opens `.storyboarder`, `.fountain` or `.fdx` | DIRECT | cli | `project open <file>` |
| Project info | Version, aspect, fps, default timing, board count, file path | DIRECT | cli | `project info` |
| Project stats | Shot count, board count, total duration incl. audio, average line mileage | DIRECT | cli | `project stats` |
| Recent projects list | List of recent projects with metadata | DIRECT | cli | `recent list` |
| Add to recent | Puts a project at the top of the recent list | DIRECT | cli | `recent add <file>` |
| Prune recents | Removes recent entries whose files are gone | DIRECT | cli | `recent prune` |
| Migrate string durations | Converts old string `duration` values to numbers | DIRECT | cli | `project migrate` |
| Migrate pre-1.6 layers | Backs up to `<name>-backup`, moves single PNG into the fill layer | DIRECT | cli | `project migrate` |
| Verify: missing layer files | Writes blank placeholder PNGs for missing layers and thumbnails | DIRECT | cli | `project verify --fix` |
| Verify: missing linked PSD | Removes `board.link` when the PSD is gone | DIRECT | cli | `project verify --fix` |
| Verify: missing posterframes | Rebuilds flattened `-posterframe.jpg` from layers | RENDER | cli | `project verify --fix` |
| Save | Writes dirty layers and the JSON (atomic via `.backup-<ts>` then rename) | DIRECT | cli | implicit in every write |
| Autosave | Debounced 5 s save after edits | DIRECT | cli | implicit |
| Save As (copy project) | Copies every used file into a new folder and renames the project file | DIRECT | cli | `project copy <dst>` |
| List project files | Every file a scene or script project uses (layers, thumbs, audio, PSD, 3D models) | DIRECT | cli | `project files` |
| Export project ZIP | Copies used files and zips them, reports missing files | DIRECT | cli | `project zip` |
| Clean up scene | Renames files to index order, drops dead links and audio, trashes unused images | DIRECT | cli | `project cleanup [--dry-run]` |
| Scene timing recompute | Recomputes shot labels (1A, 1B…), board numbers and start times | DIRECT | cli | `scene renumber` |
| Set fps | 12, 15, 23.976, 24, 25, 29.97, 30, 50, 59.94, 60 | DIRECT | cli | `scene set-fps <fps>` |
| Default board duration | Scene-level default duration in ms | DIRECT | cli | `scene set-default-duration <ms>` |
| Show project in Finder | Reveals a board layer file or `images/` | PARTIAL | cli | `board path <i> --layer <name>` |
| WebGL capability check | Refuses to open a project when WebGL fails | UI | ui | - |
| Move app to Applications | macOS prompt when run from elsewhere | UI | ui | - |

## Script projects and scenes

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Parse Fountain | Tokenizes a Fountain script into scenes with estimated duration and word counts | DIRECT | cli | `script parse <file.fountain>` |
| Parse Final Draft | Parses `.fdx` XML into scenes with timing from word counts | DIRECT | cli | `script parse <file.fdx>` |
| Insert scene IDs | Appends stable IDs to unnumbered scene headings and rewrites the script | DIRECT | cli | `script add-scene-ids <file>` |
| Script info | Title from title page, scene count, total estimated time | DIRECT | cli | `script info` |
| Locations list | Counts slugline locations | DIRECT | cli | `script locations` |
| Characters list | Counts speaking characters | DIRECT | cli | `script characters` |
| Create script project | Writes `storyboards/storyboard.settings` next to the script | DIRECT | cli | `script init <file> --aspect <r>` |
| Scene list | Scenes with number, duration, slugline, synopsis | DIRECT | cli | `scene list` |
| Open or create scene folder | Maps a scene to `Scene-<n>-<slug>-<id>/`, creating JSON and `images/` if missing | DIRECT | cli | `scene open <n>` |
| Scene script text | Action, dialogue and transitions of one scene | DIRECT | cli | `scene script <n>` |
| Copy script line to board | Fills a board's dialogue ("CHAR: text"), action, notes or duration from a script line | DIRECT | cli | `board set-from-script <i> <line>` |
| Watch script file | Reloads the project when the script changes on disk | DIRECT | cli | `script watch` |
| Next / previous scene navigation | Keyboard and transport navigation across scenes | UI | ui | use `scene open` |
| Location colors | Color per location in the outline | UI | ui | - |

## Boards

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| List boards | uid, index, shot, duration, dialogue, has 3D shot | DIRECT | cli | `board list` |
| Get board | Full board object | DIRECT | cli | `board get <uid>` |
| Board info | "Shot X", "Board N of M", line miles | DIRECT | cli | `board info <i>` |
| Add board after | New blank board after index | DIRECT | cli | `board add --after <i>` |
| Add board before | New blank board before index | DIRECT | cli | `board add --before <i>` |
| Add board at end | Append a blank board | DIRECT | cli | `board add --end` |
| Delete boards | Removes boards (keeps at least one); files stay on disk | DIRECT | cli | `board delete <i..>` |
| Duplicate board | Copies layers, thumbnail, posterframe, PSD under a new uid; clears text and audio | DIRECT | cli | `board duplicate <i>` |
| Move board by one | Reorder left / right | DIRECT | cli | `board move <i> --by -1` |
| Move boards to index | Drag-reorder equivalent for one or many boards | DIRECT | cli | `board move <i..> --to <j>` |
| Toggle new shot | Flips `newShot`, which changes shot numbering | DIRECT | cli | `board set-new-shot <i> on/off` |
| Set duration (ms) | Duration for one or many boards | DIRECT | cli | `board set-duration <i..> <ms>` |
| Set duration (frames) | Frames converted with scene fps | DIRECT | cli | `board set-duration <i> --frames <n>` |
| Suggest duration from dialogue | words x 300 ms + 300 | DIRECT | cli | `board suggest-duration <i>` |
| Set dialogue | Board dialogue text | DIRECT | cli | `board set-dialogue <i> "<text>"` |
| Set action | Board action text | DIRECT | cli | `board set-action <i> "<text>"` |
| Set notes | Board notes text | DIRECT | cli | `board set-notes <i> "<text>"` |
| Clear board artwork | Clears all layers (deletes the board if already empty) | RENDER | cli | `layer clear <i> --all` |
| Flip horizontal | Mirrors all visible layers | RENDER | cli | `board flip <i> --horizontal` |
| Flip vertical | Mirrors vertically (menu only) | RENDER | cli | `board flip <i> --vertical` |
| Translate artwork | Move all layers by dx/dy | PARTIAL | cli | `board transform <i> --dx --dy` |
| Scale artwork | Scale all layers around an anchor | PARTIAL | cli | `board transform <i> --scale --anchor x,y` |
| Erase region | Erase inside a polygon on all visible layers (lasso erase) | PARTIAL | cli | `board erase-region <i> --polygon` |
| Fill region | Fill a polygon on the fill layer with color and opacity (lasso fill) | PARTIAL | cli | `board fill-region <i> --polygon --color` |
| Move region | Cut and move a polygon of artwork | PARTIAL | cli | `board move-region <i> --polygon --dx --dy` |
| Render posterframe | Flattened full-size JPG on white | RENDER | cli | `board render <i> --posterframe` |
| Render thumbnail | 120 px-high PNG used in the strip | RENDER | cli | `board render <i> --thumbnail` |
| Render flattened PNG | Composite of all layers with opacities | RENDER | cli | `board render <i> --out <png>` |
| Line mileage | Stroke length accumulated per board | UI | ui | - |
| Go to / next / previous board | Navigation, scrubbing, range select | UI | ui | CLI addresses boards by index |
| On-canvas caption | Overlay of dialogue on the canvas | UI | ui | - |
| Thumbnail context menu | Add, duplicate, copy, paste, import, delete, reorder | UI | ui | covered by commands above |

## Layers

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Layer list | Which layers exist for a board and their files/opacity | DIRECT | cli | `layer list <i>` |
| Replace a layer with an image | Fits an image into a layer (reference by default) | RENDER | cli | `layer replace <i> reference <img>` |
| Clear one layer | Clears reference, fill, tone, pencil, ink or notes | RENDER | cli | `layer clear <i> <name>` |
| Merge fill into reference | "Merge down" | RENDER | cli | `layer merge <i> --into reference` |
| Merge reference into fill | "Merge up" | RENDER | cli | `layer merge <i> --into fill` |
| Reference opacity | Sets reference and 3D layer opacity (default 0.75) | PARTIAL | cli | `layer set-opacity <i> reference <0-1>` |
| Export layer PNG path | Path of a specific layer file for external tools | DIRECT | cli | `board path <i> --layer <name>` |

## Drawing tools and brushes

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Light pencil | Draws on reference layer, light blue, size 20 | UI | ui | - |
| Brush | Draws on fill layer, size 26 | UI | ui | - |
| Tone | Draws on tone layer, size 50, low opacity | UI | ui | - |
| Pencil | Draws on pencil layer, size 4 | UI | ui | - |
| Pen | Draws on ink layer, black, size 2 | UI | ui | - |
| Note pen | Draws on notes layer, red, size 8 | UI | ui | - |
| Eraser | Erases visible layers, size 26 | UI | ui | - |
| Pressure and tilt strokes | Pen pressure/tilt drive stroke width (PIXI WebGL engine) | UI | ui | - |
| Straight line (Shift), angle snap (Alt) | Constrained strokes | UI | ui | - |
| Auto straight line on hold | Holding still turns a stroke straight (delay pref) | UI | ui | - |
| Poly-line mode | Chained straight segments | UI | ui | - |
| Quick erase | Right button, pen eraser end or Alt | UI | ui | - |
| Lasso selection | Freeform / polyline selection with add and subtract | UI | ui | region commands in Boards |
| Move / scale artwork gizmo | Cmd-drag and Cmd+Alt-drag | UI | ui | transform commands in Boards |
| Brush size | `[` / `]` and drag, 1–256 | UI | ui | - |
| Stroke opacity per tool | 5–100 %, saved in prefs | PARTIAL | cli | `prefs set-tool <tool> --opacity 0.8` |
| Tool color | Swatch grid and hex input | PARTIAL | cli | `prefs set-tool <tool> --color #hex` |
| Palette swatches (3 per tool) | Editable quick colors | PARTIAL | cli | `prefs set-tool <tool> --palette c1,c2,c3` |
| Undo / redo (2D) | 25-step in-memory stack for images and data | UI | ui | a CLI would use its own history or git |

## Clipboard

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Copy boards | Boards plus base64 layers as JSON | PARTIAL | cli | `board export-clip <i..> --out clip.json` |
| Cut boards | Copy, then delete | PARTIAL | cli | `board cut <i..> --out clip.json` |
| Paste boards | Insert boards from clip JSON or an external image | RENDER | cli | `board paste --from clip.json` |
| Paste and replace | Replace one board's layers from clip | RENDER | cli | `board replace <i> --from clip.json` |

## Audio

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Attach audio file | Copies wav/mp3/m4a/mp4 as `<uid>-<name>` and sets `board.audio` | PARTIAL | cli | `audio set <i> <file>` (duration via ffprobe) |
| Remove audio | Clears `board.audio` (file stays) | DIRECT | cli | `audio clear <i>` |
| Refresh audio durations | Recomputes `audio.duration` for all boards | PARTIAL | cli | `audio refresh` |
| Record from microphone | Countdown, meter, waveform, saves wav | UI | ui | - |
| Audition on navigation | Plays board audio when moving between boards | UI | ui | - |
| Stop all sounds | Escape stops audio | UI | ui | - |
| Audio lanes in timeline | Waveform display | UI | ui | - |

## Playback, timeline and view

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Play / pause | Plays boards by time with audio overlap and fades | UI | ui | export video instead |
| Speak dialogue | Text-to-speech of dialogue during playback | UI | ui | - |
| Prevent display sleep | While playing | UI | ui | - |
| Boards / timeline toggle | Thumbnail strip vs proportional timeline | UI | ui | - |
| Timeline zoom and scroll | Minimap with handles | UI | ui | - |
| Drag duration in timeline | Resize a board's duration | UI | ui | `board set-duration` |
| View modes | 4 layouts (6 with a script panel) | UI | ui | - |
| Captions toggle | Show/hide caption overlay | UI | ui | - |
| Grid guide | 50 px grid | UI | ui | - |
| Center guide | Center cross | UI | ui | - |
| Thirds guide | Rule of thirds | UI | ui | - |
| Perspective guide | Toggle exists, rendering disabled in code | UI | ui | - |
| Eyeline guide | Implemented but never wired on | UI | ui | - |
| Onion skin | Previous (blue) and next (red) boards ghosted at 35 % | UI | ui | - |
| Canvas zoom, pan, actual size | Wheel zoom 0.25–5, space-drag pan | UI | ui | - |
| UI scale | Interface zoom, saved | UI | ui | - |
| Fullscreen | Standard role | UI | ui | - |
| FPS meter | Diagnostics overlay | UI | ui | - |
| Tooltips | Hover tooltips with key hints | UI | ui | - |

## Import

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Import images as new boards | PNG/JPG/PSD (folders recursive) become boards on the reference layer | RENDER | cli | `import images <files..> [--after <i>]` |
| Import image replacing reference | One image into the current board's reference layer | RENDER | cli | `layer replace <i> reference <img>` |
| Drag-and-drop files | Opens projects, attaches audio, imports images | UI | ui | covered by commands above |

## Export

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Export PDF | pdfkit layout of boards with header stats and footer | DIRECT | cli | `export pdf` |
| PDF presets (15) | Landscape Minimal 3x5/3x2/4x4/1x, Thumbs 2x2/3x3/6x5, Storytime, Pairs, Solo, Hey-oh! | DIRECT | cli | `export pdf --preset <id>` |
| PDF custom layout | Paper A4/Letter, orientation, grid 1–10, direction, text toggles, time, text size, border | DIRECT | cli | `export pdf --paper --grid RxC ...` |
| PDF all scenes | One PDF for every scene of a script project | DIRECT | cli | `export pdf --all-scenes` |
| PDF page preview | Rasterizes one page to PNG | RENDER | cli | `export pdf --page N --preview out.png` |
| Print PDF | Sends to printer via `lpr` / `lp` | DIRECT | cli | `print pdf --copies N` |
| Animated GIF | 888 px wide, per-board timing, dialogue captions, optional watermark | RENDER | cli | `export gif [--boards i..]` |
| Video (MP4) | Flattened boards + audio mix through ffmpeg, H.264 at scene fps | RENDER | cli | `export video` |
| Images (PNG per board) | Flattened `<name>-board-00001.png` files | RENDER | cli | `export images [--out dir]` |
| 3D scene to glTF | Exports a Shot Generator scene as `.glb` | RENDER | ui-3d | `sg export-gltf <i>` |

## Shot Generator: scene and board I/O

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Load a board's 3D scene | Reads `board.sg.data` (world, sceneObjects, activeCamera) | DIRECT | cli | `sg load <i>` |
| Reset scene | Default scene with one camera and ground | DIRECT | cli | `sg reset <i>` |
| Get / set raw scene JSON | Read or replace the whole scene | DIRECT | cli | `sg get <i>` / `sg set <i> --json scene.json` |
| Scene dirty status | SHA1 of serialized state vs last save | DIRECT | cli | `sg status <i>` |
| Save shot to board | Renders camera view + 900x900 plot, writes layer and thumbnails, stores `board.sg` | RENDER | ui-3d | `sg save <i>` |
| Insert shot as new board | Same, into a new board after the current one | RENDER | ui-3d | `sg insert-board <i>` |
| Render camera view | Camera view to PNG in the world's shading mode | RENDER | ui-3d | `sg render <i> [--camera <id>] --out <png>` |
| Render top-down plot | Orthographic floor plan with icons | RENDER | ui-3d | `sg render <i> --plot --out <png>` |
| Board info panel | Shows board shot, dialogue, action, notes | DIRECT | cli | `board info` |
| Undo / redo (3D) | 50-step history | DIRECT | cli | `sg undo` / `sg redo` (if CLI keeps history) |

## Shot Generator: objects and selection

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Add camera | New camera next to the active one; becomes active | DIRECT | cli | `sg camera add` |
| Add object | 1x1x1 box or a built-in/custom model in front of the camera | DIRECT | cli | `sg object add [--model <id>]` |
| Add character | Adult male 1.8 m in the default stand pose | DIRECT | cli | `sg character add [--model <id>]` |
| Add light | Spot light, intensity 0.8 | DIRECT | cli | `sg light add` |
| Add volume | 5x5x5 atmospherics volume (rain by default) | DIRECT | cli | `sg volume add [--preset rain/fog/explosion]` |
| Add image plane | Placeholder or file, copied to `models/images` | DIRECT | cli | `sg image add [--file <path>]` |
| Batch create | Create many objects from JSON | DIRECT | cli | `sg objects create --json` |
| List objects | All scene objects with type, name, transform | DIRECT | cli | `sg list <i>` |
| Delete objects | Also removes a character's attachables; active camera protected | DIRECT | cli | `sg delete <id..>` |
| Rename object | Sets name / displayName | DIRECT | cli | `sg rename <id> <name>` |
| Visibility | Show/hide (propagates to group children) | DIRECT | cli | `sg set <id> visible=false` |
| Lock / unlock | Locked objects ignore edits | DIRECT | cli | `sg lock <id>` / `sg unlock <id>` |
| Duplicate | Copies with +0.5 m offset and " copy" name; deep for groups | DIRECT | cli | `sg duplicate <id..>` |
| Group / ungroup / merge | Group handling from selection | DIRECT | cli | `sg group <id..>` / `sg ungroup <gid>` |
| Move group | Moves all children | DIRECT | cli | `sg group move <gid> --dx --dy` |
| Batch move | Set x/y/z/rotation for many ids | DIRECT | cli | `sg move --json` |
| Drop to floor | Drops objects onto the surface below (raycast, no WebGL) | PARTIAL | cli | `sg drop <id..>` |
| Select (click, shift, list, Esc) | Selection state | UI | ui | CLI addresses by id |
| Drag move in viewport / plot | Direct manipulation | UI | ui | `sg set <id> x= y=` |
| Rotation gizmo | Transform controls | UI | ui | rotate commands below |

## Shot Generator: camera

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Set camera position | x/y/z, -30..30 m | DIRECT | cli | `sg camera set <id> x= y= z=` |
| Pan | Rotation -180..180° | DIRECT | cli | `sg camera set-pan <deg>` |
| Tilt | -90..90° | DIRECT | cli | `sg camera set-tilt <deg>` |
| Roll | -45..45° | DIRECT | cli | `sg camera set-roll <deg>` |
| Field of view | 1..120° | DIRECT | cli | `sg camera set-fov <deg>` |
| Lens by focal length | 12, 16, 18, 22, 24, 35, 50, 85, 100, 120, 200, 300, 500 mm | DIRECT | cli | `sg camera set-lens <mm>` |
| Move relative to heading | Forward/right in metres | DIRECT | cli | `sg camera move --forward --right` |
| Elevate | Height change in metres | DIRECT | cli | `sg camera elevate <m>` |
| Orbit around target | Orbit a character or point by degrees | DIRECT | cli | `sg camera orbit --target <id> --deg` |
| Dolly zoom | Keep subject size while changing FOV | DIRECT | cli | `sg camera dolly-zoom --target <id> --fov` |
| Set active camera | By id or index 1–9 | DIRECT | cli | `sg camera activate <id/index>` |
| Frame by shot size | ECU, CU, MCU, medium, MLS, long, extreme long, establishing, OTS; frames a character | PARTIAL | cli | `sg camera frame --size <size> [--character <id>]` |
| Frame by camera angle | Bird's eye, high, eye, low, worm's eye | PARTIAL | cli | `sg camera frame --angle <angle>` |
| WASD fly, mouse look, orbit, truck, gamepad | Interactive camera driving | UI | ui | - |
| Swap main and plot view | T key | UI | ui | - |

## Shot Generator: characters

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Character position | x/y/z | DIRECT | cli | `sg character set <id> x= y= z=` |
| Character rotation | -180..180° | DIRECT | cli | `sg character set-rotation <id> <deg>` |
| Height | Range per model (adult 1.47–2.13 m, child, baby) | DIRECT | cli | `sg character set-height <id> <m>` |
| Head scale | 80–120 % | DIRECT | cli | `sg character set-head-scale <id> <pct>` |
| Skin tint | Color | DIRECT | cli | `sg character set-tint <id> <hex>` |
| Body shape: muscular | 0–100 % | DIRECT | cli | `sg character set-morph <id> muscular <pct>` |
| Body shape: skinny | 0–100 % | DIRECT | cli | `sg character set-morph <id> skinny <pct>` |
| Body shape: obese | 0–100 % | DIRECT | cli | `sg character set-morph <id> obese <pct>` |
| Change model | Adult/teen male/female, child, baby | DIRECT | cli | `sg character set-model <id> <model>` |
| Custom character model | Copies a .glb into the project | DIRECT | cli | `sg character set-model <id> --file <glb>` |
| Apply pose preset | One of 342 poses | DIRECT | cli | `sg character pose <id> <pose>` |
| Mirror pose | Left/right mirror of the skeleton | DIRECT | cli | `sg character mirror-pose <id>` |
| Set bone rotation | Any bone x/y/z | DIRECT | cli | `sg bone set <id> <bone> --x --y --z` |
| List bones | Skeleton bone names and rotations | DIRECT | cli | `sg bone list <id>` |
| Apply hand pose | One of 32, left/right/both | DIRECT | cli | `sg character hand-pose <id> <preset> --hand both` |
| Apply emotion (face) | Face texture preset or none | DIRECT | cli | `sg character emotion <id> <emotion/none>` |
| Hair | Default, curly, none or custom file | DIRECT | cli | `sg character hair <id> <hair>` |
| Add attachable | Glasses, mask, moustache, backpack, pistol, bound to a bone | DIRECT | cli | `sg attachable add <charId> <model>` |
| Custom attachable | File plus bind bone | DIRECT | cli | `sg attachable add <charId> --file --bone` |
| Attachable bone | Rebind to another bone | DIRECT | cli | `sg attachable set-bone <id> <bone>` |
| Attachable size | 0.7–2 | DIRECT | cli | `sg attachable set-size <id> <n>` |
| Attachable position | Offset in bone space | DIRECT | cli | `sg attachable set <id> x= y= z=` |
| Remove attachable | Delete | DIRECT | cli | `sg attachable remove <id>` |
| Apply character preset | Model, height, head, tint, morphs, attachables | PARTIAL | cli | `sg character apply-preset <id> <preset>` |
| IK posing | Drag head/hand/foot/hip targets, pole targets | UI | ui | pose via presets and bone rotations |
| FK bone gizmo | Rotate a bone with the mouse | UI | ui | `sg bone set` |

## Shot Generator: objects, lights, volumes, images

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Object position | x/y/z | DIRECT | cli | `sg object set <id> x= y= z=` |
| Object size | Box: width/height/depth; model: uniform | DIRECT | cli | `sg object set-size <id> --w --h --d` |
| Object rotation | x/y/z | DIRECT | cli | `sg object rotate <id> --x --y --z` |
| Object tint | Color | DIRECT | cli | `sg object set-tint <id> <hex>` |
| Object model | One of 47 built-ins | DIRECT | cli | `sg object set-model <id> <model>` |
| Custom object model | Copies a .glb into the project | DIRECT | cli | `sg object set-model <id> --file <glb>` |
| Light position | x/y/z | DIRECT | cli | `sg light set <id> x= y= z=` |
| Light intensity | 0.025–1 | DIRECT | cli | `sg light set-intensity <id> <v>` |
| Light cone angle | 1–90° | DIRECT | cli | `sg light set-angle <id> <deg>` |
| Light distance | 0.025–100 | DIRECT | cli | `sg light set-distance <id> <m>` |
| Light penumbra | 0–1 | DIRECT | cli | `sg light set-penumbra <id> <v>` |
| Light decay | 1–2 | DIRECT | cli | `sg light set-decay <id> <v>` |
| Light rotation / tilt | Aim the spot | DIRECT | cli | `sg light set-rotation` / `set-tilt` |
| Volume position / size / rotation | Transform | DIRECT | cli | `sg volume set <id> ...` |
| Volume preset | Rain, fog, explosion | DIRECT | cli | `sg volume set-preset <id> <preset>` |
| Volume custom images | Layer textures from files | DIRECT | cli | `sg volume set-images <id> <files..>` |
| Volume layers / opacity / color | 1–10 layers, opacity, grey | DIRECT | cli | `sg volume set <id> layers= opacity= color=` |
| Image plane file | Replace texture | DIRECT | cli | `sg image set-file <id> <path>` |
| Image plane transform | Position, scale, rotation | DIRECT | cli | `sg image set <id> ...` |
| Image plane opacity | 0.1–1 | DIRECT | cli | `sg image set-opacity <id> <v>` |
| Image visible to camera | Hide from the saved shot | DIRECT | cli | `sg image set <id> visible-to-camera=false` |

## Shot Generator: world

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Ground on/off | Ground plane | DIRECT | cli | `sg world set ground=true` |
| Background grey | 0..1 | DIRECT | cli | `sg world set-bg <v>` |
| Shading mode | Outline, Wireframe, Flat, Depth | DIRECT | cli | `sg world set-shading <mode>` |
| Room | Visible, width, length, height | DIRECT | cli | `sg room set --visible --w --l --h` |
| Environment model | Import a .glb set/location | DIRECT | cli | `sg env set-file <glb>` |
| Environment transform | Position, scale, rotation | DIRECT | cli | `sg env set x= y= z= scale= rotation=` |
| Environment visible / grayscale | Toggles | DIRECT | cli | `sg env set visible= grayscale=` |
| Ambient light | Intensity 0–1 | DIRECT | cli | `sg world set-ambient <v>` |
| Sun (directional light) | Intensity, rotation, tilt | DIRECT | cli | `sg world set-sun --intensity --rotation --tilt` |
| Fog | On/off and distance | DIRECT | cli | `sg world set-fog --visible --far <m>` |

## Shot Generator: presets and libraries

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| List models | Characters (6), objects (47), attachables (7), with search | DIRECT | cli | `sg models list --type <t> [--q]` |
| List poses | 342 poses with search | DIRECT | cli | `sg poses list [--q]` |
| List hand poses | 32 hand poses | DIRECT | cli | `sg hand-poses list` |
| List emotions | 6 built-in + custom | DIRECT | cli | `sg emotions list` |
| Create pose preset | Saves a character's skeleton (no network call) | DIRECT | cli | `sg preset pose create <charId> <name>` |
| Delete pose preset | User pose removal | DIRECT | cli | `sg preset pose delete <id>` |
| Rename pose preset | Action exists, no UI | DIRECT | cli | `sg preset pose rename <id> <name>` |
| Create hand-pose preset | From one hand (no network call) | DIRECT | cli | `sg preset hand-pose create <charId> <name> --hand` |
| Delete hand-pose preset |  | DIRECT | cli | `sg preset hand-pose delete <id>` |
| Create character preset | Saves look and attachables | DIRECT | cli | `sg preset character create <charId> <name>` |
| Create emotion from image | Face texture preset (thumbnail needs a render) | PARTIAL | cli | `sg preset emotion create <name> --image <png>` |
| Delete emotion preset | Removes files and clears references | DIRECT | cli | `sg preset emotion delete <id>` |
| Scene presets | Create/rename/delete/apply; storage exists, no UI | DIRECT | cli | `sg preset scene create/apply/delete` |
| Render pose thumbnails | Thumbnail per preset | RENDER | ui-3d | `sg poses thumbnail` |

## Shot Explorer and shot list

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Generate camera angles | Random size, angle, character and lens using composition rules (orbit, thirds, roll) | PARTIAL | cli | `sg explore <i> --count N [--seed]` |
| Render explorer thumbnails | Outline render per generated shot | RENDER | ui-3d | `sg explore <i> --render <dir>` |
| Add generated shot as camera | Inserts a camera from a generated shot | DIRECT | cli | `sg camera add --from-shot <n>` |
| Shot description | "Medium close-up, low angle on Character 2, 35mm" | DIRECT | cli | output of `sg explore` |
| Preview tween, infinite scroll | Browsing UI | UI | ui | - |
| Shot list (scene) | Groups boards into camera setups with lens, height, angles | DIRECT | cli | `shotlist <scene>` |
| Shot list (script project) | Shot list across all scenes | DIRECT | cli | `shotlist --project <script>` |

## Preferences

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Get / set any pref | Read or write `pref.json` keys | DIRECT | cli | `prefs get/set <key> <value>` |
| Tooltips | enableTooltips | DIRECT | cli | `prefs set enableTooltips` |
| Autosave | enableAutoSave | DIRECT | cli | `prefs set enableAutoSave` |
| Default board timing | defaultBoardTiming (2000 ms) | DIRECT | cli | `prefs set defaultBoardTiming` |
| Default fps for new scenes | lastUsedFps (24) | DIRECT | cli | `prefs set lastUsedFps` |
| Straight-line delay | straightLineDelayInMsecs (650) | DIRECT | cli | `prefs set straightLineDelayInMsecs` |
| High-quality drawing engine | enableHighQualityDrawingEngine | DIRECT | cli | `prefs set enableHighQualityDrawingEngine` |
| Drawing sounds | enableDrawingSoundEffects | DIRECT | cli | `prefs set enableDrawingSoundEffects` |
| Drawing melodies | enableDrawingMelodySoundEffects | DIRECT | cli | `prefs set enableDrawingMelodySoundEffects` |
| UI sounds | enableUISoundEffects | DIRECT | cli | `prefs set enableUISoundEffects` |
| High-quality audio | enableHighQualityAudio | DIRECT | cli | `prefs set enableHighQualityAudio` |
| Board audio audition | enableBoardAudition | DIRECT | cli | `prefs set enableBoardAudition` |
| Notifications | enableNotifications | DIRECT | cli | `prefs set enableNotifications` |
| Motivational messages | enableAspirationalMessages | DIRECT | cli | `prefs set enableAspirationalMessages` |
| Line-mileage notifications | allowNotificationsForLineMileage | DIRECT | cli | `prefs set allowNotificationsForLineMileage` |
| Diagnostics | enableDiagnostics | DIRECT | cli | `prefs set enableDiagnostics` |
| Sprint minutes | pomodoroTimerMinutes (25) | DIRECT | cli | `prefs set pomodoroTimerMinutes` |
| Toolbar tool settings | Color, palette, opacity per tool; captions | DIRECT | cli | `prefs set-tool` |
| Deprecated prefs | enableCanvasPaintingOpacity, enableStabilizer, enableBrushCursor, importTargetLayer (no effect) | DIRECT | cli | - |
| Slow-CPU defaults | Turns off sounds on 2-core or 2 GHz machines | DIRECT | cli | - |
| Prefs migration | Merges new defaults on version change | DIRECT | cli | `prefs migrate` |

## Keymap and language

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| List key bindings | 87 default bindings merged with user keymap | DIRECT | cli | `keymap list` |
| Set key binding | Edit `keymap.json` | DIRECT | cli | `keymap set <command> <accelerator>` |
| Keymap migration | Migrates 1.5.x / 1.7.1 bindings | DIRECT | cli | `keymap migrate` |
| Key tester window | Visual keyboard with live key test | UI | ui | - |
| List languages | en-US, ru-RU, zh-CN plus custom | DIRECT | cli | `lang list` |
| Set language | Hot-swaps UI language | DIRECT | cli | `lang set <code>` |
| Duplicate language | Copy a locale for editing | DIRECT | cli | `lang copy <code>` |
| Edit locale string | Change one key | DIRECT | cli | `lang edit <code> <key> <value>` |
| Import / export locale JSON |  | DIRECT | cli | `lang import <file>` / `lang export <code> <dir>` |
| Remove custom language |  | DIRECT | cli | `lang remove <code>` |

## App shell and system

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| ffmpeg check | Runs bundled ffmpeg `-version` | DIRECT | cli | `doctor` |
| Log file location | electron-log path | DIRECT | cli | `app log-path` |
| Help links | Website, FAQ, GitHub issues | DIRECT | cli | `help urls` |
| Welcome window | New, open, recents, version | UI | ui | - |
| New-project window | Script vs blank, aspect ratio examples | UI | ui | `project new` |
| Preferences window | Form over pref.json | UI | ui | `prefs set` |
| Unsaved-changes prompt | On close | UI | ui | - |
| Loading-status window | Project load progress | UI | ui | - |
| Error dialog | Renderer errors | UI | ui | - |
| App menu per window | Menu templates, roles, language re-render | UI | ui | - |

## Productivity and fun

| Feature | What it does | Fit | Owner | Command |
|---|---|---|---|---|
| Sketch sprint timer | Pomodoro 1–500 min | UI | ui | - |
| Sprint timelapse GIF | Records the canvas during a sprint | UI | ui | - |
| List timelapses | Last recordings | DIRECT | cli | `timelapse list` |
| Story tip | Random writing tip | DIRECT | cli | `tip` |
| Drawing sonifier | Sound driven by pen speed and pressure | UI | ui | - |
| Motivational messages | Hourly quotes | UI | ui | - |
| Line-mileage jokes | Messages at mileage milestones | UI | ui | - |
| Slow FPS warning | Suggests efficiency mode | UI | ui | - |
| Tweet sprint | Twitter intent link | UI | ui | - |
## Data model (must stay compatible with existing .storyboarder projects)
The CLI would operate on these files. No database, no server.

- **Scene file** `<name>.storyboarder`: `version`, `aspectRatio`, `fps`, `defaultBoardTiming`, `boards[]`.
- **Board**: `uid` (5 chars), `url` (`board-N-UID.png`, base name only), `newShot`, `lastEdited`, `layers{shot-generator, reference, fill, tone, pencil, ink, notes}` each `{url, opacity, thumbnail}`, `duration`, `dialogue`, `action`, `notes`, `audio{filename, duration}`, `link` (PSD), `lineMileage`, `shot`, `number`, `time`, `sg{version, data}`.
- **3D scene** `board.sg.data`: `world` (ground, background, room, environment, ambient, directional, fog, shadingMode), `activeCamera`, `sceneObjects{id → camera, object, character, light, volume, image, attachable, group}`.
- **images/**: `board-N-UID-<layer>.png`, `-thumbnail.png`, `-posterframe.jpg`, `-shot-generator-thumbnail.jpg`, `-camera-plot.png`, `.psd`, `<uid>-*.wav`.
- **Script project**: `storyboards/storyboard.settings` plus one `Scene-<n>-<slug>-<id>/` folder per scene.
- **userData**: `pref.json`, `keymap.json`, `presets/{scenes, characters, poses, hand-poses, emotions}.json`, `locales/`, `watermark.png`.
