package manga

import (
	"fmt"
	"image"
	"math"
	"strconv"
	"strings"

	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/script"
	"sb/internal/story"
)

// DefaultBorder is the panel border width in page pixels.
const DefaultBorder = 6

// Settings returns the page settings object, creating defaults.
func Settings(s *story.Scene) *ojson.Object {
	p := s.Data.Obj("page")
	if p == nil {
		p = ojson.Obj("width", 1414, "height", 2000, "readingDirection", "rtl", "firstPageSingle", true)
		s.Data.Set("page", p)
	}
	return p
}

// RTL reports right-to-left reading.
func RTL(s *story.Scene) bool { return Settings(s).Str("readingDirection") != "ltr" }

// LayoutOf is the default layout for the project's page size.
func LayoutOf(s *story.Scene) Layout {
	w, h := s.PageSize()
	return DefaultLayout(float64(w), float64(h))
}

// MakeManga turns a scene into a manga project (mode, page settings, pages[]).
func MakeManga(s *story.Scene, w, h int, rtl bool) {
	s.Data.Set("mode", "manga")
	dir := "rtl"
	if !rtl {
		dir = "ltr"
	}
	s.Data.Set("page", ojson.Obj("width", w, "height", h, "readingDirection", dir, "firstPageSingle", true))
	if s.Data.Get("pages") == nil {
		s.Data.Set("pages", []any{})
	}
}

// RequireManga errors for film projects.
func RequireManga(s *story.Scene) error {
	if !s.IsManga() {
		return fmt.Errorf("%s is a film project; pages need a manga project (`sb project new <dir> --manga` or `sb project set-mode manga`)", s.Name())
	}
	return nil
}

// NewPage makes an empty page object.
func NewPage() *ojson.Object {
	uid := script.UID()
	return ojson.Obj("id", uid, "url", "page-"+uid+".png", "panels", []any{}, "balloons", []any{}, "panelOrder", "auto", "layers", ojson.New())
}

// FindPage resolves a 1-based page number or a page id.
func FindPage(s *story.Scene, ref string) (int, *ojson.Object, error) {
	pages := s.Pages()
	for i, p := range pages {
		if strings.EqualFold(p.Str("id"), ref) {
			return i, p, nil
		}
	}
	n, err := strconv.Atoi(ref)
	if err != nil || n < 1 || n > len(pages) {
		return 0, nil, fmt.Errorf("no page %q (the project has %d pages)", ref, len(pages))
	}
	return n - 1, pages[n-1], nil
}

// Panels returns a page's panel objects.
func Panels(pg *ojson.Object) []*ojson.Object {
	var out []*ojson.Object
	for _, v := range pg.Arr("panels") {
		if p, ok := v.(*ojson.Object); ok {
			out = append(out, p)
		}
	}
	return out
}

func setPanels(pg *ojson.Object, ps []*ojson.Object) {
	arr := make([]any, len(ps))
	for i, p := range ps {
		arr[i] = p
	}
	pg.Set("panels", arr)
}

// Balloons returns a page's balloons.
func Balloons(pg *ojson.Object) []*ojson.Object {
	var out []*ojson.Object
	for _, v := range pg.Arr("balloons") {
		if b, ok := v.(*ojson.Object); ok {
			out = append(out, b)
		}
	}
	return out
}

func setBalloons(pg *ojson.Object, bs []*ojson.Object) {
	arr := make([]any, len(bs))
	for i, b := range bs {
		arr[i] = b
	}
	pg.Set("balloons", arr)
}

// Points reads a panel's polygon.
func Points(p *ojson.Object) []Pt {
	var out []Pt
	for _, v := range p.Arr("points") {
		if a, ok := v.([]any); ok && len(a) >= 2 {
			x, _ := ojson.Num(a[0])
			y, _ := ojson.Num(a[1])
			out = append(out, Pt{x, y})
		}
	}
	return out
}

func pointsValue(pts []Pt) []any {
	out := make([]any, len(pts))
	for i, p := range pts {
		out[i] = []any{p[0], p[1]}
	}
	return out
}

func nextID(prefix string, objs []*ojson.Object) string {
	n := 0
	for _, o := range objs {
		if v, err := strconv.Atoi(strings.TrimPrefix(o.Str("id"), prefix)); err == nil && v > n {
			n = v
		}
	}
	return prefix + strconv.Itoa(n+1)
}

// FindPanel resolves a panel id ("K2") or its reading-order number.
func FindPanel(pg *ojson.Object, ref string) (*ojson.Object, error) {
	ps := Panels(pg)
	for _, p := range ps {
		if strings.EqualFold(p.Str("id"), ref) {
			return p, nil
		}
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(ref, "#")); err == nil {
		for _, p := range ps {
			if int(p.NumOr("order", 0)) == n {
				return p, nil
			}
		}
	}
	return nil, fmt.Errorf("no panel %q on page %s (see `sb panel list`)", ref, pg.Str("id"))
}

// AddPanel adds a polygon panel.
func AddPanel(s *story.Scene, pg *ojson.Object, pts []Pt) *ojson.Object {
	p := ojson.Obj("id", nextID("K", Panels(pg)), "points", pointsValue(round(pts)), "border", DefaultBorder, "bleed", false, "content", nil)
	setPanels(pg, append(Panels(pg), p))
	Reorder(s, pg)
	return p
}

// ApplyTemplate replaces all panels with a template.
func ApplyTemplate(s *story.Scene, pg *ojson.Object, name string, l Layout) error {
	rects, err := Template(l, name)
	if err != nil {
		return err
	}
	setPanels(pg, nil)
	for _, r := range rects {
		AddPanel(s, pg, RectPoly(r))
	}
	return nil
}

// SplitPanel cuts a panel in two; the first part keeps the id and placement.
func SplitPanel(s *story.Scene, pg, p *ojson.Object, kind string, at, gutter, slant float64) (*ojson.Object, error) {
	a, b, err := Split(Points(p), kind, at, gutter, slant)
	if err != nil {
		return nil, err
	}
	p.Set("points", pointsValue(a))
	n := ojson.Obj("id", nextID("K", Panels(pg)), "points", pointsValue(b), "border", p.Get("border"), "bleed", p.Get("bleed"), "content", nil)
	var out []*ojson.Object
	for _, q := range Panels(pg) {
		out = append(out, q)
		if q == p {
			out = append(out, n)
		}
	}
	setPanels(pg, out)
	Reorder(s, pg)
	return n, nil
}

// MergePanels replaces two panels with the convex hull of both.
func MergePanels(s *story.Scene, pg, a, b *ojson.Object) {
	a.Set("points", pointsValue(Hull(append(Points(a), Points(b)...))))
	var out []*ojson.Object
	for _, q := range Panels(pg) {
		if q != b {
			out = append(out, q)
		}
	}
	setPanels(pg, out)
	for _, bl := range Balloons(pg) {
		if bl.Str("panel") == b.Str("id") {
			bl.Set("panel", a.Str("id"))
		}
	}
	Reorder(s, pg)
}

// DeletePanel removes a panel (balloons keep their page position).
func DeletePanel(s *story.Scene, pg, p *ojson.Object) {
	var out []*ojson.Object
	for _, q := range Panels(pg) {
		if q != p {
			out = append(out, q)
		}
	}
	setPanels(pg, out)
	for _, bl := range Balloons(pg) {
		if bl.Str("panel") == p.Str("id") {
			bl.Delete("panel")
		}
	}
	Reorder(s, pg)
}

// Reorder recomputes panel order unless the page uses a manual order.
func Reorder(s *story.Scene, pg *ojson.Object) {
	ps := Panels(pg)
	if pg.Str("panelOrder") == "manual" {
		// keep manual numbers, append new panels at the end
		maxN := 0
		for _, p := range ps {
			maxN = max(maxN, int(p.NumOr("order", 0)))
		}
		for _, p := range ps {
			if !p.Has("order") {
				maxN++
				p.Set("order", maxN)
			}
		}
		return
	}
	boxes := make([]Rect, len(ps))
	for i, p := range ps {
		boxes[i] = BBox(Points(p))
	}
	for n, i := range ReadingOrder(boxes, RTL(s)) {
		ps[i].Set("order", n+1)
	}
}

// SetManualOrder sets the order from a list of panel ids (the rest follow).
func SetManualOrder(s *story.Scene, pg *ojson.Object, ids []string) error {
	seen := map[*ojson.Object]bool{}
	n := 0
	for _, id := range ids {
		p, err := FindPanel(pg, id)
		if err != nil {
			return err
		}
		n++
		p.Set("order", n)
		seen[p] = true
	}
	pg.Set("panelOrder", "manual")
	for _, p := range InOrder(pg) {
		if !seen[p] {
			n++
			p.Set("order", n)
		}
	}
	return nil
}

// InOrder returns panels sorted by order.
func InOrder(pg *ojson.Object) []*ojson.Object {
	ps := Panels(pg)
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0 && ps[j].NumOr("order", 1e9) < ps[j-1].NumOr("order", 1e9); j-- {
			ps[j], ps[j-1] = ps[j-1], ps[j]
		}
	}
	return ps
}

// EffectivePoints applies bleed: points within the margin snap to the page edge.
func EffectivePoints(s *story.Scene, p *ojson.Object) []Pt {
	pts := Points(p)
	if !p.Bool("bleed") {
		return pts
	}
	l := LayoutOf(s)
	out := make([]Pt, len(pts))
	for i, q := range pts {
		x, y := q[0], q[1]
		if x <= l.MarginX+1 {
			x = 0
		} else if x >= l.W-l.MarginX-1 {
			x = l.W
		}
		if y <= l.MarginY+1 {
			y = 0
		} else if y >= l.H-l.MarginY-1 {
			y = l.H
		}
		out[i] = Pt{x, y}
	}
	return out
}

// PanelArea is a panel's bounding box and clip polygon (for drawing).
func PanelArea(s *story.Scene, p *ojson.Object) (Rect, []Pt) {
	pts := EffectivePoints(s, p)
	return BBox(pts), pts
}

// Spread returns the page indexes (0-based) shown with page i, as
// [right, left] for rtl and [left, right] for ltr; -1 is an empty side.
// With firstPageSingle (default) page 1 stands alone and spreads are
// [3|2], [5|4] ... in rtl (the even page is on the right).
func Spread(s *story.Scene, i int) (a, b int) {
	n := len(s.Pages())
	alone := Settings(s).Get("firstPageSingle") != false
	start := 0
	if alone {
		if i == 0 {
			return 0, -1
		}
		start = 1
	}
	first := start + ((i-start)/2)*2
	second := first + 1
	if second >= n {
		second = -1
	}
	return first, second
}

// ParsePoints reads "x,y x,y ..." or a JSON list.
func ParsePoints(s string) ([]Pt, error) {
	pts, err := story.ParsePolygon(s)
	if err != nil {
		return nil, err
	}
	return pts, nil
}

// ParseRect reads "x,y,w,h".
func ParseRect(s string) (Rect, error) {
	var r Rect
	if n, _ := fmt.Sscanf(strings.ReplaceAll(s, " ", ""), "%g,%g,%g,%g", &r.X, &r.Y, &r.W, &r.H); n != 4 || r.W <= 0 || r.H <= 0 {
		return r, fmt.Errorf("rect must be x,y,w,h with positive size")
	}
	return r, nil
}

// LikeSize is a canvas with the box's aspect whose long side is long.
func LikeSize(box Rect, long float64) (int, int) {
	if box.W >= box.H {
		return int(long), max(1, int(math.Round(long*box.H/box.W)))
	}
	return max(1, int(math.Round(long*box.W/box.H))), int(long)
}

// Resize changes the page size, scaling every panel, balloon and panel
// content offset so layouts keep their proportions (page layer images are
// scaled when next loaded).
func Resize(s *story.Scene, w, h int) {
	ow, oh := s.PageSize()
	sx, sy := float64(w)/float64(ow), float64(h)/float64(oh)
	st := Settings(s)
	st.Set("width", w)
	st.Set("height", h)
	if sx == 1 && sy == 1 {
		return
	}
	for _, pg := range s.Pages() {
		for _, p := range Panels(pg) {
			pts := Points(p)
			for i := range pts {
				pts[i] = Pt{pts[i][0] * sx, pts[i][1] * sy}
			}
			p.Set("points", pointsValue(round(pts)))
			if ct := p.Obj("content"); ct != nil {
				ct.Set("x", ct.NumOr("x", 0)*sx)
				ct.Set("y", ct.NumOr("y", 0)*sy)
			}
		}
		for _, b := range Balloons(pg) {
			b.Set("x", b.NumOr("x", 0)*sx)
			b.Set("y", b.NumOr("y", 0)*sy)
			b.Set("w", b.NumOr("w", 0)*sx)
			b.Set("h", b.NumOr("h", 0)*sy)
			if t, ok := tail(b); ok {
				b.Set("tail", []any{t[0] * sx, t[1] * sy})
			}
		}
	}
}

// Clamp keeps a value inside [lo, hi].
func Clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// ImportPages makes one new page per image, the image on the page reference
// layer at opacity 1, fit inside the page ("fit") or covering it ("fill").
func ImportPages(s *story.Scene, files []string, insertAt int, fit string) ([]*ojson.Object, []string, error) {
	w, h := s.PageSize()
	var added []*ojson.Object
	var failed []string
	for _, f := range files {
		img, err := render.Load(f)
		if err != nil {
			failed = append(failed, f+": "+err.Error())
			continue
		}
		pg := NewPage()
		pages := s.Pages()
		pos := min(insertAt, len(pages))
		s.SetPages(append(pages[:pos], append([]*ojson.Object{pg}, pages[pos:]...)...))
		var fitted *image.RGBA
		if fit == "fill" {
			fitted = render.CoverImage(img, w, h)
		} else {
			fitted = render.FitImage(img, w, h)
		}
		if err := story.SaveLayer(s, pg, "reference", fitted); err != nil {
			return added, failed, err
		}
		story.Layer(pg, "reference").Set("opacity", 1.0)
		if err := RefreshPage(s, pg); err != nil {
			return added, failed, err
		}
		added = append(added, pg)
		insertAt++
	}
	return added, failed, nil
}

// ImportIntoPanel puts an image on the page reference layer, fitted inside a
// panel's box and clipped to its shape (replacing what was there), and sets
// the page reference layer to 75 % opacity.
func ImportIntoPanel(s *story.Scene, pg, p *ojson.Object, file string) error {
	img, err := render.Load(file)
	if err != nil {
		return err
	}
	box, clip := PanelArea(s, p)
	piece := render.FitImage(img, int(box.W), int(box.H))
	w, h := s.PageSize()
	layer := story.LoadLayer(s, pg, "reference")
	mask := render.PolygonMask(w, h, clip)
	render.Erase(layer, mask)
	overlay := render.New(w, h)
	render.DrawOver(overlay, image.Pt(int(box.X), int(box.Y)), piece, 1)
	for i := 3; i < len(overlay.Pix); i += 4 {
		m := uint32(mask.Pix[i/4])
		for c := 0; c < 4; c++ {
			overlay.Pix[i-3+c] = uint8(uint32(overlay.Pix[i-3+c]) * m / 255)
		}
	}
	render.DrawOver(layer, image.Point{}, overlay, 1)
	if err := story.SaveLayer(s, pg, "reference", layer); err != nil {
		return err
	}
	story.Layer(pg, "reference").Set("opacity", story.DefaultReferenceOpacity)
	return RefreshPage(s, pg)
}
