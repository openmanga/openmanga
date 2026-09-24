// Package manga implements manga pages: panels (koma) as polygons, balloons,
// reading order, spreads, and page renders. Pages live in the .storyboarder
// next to the boards (see docs/manga-format.md).
package manga

import (
	"fmt"
	"math"
	"sort"
)

// Pt is a point in page pixels.
type Pt = [2]float64

// Rect is an axis-aligned rectangle.
type Rect struct{ X, Y, W, H float64 }

func RectPoly(r Rect) []Pt {
	return []Pt{{r.X, r.Y}, {r.X + r.W, r.Y}, {r.X + r.W, r.Y + r.H}, {r.X, r.Y + r.H}}
}

// BBox of a polygon.
func BBox(pts []Pt) Rect {
	if len(pts) == 0 {
		return Rect{}
	}
	x0, y0, x1, y1 := pts[0][0], pts[0][1], pts[0][0], pts[0][1]
	for _, p := range pts[1:] {
		x0, y0 = math.Min(x0, p[0]), math.Min(y0, p[1])
		x1, y1 = math.Max(x1, p[0]), math.Max(y1, p[1])
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// clipHalf keeps the part of a polygon where a*x + b*y <= c (Sutherland–Hodgman).
func clipHalf(pts []Pt, a, b, c float64) []Pt {
	var out []Pt
	in := func(p Pt) bool { return a*p[0]+b*p[1] <= c+1e-9 }
	for i := range pts {
		cur, next := pts[i], pts[(i+1)%len(pts)]
		if in(cur) {
			out = append(out, cur)
		}
		if in(cur) != in(next) {
			d1 := a*cur[0] + b*cur[1] - c
			d2 := a*next[0] + b*next[1] - c
			t := d1 / (d1 - d2)
			out = append(out, Pt{cur[0] + (next[0]-cur[0])*t, cur[1] + (next[1]-cur[1])*t})
		}
	}
	return round(out)
}

func round(pts []Pt) []Pt {
	for i := range pts {
		pts[i] = Pt{math.Round(pts[i][0]*100) / 100, math.Round(pts[i][1]*100) / 100}
	}
	return pts
}

// Split cuts a polygon in two with a gutter. kind is horizontal (a horizontal
// cut: top and bottom), vertical (left and right) or diagonal (a cut rising
// from bottom-left to top-right by slant x height). at is the cut position
// as a fraction of the bounding box (0-1).
func Split(pts []Pt, kind string, at, gutter, slant float64) ([]Pt, []Pt, error) {
	b := BBox(pts)
	g := gutter / 2
	var p1, p2 []Pt
	switch kind {
	case "horizontal":
		y := b.Y + at*b.H
		p1 = clipHalf(pts, 0, 1, y-g)
		p2 = clipHalf(pts, 0, -1, -(y + g))
	case "vertical":
		x := b.X + at*b.W
		p1 = clipHalf(pts, 1, 0, x-g)
		p2 = clipHalf(pts, -1, 0, -(x + g))
	case "diagonal":
		// line through (x, y) with direction (w, -slant*h)
		cx, cy := b.X+at*b.W, b.Y+b.H/2
		dx, dy := b.W, -slant*b.H
		l := math.Hypot(dx, dy)
		nx, ny := -dy/l, dx/l // normal pointing down-right side
		c := nx*cx + ny*cy
		p1 = clipHalf(pts, nx, ny, c-g)
		p2 = clipHalf(pts, -nx, -ny, -(c + g))
	default:
		return nil, nil, fmt.Errorf("split must be horizontal, vertical or diagonal")
	}
	if len(p1) < 3 || len(p2) < 3 {
		return nil, nil, fmt.Errorf("the cut leaves an empty panel (check --at and --gutter)")
	}
	return p1, p2, nil
}

// Hull is the convex hull of points (monotone chain), clockwise in y-down space.
func Hull(pts []Pt) []Pt {
	p := append([]Pt(nil), pts...)
	sort.Slice(p, func(i, j int) bool {
		if p[i][0] != p[j][0] {
			return p[i][0] < p[j][0]
		}
		return p[i][1] < p[j][1]
	})
	cross := func(o, a, b Pt) float64 { return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0]) }
	var lower, upper []Pt
	for _, q := range p {
		for len(lower) >= 2 && cross(lower[len(lower)-2], lower[len(lower)-1], q) <= 0 {
			lower = lower[:len(lower)-1]
		}
		lower = append(lower, q)
	}
	for i := len(p) - 1; i >= 0; i-- {
		q := p[i]
		for len(upper) >= 2 && cross(upper[len(upper)-2], upper[len(upper)-1], q) <= 0 {
			upper = upper[:len(upper)-1]
		}
		upper = append(upper, q)
	}
	return append(lower[:len(lower)-1], upper[:len(upper)-1]...)
}

// Layout is the page geometry used by templates.
type Layout struct {
	W, H             float64
	MarginX, MarginY float64
	GutterX, GutterY float64 // between side-by-side panels / between tiers
}

// DefaultLayout derives margins and gutters from the page size.
func DefaultLayout(w, h float64) Layout {
	return Layout{W: w, H: h, MarginX: math.Round(w * 0.08), MarginY: math.Round(h * 0.07), GutterX: math.Round(w * 0.017), GutterY: math.Round(h * 0.024)}
}

// Live is the area inside the margins.
func (l Layout) Live() Rect {
	return Rect{l.MarginX, l.MarginY, l.W - 2*l.MarginX, l.H - 2*l.MarginY}
}

func (l Layout) rows(area Rect, weights []float64, cols []int) [][]Rect {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	avail := area.H - l.GutterY*float64(len(weights)-1)
	y := area.Y
	var out [][]Rect
	for i, w := range weights {
		h := avail * w / total
		n := cols[i]
		cw := (area.W - l.GutterX*float64(n-1)) / float64(n)
		var row []Rect
		for c := 0; c < n; c++ {
			row = append(row, Rect{math.Round(area.X + float64(c)*(cw+l.GutterX)), math.Round(y), math.Round(cw), math.Round(h)})
		}
		out = append(out, row)
		y += h + l.GutterY
	}
	return out
}

// Templates lists the template names.
var Templates = []string{"splash", "2-tier", "3-tier", "4-koma", "big-plus-2", "grid RxC (e.g. grid 3x2)"}

// Template returns panel rectangles for a template name.
func Template(l Layout, name string) ([]Rect, error) {
	live := l.Live()
	var rows [][]Rect
	switch name {
	case "splash":
		rows = [][]Rect{{live}}
	case "2-tier":
		rows = l.rows(live, []float64{1, 1}, []int{1, 1})
	case "3-tier":
		rows = l.rows(live, []float64{1, 1, 1}, []int{1, 1, 1})
	case "4-koma":
		rows = l.rows(live, []float64{1, 1, 1, 1}, []int{1, 1, 1, 1})
	case "big-plus-2":
		rows = l.rows(live, []float64{3, 2}, []int{1, 2})
	default:
		var r, c int
		if n, _ := fmt.Sscanf(name, "grid %dx%d", &r, &c); n != 2 {
			if n, _ := fmt.Sscanf(name, "grid-%dx%d", &r, &c); n != 2 {
				return nil, fmt.Errorf("unknown template %q (splash, 2-tier, 3-tier, 4-koma, big-plus-2, grid RxC)", name)
			}
		}
		if r < 1 || c < 1 || r > 12 || c > 12 {
			return nil, fmt.Errorf("grid rows and columns must be 1-12")
		}
		w := make([]float64, r)
		cols := make([]int, r)
		for i := range w {
			w[i], cols[i] = 1, c
		}
		rows = l.rows(live, w, cols)
	}
	var out []Rect
	for _, row := range rows {
		out = append(out, row...)
	}
	return out, nil
}

// ReadingOrder sorts panel boxes into reading order: rows top to bottom, then
// right to left (rtl) or left to right (ltr) within a row. It returns indexes.
func ReadingOrder(boxes []Rect, rtl bool) []int {
	idx := make([]int, len(boxes))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return boxes[idx[a]].Y < boxes[idx[b]].Y })
	var rows [][]int
	for _, i := range idx {
		c := boxes[i].Y + boxes[i].H/2
		placed := false
		for r := range rows {
			top := boxes[rows[r][0]]
			if c >= top.Y && c <= top.Y+top.H {
				rows[r] = append(rows[r], i)
				placed = true
				break
			}
		}
		if !placed {
			rows = append(rows, []int{i})
		}
	}
	var out []int
	for _, row := range rows {
		sort.SliceStable(row, func(a, b int) bool {
			if rtl {
				return boxes[row[a]].X+boxes[row[a]].W > boxes[row[b]].X+boxes[row[b]].W
			}
			return boxes[row[a]].X < boxes[row[b]].X
		})
		out = append(out, row...)
	}
	return out
}
