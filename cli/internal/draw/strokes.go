package draw

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"

	"golang.org/x/image/vector"

	"sb/internal/render"
)

// Tool is a drawing tool's defaults (shared/reducers/toolbar.js initialState).
type Tool struct {
	Name    string
	Color   color.RGBA
	Size    float64
	Opacity float64 // nodeOpacity: the stroke's alpha
	Layer   string
	Grain   bool // pencil-like texture
	Erase   bool
}

func rgb(n uint32) color.RGBA { return color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 255} }

// Tools are the original toolbar tools with their default layer.
var Tools = map[string]Tool{
	"light-pencil": {"light-pencil", rgb(0x90CBF9), 20, 0.25, "reference", true, false},
	"brush":        {"brush", rgb(0x90CBF9), 26, 0.7, "fill", false, false},
	"tone":         {"tone", rgb(0x162A3F), 50, 0.15, "tone", false, false},
	"pencil":       {"pencil", rgb(0x121212), 4, 0.45, "pencil", true, false},
	"pen":          {"pen", rgb(0x000000), 2, 0.9, "ink", false, false},
	"note-pen":     {"note-pen", rgb(0xF44336), 8, 0.9, "notes", false, false},
	"eraser":       {"eraser", rgb(0xFFFFFF), 26, 1, "", false, true},
}

// Stroke is one line of points [x, y] or [x, y, pressure 0-1].
type Stroke struct {
	Points  [][]float64 `json:"points"`
	Color   string      `json:"color,omitempty"`
	Size    float64     `json:"size,omitempty"`
	Opacity *float64    `json:"opacity,omitempty"`
}

// StrokeDoc is the JSON accepted by `draw strokes`.
type StrokeDoc struct {
	Tool    string   `json:"tool,omitempty"`
	Color   string   `json:"color,omitempty"`
	Size    float64  `json:"size,omitempty"`
	Opacity *float64 `json:"opacity,omitempty"`
	Smooth  *bool    `json:"smooth,omitempty"`
	Strokes []Stroke `json:"strokes"`
}

// ParseStrokes accepts {"strokes": [...]}, a list of strokes, or a list of point lists.
func ParseStrokes(data []byte) (StrokeDoc, error) {
	var doc StrokeDoc
	if err := json.Unmarshal(data, &doc); err == nil && doc.Strokes != nil {
		return doc, nil
	}
	if err := json.Unmarshal(data, &doc.Strokes); err == nil && len(doc.Strokes) > 0 && doc.Strokes[0].Points != nil {
		return doc, nil
	}
	var lists [][][]float64
	if err := json.Unmarshal(data, &lists); err != nil {
		return doc, fmt.Errorf(`strokes JSON must be {"strokes":[{"points":[[x,y,pressure],...]}]}, a list of strokes, or a list of point lists: %v`, err)
	}
	doc.Strokes = nil
	for _, l := range lists {
		doc.Strokes = append(doc.Strokes, Stroke{Points: l})
	}
	return doc, nil
}

type pt struct{ x, y, p float64 }

// smooth runs Catmull-Rom through the points (pressure interpolated too).
func smooth(in []pt) []pt {
	if len(in) < 3 {
		return in
	}
	var out []pt
	at := func(i int) pt { return in[max(0, min(len(in)-1, i))] }
	cr := func(a, b, c, d, t float64) float64 {
		return 0.5 * (2*b + (-a+c)*t + (2*a-5*b+4*c-d)*t*t + (-a+3*b-3*c+d)*t*t*t)
	}
	for i := 0; i < len(in)-1; i++ {
		p0, p1, p2, p3 := at(i-1), at(i), at(i+1), at(i+2)
		seg := math.Hypot(p2.x-p1.x, p2.y-p1.y)
		steps := max(1, min(24, int(seg/3)))
		for s := 0; s < steps; s++ {
			t := float64(s) / float64(steps)
			out = append(out, pt{cr(p0.x, p1.x, p2.x, p3.x, t), cr(p0.y, p1.y, p2.y, p3.y, t), p1.p + (p2.p-p1.p)*t})
		}
	}
	return append(out, in[len(in)-1])
}

// grain is a stable per-pixel texture in [0.55, 1] for pencil tools.
func grain(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ (h >> 13)) * 1274126177
	return 0.55 + 0.45*float64(h>>24)/255
}

func addCircle(z *vector.Rasterizer, cx, cy, r float64) {
	const n = 16
	z.MoveTo(float32(cx+r), float32(cy))
	for i := 1; i < n; i++ {
		a := 2 * math.Pi * float64(i) / n
		z.LineTo(float32(cx+r*math.Cos(a)), float32(cy+r*math.Sin(a)))
	}
	z.ClosePath()
}

// quad adds a counter-clockwise quad (consistent winding so shapes union).
func addQuad(z *vector.Rasterizer, q [4][2]float64) {
	area := 0.0
	for i := 0; i < 4; i++ {
		j := (i + 1) % 4
		area += q[i][0]*q[j][1] - q[j][0]*q[i][1]
	}
	if area < 0 {
		q[1], q[3] = q[3], q[1]
	}
	z.MoveTo(float32(q[0][0]), float32(q[0][1]))
	for _, p := range q[1:] {
		z.LineTo(float32(p[0]), float32(p[1]))
	}
	z.ClosePath()
}

// StrokeMask rasterizes one stroke whose width is size x pressure at each point.
func StrokeMask(w, h int, pts [][]float64, size float64, doSmooth bool) *image.Alpha {
	var in []pt
	for _, p := range pts {
		if len(p) < 2 {
			continue
		}
		pr := 1.0
		if len(p) > 2 {
			pr = math.Max(0, math.Min(1, p[2]))
		}
		in = append(in, pt{p[0], p[1], pr})
	}
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	if len(in) == 0 {
		return mask
	}
	if doSmooth {
		in = smooth(in)
	}
	radius := func(p pt) float64 { return math.Max(0.35, size*p.p/2) }
	z := vector.NewRasterizer(w, h)
	for i, p := range in {
		// the circle orientation must match the quads (counter-clockwise in y-down space)
		addCircle(z, p.x, p.y, radius(p))
		if i == 0 {
			continue
		}
		a, b := in[i-1], p
		dx, dy := b.x-a.x, b.y-a.y
		l := math.Hypot(dx, dy)
		if l == 0 {
			continue
		}
		nx, ny := -dy/l, dx/l
		ra, rb := radius(a), radius(b)
		addQuad(z, [4][2]float64{{a.x + nx*ra, a.y + ny*ra}, {b.x + nx*rb, b.y + ny*rb}, {b.x - nx*rb, b.y - ny*rb}, {a.x - nx*ra, a.y - ny*ra}})
	}
	z.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	return mask
}

// Strokes renders a stroke document into an overlay (area-local coordinates).
// It returns the overlay and whether it is an eraser pass.
func Strokes(layerW, layerH int, a Area, doc StrokeDoc, tool Tool) (*image.RGBA, error) {
	overlay := render.New(layerW, layerH)
	doSmooth := doc.Smooth == nil || *doc.Smooth
	for i, s := range doc.Strokes {
		size := tool.Size
		if doc.Size > 0 {
			size = doc.Size
		}
		if s.Size > 0 {
			size = s.Size
		}
		col := tool.Color
		for _, c := range []string{doc.Color, s.Color} {
			if c != "" {
				pc, err := render.ParseColor(c)
				if err != nil {
					return nil, fmt.Errorf("stroke %d: %v", i+1, err)
				}
				col = pc
			}
		}
		op := tool.Opacity
		if doc.Opacity != nil {
			op = *doc.Opacity
		}
		if s.Opacity != nil {
			op = *s.Opacity
		}
		pts := make([][]float64, len(s.Points))
		for k, p := range s.Points {
			q := append([]float64(nil), p...)
			if len(q) >= 2 {
				q[0] += a.X
				q[1] += a.Y
			}
			pts[k] = q
		}
		mask := StrokeMask(layerW, layerH, pts, size, doSmooth)
		if tool.Grain {
			b := mask.Bounds()
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					i := mask.PixOffset(x, y)
					if mask.Pix[i] != 0 {
						mask.Pix[i] = uint8(float64(mask.Pix[i]) * grain(x, y))
					}
				}
			}
		}
		render.FillMask(overlay, mask, col, op)
	}
	return overlay, nil
}
