package manga

import (
	"image/color"
	"math"
	"os"
	"path/filepath"
	"testing"

	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/story"
)

func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "sb-userdata-")
	os.Setenv("SB_USER_DATA", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func newProject(t *testing.T, rtl bool) (*story.Scene, *ojson.Object) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.storyboarder")
	story.WriteJSON(path, ojson.Obj("aspectRatio", 1.7777777777777777, "fps", 24, "defaultBoardTiming", 2000, "boards", []any{}), "  ")
	s, err := story.LoadScene(path)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(s.ImagesDir(), 0o755)
	MakeManga(s, 1000, 1400, rtl)
	pg := NewPage()
	s.SetPages([]*ojson.Object{pg})
	return s, pg
}

func TestTemplates(t *testing.T) {
	l := Layout{W: 1000, H: 1400, MarginX: 100, MarginY: 100, GutterX: 20, GutterY: 40}
	r, _ := Template(l, "splash")
	if len(r) != 1 || r[0] != (Rect{100, 100, 800, 1200}) {
		t.Errorf("splash %v", r)
	}
	r, _ = Template(l, "3-tier")
	// (1200 - 2*40) / 3 = 373.33 per tier
	if len(r) != 3 || r[0] != (Rect{100, 100, 800, 373}) || r[1].Y != 513 || r[2].Y+r[2].H != 1300 {
		t.Errorf("3-tier %v", r)
	}
	r, _ = Template(l, "big-plus-2")
	if len(r) != 3 || r[1].W != 390 || r[2].X != 510 || r[0].H != 696 {
		t.Errorf("big-plus-2 %v", r)
	}
	r, _ = Template(l, "grid 2x3")
	if len(r) != 6 || r[1].X != 373 {
		t.Errorf("grid %v", r)
	}
	if _, err := Template(l, "nope"); err == nil {
		t.Error("unknown template should fail")
	}
}

func TestSplitWithGutter(t *testing.T) {
	a, b, err := Split(RectPoly(Rect{0, 0, 100, 200}), "horizontal", 0.5, 20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if BBox(a) != (Rect{0, 0, 100, 90}) || BBox(b) != (Rect{0, 110, 100, 90}) {
		t.Errorf("horizontal %v %v", BBox(a), BBox(b))
	}
	a, b, _ = Split(RectPoly(Rect{0, 0, 100, 200}), "vertical", 0.3, 10, 0)
	if BBox(a).W != 25 || BBox(b).X != 35 {
		t.Errorf("vertical %v %v", BBox(a), BBox(b))
	}
	a, b, _ = Split(RectPoly(Rect{0, 0, 100, 200}), "diagonal", 0.5, 10, 0.3)
	if len(a) < 3 || len(b) < 3 || BBox(a).Y != 0 || BBox(b).Y+BBox(b).H != 200 {
		t.Errorf("diagonal %v %v", a, b)
	}
}

func TestReadingOrder(t *testing.T) {
	boxes := []Rect{{0, 0, 40, 40}, {60, 0, 40, 40}, {0, 60, 100, 40}}
	rtl := ReadingOrder(boxes, true)
	ltr := ReadingOrder(boxes, false)
	if rtl[0] != 1 || rtl[1] != 0 || rtl[2] != 2 || ltr[0] != 0 || ltr[1] != 1 {
		t.Errorf("rtl %v ltr %v", rtl, ltr)
	}
	s, pg := newProject(t, true)
	ApplyTemplate(s, pg, "grid 1x2", LayoutOf(s))
	ps := Panels(pg)
	if ps[1].NumOr("order", 0) != 1 { // the right panel reads first
		t.Errorf("rtl page order: %v %v", ps[0].Get("order"), ps[1].Get("order"))
	}
	SetManualOrder(s, pg, []string{ps[0].Str("id")})
	if ps[0].NumOr("order", 0) != 1 || pg.Str("panelOrder") != "manual" {
		t.Error("manual order")
	}
}

func TestBalloonInsideBounds(t *testing.T) {
	s, pg := newProject(t, true)
	b, err := AddBalloon(pg, "speech", "Hello there", 300, 400, 200, 120, 32, false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	img := RenderBalloons(s, pg)
	sw := int(strokeWidth(b)) + 2
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y).A == 0 {
				continue
			}
			if x < 300-sw || x > 500+sw || y < 400-sw || y > 520+sw {
				t.Fatalf("balloon pixel outside its box at %d,%d", x, y)
			}
		}
	}
	if c := img.RGBAAt(400, 460); c.A == 0 {
		t.Error("balloon center not painted")
	}
	for _, typ := range BalloonTypes {
		if _, err := AddBalloon(pg, typ, "縦書き text", 50, 50, 0, 0, 0, true, &Pt{10, 10}, ""); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
	}
	RenderBalloons(s, pg) // every type renders
}

func TestSpreads(t *testing.T) {
	s, _ := newProject(t, true)
	s.SetPages([]*ojson.Object{NewPage(), NewPage(), NewPage(), NewPage(), NewPage()})
	for i, want := range [][2]int{{0, -1}, {1, 2}, {1, 2}, {3, 4}, {3, 4}} {
		a, b := Spread(s, i)
		if a != want[0] || b != want[1] {
			t.Errorf("page %d spread %d,%d want %v", i+1, a, b, want)
		}
	}
	if _, nums := RenderSpread(s, 0); nums != [2]int{1, 0} { // rtl: page 1 alone on the left
		t.Errorf("rtl first page %v", nums)
	}
	Settings(s).Set("firstPageSingle", false)
	if a, b := Spread(s, 1); a != 0 || b != 1 {
		t.Errorf("without firstPageSingle page 2 pairs with page 1, got %d,%d", a, b)
	}
	Settings(s).Set("firstPageSingle", true)
	_, nums := RenderSpread(s, 1)
	if nums != [2]int{3, 2} { // rtl: [3|2]
		t.Errorf("rtl spread sides %v", nums)
	}
	Settings(s).Set("readingDirection", "ltr")
	if _, nums := RenderSpread(s, 0); nums != [2]int{0, 1} { // ltr: page 1 alone on the right
		t.Errorf("ltr first page %v", nums)
	}
	if _, nums := RenderSpread(s, 1); nums != [2]int{2, 3} {
		t.Errorf("ltr spread sides %v", nums)
	}
}

func TestPlacedBoardClippedToPanel(t *testing.T) {
	s, pg := newProject(t, true)
	b, err := story.InsertSizedBoard(s, 0, 400, 300)
	if err != nil {
		t.Fatal(err)
	}
	// a board filled solid red
	if err := story.SaveLayer(s, b, "fill", render.NewFilled(400, 300, color.RGBA{255, 0, 0, 255})); err != nil {
		t.Fatal(err)
	}
	// a triangular panel
	p := AddPanel(s, pg, []Pt{{100, 100}, {600, 100}, {100, 600}})
	p.Set("border", 0)
	p.Set("content", ojson.Obj("board", story.UID(b), "x", 0, "y", 0, "scale", 1, "rotation", 0, "fit", "fill"))
	img, warnings := Compose(s, pg)
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	if c := img.RGBAAt(150, 150); c.G > 10 || c.R < 245 {
		t.Errorf("inside the panel should show the board, got %v", c)
	}
	for _, xy := range [][2]int{{500, 500}, {50, 50}, {700, 300}} {
		if c := img.RGBAAt(xy[0], xy[1]); c != (color.RGBA{255, 255, 255, 255}) {
			t.Errorf("outside the panel at %v should stay paper white, got %v", xy, c)
		}
	}
	// fit = inside the box keeps the aspect: a 4:3 board in a 500x500 box leaves paper at the bottom
	p.Obj("content").Set("fit", "fit")
	img, _ = Compose(s, pg)
	if c := img.RGBAAt(110, 580); c != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("fit mode should not cover the whole box, got %v", c)
	}
	w, h := story.SizeOf(s, b)
	if w != 400 || h != 300 {
		t.Errorf("board size %dx%d", w, h)
	}
}

func TestContentMapping(t *testing.T) {
	s, pg := newProject(t, true)
	b, _ := story.InsertSizedBoard(s, 0, 400, 200)
	p := AddPanel(s, pg, RectPoly(Rect{100, 100, 400, 400}))
	p.Set("content", ojson.Obj("board", story.UID(b), "x", 10, "y", -20, "scale", 1.5, "rotation", 30, "fit", "fit"))
	c, err := ContentOf(s, p)
	if err != nil || c == nil {
		t.Fatal(err)
	}
	// the board center lands on the panel center plus the offset
	if x, y := c.ToPage(200, 100); math.Abs(x-310) > 1e-9 || math.Abs(y-280) > 1e-9 {
		t.Errorf("center maps to %v,%v", x, y)
	}
	// fit: 400 px wide board in a 400 px box -> 1 page px per board px, times scale 1.5
	x0, y0 := c.ToPage(0, 100)
	if d := math.Hypot(x0-310, y0-280); math.Abs(d-300) > 1e-9 {
		t.Errorf("scale: half width maps to %v px", d)
	}
	for _, pt := range [][2]float64{{0, 0}, {123, 45}, {400, 200}} {
		px, py := c.ToPage(pt[0], pt[1])
		bx, by := c.ToBoard(px, py)
		if math.Abs(bx-pt[0]) > 1e-6 || math.Abs(by-pt[1]) > 1e-6 {
			t.Errorf("round trip %v -> %v,%v", pt, bx, by)
		}
	}
	if !InsidePolygon(RectPoly(Rect{100, 100, 400, 400}), 300, 300) || InsidePolygon(RectPoly(Rect{100, 100, 400, 400}), 50, 300) {
		t.Error("point in polygon")
	}
	if w, h := LikeSize(Rect{0, 0, 600, 300}, 1800); w != 1800 || h != 900 {
		t.Errorf("like size %dx%d", w, h)
	}
	if w, h := LikeSize(Rect{0, 0, 300, 900}, 1800); w != 600 || h != 1800 {
		t.Errorf("like size tall %dx%d", w, h)
	}
}
