package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/signintech/gopdf"

	"sb/internal/manga"
	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/story"
)

func init() {
	register(
		&Cmd{Path: "project set-mode", Args: "<manga|film>", Short: "Switch a project between manga (pages[] next to boards) and film mode",
			Flags: []string{"page-size=WxH page size in px for manga (default 1414x2000, 1:1.414)", "reading=rtl (default) or ltr"}, Run: cmdSetMode},

		&Cmd{Path: "page setup", Short: "Page settings of a manga project: size (layouts scale with it), reading direction, first page single or paired",
			Flags: []string{"size=WxH page px", "reading=rtl or ltr (panel order is recomputed)", "first-page=single (page 1 alone) or paired (spreads start at page 1)"}, Run: cmdPageSetup},
		&Cmd{Path: "page add", Short: "Add a page (manga), optionally with a panel template", Flags: []string{"after=insert after page <p> (default: at the end)", "template=splash, 2-tier, 3-tier, 4-koma, big-plus-2 or grid-RxC", "gutter=gutter px for the template"}, Run: cmdPageAdd},
		&Cmd{Path: "page list", Short: "Pages with panel and balloon counts and their spread", Run: cmdPageList},
		&Cmd{Path: "page info", Args: "<p>", Short: "One page: panels in reading order, placements, balloons, layers, files", Run: cmdPageInfo},
		&Cmd{Path: "page delete", Args: "<p..>", Short: "Delete pages (files stay on disk)", Run: cmdPageDelete},
		&Cmd{Path: "page move", Args: "<p>", Short: "Move a page to another position", Flags: []string{"to=target page number"}, Run: cmdPageMove},
		&Cmd{Path: "page template", Args: "<p> <name>", Short: "Replace the page's panels: splash, 2-tier, 3-tier, 4-koma, big-plus-2, grid-RxC (e.g. grid-3x2)", Flags: []string{"gutter=gutter px (default: 2.4% of height between tiers, 1.7% of width between columns)"}, Run: cmdPageTemplate},
		&Cmd{Path: "page render", Args: "<p>", Short: "Render a page (placed boards, page layers, frames, balloons) to PNG", Flags: []string{"out=output PNG (required)", "grid overlay a coordinate grid and panel tags (id #order)", "grid-step=grid spacing px (default 100)"}, Run: cmdPageRender},
		&Cmd{Path: "pages contact-sheet", Short: "All pages in one PNG, for reviewing the whole chapter", Flags: []string{"out=output PNG (required)", "pages=pages, e.g. 1-8 (default all)", "cols=columns (default 6)", "width=cell width px (default 300)"}, Run: cmdPageContactSheet},
		&Cmd{Path: "spread render", Args: "<p>", Short: "Render the two-page spread containing page p, in reading order",
			Long:  "rtl: page 1 alone (on the left), then [3|2], [5|4]: the even page is on the right.\nltr: page 1 alone (on the right), then [2|3], [4|5]. With page.firstPageSingle false spreads start at page 1.",
			Flags: []string{"out=output PNG (required)"}, Run: cmdSpreadRender},

		&Cmd{Path: "panel add", Args: "<p>", Short: "Add a panel", Flags: []string{"rect=x,y,w,h in page px", `polygon=points "x,y x,y x,y ..."`, "border=border px (default 6)"}, Run: cmdPanelAdd},
		&Cmd{Path: "panel split", Args: "<p> <panel>", Short: "Split a panel in two with a gutter",
			Flags: []string{"horizontal cut across: top and bottom panels", "vertical cut down: left and right panels", "diagonal slanted cut rising to the right", "at=cut position 0-1 (default 0.5)", "gutter=gutter px", "slant=diagonal rise as a fraction of the panel height (default 0.3)"}, Run: cmdPanelSplit},
		&Cmd{Path: "panel merge", Args: "<p> <panel> <panel>", Short: "Merge two panels into their convex hull", Run: cmdPanelMerge},
		&Cmd{Path: "panel set", Args: "<p> <panel>", Short: "Change panel points, border or bleed", Flags: []string{"points=new polygon", "rect=new rectangle x,y,w,h", "border=border px (0 = none)", "bleed=true|false: edges near the page edge extend to it (no border there)"}, Run: cmdPanelSet},
		&Cmd{Path: "panel delete", Args: "<p> <panel..>", Short: "Delete panels", Run: cmdPanelDelete},
		&Cmd{Path: "panel list", Args: "<p>", Short: "Panels in reading order with boxes and placements", Run: cmdPanelList},
		&Cmd{Path: "panel order", Args: "<p> [panel..]", Short: "Set a manual reading order (panels listed first), or --auto", Flags: []string{"auto back to automatic order (rows top to bottom, then by reading direction)"}, Run: cmdPanelOrder},
		&Cmd{Path: "panel place", Args: "<p> <panel> <board>", Short: "Show a board's drawing in a panel (panel content), clipped to the panel; a board can appear in several panels",
			Flags: []string{"x=offset of the board center from the panel center, px", "y=offset px", "scale=multiplier on top of --fit (default 1)", "rotation=degrees", "fit=fill (cover the panel, default), fit (inside the panel) or none (1:1 pixels)"}, Run: cmdPanelPlace},
		&Cmd{Path: "panel map", Args: "<p> <panel>", Short: "Convert a point between the placed board's pixels, the page and the panel (honors fit, scale, offset, rotation)",
			Long:  "Give one of --board-point, --page-point or --panel-point; all three coordinates come back,\nplus whether the point is inside the panel shape and inside the board canvas.\nUse it to aim balloon tails and page-layer strokes at placed art.",
			Flags: []string{"board-point=x,y in board pixels", "page-point=x,y in page pixels", "panel-point=x,y panel-local (0,0 = panel box top-left)"}, Run: cmdPanelMap},
		&Cmd{Path: "panel clear-content", Args: "<p> <panel>", Short: "Remove the board from a panel (content becomes null)", Run: cmdPanelClearContent},

		&Cmd{Path: "balloon add", Args: "<p> <text>", Short: "Add a speech/thought/shout/whisper/narration/sfx balloon (size fits the text unless --w/--h)",
			Long:  "Coordinates are page px, or panel-local with --panel. --x/--y is the balloon's top-left corner\n(default: near the top of the panel/page on the reading side). \\n in text starts a new line.",
			Flags: []string{"type=speech (default), thought, shout, whisper, narration or sfx", "x=left px", "y=top px", "w=width px", "h=height px", "tail=x,y the tail points at (speaker)", "panel=panel id: coordinates are panel-local", "size=font size px (default 32)", "vertical vertical text (columns right to left)"}, Run: cmdBalloonAdd},
		&Cmd{Path: "balloon set", Args: "<p> <balloon>", Short: "Change a balloon", Flags: []string{"text=new text", "type=balloon type", "x=px", "y=px", "w=px", "h=px", "tail=x,y or none", "size=font size", "vertical=true|false", "fit resize the box to the text"}, Run: cmdBalloonSet},
		&Cmd{Path: "balloon delete", Args: "<p> <balloon..>", Short: "Delete balloons", Run: cmdBalloonDelete},
		&Cmd{Path: "balloon list", Args: "<p>", Short: "Balloons of a page", Run: cmdBalloonList},

		&Cmd{Path: "export pages", Short: "Export manga pages as PNG files or one PDF (one page per sheet)",
			Flags: []string{"png PNG per page (default)", "pdf one PDF", "pages=pages, e.g. 1-8 (default all)", "out=output folder (png) or file (pdf)",
				"include-notes include the notes layer (left out by default)", "page-numbers print the page number in the bottom margin",
				"dpi=with --pdf: rasterize pages for this print resolution (default: the page's own pixels)", "crop-marks with --pdf: add a 12.7 mm (0.5 in) margin with trim marks around each page"}, Run: cmdExportPages},
	)
}

func openManga() (*story.Scene, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	return s, manga.RequireManga(s)
}

func parseSize(v string) (int, int, error) {
	var w, h int
	if _, err := fmt.Sscanf(strings.ToLower(v), "%dx%d", &w, &h); err != nil || w < 100 || h < 100 {
		return 0, 0, usagef("page size must be WxH in px (e.g. 1414x2000)")
	}
	return w, h, nil
}

func readingFlag(c *Ctx) (bool, error) {
	switch c.Flag("reading") {
	case "", "rtl":
		return true, nil
	case "ltr":
		return false, nil
	}
	return false, usagef("--reading must be rtl or ltr")
}

// pageSave refreshes a page's derived files and saves the project.
func pageSave(s *story.Scene, pg *ojson.Object) error {
	if err := manga.RefreshPage(s, pg); err != nil {
		return err
	}
	return s.Save()
}

func pageRow(s *story.Scene, i int, pg *ojson.Object) map[string]any {
	a, b := manga.Spread(s, i)
	spread := []int{a + 1}
	if b >= 0 {
		spread = append(spread, b+1)
	}
	return map[string]any{"page": i + 1, "id": pg.Str("id"), "panels": len(manga.Panels(pg)), "balloons": len(manga.Balloons(pg)), "spread": spread,
		"thumbnail": s.ImagePath(story.ThumbnailFile(pg)), "posterframe": s.ImagePath(story.PosterframeFile(pg))}
}

func cmdSetMode(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	switch c.Arg(0) {
	case "manga":
		w, h := 1414, 2000
		if v := c.Flag("page-size"); v != "" {
			if w, h, err = parseSize(v); err != nil {
				return nil, err
			}
		}
		rtl, err := readingFlag(c)
		if err != nil {
			return nil, err
		}
		manga.MakeManga(s, w, h, rtl)
	case "film":
		s.Data.Delete("mode") // pages stay in the file, unused in film mode
	default:
		return nil, usagef("mode must be manga or film")
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("%s is now in %s mode", s.Name(), c.Arg(0))
	return map[string]any{"mode": c.Arg(0), "page": s.Data.Get("page")}, nil
}

func cmdPageAdd(c *Ctx) (any, error) {
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	pages := s.Pages()
	pos := len(pages)
	if v := c.Flag("after"); v != "" {
		i, _, err := manga.FindPage(s, v)
		if err != nil {
			return nil, err
		}
		pos = i + 1
	}
	pg := manga.NewPage()
	s.SetPages(append(pages[:pos], append([]*ojson.Object{pg}, pages[pos:]...)...))
	if t := c.Flag("template"); t != "" {
		if err := applyTemplate(c, s, pg, t); err != nil {
			return nil, err
		}
	}
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	c.Printf("Added page %d (%s) with %d panels", pos+1, pg.Str("id"), len(manga.Panels(pg)))
	return pageRow(s, pos, pg), nil
}

func applyTemplate(c *Ctx, s *story.Scene, pg *ojson.Object, name string) error {
	l := manga.LayoutOf(s)
	if f, ok, err := c.Float("gutter"); err != nil {
		return err
	} else if ok {
		l.GutterX, l.GutterY = f, f
	}
	name = strings.Replace(name, "grid-", "grid ", 1)
	return manga.ApplyTemplate(s, pg, name, l)
}

func cmdPageList(c *Ctx) (any, error) {
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	for i, pg := range s.Pages() {
		r := pageRow(s, i, pg)
		rows = append(rows, r)
		c.Printf("%3d  %-5s  %d panels, %d balloons, spread %v\n", i+1, pg.Str("id"), r["panels"], r["balloons"], r["spread"])
	}
	w, h := s.PageSize()
	if rows == nil {
		rows = []map[string]any{}
		c.Printf("No pages yet (sb page add --template 3-tier)")
	}
	return map[string]any{"pages": rows, "pageSize": []int{w, h}, "readingDirection": manga.Settings(s).Str("readingDirection")}, nil
}

func panelRow(s *story.Scene, p *ojson.Object) map[string]any {
	box, _ := manga.PanelArea(s, p)
	r := map[string]any{"id": p.Str("id"), "order": p.Get("order"), "box": []float64{box.X, box.Y, box.W, box.H}, "points": p.Get("points"), "border": p.Get("border"), "bleed": p.Get("bleed")}
	r["content"] = p.Get("content")
	return r
}

func cmdPageInfo(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	i, pg, err := manga.FindPage(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	res := pageRow(s, i, pg)
	var panels []map[string]any
	for _, p := range manga.InOrder(pg) {
		panels = append(panels, panelRow(s, p))
	}
	res["panels"] = panels
	res["balloons"] = pg.Get("balloons")
	res["layers"] = story.OrderedLayers(pg)
	res["panelOrder"] = pg.Str("panelOrder")
	w, h := s.PageSize()
	res["size"] = []int{w, h}
	c.Printf("Page %d (%s), %dx%d, %d panels (%s order)\n", i+1, pg.Str("id"), w, h, len(panels), pg.Str("panelOrder"))
	for _, p := range panels {
		c.Printf("  #%v %-4s box %v", p["order"], p["id"], p["box"])
		if b, ok := p["content"].(*ojson.Object); ok {
			c.Printf("  board %s", b.Str("board"))
		}
		c.Printf("\n")
	}
	for _, b := range manga.Balloons(pg) {
		c.Printf("  %s %s: %q\n", b.Str("id"), b.Str("type"), b.Str("text"))
	}
	return res, nil
}

func pageIndexes(s *story.Scene, specs []string) ([]int, error) {
	var out []int
	for _, spec := range strings.Split(strings.Join(specs, ","), ",") {
		if a, b, ok := strings.Cut(spec, "-"); ok {
			x, e1 := strconv.Atoi(a)
			y, e2 := strconv.Atoi(b)
			if e1 != nil || e2 != nil || x < 1 || y < x || y > len(s.Pages()) {
				return nil, usagef("bad page range %q", spec)
			}
			for i := x; i <= y; i++ {
				out = append(out, i-1)
			}
			continue
		}
		if spec == "all" {
			for i := range s.Pages() {
				out = append(out, i)
			}
			continue
		}
		i, _, err := manga.FindPage(s, spec)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, nil
}

func cmdPageDelete(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	idx, err := pageIndexes(s, c.Pos)
	if err != nil {
		return nil, err
	}
	del := map[int]bool{}
	for _, i := range idx {
		del[i] = true
	}
	var keep []*ojson.Object
	for i, pg := range s.Pages() {
		if !del[i] {
			keep = append(keep, pg)
		}
	}
	s.SetPages(keep)
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("Deleted %d page(s); %d left", len(del), len(keep))
	return map[string]any{"deleted": len(del), "pages": len(keep)}, nil
}

func cmdPageMove(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	i, pg, err := manga.FindPage(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	to, err := strconv.Atoi(c.Flag("to"))
	if err != nil || to < 1 {
		return nil, usagef("--to <page number> is required")
	}
	pages := s.Pages()
	pages = append(pages[:i], pages[i+1:]...)
	to = min(to-1, len(pages))
	s.SetPages(append(pages[:to], append([]*ojson.Object{pg}, pages[to:]...)...))
	if err := s.Save(); err != nil {
		return nil, err
	}
	return cmdPageList(c)
}

func cmdPageTemplate(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	i, pg, err := manga.FindPage(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	if err := applyTemplate(c, s, pg, strings.Join(c.Pos[1:], " ")); err != nil {
		return nil, usagef("%v", err)
	}
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	var panels []map[string]any
	for _, p := range manga.InOrder(pg) {
		panels = append(panels, panelRow(s, p))
		box, _ := manga.PanelArea(s, p)
		c.Printf("#%v %s  %.0f,%.0f %.0fx%.0f\n", p.Get("order"), p.Str("id"), box.X, box.Y, box.W, box.H)
	}
	return map[string]any{"page": i + 1, "panels": panels}, nil
}

func renderStep(c *Ctx) (float64, error) {
	step := 100.0
	if f, ok, err := c.Float("grid-step"); err != nil {
		return 0, err
	} else if ok && f > 0 {
		step = f
	}
	return step, nil
}

func cmdPageRender(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	if c.Flag("out") == "" {
		return nil, usagef("--out <png> is required")
	}
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	i, pg, err := manga.FindPage(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	img, warnings := manga.Compose(s, pg)
	if c.Bool("grid") {
		step, err := renderStep(c)
		if err != nil {
			return nil, err
		}
		manga.Guides(img, s, pg, step)
	}
	abs, _ := filepath.Abs(c.Flag("out"))
	os.MkdirAll(filepath.Dir(abs), 0o755)
	if err := render.SavePNG(abs, img); err != nil {
		return nil, err
	}
	c.Printf("Wrote page %d to %s", i+1, abs)
	for _, w := range warnings {
		c.Printf("\n  warning: %s", w)
	}
	if warnings == nil {
		warnings = []string{}
	}
	return map[string]any{"page": i + 1, "path": abs, "width": img.Bounds().Dx(), "height": img.Bounds().Dy(), "warnings": warnings}, nil
}

func cmdSpreadRender(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	if c.Flag("out") == "" {
		return nil, usagef("--out <png> is required")
	}
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	i, _, err := manga.FindPage(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	img, nums := manga.RenderSpread(s, i)
	abs, _ := filepath.Abs(c.Flag("out"))
	os.MkdirAll(filepath.Dir(abs), 0o755)
	if err := render.SavePNG(abs, img); err != nil {
		return nil, err
	}
	side := func(n int) string {
		if n == 0 {
			return "blank"
		}
		return fmt.Sprintf("page %d", n)
	}
	c.Printf("Wrote %s: left %s, right %s", abs, side(nums[0]), side(nums[1]))
	return map[string]any{"path": abs, "left": nums[0], "right": nums[1], "width": img.Bounds().Dx(), "height": img.Bounds().Dy()}, nil
}

func cmdPageContactSheet(c *Ctx) (any, error) {
	out := c.Flag("out")
	if out == "" {
		return nil, usagef("--out <png> is required")
	}
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	spec := c.Flag("pages")
	if spec == "" {
		spec = "all"
	}
	idx, err := pageIndexes(s, []string{spec})
	if err != nil {
		return nil, err
	}
	cols, width := 6, 300
	if c.Flag("cols") != "" || c.Flag("width") != "" {
		cols, width, err = sheetFlags(c)
		if err != nil {
			return nil, err
		}
		if c.Flag("cols") == "" {
			cols = 6
		}
	}
	var cells []render.Cell
	for _, i := range idx {
		pg := s.Pages()[i]
		img, _ := manga.Compose(s, pg)
		cells = append(cells, render.Cell{Image: img, Title: fmt.Sprintf("p.%d", i+1), Caption: fmt.Sprintf("%d panels, %d balloons", len(manga.Panels(pg)), len(manga.Balloons(pg)))})
	}
	return writeSheet(c, out, cells, cols, width)
}

func findPanelArgs(c *Ctx, n int) (*story.Scene, int, *ojson.Object, *ojson.Object, error) {
	if err := c.Need(n); err != nil {
		return nil, 0, nil, nil, err
	}
	s, err := openManga()
	if err != nil {
		return nil, 0, nil, nil, err
	}
	i, pg, err := manga.FindPage(s, c.Arg(0))
	if err != nil {
		return nil, 0, nil, nil, err
	}
	if n < 2 {
		return s, i, pg, nil, nil
	}
	p, err := manga.FindPanel(pg, c.Arg(1))
	return s, i, pg, p, err
}

func cmdPanelAdd(c *Ctx) (any, error) {
	s, _, pg, _, err := findPanelArgs(c, 1)
	if err != nil {
		return nil, err
	}
	var pts []manga.Pt
	switch {
	case c.Flag("rect") != "":
		r, err := manga.ParseRect(c.Flag("rect"))
		if err != nil {
			return nil, usagef("%v", err)
		}
		pts = manga.RectPoly(r)
	case c.Flag("polygon") != "":
		if pts, err = manga.ParsePoints(c.Flag("polygon")); err != nil {
			return nil, usagef("%v", err)
		}
	default:
		return nil, usagef("give --rect or --polygon")
	}
	p := manga.AddPanel(s, pg, pts)
	if f, ok, err := c.Float("border"); err != nil {
		return nil, err
	} else if ok {
		p.Set("border", f)
	}
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	c.Printf("Added panel %s (#%v)", p.Str("id"), p.Get("order"))
	return panelRow(s, p), nil
}

func cmdPanelSplit(c *Ctx) (any, error) {
	s, _, pg, p, err := findPanelArgs(c, 2)
	if err != nil {
		return nil, err
	}
	kind := ""
	for _, k := range []string{"horizontal", "vertical", "diagonal"} {
		if c.Bool(k) {
			kind = k
		}
	}
	if kind == "" {
		return nil, usagef("give --horizontal, --vertical or --diagonal")
	}
	l := manga.LayoutOf(s)
	gutter := l.GutterX
	if kind == "horizontal" {
		gutter = l.GutterY
	}
	at, slant := 0.5, 0.3
	for name, dst := range map[string]*float64{"gutter": &gutter, "at": &at, "slant": &slant} {
		if f, ok, err := c.Float(name); err != nil {
			return nil, err
		} else if ok {
			*dst = f
		}
	}
	n, err := manga.SplitPanel(s, pg, p, kind, manga.Clamp(at, 0.05, 0.95), gutter, slant)
	if err != nil {
		return nil, err
	}
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	c.Printf("Split %s into %s and %s", p.Str("id"), p.Str("id"), n.Str("id"))
	return map[string]any{"panels": []map[string]any{panelRow(s, p), panelRow(s, n)}}, nil
}

func cmdPanelMerge(c *Ctx) (any, error) {
	s, _, pg, a, err := findPanelArgs(c, 3)
	if err != nil {
		return nil, err
	}
	b, err := manga.FindPanel(pg, c.Arg(2))
	if err != nil {
		return nil, err
	}
	if a == b {
		return nil, usagef("give two different panels")
	}
	manga.MergePanels(s, pg, a, b)
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	return panelRow(s, a), nil
}

func cmdPanelSet(c *Ctx) (any, error) {
	s, _, pg, p, err := findPanelArgs(c, 2)
	if err != nil {
		return nil, err
	}
	var pts []manga.Pt
	if v := c.Flag("points"); v != "" {
		if pts, err = manga.ParsePoints(v); err != nil {
			return nil, usagef("%v", err)
		}
	}
	if v := c.Flag("rect"); v != "" {
		r, err := manga.ParseRect(v)
		if err != nil {
			return nil, usagef("%v", err)
		}
		pts = manga.RectPoly(r)
	}
	if pts != nil {
		arr := make([]any, len(pts))
		for i, q := range pts {
			arr[i] = []any{q[0], q[1]}
		}
		p.Set("points", arr)
	}
	if f, ok, err := c.Float("border"); err != nil {
		return nil, err
	} else if ok {
		p.Set("border", f)
	}
	if v := c.Flag("bleed"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, usagef("--bleed expects true or false")
		}
		p.Set("bleed", b)
	}
	manga.Reorder(s, pg)
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	return panelRow(s, p), nil
}

func cmdPanelDelete(c *Ctx) (any, error) {
	s, _, pg, _, err := findPanelArgs(c, 2)
	if err != nil {
		return nil, err
	}
	for _, id := range c.Pos[1:] {
		p, err := manga.FindPanel(pg, id)
		if err != nil {
			return nil, err
		}
		manga.DeletePanel(s, pg, p)
	}
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	return map[string]any{"panels": len(manga.Panels(pg))}, nil
}

func cmdPanelList(c *Ctx) (any, error) {
	s, i, pg, _, err := findPanelArgs(c, 1)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	for _, p := range manga.InOrder(pg) {
		r := panelRow(s, p)
		rows = append(rows, r)
		c.Printf("#%-3v %-4s box %v", r["order"], r["id"], r["box"])
		if b, ok := r["content"].(*ojson.Object); ok {
			c.Printf("  board %s", b.Str("board"))
		}
		c.Printf("\n")
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	return map[string]any{"page": i + 1, "panels": rows, "panelOrder": pg.Str("panelOrder")}, nil
}

func cmdPanelOrder(c *Ctx) (any, error) {
	s, _, pg, _, err := findPanelArgs(c, 1)
	if err != nil {
		return nil, err
	}
	if c.Bool("auto") {
		pg.Set("panelOrder", "auto")
		manga.Reorder(s, pg)
	} else {
		if len(c.Pos) < 2 {
			return nil, usagef("list panel ids in reading order, or --auto")
		}
		if err := manga.SetManualOrder(s, pg, c.Pos[1:]); err != nil {
			return nil, err
		}
	}
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	return cmdPanelList(c)
}

func cmdPanelPlace(c *Ctx) (any, error) {
	s, _, pg, p, err := findPanelArgs(c, 3)
	if err != nil {
		return nil, err
	}
	_, b, err := oneBoard(s, c.Arg(2))
	if err != nil {
		return nil, err
	}
	pl := p.Obj("content")
	if pl == nil || pl.Str("board") != story.UID(b) {
		pl = ojson.Obj("board", story.UID(b), "x", 0, "y", 0, "scale", 1, "rotation", 0, "fit", "fill")
	}
	if v := c.Flag("fit"); v != "" {
		if v != "fill" && v != "fit" && v != "none" {
			return nil, usagef("--fit must be fill, fit or none")
		}
		pl.Set("fit", v)
	}
	for _, k := range []string{"x", "y", "scale", "rotation"} {
		if f, ok, err := c.Float(k); err != nil {
			return nil, err
		} else if ok {
			pl.Set(k, f)
		}
	}
	p.Set("content", pl)
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	c.Printf("Placed board %s in panel %s", story.UID(b), p.Str("id"))
	return panelRow(s, p), nil
}

func cmdPanelClearContent(c *Ctx) (any, error) {
	s, _, pg, p, err := findPanelArgs(c, 2)
	if err != nil {
		return nil, err
	}
	p.Set("content", nil)
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	return panelRow(s, p), nil
}

func parsePt(v string) (*manga.Pt, error) {
	var x, y float64
	if n, _ := fmt.Sscanf(v, "%g,%g", &x, &y); n != 2 {
		return nil, usagef("expected x,y, got %q", v)
	}
	return &manga.Pt{x, y}, nil
}

func cmdBalloonAdd(c *Ctx) (any, error) {
	s, _, pg, _, err := findBalloonPage(c)
	if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(strings.Join(c.Pos[1:], " "), `\n`, "\n")
	typ := c.Flag("type")
	if typ == "" {
		typ = "speech"
	}
	vals := map[string]float64{}
	for _, k := range []string{"x", "y", "w", "h", "size"} {
		if f, ok, err := c.Float(k); err != nil {
			return nil, err
		} else if ok {
			vals[k] = f
		}
	}
	// origin: the panel box (panel-local coordinates) or the page
	w, h := s.PageSize()
	area := manga.Rect{W: float64(w), H: float64(h)}
	panelID := ""
	if id := c.Flag("panel"); id != "" {
		p, err := manga.FindPanel(pg, id)
		if err != nil {
			return nil, err
		}
		area, _ = manga.PanelArea(s, p)
		panelID = p.Str("id")
	}
	var tl *manga.Pt
	if v := c.Flag("tail"); v != "" {
		if tl, err = parsePt(v); err != nil {
			return nil, err
		}
		tl = &manga.Pt{tl[0] + area.X, tl[1] + area.Y}
	}
	b, err := manga.AddBalloon(pg, typ, text, 0, 0, vals["w"], vals["h"], vals["size"], c.Bool("vertical"), tl, panelID)
	if err != nil {
		return nil, usagef("%v", err)
	}
	bw, bh := b.NumOr("w", 0), b.NumOr("h", 0)
	x, hasX := vals["x"]
	y, hasY := vals["y"]
	pad := 20.0
	if !hasX {
		// the reading side: top-right for rtl, top-left for ltr
		x = pad
		if manga.RTL(s) {
			x = area.W - bw - pad
		}
	}
	if !hasY {
		y = pad
	}
	b.Set("x", area.X+x)
	b.Set("y", area.Y+y)
	_ = bh
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	c.Printf("Added %s balloon %s at %.0f,%.0f (%.0fx%.0f)", typ, b.Str("id"), b.NumOr("x", 0), b.NumOr("y", 0), bw, bh)
	return b, nil
}

func cmdBalloonSet(c *Ctx) (any, error) {
	s, _, pg, _, err := findBalloonPage(c)
	if err != nil {
		return nil, err
	}
	b, err := manga.FindBalloon(pg, c.Arg(1))
	if err != nil {
		return nil, err
	}
	if v, ok := c.Flags["text"]; ok {
		b.Set("text", strings.ReplaceAll(v, `\n`, "\n"))
	}
	if v := c.Flag("type"); v != "" {
		b.Set("type", v)
	}
	for _, k := range []string{"x", "y", "w", "h", "size"} {
		if f, ok, err := c.Float(k); err != nil {
			return nil, err
		} else if ok {
			key := k
			if k == "size" {
				key = "fontSize"
			}
			b.Set(key, f)
		}
	}
	if v := c.Flag("vertical"); v != "" {
		bv, err := strconv.ParseBool(v)
		if err != nil {
			return nil, usagef("--vertical expects true or false")
		}
		b.Set("vertical", bv)
	}
	if v := c.Flag("tail"); v != "" {
		if v == "none" {
			b.Delete("tail")
		} else {
			t, err := parsePt(v)
			if err != nil {
				return nil, err
			}
			b.Set("tail", []any{t[0], t[1]})
		}
	}
	if c.Bool("fit") {
		w, h := manga.AutoSize(b)
		b.Set("w", w)
		b.Set("h", h)
	}
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	return b, nil
}

func cmdBalloonDelete(c *Ctx) (any, error) {
	s, _, pg, _, err := findBalloonPage(c)
	if err != nil {
		return nil, err
	}
	for _, id := range c.Pos[1:] {
		b, err := manga.FindBalloon(pg, id)
		if err != nil {
			return nil, err
		}
		manga.DeleteBalloon(pg, b)
	}
	if err := pageSave(s, pg); err != nil {
		return nil, err
	}
	return map[string]any{"balloons": len(manga.Balloons(pg))}, nil
}

func cmdBalloonList(c *Ctx) (any, error) {
	_, i, pg, _, err := findPanelArgs(c, 1)
	if err != nil {
		return nil, err
	}
	bs := manga.Balloons(pg)
	for _, b := range bs {
		c.Printf("%-4s %-9s %4.0f,%-4.0f %4.0fx%-4.0f %q\n", b.Str("id"), b.Str("type"), b.NumOr("x", 0), b.NumOr("y", 0), b.NumOr("w", 0), b.NumOr("h", 0), b.Str("text"))
	}
	if bs == nil {
		bs = []*ojson.Object{}
	}
	return map[string]any{"page": i + 1, "balloons": bs}, nil
}

func cmdExportPages(c *Ctx) (any, error) {
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	spec := c.Flag("pages")
	if spec == "" {
		spec = "all"
	}
	idx, err := pageIndexes(s, []string{spec})
	if err != nil {
		return nil, err
	}
	if len(idx) == 0 {
		return nil, fmt.Errorf("no pages to export")
	}
	exports, err := story.ExportsDir(s)
	if err != nil {
		return nil, err
	}
	if c.Bool("pdf") {
		out := c.Flag("out")
		if out == "" {
			out = filepath.Join(exports, s.Name()+" pages "+story.Stamp()+".pdf")
		}
		abs, _ := filepath.Abs(out)
		opts := pdfOpts{notes: c.Bool("include-notes"), numbers: c.Bool("page-numbers"), marks: c.Bool("crop-marks")}
		if f, ok, err := c.Float("dpi"); err != nil {
			return nil, err
		} else if ok {
			if f < 36 || f > 1200 {
				return nil, usagef("--dpi must be 36-1200")
			}
			opts.dpi = f
		}
		if err := writePagesPDF(s, idx, abs, opts); err != nil {
			return nil, err
		}
		c.Printf("Wrote %s (%d pages)", abs, len(idx))
		return map[string]any{"path": abs, "pages": len(idx)}, nil
	}
	dir := c.Flag("out")
	if dir == "" {
		dir = filepath.Join(exports, s.Name()+" pages "+story.Stamp())
	}
	dir, _ = filepath.Abs(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var files []string
	for _, i := range idx {
		img, _ := manga.ComposeWith(s, s.Pages()[i], c.Bool("include-notes"))
		if c.Bool("page-numbers") {
			manga.PageNumber(img, i+1)
		}
		p := filepath.Join(dir, fmt.Sprintf("%s-page-%03d.png", s.Name(), i+1))
		if err := render.SavePNG(p, img); err != nil {
			return nil, err
		}
		files = append(files, p)
	}
	c.Printf("Exported %d pages to %s", len(files), dir)
	return map[string]any{"files": files}, nil
}

type pdfOpts struct {
	notes, numbers, marks bool
	dpi                   float64
}

// writePagesPDF puts one page per sheet. The trim size has the page ratio with
// the long side of A4 (841.89 pt); crop marks add a 36 pt margin with marks.
func writePagesPDF(s *story.Scene, idx []int, out string, o pdfOpts) error {
	w, h := s.PageSize()
	th := 841.89
	tw := th * float64(w) / float64(h)
	if w > h {
		tw, th = 841.89, 841.89*float64(h)/float64(w)
	}
	m := 0.0
	if o.marks {
		m = 36
	}
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: gopdf.Rect{W: tw + 2*m, H: th + 2*m}})
	pdf.SetInfo(gopdf.PdfInfo{Title: s.Name(), Creator: "Storyboarder v" + story.AppVersion, Producer: "sb (Storyboarder Next)"})
	tmp, err := os.MkdirTemp("", "sb-pages-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, i := range idx {
		img, _ := manga.ComposeWith(s, s.Pages()[i], o.notes)
		if o.numbers {
			manga.PageNumber(img, i+1)
		}
		if o.dpi > 0 {
			img = render.Resize(img, int(math.Round(tw/72*o.dpi)), int(math.Round(th/72*o.dpi)))
		}
		p := filepath.Join(tmp, fmt.Sprintf("%03d.jpg", i))
		if err := render.SaveJPEG(p, img, 90); err != nil {
			return err
		}
		pdf.AddPage()
		if err := pdf.Image(p, m, m, &gopdf.Rect{W: tw, H: th}); err != nil {
			return err
		}
		if o.marks {
			pdf.SetStrokeColor(0, 0, 0)
			pdf.SetLineWidth(0.25)
			gap, l := 6.0, 24.0
			for _, x := range []float64{m, m + tw} {
				for _, y := range []float64{m, m + th} {
					dx, dy := -1.0, -1.0
					if x > m {
						dx = 1
					}
					if y > m {
						dy = 1
					}
					pdf.Line(x+dx*gap, y, x+dx*(gap+l), y)
					pdf.Line(x, y+dy*gap, x, y+dy*(gap+l))
				}
			}
		}
	}
	os.MkdirAll(filepath.Dir(out), 0o755)
	return pdf.WritePdf(out)
}

// findBalloonPage reads `<p> <second arg>` without treating the second as a panel.
func findBalloonPage(c *Ctx) (*story.Scene, int, *ojson.Object, *ojson.Object, error) {
	if err := c.Need(2); err != nil {
		return nil, 0, nil, nil, err
	}
	return findPanelArgs(c, 1)
}

func cmdPanelMap(c *Ctx) (any, error) {
	s, _, _, p, err := findPanelArgs(c, 2)
	if err != nil {
		return nil, err
	}
	content, err := manga.ContentOf(s, p)
	if err != nil {
		return nil, err
	}
	box, clip := manga.PanelArea(s, p)
	var px, py float64
	switch {
	case c.Flag("board-point") != "":
		if content == nil {
			return nil, fmt.Errorf("panel %s has no board (sb panel place)", p.Str("id"))
		}
		pt, err := parsePt(c.Flag("board-point"))
		if err != nil {
			return nil, err
		}
		px, py = content.ToPage(pt[0], pt[1])
	case c.Flag("page-point") != "":
		pt, err := parsePt(c.Flag("page-point"))
		if err != nil {
			return nil, err
		}
		px, py = pt[0], pt[1]
	case c.Flag("panel-point") != "":
		pt, err := parsePt(c.Flag("panel-point"))
		if err != nil {
			return nil, err
		}
		px, py = pt[0]+box.X, pt[1]+box.Y
	default:
		return nil, usagef("give --board-point, --page-point or --panel-point")
	}
	r2 := func(v float64) float64 { return math.Round(v*100) / 100 }
	res := map[string]any{
		"panel":       p.Str("id"),
		"page":        []float64{r2(px), r2(py)},
		"panelLocal":  []float64{r2(px - box.X), r2(py - box.Y)},
		"insidePanel": manga.InsidePolygon(clip, px, py),
	}
	c.Printf("page %.1f,%.1f  panel-local %.1f,%.1f  inside panel: %v", px, py, px-box.X, py-box.Y, res["insidePanel"])
	if content != nil {
		bx, by := content.ToBoard(px, py)
		res["board"] = story.UID(content.Board)
		res["boardPoint"] = []float64{r2(bx), r2(by)}
		res["insideBoard"] = bx >= 0 && by >= 0 && bx <= float64(content.W) && by <= float64(content.H)
		c.Printf("\nboard %s %.1f,%.1f  inside board: %v", story.UID(content.Board), bx, by, res["insideBoard"])
	}
	return res, nil
}

func firstPageFlag(c *Ctx) (*bool, error) {
	switch c.Flag("first-page") {
	case "":
		return nil, nil
	case "single":
		t := true
		return &t, nil
	case "paired":
		f := false
		return &f, nil
	}
	return nil, usagef("--first-page must be single or paired")
}

func cmdPageSetup(c *Ctx) (any, error) {
	s, err := openManga()
	if err != nil {
		return nil, err
	}
	st := manga.Settings(s)
	if v := c.Flag("size"); v != "" {
		w, h, err := parseSize(v)
		if err != nil {
			return nil, err
		}
		manga.Resize(s, w, h)
	}
	if c.Flag("reading") != "" {
		rtl, err := readingFlag(c)
		if err != nil {
			return nil, err
		}
		st.Set("readingDirection", map[bool]string{true: "rtl", false: "ltr"}[rtl])
	}
	fp, err := firstPageFlag(c)
	if err != nil {
		return nil, err
	}
	if fp != nil {
		st.Set("firstPageSingle", *fp)
	}
	for _, pg := range s.Pages() {
		manga.Reorder(s, pg)
		if err := manga.RefreshPage(s, pg); err != nil {
			return nil, err
		}
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	w, h := s.PageSize()
	c.Printf("Pages %dx%d, %s, first page %s", w, h, st.Str("readingDirection"), map[bool]string{true: "single", false: "paired"}[st.Get("firstPageSingle") != false])
	return map[string]any{"page": st, "pages": len(s.Pages())}, nil
}
