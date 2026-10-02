# The drawing loop (for Claude)

Claude draws through `sb` alone: write drawing instructions, render, look at
the PNG, adjust, repeat. Every command below takes `--json` and reports the
files it wrote.

## 0. Know the canvas

```sh
sb project info --json            # boards: aspect -> size is 900 px high x aspect (16:9 = 1600x900)
sb page info 1 --json             # manga page: size (default 1414x2000) and panel boxes
```

Coordinates are pixels, origin top-left, y down. With `--page <p> --panel <id>`
they are panel-local: `0,0` is the top-left of the panel's bounding box, and
everything is clipped to the panel shape.

## 1. See where things go

```sh
sb board render 1 --grid --out /tmp/b1.png          # labelled 100 px grid
sb page render 1 --grid --out /tmp/p1.png           # grid + panel tags "K2 #1" (id, reading order)
```

Zoom in to check details or place strokes precisely: `--crop x,y,w,h` (board/page
px) and `--scale f`; `page render --panel K2` crops to that panel's box. Grid
labels stay in board/page coordinates, so numbers read off a zoomed render go
straight into draw commands (panel-local = page minus the panel box origin,
reported as `crop` in the JSON).

```sh
sb page render 1 --panel K2 --grid --grid-step 50 --out /tmp/k2.png
sb page render 1 --crop 700,650,400,300 --scale 2 --grid --out /tmp/zoom.png
sb board render 1 --crop 600,300,400,300 --scale 1.5 --grid --out /tmp/b1z.png
```

Read the PNG. Use the grid numbers to place the next strokes.

## 2. Draw

Rough shapes and composition: SVG (paths, beziers, arcs, rect/circle/ellipse,
lines, polylines, polygons, groups with transforms, text). No `width`/`height`/
`viewBox` means SVG units are pixels.

```sh
sb draw svg 1 --layer pencil - <<'EOF'
<svg xmlns="http://www.w3.org/2000/svg">
  <ellipse cx="800" cy="300" rx="90" ry="110" fill="none" stroke="#333" stroke-width="4"/>
  <path d="M 800 410 C 780 520, 820 600, 800 700" fill="none" stroke="#333" stroke-width="4" stroke-linecap="round"/>
</svg>
EOF
```

Hand-drawn lines: pressure strokes. Width tapers with the third value (0-1);
points are smoothed. Tools carry the original app's defaults and pick the layer:
`pencil` (4 px, grain, pencil layer), `pen` (2 px black, ink), `light-pencil`
(20 px #90CBF9, reference), `brush` (26 px, fill), `tone` (50 px at 0.15, tone),
`note-pen` (8 px red, notes), `eraser`.

```sh
sb draw strokes 1 --tool pen - <<'EOF'
{"strokes": [
  {"points": [[700,300,0.2],[760,250,0.8],[840,250,1],[900,300,0.3]]},
  {"size": 3, "points": [[780,330,0.5],[800,340,1],[820,330,0.5]]}
]}
EOF
```

Curves you can describe as SVG paths but want to look inked: `draw path` turns
each `<path d>` (lines, beziers, arcs; one stroke per subpath) into a pressure
stroke with tapered ends, using the same tools and rasterizer as `draw strokes`.

```sh
sb draw path --page 1 --panel K1 --tool pen --size 8 --taper both - <<'EOF'
<svg>
  <path d="M 100 300 C 250 50, 450 50, 600 300"/>
  <path data-taper="start" stroke-width="12" d="M 700 350 A 150 80 0 0 1 1100 350"/>
</svg>
EOF
```

`--taper both|start|end|none`, `--min-pressure 0.15` (pressure at a tapered end;
it reaches 1 after 30% of the length). Per path: `data-taper`,
`data-min-pressure`, `stroke-width` (size), `stroke="#rrggbb"`. A bare `d`
string works too: `echo 'M 10 10 Q 60 0 110 40' | sb draw path 1 -`.

Screentone: `draw tone` fills a `--rect x,y,w,h` or `--polygon "x,y ..."` with
dots, lines or crosshatch on the `tone` layer (clipped to the panel with
`--panel`). `--density` is the share covered by ink; `--gradient x1,y1,x2,y2`
ramps it to `--density-to` (skies, fades).

```sh
sb draw tone --page 1 --panel K2 --rect 0,0,1200,560 --pattern dots --spacing 10 \
  --density 0.6 --gradient 0,0,0,450 --density-to 0          # sky fading down
sb draw tone --page 1 --panel K3 --polygon "50,50 500,80 400,480 30,400" \
  --pattern lines --spacing 6 --density 0.4 --angle 30       # shadow
```

Figure reference: `pose draw` projects one of the 342 pose presets (forward
kinematics on the character skeleton) and draws a light-blue mannequin — tapered
bones, joint dots, head ellipse with eyes, far-side limbs lighter — on the
`reference` layer to trace over. `--x/--y` is the floor point under the figure,
`--height` its standing height in px. The JSON lists every joint's position
(`joints`, area-local), handy for aiming hands, eyes and balloon tails.

```sh
sb pose list --q run                                  # "232  Run leaning forward", ...
sb pose draw 232 --page 1 --panel K2 --view 3q --height 480 --x 300 --y 520
sb pose draw "stand" 1 --view side --model adult-female --preview /tmp/b1.png
```

Labels: `sb draw text 1 --x 60 --y 40 --size 40 --font bold "INT. KITCHEN"`.
Corrections: `sb draw erase 1 --layer pencil --rect 600,200,300,300`
(or `--polygon "x,y x,y x,y"`, `--all` for every layer).

Add `--preview /tmp/b1.png --grid` to any draw command to get the render in the
same call.

Manga pages: the same commands with `--page <p>` (and `--panel <id>`):

```sh
sb page template 1 3-tier
sb draw strokes --page 1 --panel K1 --tool pencil strokes.json --preview /tmp/p1.png --grid
sb balloon add 1 "Where are you going?" --panel K1 --tail 420,300
```

Or draw a board (any canvas size: `sb board add --size 900x1600`) and show it in a
panel: `sb panel place 1 K2 3 --fit fill --scale 1.2 --x -40`.
`sb board add --like-panel 1:K2 --place` makes a board with that panel's aspect
(long side 1800 px) and shows it there in one step.

Aim at placed art with `panel map`: it converts between the placed board's
pixels, page pixels and panel-local coordinates (honoring fit, scale, offset,
rotation):

```sh
sb panel map 1 K2 --board-point 640,210 --json   # where the character's mouth is on the page
# -> {"page":[..],"panelLocal":[..],"boardPoint":[640,210],"insidePanel":true,...}
sb balloon add 1 "Hey!" --panel K2 --tail <panelLocal x>,<panelLocal y>
sb panel map 1 K2 --page-point 900,520           # the reverse: which board pixel is under a page point
```

## 3. Look

```sh
sb board render 1 --out /tmp/b1.png                 # the composite
sb board render 1 --layer ink --out /tmp/ink.png    # one layer on white
sb page render 1 --out /tmp/p1.png
sb spread render 2 --out /tmp/s.png                 # facing pages in reading order
sb project contact-sheet --out /tmp/all.png         # every board with number, shot, dialogue
sb pages contact-sheet --out /tmp/pages.png         # every page
```

Critique against the goal: silhouette readable? Eye line and panel flow right?
Balloons in reading order (right to left, top to bottom for rtl)?

## 4. Adjust or go back

```sh
sb layer history 1 pencil        # kept versions (newest first)
sb layer undo 1 pencil           # restore the previous version (repeat to go further)
sb layer redo 1 pencil           # re-apply what undo removed (until the next edit)
sb layer undo --page 1 ink
sb draw svg 1 --layer pencil --replace new.svg      # redraw the layer from scratch
```

Each layer keeps its last 20 versions in `images/.history/` (redo list in
`.history/<layer>/redo/`, cleared by any new edit). Page frames and balloons are data: change them
with `panel set/split/merge` and `balloon set`, never by drawing.

## Tips

- Block in with `light-pencil` on the reference layer, then refine with `pencil`,
  then `pen` on ink. Tone with `tone`.
- Keep strokes short (10–40 points); vary pressure at the ends for a natural taper.
- Render after every few strokes; small corrections beat big rewrites.
