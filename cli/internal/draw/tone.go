package draw

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"sb/internal/render"
)

// ToneSpec is a screentone: a dot or line pattern whose ink coverage is
// Density (0-1), optionally ramping to DensityTo along Gradient.
type ToneSpec struct {
	Pattern   string  // dots | lines | crosshatch
	Spacing   float64 // px between dot centers / line centers
	Density   float64
	DensityTo float64
	Gradient  *[4]float64 // x1,y1,x2,y2 (area-local); nil = flat
	Angle     float64     // degrees
	Color     color.RGBA
}

// Patterns are the screentone patterns.
var Patterns = []string{"dots", "lines", "crosshatch"}

// Tone fills the polygon (area-local points) with a screentone. Coverage is
// exact on average: a dot of radius s*sqrt(d/pi) covers d of its cell (above
// 0.5, white dots of the remaining area sit between them), a line
// of width s*d covers d, crosshatch lines of width s*(1-sqrt(1-d)) cover d
// together. Edges are antialiased over one pixel.
func Tone(layerW, layerH int, a Area, poly [][2]float64, t ToneSpec) (*image.RGBA, error) {
	ok := false
	for _, p := range Patterns {
		ok = ok || p == t.Pattern
	}
	if !ok {
		return nil, fmt.Errorf("pattern must be dots, lines or crosshatch")
	}
	if t.Spacing < 2 {
		return nil, fmt.Errorf("spacing must be at least 2 px")
	}
	for _, d := range []float64{t.Density, t.DensityTo} {
		if d < 0 || d > 1 {
			return nil, fmt.Errorf("density must be between 0 and 1")
		}
	}
	pts := make([][2]float64, len(poly))
	for i, p := range poly {
		pts[i] = [2]float64{p[0] + a.X, p[1] + a.Y}
	}
	mask := render.PolygonMask(layerW, layerH, pts)
	sin, cos := math.Sincos(t.Angle * math.Pi / 180)
	var gx, gy, gdx, gdy, gl2 float64
	if g := t.Gradient; g != nil {
		gx, gy = g[0]+a.X, g[1]+a.Y
		gdx, gdy = g[2]-g[0], g[3]-g[1]
		gl2 = gdx*gdx + gdy*gdy
	}
	s := t.Spacing
	// distance from v to the nearest multiple of s
	off := func(v float64) float64 { return math.Abs(v - s*math.Round(v/s)) }
	cover := func(halfWidth, dist float64) float64 { return math.Max(0, math.Min(1, halfWidth-dist+0.5)) }
	overlay := render.New(layerW, layerH)
	b := mask.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			m := mask.Pix[mask.PixOffset(x, y)]
			if m == 0 {
				continue
			}
			px, py := float64(x)+0.5, float64(y)+0.5
			d := t.Density
			if gl2 > 0 {
				k := math.Max(0, math.Min(1, ((px-gx)*gdx+(py-gy)*gdy)/gl2))
				d += (t.DensityTo - d) * k
			}
			if d <= 0 {
				continue
			}
			u, v := px*cos+py*sin, -px*sin+py*cos
			var c float64
			switch t.Pattern {
			case "dots":
				if d <= 0.5 {
					c = cover(s*math.Sqrt(d/math.Pi), math.Hypot(off(u), off(v)))
				} else { // dark tones: white dots between the black ones' centers
					c = 1 - cover(s*math.Sqrt((1-d)/math.Pi), math.Hypot(off(u+s/2), off(v+s/2)))
				}
			case "lines":
				c = cover(s*d/2, off(v))
			case "crosshatch":
				w := s * (1 - math.Sqrt(1-d)) / 2
				c = math.Max(cover(w, off(u)), cover(w, off(v)))
			}
			if d >= 1 {
				c = 1
			}
			al := c * float64(m) / 255
			i := overlay.PixOffset(x, y)
			overlay.Pix[i+0] = uint8(float64(t.Color.R)*al + 0.5)
			overlay.Pix[i+1] = uint8(float64(t.Color.G)*al + 0.5)
			overlay.Pix[i+2] = uint8(float64(t.Color.B)*al + 0.5)
			overlay.Pix[i+3] = uint8(255*al + 0.5)
		}
	}
	return overlay, nil
}
