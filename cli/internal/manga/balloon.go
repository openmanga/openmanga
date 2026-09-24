package manga

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"golang.org/x/image/vector"

	"sb/internal/draw"
	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/story"
)

// BalloonTypes are the supported balloon shapes.
var BalloonTypes = []string{"speech", "thought", "shout", "whisper", "narration", "sfx"}

// DefaultFontSize for balloon text at a 2000 px page height.
const DefaultFontSize = 32

// Balloon field access.
func bbox(b *ojson.Object) Rect {
	return Rect{b.NumOr("x", 0), b.NumOr("y", 0), b.NumOr("w", 0), b.NumOr("h", 0)}
}

func tail(b *ojson.Object) (Pt, bool) {
	a := b.Arr("tail")
	if len(a) < 2 {
		return Pt{}, false
	}
	x, _ := ojson.Num(a[0])
	y, _ := ojson.Num(a[1])
	return Pt{x, y}, true
}

// FindBalloon resolves a balloon id.
func FindBalloon(pg *ojson.Object, id string) (*ojson.Object, error) {
	for _, b := range Balloons(pg) {
		if strings.EqualFold(b.Str("id"), id) {
			return b, nil
		}
	}
	return nil, fmt.Errorf("no balloon %q on page %s", id, pg.Str("id"))
}

func validType(t string) bool {
	for _, x := range BalloonTypes {
		if x == t {
			return true
		}
	}
	return false
}

// textSpec is the text layout of a balloon (centered in its box).
func textSpec(b *ojson.Object) draw.TextSpec {
	r := bbox(b)
	size := b.NumOr("fontSize", DefaultFontSize)
	t := draw.TextSpec{Text: b.Str("text"), Size: size, Font: "regular", Color: color.Black, Align: "center", Vertical: b.Bool("vertical")}
	inset := 0.72 // text fits inside the ellipse
	switch b.Str("type") {
	case "narration":
		inset = 0.88
	case "sfx":
		t.Font = "bold"
		inset = 1
	case "shout":
		t.Font = "bold"
	}
	if t.Vertical {
		t.Width = math.Max(1, math.Floor(r.H*inset/(size*1.05)))
	} else {
		t.Width = r.W * inset
	}
	return t
}

// AutoSize returns a box that fits the text.
func AutoSize(b *ojson.Object) (float64, float64) {
	t := textSpec(b)
	face := render.FallbackFace(render.FontRegular, t.Size)
	var tw, th float64
	if t.Vertical {
		cols := 0
		longest := 0
		for _, p := range strings.Split(t.Text, "\n") {
			n := len([]rune(p))
			longest = max(longest, n)
			cols++
		}
		longest = min(longest, 12)
		th = float64(longest) * t.Size * 1.05
		tw = float64(cols) * t.Size * 1.2
		if len([]rune(t.Text)) > longest*cols {
			tw = math.Ceil(float64(len([]rune(t.Text)))/float64(longest)) * t.Size * 1.2
		}
	} else {
		lines := draw.WrapText(face, t.Text, t.Size*9)
		for _, l := range lines {
			tw = math.Max(tw, render.Measure(face, l))
		}
		th = float64(len(lines)) * t.Size * 1.25
	}
	switch b.Str("type") {
	case "narration":
		return math.Ceil(tw + t.Size*1.2), math.Ceil(th + t.Size)
	case "sfx":
		return math.Ceil(tw + t.Size), math.Ceil(th + t.Size*0.5)
	}
	return math.Ceil(tw*1.45 + t.Size), math.Ceil(th*1.45 + t.Size)
}

// AddBalloon creates a balloon. x, y, w, h, tail are page coordinates; w/h 0 = auto.
func AddBalloon(pg *ojson.Object, typ, text string, x, y, w, h, size float64, vertical bool, tl *Pt, panel string) (*ojson.Object, error) {
	if !validType(typ) {
		return nil, fmt.Errorf("balloon type must be one of %s", strings.Join(BalloonTypes, ", "))
	}
	if size <= 0 {
		size = DefaultFontSize
	}
	b := ojson.Obj("id", nextID("B", Balloons(pg)), "type", typ, "text", text, "fontSize", size, "vertical", vertical)
	if w <= 0 || h <= 0 {
		aw, ah := AutoSize(b)
		if w <= 0 {
			w = aw
		}
		if h <= 0 {
			h = ah
		}
	}
	b.Set("x", x)
	b.Set("y", y)
	b.Set("w", w)
	b.Set("h", h)
	if tl != nil {
		b.Set("tail", []any{tl[0], tl[1]})
	}
	if panel != "" {
		b.Set("panel", panel)
	}
	setBalloons(pg, append(Balloons(pg), b))
	return b, nil
}

// DeleteBalloon removes a balloon.
func DeleteBalloon(pg, b *ojson.Object) {
	var out []*ojson.Object
	for _, x := range Balloons(pg) {
		if x != b {
			out = append(out, x)
		}
	}
	setBalloons(pg, out)
}

// ---- rendering ----

type shape struct {
	z *vector.Rasterizer
}

func newShape(w, h int) *shape { return &shape{vector.NewRasterizer(w, h)} }

func (s *shape) poly(pts []Pt) {
	if len(pts) < 3 {
		return
	}
	// consistent winding so overlapping parts union
	area := 0.0
	for i := range pts {
		j := (i + 1) % len(pts)
		area += pts[i][0]*pts[j][1] - pts[j][0]*pts[i][1]
	}
	if area < 0 {
		r := make([]Pt, len(pts))
		for i := range pts {
			r[i] = pts[len(pts)-1-i]
		}
		pts = r
	}
	s.z.MoveTo(float32(pts[0][0]), float32(pts[0][1]))
	for _, p := range pts[1:] {
		s.z.LineTo(float32(p[0]), float32(p[1]))
	}
	s.z.ClosePath()
}

func ellipsePts(cx, cy, rx, ry float64, n int) []Pt {
	pts := make([]Pt, n)
	for i := range pts {
		a := 2 * math.Pi * float64(i) / float64(n)
		pts[i] = Pt{cx + rx*math.Cos(a), cy + ry*math.Sin(a)}
	}
	return pts
}

func (s *shape) fill(dst *image.RGBA, c color.Color) {
	mask := image.NewAlpha(dst.Bounds())
	s.z.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	render.FillMask(dst, mask, c, 1)
}

// outline builds a balloon's shapes grown by `grow` (for the black border pass).
func outline(b *ojson.Object, w, h int, grow float64) *shape {
	s := newShape(w, h)
	r := bbox(b)
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	rx, ry := r.W/2+grow, r.H/2+grow
	t, hasTail := tail(b)
	switch b.Str("type") {
	case "narration":
		s.poly(RectPoly(Rect{r.X - grow, r.Y - grow, r.W + 2*grow, r.H + 2*grow}))
		return s
	case "shout":
		n := 18
		pts := make([]Pt, 2*n)
		for i := range pts {
			a := math.Pi * float64(i) / float64(n)
			k := 1.18
			if i%2 == 1 {
				k = 0.92
			}
			pts[i] = Pt{cx + (r.W/2*k+grow)*math.Cos(a), cy + (r.H/2*k+grow)*math.Sin(a)}
		}
		s.poly(pts)
	case "thought":
		s.poly(ellipsePts(cx, cy, rx*0.92, ry*0.92, 64))
		bump := math.Min(r.W, r.H) * 0.16
		for i := 0; i < 14; i++ {
			a := 2 * math.Pi * float64(i) / 14
			s.poly(ellipsePts(cx+r.W/2*0.9*math.Cos(a), cy+r.H/2*0.9*math.Sin(a), bump+grow, bump+grow, 24))
		}
		if hasTail {
			for k, f := range []float64{0.35, 0.6, 0.82} {
				px, py := cx+(t[0]-cx)*(0.55+f*0.45), cy+(t[1]-cy)*(0.55+f*0.45)
				rad := bump * (0.7 - 0.18*float64(k))
				s.poly(ellipsePts(px, py, rad+grow, rad+grow, 20))
			}
		}
		return s
	default: // speech, whisper
		s.poly(ellipsePts(cx, cy, rx, ry, 96))
	}
	if hasTail {
		// tail base on the ellipse, pointing at the target
		ang := math.Atan2((t[1]-cy)/(r.H/2), (t[0]-cx)/(r.W/2))
		inside := math.Pow((t[0]-cx)/(r.W/2), 2)+math.Pow((t[1]-cy)/(r.H/2), 2) <= 1
		if !inside {
			d := 0.3
			b1 := Pt{cx + r.W/2*0.85*math.Cos(ang-d), cy + r.H/2*0.85*math.Sin(ang-d)}
			b2 := Pt{cx + r.W/2*0.85*math.Cos(ang+d), cy + r.H/2*0.85*math.Sin(ang+d)}
			tip := t
			if grow > 0 {
				mx, my := (b1[0]+b2[0])/2, (b1[1]+b2[1])/2
				l := math.Hypot(tip[0]-mx, tip[1]-my)
				tip = Pt{tip[0] + (tip[0]-mx)/l*grow*1.5, tip[1] + (tip[1]-my)/l*grow*1.5}
				b1 = Pt{b1[0] + (b1[0]-b2[0])/2*0.2, b1[1] + (b1[1]-b2[1])/2*0.2}
				b2 = Pt{b2[0] + (b2[0]-b1[0])/2*0.2, b2[1] + (b2[1]-b1[1])/2*0.2}
			}
			s.poly([]Pt{b1, tip, b2})
		}
	}
	return s
}

// strokeWidth scales balloon lines with the text size.
func strokeWidth(b *ojson.Object) float64 {
	return math.Max(2, b.NumOr("fontSize", DefaultFontSize)/10)
}

// RenderBalloons draws all balloons of a page on a transparent canvas.
func RenderBalloons(s *story.Scene, pg *ojson.Object) *image.RGBA {
	w, h := s.PageSize()
	dst := render.New(w, h)
	for _, b := range Balloons(pg) {
		sw := strokeWidth(b)
		switch b.Str("type") {
		case "sfx":
			t := textSpec(b)
			r := bbox(b)
			t.X, t.Y = r.X+r.W/2, r.Y
			if t.Vertical {
				t.X = r.X + r.W - t.Size*0.6
			}
			// white outline under black letters
			grown := render.New(w, h)
			for _, off := range [][2]float64{{-3, 0}, {3, 0}, {0, -3}, {0, 3}, {-2, -2}, {2, 2}, {-2, 2}, {2, -2}} {
				tt := t
				tt.X += off[0]
				tt.Y += off[1]
				draw.DrawText(grown, withColor(tt, color.White))
			}
			render.DrawOver(dst, image.Point{}, grown, 1)
			draw.DrawText(dst, t)
			continue
		case "whisper":
			outline(b, w, h, 0).fill(dst, color.White)
			dashes(dst, b, sw)
		default:
			outline(b, w, h, sw).fill(dst, color.Black)
			outline(b, w, h, 0).fill(dst, color.White)
		}
		drawBalloonText(dst, b)
	}
	return dst
}

func withColor(t draw.TextSpec, c color.Color) draw.TextSpec { t.Color = c; return t }

func dashes(dst *image.RGBA, b *ojson.Object, sw float64) {
	r := bbox(b)
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	n := 36
	for i := 0; i < n; i += 2 {
		a0, a1 := 2*math.Pi*float64(i)/float64(n), 2*math.Pi*float64(i+1)/float64(n)
		pts := [][]float64{{cx + r.W/2*math.Cos(a0), cy + r.H/2*math.Sin(a0)}, {cx + r.W/2*math.Cos(a1), cy + r.H/2*math.Sin(a1)}}
		render.FillMask(dst, draw.StrokeMask(w, h, pts, sw, false), color.Black, 1)
	}
}

func drawBalloonText(dst *image.RGBA, b *ojson.Object) {
	t := textSpec(b)
	r := bbox(b)
	if t.Vertical {
		cols := strings.Count(t.Text, "\n") + 1
		per := int(t.Width)
		if per > 0 {
			cols = 0
			for _, p := range strings.Split(t.Text, "\n") {
				cols += max(1, int(math.Ceil(float64(len([]rune(p)))/float64(per))))
			}
		}
		blockW := float64(cols) * t.Size * 1.2
		t.X = r.X + r.W/2 + blockW/2 - t.Size*0.6
		longest := math.Min(t.Width, float64(maxRunes(t.Text)))
		t.Y = r.Y + r.H/2 - longest*t.Size*1.05/2
	} else {
		face := render.FallbackFace(render.FontRegular, t.Size)
		lines := draw.WrapText(face, t.Text, t.Width)
		t.X = r.X + r.W/2
		t.Y = r.Y + r.H/2 - float64(len(lines))*t.Size*1.25/2
	}
	draw.DrawText(dst, t)
}

func maxRunes(s string) int {
	m := 0
	for _, p := range strings.Split(s, "\n") {
		m = max(m, len([]rune(p)))
	}
	return m
}
