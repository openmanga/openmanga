package main

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sb/internal/draw"
	"sb/internal/manga"
	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/story"
)

var targetFlags = []string{
	"layer=layer to draw on (reference, fill, tone, pencil, ink, notes)",
	"page=draw on a manga page instead of a board (page number or id)",
	"panel=with --page: panel id or reading number; coordinates become panel-local (0,0 = panel box top-left) and drawing is clipped to the panel",
	"preview=also write a flattened PNG of the board/page here",
	"grid show the coordinate grid (and panel tags) in --preview",
}

// drawTarget is a board or a manga page (optionally one panel of it).
type drawTarget struct {
	s      *story.Scene
	obj    *ojson.Object
	isPage bool
	index  int
	w, h   int
	area   draw.Area
	rest   []string // positional args after the board
}

func (t *drawTarget) label() string {
	if t.isPage {
		return fmt.Sprintf("page %d", t.index+1)
	}
	return fmt.Sprintf("board %d", t.index+1)
}

// resolveTarget reads `<board> ...` or `--page <p> [--panel <id>] ...`.
func resolveTarget(c *Ctx) (*drawTarget, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	t := &drawTarget{s: s}
	if p := c.Flag("page"); p != "" {
		if err := manga.RequireManga(s); err != nil {
			return nil, err
		}
		i, pg, err := manga.FindPage(s, p)
		if err != nil {
			return nil, err
		}
		t.obj, t.isPage, t.index, t.rest = pg, true, i, c.Pos
	} else {
		if c.Flag("panel") != "" {
			return nil, usagef("--panel needs --page")
		}
		if len(c.Pos) == 0 {
			return nil, usagef("which board? give a board number (or --page for a manga page)")
		}
		i, b, err := oneBoard(s, c.Pos[0])
		if err != nil {
			return nil, err
		}
		t.obj, t.index, t.rest = b, i, c.Pos[1:]
	}
	t.w, t.h = story.SizeOf(s, t.obj)
	t.area = draw.FullArea(t.w, t.h)
	if id := c.Flag("panel"); id != "" {
		p, err := manga.FindPanel(t.obj, id)
		if err != nil {
			return nil, err
		}
		box, clip := manga.PanelArea(s, p)
		t.area = draw.Area{X: box.X, Y: box.Y, W: box.W, H: box.H, Clip: clip}
	}
	return t, nil
}

func (t *drawTarget) checkLayer(name string) error {
	if t.isPage {
		for _, l := range manga.PageLayers {
			if l == name {
				return nil
			}
		}
		return usagef("page layers: %s", strings.Join(manga.PageLayers, ", "))
	}
	if !story.ValidLayer(name) {
		return usagef("board layers: %s", strings.Join(story.DrawingLayers, ", "))
	}
	return nil
}

// write composites an overlay into a layer and refreshes the art.
func (t *drawTarget) write(c *Ctx, layer string, overlay *image.RGBA, erase, replace bool) (map[string]any, error) {
	img := story.LoadLayer(t.s, t.obj, layer)
	if replace {
		draw.ClearArea(img, t.area)
	}
	draw.Apply(img, overlay, t.area, erase)
	if err := story.SaveLayer(t.s, t.obj, layer, img); err != nil {
		return nil, err
	}
	if _, err := story.RefreshArt(t.s, t.obj); err != nil {
		return nil, err
	}
	if err := t.s.Save(); err != nil {
		return nil, err
	}
	return t.result(c, layer)
}

func (t *drawTarget) result(c *Ctx, layer string) (map[string]any, error) {
	res := map[string]any{
		"target":      t.label(),
		"layer":       layer,
		"file":        t.s.ImagePath(story.Layer(t.obj, layer).Str("url")),
		"posterframe": t.s.ImagePath(story.PosterframeFile(t.obj)),
		"thumbnail":   t.s.ImagePath(story.ThumbnailFile(t.obj)),
		"history":     len(story.LayerHistory(t.s, t.obj, layer)),
		"size":        []int{t.w, t.h},
	}
	if t.area.Clip != nil {
		res["panelBox"] = []float64{t.area.X, t.area.Y, t.area.W, t.area.H}
	}
	c.Printf("Drew on %s layer %s: %s", t.label(), layer, res["file"])
	if out := c.Flag("preview"); out != "" {
		p, err := renderTarget(t, out, c.Bool("grid"), 100, "")
		if err != nil {
			return nil, err
		}
		res["preview"] = p
		c.Printf("\nPreview: %s", p)
	}
	return res, nil
}

// renderTarget writes a flattened (or single-layer) PNG of a board or page.
func renderTarget(t *drawTarget, out string, grid bool, step float64, layer string) (string, error) {
	var img *image.RGBA
	switch {
	case layer != "":
		img = render.White(t.w, t.h)
		render.DrawOver(img, image.Point{}, story.LoadLayer(t.s, t.obj, layer), 1)
	case t.isPage:
		img, _ = manga.Compose(t.s, t.obj)
	default:
		img, _ = story.Flatten(t.s, t.obj, t.w, t.h)
	}
	if grid {
		if t.isPage {
			manga.Guides(img, t.s, t.obj, step)
		} else {
			render.Grid(img, step, 1)
		}
	}
	abs, _ := filepath.Abs(out)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	return abs, render.SavePNG(abs, img)
}

func readInput(arg string) ([]byte, error) {
	if arg == "" || arg == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(arg)
}

func init() {
	register(
		&Cmd{Path: "draw svg", Args: "[board] <file.svg|->", Short: "Rasterize SVG onto a layer (paths, beziers, arcs, shapes, strokes, fills, groups/transforms, text)",
			Long:  "Without width/height/viewBox on <svg>, SVG units are layer pixels (panel-local with --panel).\nWith them, the SVG is scaled so its width fills the board/panel width.\nBoards are 900 px high x aspect; manga pages default to 1414x2000.",
			Flags: append([]string{"replace clear the layer (or the panel area) first"}, targetFlags...), Run: cmdDrawSVG},
		&Cmd{Path: "draw strokes", Args: "[board] <file.json|->", Short: "Hand-drawn strokes: point lists [x,y,pressure] with width tapering by pressure",
			Long:  `JSON: {"tool":"pencil","color":"#222","size":4,"opacity":0.5,"strokes":[{"points":[[x,y,p],...]}]}` + "\nor a list of strokes, or a list of point lists. Points are smoothed (Catmull-Rom) unless \"smooth\": false.\nTools: pencil (4 px, grain), pen (2 px black), light-pencil (20 px #90CBF9), brush (26 px), tone (50 px at 0.15),\nnote-pen (8 px red), eraser. The tool picks the default layer.",
			Flags: append([]string{"tool=pencil, pen, light-pencil, brush, tone, note-pen or eraser (default pencil)", "replace clear the layer (or the panel area) first"}, targetFlags...), Run: cmdDrawStrokes},
		&Cmd{Path: "draw path", Args: "[board] <file.svg|->", Short: "SVG path(s) as tapered pressure strokes, drawn like draw strokes",
			Long:  "Every <path d> (or a bare path \"d\" string) becomes one stroke per subpath; lines, beziers and arcs are\nsampled every 2 px. Pressure rises from --min-pressure at a tapered end to 1 over 30% of the length.\nPer-path attributes: data-taper, data-min-pressure, stroke-width (size), stroke=\"#rrggbb\" (color).\nCoordinates are layer pixels (panel-local with --panel); transform attributes are not applied.",
			Flags: append([]string{"tool=pencil, pen, light-pencil, brush, tone, note-pen or eraser (default pencil)", "taper=both, start, end or none (default both)", "min-pressure=pressure at a tapered end, 0-1 (default 0.15)", "size=stroke width px (default: the tool's)", "color=#rrggbb (default: the tool's)", "replace clear the layer (or the panel area) first"}, targetFlags...), Run: cmdDrawPath},
		&Cmd{Path: "draw tone", Args: "[board]", Short: "Screentone fill (dots, lines, crosshatch) of a rectangle or polygon, optionally a density gradient",
			Long:  "Density is the share of the area covered by ink (0.3 = 30% grey). With --gradient x1,y1,x2,y2 the density goes from\n--density at x1,y1 to --density-to at x2,y2 (skies: dense at the top, fading down). Clipped to the panel with --panel.",
			Flags: append([]string{"rect=x,y,w,h", `polygon=points "x,y x,y x,y"`, "pattern=dots, lines or crosshatch (default dots)", "spacing=px between dots/lines (default 8)", "density=ink coverage 0-1 (default 0.3)", "angle=pattern angle in degrees (default 45)", "gradient=x1,y1,x2,y2: ramp density from the first point to the second", "density-to=density at the gradient end (default 0)", "color=#rrggbb (default black)", "replace clear the layer (or the panel area) first"}, targetFlags...), Run: cmdDrawTone},
		&Cmd{Path: "draw text", Args: "[board] <text>", Short: "Text label at x,y (top-left of the first line; center/right align around x)",
			Flags: append([]string{"x=x in px", "y=y in px", "size=font size px (default 32)", "font=thin, light, regular or bold", "color=#rrggbb (default black)", "align=left, center or right", "width=wrap width px", "vertical top-to-bottom columns, right to left"}, targetFlags...), Run: cmdDrawText},
		&Cmd{Path: "draw erase", Args: "[board]", Short: "Erase a rectangle or polygon on a layer (all drawing layers with --all)",
			Flags: append([]string{"rect=x,y,w,h", `polygon=points "x,y x,y x,y"`, "all erase on every drawing layer"}, targetFlags...), Run: cmdDrawErase},

		&Cmd{Path: "layer history", Args: "[board] <layer>", Short: "Kept previous versions of a layer (newest first; images/.history, 20 per layer)", Flags: []string{"page=manga page instead of a board"}, Run: cmdLayerHistory},
		&Cmd{Path: "layer undo", Args: "[board] <layer>", Short: "Restore the previous version of a layer (repeatable; the undone version can be redone)", Flags: []string{"page=manga page instead of a board"}, Run: cmdLayerUndo},
		&Cmd{Path: "layer redo", Args: "[board] <layer>", Short: "Re-apply the last undone version of a layer (until a new edit)", Flags: []string{"page=manga page instead of a board"}, Run: cmdLayerRedo},

		&Cmd{Path: "project contact-sheet", Short: "All boards in one PNG with number, shot and dialogue, for reviewing a sequence",
			Flags: []string{"out=output PNG (required)", "boards=boards to include (default all)", "cols=columns (default 4)", "width=cell width px (default 400)"}, Run: cmdContactSheet},
	)
}

func overlayFor(t *drawTarget) *image.RGBA { return render.New(t.w, t.h) }

func cmdDrawSVG(c *Ctx) (any, error) {
	t, err := resolveTarget(c)
	if err != nil {
		return nil, err
	}
	layer := c.Flag("layer")
	if layer == "" {
		layer = "ink"
	}
	if err := t.checkLayer(layer); err != nil {
		return nil, err
	}
	data, err := readInput(firstOr(t.rest, "-"))
	if err != nil {
		return nil, err
	}
	overlay, err := draw.SVG(data, t.w, t.h, t.area)
	if err != nil {
		return nil, err
	}
	return t.write(c, layer, overlay, false, c.Bool("replace"))
}

func firstOr(a []string, def string) string {
	if len(a) > 0 {
		return a[0]
	}
	return def
}

func cmdDrawStrokes(c *Ctx) (any, error) {
	t, err := resolveTarget(c)
	if err != nil {
		return nil, err
	}
	data, err := readInput(firstOr(t.rest, "-"))
	if err != nil {
		return nil, err
	}
	doc, err := draw.ParseStrokes(data)
	if err != nil {
		return nil, usagef("%v", err)
	}
	return drawStrokeDoc(c, t, doc)
}

func cmdDrawPath(c *Ctx) (any, error) {
	t, err := resolveTarget(c)
	if err != nil {
		return nil, err
	}
	data, err := readInput(firstOr(t.rest, "-"))
	if err != nil {
		return nil, err
	}
	spec := draw.PathSpec{Taper: c.Flag("taper"), MinPressure: 0.15}
	if spec.Taper == "" {
		spec.Taper = "both"
	}
	doc := draw.StrokeDoc{Color: c.Flag("color"), Smooth: new(bool)}
	for name, dst := range map[string]*float64{"min-pressure": &spec.MinPressure, "size": &doc.Size} {
		if f, ok, err := c.Float(name); err != nil {
			return nil, err
		} else if ok {
			*dst = f
		}
	}
	if doc.Strokes, err = draw.PathStrokes(data, spec); err != nil {
		return nil, usagef("%v", err)
	}
	return drawStrokeDoc(c, t, doc)
}

// drawStrokeDoc rasterizes strokes with the --tool (or document) tool defaults
// and writes them to --layer (default: the tool's layer).
func drawStrokeDoc(c *Ctx, t *drawTarget, doc draw.StrokeDoc) (any, error) {
	name := c.Flag("tool")
	if name == "" {
		name = doc.Tool
	}
	if name == "" {
		name = "pencil"
	}
	tool, ok := draw.Tools[name]
	if !ok {
		return nil, usagef("unknown tool %q", name)
	}
	// prefs toolbar colors/opacity override the tool defaults, like the app
	if p, err := story.LoadPrefs(); err == nil {
		if v, ok := story.PrefsGetPath(p, "toolbar.tools."+name+".color"); ok {
			if n, ok := ojson.Num(v); ok {
				tool.Color = color.RGBA{uint8(int(n) >> 16), uint8(int(n) >> 8), uint8(n), 255}
			}
		}
	}
	overlay, err := draw.Strokes(t.w, t.h, t.area, doc, tool)
	if err != nil {
		return nil, usagef("%v", err)
	}
	layer := c.Flag("layer")
	if tool.Erase {
		layers := []string{layer}
		if layer == "" {
			layers = story.OrderedLayers(t.obj)
		}
		var res map[string]any
		for _, l := range layers {
			if l == "frames" || l == "balloons" || l == "shot-generator" {
				continue
			}
			o := render.ToRGBA(overlay)
			if res, err = t.write(c, l, o, true, false); err != nil {
				return nil, err
			}
		}
		if res == nil {
			return nil, fmt.Errorf("nothing to erase: %s has no layers", t.label())
		}
		return res, nil
	}
	if layer == "" {
		layer = tool.Layer
	}
	if err := t.checkLayer(layer); err != nil {
		return nil, err
	}
	return t.write(c, layer, overlay, false, c.Bool("replace"))
}

func cmdDrawText(c *Ctx) (any, error) {
	t, err := resolveTarget(c)
	if err != nil {
		return nil, err
	}
	if len(t.rest) == 0 {
		return nil, usagef("give the text to draw")
	}
	layer := c.Flag("layer")
	if layer == "" {
		layer = "ink"
	}
	if err := t.checkLayer(layer); err != nil {
		return nil, err
	}
	spec := draw.TextSpec{Text: strings.ReplaceAll(strings.Join(t.rest, " "), `\n`, "\n"), Font: c.Flag("font"), Align: c.Flag("align"), Vertical: c.Bool("vertical")}
	if _, err := draw.FontBytes(spec.Font); err != nil {
		return nil, usagef("%v", err)
	}
	for name, dst := range map[string]*float64{"x": &spec.X, "y": &spec.Y, "size": &spec.Size, "width": &spec.Width} {
		if f, ok, err := c.Float(name); err != nil {
			return nil, err
		} else if ok {
			*dst = f
		}
	}
	if v := c.Flag("color"); v != "" {
		col, err := render.ParseColor(v)
		if err != nil {
			return nil, usagef("%v", err)
		}
		spec.Color = col
	}
	return t.write(c, layer, draw.Text(t.w, t.h, t.area, spec), false, false)
}

func cmdDrawErase(c *Ctx) (any, error) {
	t, err := resolveTarget(c)
	if err != nil {
		return nil, err
	}
	pts, err := shapeFlag(c)
	if err != nil {
		return nil, err
	}
	for i := range pts {
		pts[i][0] += t.area.X
		pts[i][1] += t.area.Y
	}
	mask := render.PolygonMask(t.w, t.h, pts)
	overlay := render.New(t.w, t.h)
	render.FillMask(overlay, mask, color.Black, 1)
	layers := []string{c.Flag("layer")}
	if c.Bool("all") {
		layers = nil
		for _, l := range story.OrderedLayers(t.obj) {
			if t.checkLayer(l) == nil {
				layers = append(layers, l)
			}
		}
	} else if layers[0] == "" {
		return nil, usagef("give --layer or --all")
	}
	var res map[string]any
	for _, l := range layers {
		if err := t.checkLayer(l); err != nil {
			return nil, err
		}
		if res, err = t.write(c, l, render.ToRGBA(overlay), true, false); err != nil {
			return nil, err
		}
	}
	if res == nil {
		return map[string]any{"erased": 0}, nil
	}
	return res, nil
}

// shapeFlag reads --rect x,y,w,h or --polygon "x,y x,y x,y".
func shapeFlag(c *Ctx) ([][2]float64, error) {
	switch {
	case c.Flag("rect") != "":
		r, err := manga.ParseRect(c.Flag("rect"))
		if err != nil {
			return nil, usagef("%v", err)
		}
		return manga.RectPoly(r), nil
	case c.Flag("polygon") != "":
		pts, err := story.ParsePolygon(c.Flag("polygon"))
		if err != nil {
			return nil, usagef("%v", err)
		}
		return pts, nil
	}
	return nil, usagef("give --rect or --polygon")
}

func cmdDrawTone(c *Ctx) (any, error) {
	t, err := resolveTarget(c)
	if err != nil {
		return nil, err
	}
	pts, err := shapeFlag(c)
	if err != nil {
		return nil, err
	}
	layer := c.Flag("layer")
	if layer == "" {
		layer = "tone"
	}
	if err := t.checkLayer(layer); err != nil {
		return nil, err
	}
	spec := draw.ToneSpec{Pattern: c.Flag("pattern"), Spacing: 8, Density: 0.3, Angle: 45, Color: color.RGBA{0, 0, 0, 255}}
	if spec.Pattern == "" {
		spec.Pattern = "dots"
	}
	for name, dst := range map[string]*float64{"spacing": &spec.Spacing, "density": &spec.Density, "density-to": &spec.DensityTo, "angle": &spec.Angle} {
		if f, ok, err := c.Float(name); err != nil {
			return nil, err
		} else if ok {
			*dst = f
		}
	}
	if v := c.Flag("gradient"); v != "" {
		var g [4]float64
		if n, _ := fmt.Sscanf(strings.ReplaceAll(v, " ", ""), "%g,%g,%g,%g", &g[0], &g[1], &g[2], &g[3]); n != 4 {
			return nil, usagef("--gradient must be x1,y1,x2,y2")
		}
		spec.Gradient = &g
	} else if c.Has("density-to") {
		return nil, usagef("--density-to needs --gradient")
	}
	if v := c.Flag("color"); v != "" {
		if spec.Color, err = render.ParseColor(v); err != nil {
			return nil, usagef("%v", err)
		}
	}
	overlay, err := draw.Tone(t.w, t.h, t.area, pts, spec)
	if err != nil {
		return nil, usagef("%v", err)
	}
	return t.write(c, layer, overlay, false, c.Bool("replace"))
}

// layerArgs reads `[board] <layer>` or `--page <p> <layer>`.
func layerArgs(c *Ctx) (*drawTarget, string, error) {
	t, err := resolveTarget(c)
	if err != nil {
		return nil, "", err
	}
	if len(t.rest) == 0 {
		return nil, "", usagef("give a layer name")
	}
	return t, t.rest[0], nil
}

func cmdLayerHistory(c *Ctx) (any, error) {
	t, layer, err := layerArgs(c)
	if err != nil {
		return nil, err
	}
	h := story.LayerHistory(t.s, t.obj, layer)
	for _, e := range h {
		what := e.Path
		if e.Absent {
			what = "(layer did not exist)"
		}
		c.Printf("%2d  %s  %s\n", e.Index, e.Time, what)
	}
	if len(h) == 0 {
		c.Printf("No history for %s layer %s", t.label(), layer)
	}
	return map[string]any{"target": t.label(), "layer": layer, "versions": h, "depth": story.HistoryDepth}, nil
}

func cmdLayerUndo(c *Ctx) (any, error) { return layerStep(c, true) }
func cmdLayerRedo(c *Ctx) (any, error) { return layerStep(c, false) }

func layerStep(c *Ctx, undo bool) (any, error) {
	t, layer, err := layerArgs(c)
	if err != nil {
		return nil, err
	}
	var e story.HistoryEntry
	if undo {
		e, err = story.UndoLayer(t.s, t.obj, layer)
	} else {
		e, err = story.RedoLayer(t.s, t.obj, layer)
	}
	if err != nil {
		return nil, err
	}
	if err := t.s.Save(); err != nil {
		return nil, err
	}
	u, r := len(story.LayerHistory(t.s, t.obj, layer)), story.RedoCount(t.s, t.obj, layer)
	c.Printf("%s the %s layer of %s (%d undo, %d redo left)", map[bool]string{true: "Undid", false: "Redid"}[undo], layer, t.label(), u, r)
	res := map[string]any{"target": t.label(), "layer": layer, "removedLayer": e.Absent, "undo": u, "redo": r, "remaining": u,
		"posterframe": t.s.ImagePath(story.PosterframeFile(t.obj)), "thumbnail": t.s.ImagePath(story.ThumbnailFile(t.obj))}
	if l := story.Layer(t.obj, layer); l != nil {
		res["file"] = t.s.ImagePath(l.Str("url"))
	}
	return res, nil
}

func cmdContactSheet(c *Ctx) (any, error) {
	out := c.Flag("out")
	if out == "" {
		return nil, usagef("--out <png> is required")
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	story.UpdateTiming(s)
	spec := c.Flag("boards")
	if spec == "" {
		spec = "all"
	}
	idx, err := boardIndexes(s, spec)
	if err != nil {
		return nil, err
	}
	cols, width, err := sheetFlags(c)
	if err != nil {
		return nil, err
	}
	cw := width
	var cells []render.Cell
	for _, i := range sortedInts(idx) {
		b := s.Boards()[i]
		bw, bh := story.SizeOf(s, b)
		img, _ := story.Flatten(s, b, cw, int(float64(cw)*float64(bh)/float64(bw)))
		cells = append(cells, render.Cell{Image: img, Title: fmt.Sprintf("%d   %s   %s", i+1, b.Str("shot"), msToTime(story.Duration(s, b))), Caption: b.Str("dialogue")})
	}
	return writeSheet(c, out, cells, cols, cw)
}

func sheetFlags(c *Ctx) (int, int, error) {
	cols, width := 4, 400
	var err error
	if v := c.Flag("cols"); v != "" {
		if cols, err = strconv.Atoi(v); err != nil || cols < 1 {
			return 0, 0, usagef("--cols expects a positive number")
		}
	}
	if v := c.Flag("width"); v != "" {
		if width, err = strconv.Atoi(v); err != nil || width < 50 {
			return 0, 0, usagef("--width expects a pixel width >= 50")
		}
	}
	return cols, width, nil
}

func writeSheet(c *Ctx, out string, cells []render.Cell, cols, cw int) (any, error) {
	img := render.ContactSheet(cells, cols, cw)
	abs, _ := filepath.Abs(out)
	os.MkdirAll(filepath.Dir(abs), 0o755)
	if err := render.SavePNG(abs, img); err != nil {
		return nil, err
	}
	c.Printf("Wrote %s (%d items, %dx%d)", abs, len(cells), img.Bounds().Dx(), img.Bounds().Dy())
	return map[string]any{"path": abs, "count": len(cells), "width": img.Bounds().Dx(), "height": img.Bounds().Dy()}, nil
}
