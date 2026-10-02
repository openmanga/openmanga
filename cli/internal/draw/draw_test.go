package draw

import (
	"errors"
	"image"
	"image/color"
	"math"
	"testing"

	"sb/internal/render"
)

func newFilled(w, h int, c color.Color) *image.RGBA { return render.NewFilled(w, h, c) }

func TestSVGPixels(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect x="10" y="20" width="30" height="40" fill="#ff0000"/><path d="M 60 10 L 90 10" stroke="#0000ff" stroke-width="6"/></svg>`)
	img, err := SVG(svg, 100, 100, FullArea(100, 100))
	if err != nil {
		t.Fatal(err)
	}
	if c := img.RGBAAt(25, 40); c.R < 250 || c.A < 250 || c.G > 5 {
		t.Errorf("inside rect %v", c)
	}
	if c := img.RGBAAt(5, 5); c.A != 0 {
		t.Errorf("outside should be transparent, got %v", c)
	}
	if c := img.RGBAAt(75, 10); c.B < 250 || c.A < 250 {
		t.Errorf("stroke %v", c)
	}
	// with an area offset (panel-local coordinates)
	img, _ = SVG(svg, 200, 200, Area{X: 100, Y: 100, W: 100, H: 100})
	if c := img.RGBAAt(125, 140); c.R < 250 || c.A < 250 {
		t.Errorf("offset rect %v", c)
	}
	if _, err := SVG([]byte("<g/>"), 10, 10, FullArea(10, 10)); err == nil {
		t.Error("non-svg root should fail")
	}
}

func coverage(pts [][]float64, x int) int {
	m := StrokeMask(200, 100, pts, 20, false)
	n := 0
	for y := 0; y < 100; y++ {
		if m.AlphaAt(x, y).A > 128 {
			n++
		}
	}
	return n
}

func TestStrokeTaper(t *testing.T) {
	pts := [][]float64{{10, 50, 0.1}, {100, 50, 1}, {190, 50, 0.1}}
	thin, thick := coverage(pts, 25), coverage(pts, 100)
	if thick < 18 || thin > thick/2 {
		t.Errorf("taper: width %d near the thin end vs %d in the middle", thin, thick)
	}
	doc, err := ParseStrokes([]byte(`[[[10,10],[50,50]]]`))
	if err != nil || len(doc.Strokes) != 1 || len(doc.Strokes[0].Points) != 2 {
		t.Fatalf("point-list form: %v %+v", err, doc)
	}
	o, err := Strokes(100, 100, FullArea(100, 100), doc, Tools["pen"])
	if err != nil || o.RGBAAt(30, 30).A == 0 {
		t.Errorf("pen stroke missing: %v", err)
	}
}

func TestClipLeavesOutsideUntouched(t *testing.T) {
	layer := newFilled(100, 100, color.RGBA{0, 255, 0, 255})
	overlay := newFilled(100, 100, color.RGBA{255, 0, 0, 255})
	a := Area{X: 20, Y: 20, W: 40, H: 40, Clip: [][2]float64{{20, 20}, {60, 20}, {60, 60}, {20, 60}}}
	Apply(layer, overlay, a, false)
	if layer.RGBAAt(40, 40) != (color.RGBA{255, 0, 0, 255}) || layer.RGBAAt(80, 80) != (color.RGBA{0, 255, 0, 255}) || layer.RGBAAt(10, 40) != (color.RGBA{0, 255, 0, 255}) {
		t.Errorf("clip: inside %v outside %v", layer.RGBAAt(40, 40), layer.RGBAAt(80, 80))
	}
}

func TestPathStrokesTaper(t *testing.T) {
	svg := []byte(`<svg><path d="M 10 50 C 40 0, 160 100, 190 50 M 10 90 L 190 90"/><path data-taper="none" stroke-width="7" stroke="#ff0000" d="M 0 0 A 20 20 0 0 1 40 0"/></svg>`)
	strokes, err := PathStrokes(svg, PathSpec{Taper: "both", MinPressure: 0.2})
	if err != nil || len(strokes) != 3 {
		t.Fatalf("want 3 strokes (2 subpaths + 1 path), got %d: %v", len(strokes), err)
	}
	s := strokes[0].Points
	first, mid, last := s[0][2], s[len(s)/2][2], s[len(s)-1][2]
	if math.Abs(first-0.2) > 1e-9 || math.Abs(last-0.2) > 1e-9 || mid != 1 || len(s) < 50 {
		t.Errorf("taper: ends %g/%g, middle %g, %d points", first, last, mid, len(s))
	}
	arc := strokes[2]
	if arc.Size != 7 || arc.Color != "#ff0000" || arc.Points[0][2] != 1 {
		t.Errorf("per-path overrides: %+v", arc.Points[0])
	}
	// the arc bulges away from y=0 (sampled, not a straight chord)
	bulge := 0.0
	for _, p := range arc.Points {
		bulge = math.Max(bulge, math.Abs(p[1]))
	}
	if bulge < 15 {
		t.Errorf("arc not sampled: max |y| %g", bulge)
	}
}

func TestToneCoverage(t *testing.T) {
	mean := func(img *image.RGBA, x0, x1 int) float64 {
		s := 0.0
		for y := 0; y < 200; y++ {
			for x := x0; x < x1; x++ {
				s += float64(img.RGBAAt(x, y).A) / 255
			}
		}
		return s / float64(200*(x1-x0))
	}
	square := [][2]float64{{0, 0}, {200, 0}, {200, 200}, {0, 200}}
	for _, c := range []struct {
		pattern string
		d       float64
	}{{"dots", 0.3}, {"dots", 0.7}, {"lines", 0.4}, {"crosshatch", 0.5}} {
		img, err := Tone(300, 200, FullArea(300, 200), square, ToneSpec{Pattern: c.pattern, Spacing: 10, Density: c.d, Angle: 30, Color: color.RGBA{A: 255}})
		if err != nil {
			t.Fatal(err)
		}
		if m := mean(img, 0, 200); math.Abs(m-c.d) > 0.04 {
			t.Errorf("%s at %g: coverage %.3f", c.pattern, c.d, m)
		}
		if mean(img, 210, 300) != 0 {
			t.Errorf("%s: ink outside the polygon", c.pattern)
		}
	}
	g := [4]float64{0, 0, 200, 0}
	img, _ := Tone(200, 200, FullArea(200, 200), square, ToneSpec{Pattern: "dots", Spacing: 8, Density: 0.6, DensityTo: 0, Gradient: &g})
	if l, r := mean(img, 0, 40), mean(img, 160, 200); l < 0.45 || r > 0.1 {
		t.Errorf("gradient: left %.2f right %.2f", l, r)
	}
}

func TestSVGMissingFontIsAnError(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><text x="10" y="50" style="font-family: No Such Font 42; font-size: 30px">hi</text></svg>`)
	_, err := SVG(svg, 100, 100, FullArea(100, 100))
	var fe FontError
	if !errors.As(err, &fe) || fe.Family != "No Such Font 42" || fe.Code() != "font_not_found" {
		t.Fatalf("want FontError, got %v", err)
	}
}
