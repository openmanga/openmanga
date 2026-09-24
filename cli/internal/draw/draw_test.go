package draw

import (
	"image"
	"image/color"
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
