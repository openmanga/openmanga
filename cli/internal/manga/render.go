package manga

import (
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"

	"sb/internal/draw"
	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/story"
)

func init() {
	story.PageArtHook = RefreshPage
	story.BoardArtHook = RefreshPagesUsing
}

// PageLayers are the drawing layers a page can have (art across panels).
var PageLayers = []string{"reference", "fill", "tone", "pencil", "ink", "notes"}

// RenderFrames draws panel borders (edges on the page boundary are skipped for bleeds).
func RenderFrames(s *story.Scene, pg *ojson.Object) *image.RGBA {
	w, h := s.PageSize()
	dst := render.New(w, h)
	onEdge := func(a, b Pt) bool {
		return (a[0] <= 0 && b[0] <= 0) || (a[1] <= 0 && b[1] <= 0) ||
			(a[0] >= float64(w) && b[0] >= float64(w)) || (a[1] >= float64(h) && b[1] >= float64(h))
	}
	for _, p := range Panels(pg) {
		bw := p.NumOr("border", DefaultBorder)
		if bw <= 0 {
			continue
		}
		pts := EffectivePoints(s, p)
		for i := range pts {
			a, b := pts[i], pts[(i+1)%len(pts)]
			if onEdge(a, b) {
				continue
			}
			m := draw.StrokeMask(w, h, [][]float64{{a[0], a[1]}, {b[0], b[1]}}, bw, false)
			render.FillMask(dst, m, color.Black, 1)
		}
	}
	return dst
}

// FitModes for panel content.
var FitModes = []string{"fill", "fit", "none"}

// placement draws a panel's content: the board's flattened drawing, centered
// on the panel box center plus (x, y), sized by fit (fill = cover the box,
// fit = inside the box, none = 1 board px per page px) times scale, rotated
// by rotation degrees, clipped to the panel polygon.
// Content is a panel's placed board and its board-to-page transform.
type Content struct {
	Board *ojson.Object
	W, H  int      // board canvas size
	M     f64.Aff3 // board px -> page px
}

// ContentOf resolves a panel's content (nil when the panel is empty). The
// board's center goes to the panel box center plus (x, y), sized by fit
// (fill = cover the box, fit = inside the box, none = 1:1) times scale,
// rotated by rotation degrees.
func ContentOf(s *story.Scene, p *ojson.Object) (*Content, error) {
	pl := p.Obj("content")
	if pl == nil {
		return nil, nil
	}
	var board *ojson.Object
	for _, b := range s.Boards() {
		if story.UID(b) == pl.Str("board") {
			board = b
		}
	}
	if board == nil {
		return nil, fmt.Errorf("panel %s shows missing board %s", p.Str("id"), pl.Str("board"))
	}
	bw, bh := story.SizeOf(s, board)
	box, _ := PanelArea(s, p)
	base := math.Max(box.W/float64(bw), box.H/float64(bh))
	switch pl.Str("fit") {
	case "fit":
		base = math.Min(box.W/float64(bw), box.H/float64(bh))
	case "none":
		base = 1
	}
	k := base * pl.NumOr("scale", 1)
	rot := pl.NumOr("rotation", 0) * math.Pi / 180
	cx, cy := box.X+box.W/2+pl.NumOr("x", 0), box.Y+box.H/2+pl.NumOr("y", 0)
	cos, sin := math.Cos(rot)*k, math.Sin(rot)*k
	m := f64.Aff3{cos, -sin, cx - cos*float64(bw)/2 + sin*float64(bh)/2, sin, cos, cy - sin*float64(bw)/2 - cos*float64(bh)/2}
	return &Content{Board: board, W: bw, H: bh, M: m}, nil
}

// ToPage maps a board pixel to page pixels.
func (c *Content) ToPage(x, y float64) (float64, float64) {
	m := c.M
	return m[0]*x + m[1]*y + m[2], m[3]*x + m[4]*y + m[5]
}

// ToBoard maps a page pixel to board pixels.
func (c *Content) ToBoard(x, y float64) (float64, float64) {
	m := c.M
	det := m[0]*m[4] - m[1]*m[3]
	dx, dy := x-m[2], y-m[5]
	return (m[4]*dx - m[1]*dy) / det, (-m[3]*dx + m[0]*dy) / det
}

// InsidePolygon is an even-odd point-in-polygon test.
func InsidePolygon(pts []Pt, x, y float64) bool {
	in := false
	for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
		a, b := pts[i], pts[j]
		if (a[1] > y) != (b[1] > y) && x < (b[0]-a[0])*(y-a[1])/(b[1]-a[1])+a[0] {
			in = !in
		}
	}
	return in
}

// placement draws a panel's content clipped to the panel polygon.
func placement(s *story.Scene, dst *image.RGBA, p *ojson.Object) error {
	c, err := ContentOf(s, p)
	if err != nil || c == nil {
		return err
	}
	img, _ := story.Flatten(s, c.Board, c.W, c.H)
	_, clip := PanelArea(s, p)
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	layer := render.New(w, h)
	xdraw.CatmullRom.Transform(layer, c.M, img, img.Bounds(), xdraw.Over, nil)
	mask := render.PolygonMask(w, h, clip)
	stddraw.DrawMask(dst, dst.Bounds(), layer, image.Point{}, mask, image.Point{}, stddraw.Over)
	return nil
}

// Compose renders a page: white paper, placed boards clipped to panels, page
// drawing layers, frames, balloons, notes. Frames and balloons are rendered
// from data. Missing placed boards are reported, not fatal.
func Compose(s *story.Scene, pg *ojson.Object) (*image.RGBA, []string) {
	return ComposeWith(s, pg, true)
}

// ComposeWith is Compose with the notes layer optional (exports leave it out).
func ComposeWith(s *story.Scene, pg *ojson.Object, notes bool) (*image.RGBA, []string) {
	w, h := s.PageSize()
	dst := render.White(w, h)
	var warnings []string
	for _, p := range InOrder(pg) {
		if err := placement(s, dst, p); err != nil {
			warnings = append(warnings, err.Error())
		}
	}
	for _, name := range PageLayers[:5] {
		drawLayerFile(s, dst, pg, name)
	}
	render.DrawOver(dst, image.Point{}, RenderFrames(s, pg), 1)
	render.DrawOver(dst, image.Point{}, RenderBalloons(s, pg), 1)
	if notes {
		drawLayerFile(s, dst, pg, "notes")
	}
	return dst, warnings
}

func drawLayerFile(s *story.Scene, dst *image.RGBA, pg *ojson.Object, name string) {
	if story.Layer(pg, name) == nil {
		return
	}
	img, err := render.Load(s.ImagePath(story.Layer(pg, name).Str("url")))
	if err != nil {
		return
	}
	render.DrawScaled(dst, dst.Bounds(), img, story.LayerOpacity(pg, name))
}

// RefreshPage re-renders the derived frames and balloons layers, the page
// thumbnail (120 px high) and posterframe (full page JPG).
func RefreshPage(s *story.Scene, pg *ojson.Object) error {
	layers := story.Layers(pg, true)
	for name, img := range map[string]*image.RGBA{"frames": RenderFrames(s, pg), "balloons": RenderBalloons(s, pg)} {
		file := story.LayerFile(pg, name)
		layers.Set(name, ojson.Obj("url", file))
		if err := render.SavePNG(s.ImagePath(file), img); err != nil {
			return err
		}
	}
	full, _ := Compose(s, pg)
	if err := render.SaveJPEG(s.ImagePath(story.PosterframeFile(pg)), full, render.JPEGQuality); err != nil {
		return err
	}
	w, h := s.PageSize()
	return render.SavePNG(s.ImagePath(story.ThumbnailFile(pg)), render.Resize(full, int(math.Round(120*float64(w)/float64(h))), 120))
}

// RefreshPagesUsing re-renders pages that place a board.
func RefreshPagesUsing(s *story.Scene, uid string) error {
	for _, pg := range s.Pages() {
		for _, p := range Panels(pg) {
			if pl := p.Obj("content"); pl != nil && pl.Str("board") == uid {
				if err := RefreshPage(s, pg); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// Guides overlays a coordinate grid (every step px, labelled) and panel tags
// "K2 #1" (id and reading order) for placing drawings precisely. dst shows the
// page from ox, oy at scale (a crop/zoom); labels stay in page coordinates.
func Guides(dst *image.RGBA, s *story.Scene, pg *ojson.Object, step, ox, oy, scale float64) {
	render.GridAt(dst, step, scale, ox, oy)
	if pg == nil {
		return
	}
	face := render.Face(render.FontBold, 28)
	for _, p := range Panels(pg) {
		box := BBox(EffectivePoints(s, p))
		vw, vh := float64(dst.Bounds().Dx())/scale, float64(dst.Bounds().Dy())/scale
		if box.X+box.W <= ox || box.Y+box.H <= oy || box.X >= ox+vw || box.Y >= oy+vh {
			continue
		}
		// keep the tag inside the view when the panel starts above/left of it
		x, y := math.Max(0, (box.X-ox)*scale), math.Max(0, (box.Y-oy)*scale)
		label := fmt.Sprintf("%s #%d", p.Str("id"), int(p.NumOr("order", 0)))
		tw := render.Measure(face, label)
		r := image.Rect(int(x)+8, int(y)+8, int(x+tw)+24, int(y)+48)
		render.FillRect(dst, r, color.NRGBA{0, 90, 220, 220})
		render.DrawText(dst, face, label, x+16, y+38, color.White, "left")
	}
}

// PageNumber draws the page number centered in the bottom margin.
func PageNumber(dst *image.RGBA, n int) {
	b := dst.Bounds()
	size := math.Max(12, float64(b.Dy())*0.014)
	face := render.Face(render.FontRegular, size)
	render.DrawText(dst, face, fmt.Sprint(n), float64(b.Dx())/2, float64(b.Dy())-float64(b.Dy())*0.035+size/2, color.Black, "center")
}

// RenderSpread puts a page and its facing page side by side in reading order.
// It returns the image and the 1-based page numbers from left to right (0 = blank).
func RenderSpread(s *story.Scene, i int) (*image.RGBA, [2]int) {
	a, b := Spread(s, i)
	w, h := s.PageSize()
	rtl := RTL(s)
	left, right := a, b
	if rtl {
		left, right = b, a
	}
	if b == -1 && a == 0 && Settings(s).Get("firstPageSingle") != false {
		// page 1 alone: a left page in rtl books, a right page in ltr books
		if rtl {
			left, right = 0, -1
		} else {
			left, right = -1, 0
		}
	}
	gap := 4
	dst := render.NewFilled(2*w+gap, h, color.RGBA{200, 200, 200, 255})
	var nums [2]int
	pages := s.Pages()
	for side, idx := range []int{left, right} {
		if idx < 0 || idx >= len(pages) {
			continue
		}
		img, _ := Compose(s, pages[idx])
		stddraw.Draw(dst, image.Rect(side*(w+gap), 0, side*(w+gap)+w, h), img, image.Point{}, stddraw.Src)
		nums[side] = idx + 1
	}
	return dst, nums
}
