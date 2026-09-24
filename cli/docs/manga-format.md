# Manga projects: file format

A manga project is a normal `.storyboarder` file with three extra root keys.
Everything else (boards, layers, images/) is unchanged, so the original app still
opens the file and shows the boards; it ignores `mode`, `page` and `pages`.

```json
{
  "version": "3.0.0",
  "aspectRatio": 1.7777777777777777,
  "fps": 24,
  "defaultBoardTiming": 2000,
  "boards": [ ...free drawings, as in any project... ],
  "mode": "manga",
  "page": { "width": 1414, "height": 2000, "readingDirection": "rtl", "firstPageSingle": true },
  "pages": [
    {
      "id": "7BUNG",
      "url": "page-7BUNG.png",
      "panelOrder": "auto",
      "panels": [
        { "id": "K1", "points": [[113,140],[1301,140],[1301,681],[113,681]],
          "border": 6, "bleed": false, "order": 1,
          "content": { "board": "Q9UBB", "x": 0, "y": 0, "scale": 1, "rotation": 0, "fit": "fill" } }
      ],
      "balloons": [
        { "id": "B1", "type": "speech", "text": "Where are you going?", "fontSize": 32,
          "vertical": false, "x": 930, "y": 160, "w": 351, "h": 148,
          "tail": [413, 390], "panel": "K1" }
      ],
      "layers": {
        "pencil":   { "url": "page-7BUNG-pencil.png" },
        "frames":   { "url": "page-7BUNG-frames.png" },
        "balloons": { "url": "page-7BUNG-balloons.png" }
      }
    }
  ]
}
```

## Boards

Boards are free drawings, independent of pages, in `boards[]` as in any
project (the original app opens them as a board sequence). A board may have
its own canvas, `"size": {"width": 900, "height": 2400}` (`sb board add --size
900x2400`); without it the canvas is 900 px high x the project `aspectRatio`.
Film exports (GIF, video) scale every board to the project size. Boards may
also carry `"name"` and `"description"` (the original app ignores both).

Layer `opacity` is honored on every drawing layer (default 1, reference 0.75);
the original app only reads it on reference. Setting it on a layer that has no
file yet writes a blank PNG so the entry always has an image.

## Pages

- Size in pixels (`page.width` x `page.height`, default 1414x2000 = 1:1.414).
  `sb page setup --size` rescales every panel, balloon and content offset.
- `readingDirection`: `rtl` (default) or `ltr`.
- Spreads: with `firstPageSingle` (default true) page 1 stands alone (a left page
  in rtl, a right page in ltr) and then pages pair up: rtl `[3|2]`, `[5|4]`
  (the even page is on the right), ltr `[2|3]`, `[4|5]`. With it false, spreads
  start at page 1.
- Files in `images/`: `page-<id>-<layer>.png` for page drawing layers
  (reference, fill, tone, pencil, ink, notes: art that crosses panel borders),
  `-frames.png` and `-balloons.png` (derived, see below), `-thumbnail.png`
  (120 px high) and `-posterframe.jpg` (full page composite).

## Panels (koma)

- `points`: polygon in page pixels (rectangles are 4 points, clockwise from top-left).
- `border`: border width in px (0 = none). `bleed`: vertices within the page
  margin snap to the page edge, and edges on the page edge get no border.
- `order`: reading order, 1-based. With `panelOrder: "auto"` it is recomputed on
  every change: panels are grouped into rows top to bottom (a panel joins a row
  when its vertical center lies inside the row's first panel), then sorted right
  to left (rtl, by right edge) or left to right (ltr, by left edge). `sb panel
  order <p> <ids..>` sets `panelOrder: "manual"`.
- Templates use margins of 8 % (sides) and 7 % (top/bottom) of the page and
  gutters of 1.7 % of the width between columns and 2.4 % of the height between
  tiers (`--gutter` overrides both).
- Splits clip the polygon with two parallel half-planes `gutter` apart; the
  first part keeps the id and placement, the second gets the next `K<n>`. Merge
  replaces two panels with the convex hull of their points.
- `content`: `null`, or a board shown in the panel. The board's flattened
  drawing is centered on the panel's bounding-box center plus (`x`, `y`) px,
  sized by `fit` (`fill`: cover the box, `fit`: inside the box, `none`: one board
  pixel per page pixel) times `scale`, rotated by `rotation` degrees, and clipped
  to the panel shape. The same board can appear in several panels
  (`sb panel place`, `sb panel clear-content`, `sb board add --like-panel <p>:<panel>
  [--place]`). `sb panel map` converts points between board, page and panel
  coordinates with the same transform.

## Balloons

- `type`: speech (ellipse + triangular tail), thought (cloud + bubbles toward the
  tail), shout (spiky burst), whisper (dashed ellipse), narration (rectangle, no
  tail), sfx (bold letters with a white outline, no shape).
- `x, y, w, h`: the bounding box of the shape in page pixels; `tail`: the point the tail aims at.
  `panel` records the panel the balloon was placed for (commands with `--panel`
  take panel-local coordinates and convert them).
- Text is centered in the box (wrapped to 72 % of the width in ellipses, 88 %
  in narration boxes). `vertical: true` sets columns top to bottom, right to
  left. Glyphs THICCCBOI lacks (Japanese, etc.) use the system fallback font
  (`SB_FALLBACK_FONT`).

## Derived layers

`frames` (panel borders) and `balloons` are rendered from the data after every
change and stored as PNGs so the app can show them without re-rendering. Never
edit those files: they are overwritten. Page composite order: white paper,
placed boards (clipped to their panels, in reading order), page layers
reference, fill, tone, pencil, ink, then frames, balloons, and notes on top.

On boards, `frames` and `balloons` are not used; board composites keep the
original seven-layer order.
