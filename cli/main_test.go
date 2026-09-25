package main

import (
	"encoding/json"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"sb/internal/render"
)

// sb runs the CLI in-process and returns stdout and the exit code.
func sb(t *testing.T, args ...string) (string, int) {
	t.Helper()
	projectArg, sceneArg = "", 0
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	code := run(args)
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out), code
}

func TestCLIEndToEnd(t *testing.T) {
	t.Setenv("SB_USER_DATA", t.TempDir())
	dir := filepath.Join(t.TempDir(), "demo")
	out, code := sb(t, "project", "new", dir, "--aspect", "2.39", "--json")
	if code != 0 {
		t.Fatalf("project new: %s", out)
	}
	p := "--project=" + dir
	for _, args := range [][]string{
		{"board", "add", p},
		{"board", "set-dialogue", "2", "Hello", "there", p},
		{"board", "set-new-shot", "2", "on", p},
		{"board", "set-duration", "1-2", "1500", p},
	} {
		if out, code := sb(t, append(args, "--json")...); code != 0 {
			t.Fatalf("%v: %s", args, out)
		}
	}
	out, _ = sb(t, "board", "list", p, "--json")
	var res struct {
		Boards []struct {
			Shot     string  `json:"shot"`
			Dialogue string  `json:"dialogue"`
			Duration float64 `json:"duration"`
			Time     float64 `json:"time"`
		} `json:"boards"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("bad JSON %q: %v", out, err)
	}
	if len(res.Boards) != 2 || res.Boards[1].Shot != "2A" || res.Boards[1].Dialogue != "Hello there" || res.Boards[1].Time != 1500 {
		t.Errorf("unexpected boards: %+v", res.Boards)
	}
	// the file on disk is what the original app writes
	data, _ := os.ReadFile(filepath.Join(dir, "demo.storyboarder"))
	if !strings.Contains(string(data), "\n  \"aspectRatio\": 2.39,\n") || !strings.Contains(string(data), `"version": "3.0.0"`) {
		t.Errorf("unexpected file:\n%s", data)
	}
	for _, f := range []string{"images"} {
		if st, err := os.Stat(filepath.Join(dir, f)); err != nil || !st.IsDir() {
			t.Errorf("missing %s", f)
		}
	}
}

func TestJSONErrors(t *testing.T) {
	out, code := sb(t, "board", "get", "1", "--project", "/does/not/exist", "--json")
	var e struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if code == 0 || json.Unmarshal([]byte(out), &e) != nil || e.Error.Message == "" {
		t.Errorf("expected a JSON error, got %d %q", code, out)
	}
	if _, code := sb(t, "board", "add", "--bogus", "--json"); code != 2 {
		t.Errorf("unknown flag should exit 2, got %d", code)
	}
}

func TestHelpListsEveryCommand(t *testing.T) {
	out, code := sb(t, "help")
	if code != 0 {
		t.Fatal("help failed")
	}
	for _, c := range commands {
		if !strings.Contains(out, "sb "+c.Path) {
			t.Errorf("help is missing %q", c.Path)
		}
	}
	seen := map[string]bool{}
	for _, c := range commands {
		if seen[c.Path] {
			t.Errorf("duplicate command %q", c.Path)
		}
		seen[c.Path] = true
	}
}

func TestMangaPanelDrawing(t *testing.T) {
	t.Setenv("SB_USER_DATA", t.TempDir())
	dir := filepath.Join(t.TempDir(), "comic")
	if out, code := sb(t, "project", "new", dir, "--manga", "--template", "grid-1x2", "--json"); code != 0 {
		t.Fatal(out)
	}
	p := "--project=" + dir
	svg := filepath.Join(dir, "fill.svg")
	os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect x="-5000" y="-5000" width="10000" height="10000" fill="#000"/></svg>`), 0o644)
	out, code := sb(t, "draw", "svg", "--page", "1", "--panel", "K1", "--layer", "ink", svg, p, "--json")
	if code != 0 {
		t.Fatal(out)
	}
	var res struct {
		File     string    `json:"file"`
		PanelBox []float64 `json:"panelBox"`
	}
	json.Unmarshal([]byte(out), &res)
	img, err := loadPNG(res.File)
	if err != nil {
		t.Fatal(err)
	}
	x, y := int(res.PanelBox[0]), int(res.PanelBox[1])
	if img.RGBAAt(x+10, y+10).A == 0 {
		t.Error("inside the panel should be painted")
	}
	if img.RGBAAt(5, 5).A != 0 || img.RGBAAt(x-5, y+10).A != 0 {
		t.Error("pixels outside the panel must stay untouched")
	}
	out, code = sb(t, "page", "render", "1", "--out", filepath.Join(dir, "p.png"), p, "--json")
	if code != 0 || !strings.Contains(out, `"path"`) {
		t.Errorf("page render: %s", out)
	}
}

func TestRenderCropScale(t *testing.T) {
	t.Setenv("SB_USER_DATA", t.TempDir())
	dir := filepath.Join(t.TempDir(), "comic")
	if out, code := sb(t, "project", "new", dir, "--manga", "--template", "3-tier", "--json"); code != 0 {
		t.Fatal(out)
	}
	p := "--project=" + dir
	out := filepath.Join(dir, "crop.png")
	if res, code := sb(t, "page", "render", "1", "--out", out, "--crop", "150,150,300,300", "--scale", "2", "--grid", p, "--json"); code != 0 || !strings.Contains(res, `"width":600`) || !strings.Contains(res, `"height":600`) {
		t.Fatalf("crop render: %s", res)
	}
	img, _ := loadPNG(out)
	// source x=200 is 50 px into the crop, 100 px at 2x: a grid line; x=150 source (0 in the image) is not
	if c := img.RGBAAt(100, 400); c.R < 200 || c.G > 200 {
		t.Errorf("grid line for x=200 expected at 100, got %v", c)
	}
	if c := img.RGBAAt(160, 400); c.G < 200 {
		t.Errorf("no grid line expected at 160, got %v", c)
	}
	var res struct {
		Width, Height int
		Crop          []float64
	}
	s, code := sb(t, "page", "render", "1", "--panel", "K2", "--scale", "0.5", "--out", out, p, "--json")
	json.Unmarshal([]byte(s), &res)
	if code != 0 || len(res.Crop) != 4 || res.Width != int(res.Crop[2]/2+0.5) || res.Height != int(res.Crop[3]/2+0.5) {
		t.Errorf("panel render: %s", s)
	}
	if s, code := sb(t, "page", "render", "1", "--crop", "5000,5000,10,10", "--out", out, p, "--json"); code != 2 {
		t.Errorf("crop outside the page should be a usage error: %s", s)
	}
}

func loadPNG(p string) (*image.RGBA, error) {
	img, err := render.Load(p)
	if err != nil {
		return nil, err
	}
	return render.ToRGBA(img), nil
}

func TestLikePanelAndMap(t *testing.T) {
	t.Setenv("SB_USER_DATA", t.TempDir())
	dir := filepath.Join(t.TempDir(), "comic")
	if out, code := sb(t, "project", "new", dir, "--manga", "--template", "3-tier", "--json"); code != 0 {
		t.Fatal(out)
	}
	p := "--project=" + dir
	out, code := sb(t, "board", "add", "--like-panel", "1:K1", "--place", p, "--json")
	if code != 0 {
		t.Fatal(out)
	}
	var added struct {
		UID      string `json:"uid"`
		Size     []int  `json:"size"`
		PlacedIn string `json:"placedIn"`
	}
	json.Unmarshal([]byte(out), &added)
	// K1 of a 3-tier page is wide: long side 1800
	if added.Size[0] != 1800 || added.Size[1] >= 1800 || added.PlacedIn == "" {
		t.Fatalf("like-panel board: %s", out)
	}
	out, code = sb(t, "panel", "map", "1", "K1", "--board-point", "900,"+strconv.Itoa(added.Size[1]/2), p, "--json")
	if code != 0 {
		t.Fatal(out)
	}
	var m struct {
		Page        []float64 `json:"page"`
		PanelLocal  []float64 `json:"panelLocal"`
		BoardPoint  []float64 `json:"boardPoint"`
		InsidePanel bool      `json:"insidePanel"`
	}
	json.Unmarshal([]byte(out), &m)
	// the board center sits at the panel center (fill, no offset)
	if !m.InsidePanel || m.PanelLocal[0] < 580 || m.PanelLocal[0] > 610 || m.BoardPoint[0] != 900 {
		t.Errorf("map: %s", out)
	}
	if out, code := sb(t, "panel", "map", "1", "K1", "--page-point", "10,10", p, "--json"); code != 0 || !strings.Contains(out, `"insidePanel":false`) {
		t.Errorf("page point outside: %s", out)
	}
}

func TestFrontendRequests(t *testing.T) {
	t.Setenv("SB_USER_DATA", t.TempDir())
	dir := filepath.Join(t.TempDir(), "comic")
	if out, code := sb(t, "project", "new", dir, "--manga", "--pages", "3", "--first-page", "paired", "--template", "2-tier", "--json"); code != 0 {
		t.Fatal(out)
	}
	p := "--project=" + dir
	run := func(args ...string) string {
		t.Helper()
		out, code := sb(t, append(args, p, "--json")...)
		if code != 0 {
			t.Fatalf("%v: %s", args, out)
		}
		return out
	}
	run("board", "set-name", "1", "Hero", "pose")
	run("board", "set-description", "1", "rough")
	run("panel", "place", "2", "K1", "1")
	var list struct {
		Boards []struct {
			Name, Description, Thumbnail string
			Size                         []int
		} `json:"boards"`
	}
	json.Unmarshal([]byte(run("board", "list")), &list)
	if list.Boards[0].Name != "Hero pose" || list.Boards[0].Description != "rough" || len(list.Boards[0].Size) != 2 || !filepath.IsAbs(list.Boards[0].Thumbnail) {
		t.Errorf("board list: %+v", list.Boards)
	}
	if out := run("board", "info", "1"); !strings.Contains(out, `"usedIn":[{"page":2,"pageId":`) {
		t.Errorf("usedIn: %s", out)
	}
	var layers struct {
		Layers []struct {
			Name   string
			Exists bool
		} `json:"layers"`
	}
	json.Unmarshal([]byte(run("layer", "list", "--page", "1")), &layers)
	if len(layers.Layers) < 6 || layers.Layers[0].Exists {
		t.Errorf("layer list: %+v", layers.Layers)
	}
	run("layer", "set-opacity", "--page", "1", "ink", "0.4")
	if out := run("layer", "list", "--page", "1"); !strings.Contains(out, `"name":"ink","opacity":0.4`) && !strings.Contains(out, `"opacity":0.4`) {
		t.Errorf("page ink opacity: %s", out)
	}
	out := run("page", "setup", "--size", "1000x1400", "--reading", "ltr", "--first-page", "single")
	if !strings.Contains(out, `"width":1000`) || !strings.Contains(out, `"readingDirection":"ltr"`) {
		t.Errorf("page setup: %s", out)
	}
	img := filepath.Join(dir, "ref.png")
	render.SavePNG(img, render.NewFilled(300, 200, color.RGBA{0, 0, 0, 255}))
	if out := run("import", "images", img, "--pages", "--after", "1"); !strings.Contains(out, `"imported":1`) {
		t.Errorf("import pages: %s", out)
	}
	if out := run("import", "images", img, "--page", "1", "--panel", "K1"); !strings.Contains(out, `"panel":"K1"`) {
		t.Errorf("import into panel: %s", out)
	}
	run("recent", "add", dir)
	var rec struct {
		Recent []struct {
			Mode      string
			Pages     int
			Boards    int
			Exists    bool
			Thumbnail string
		} `json:"recent"`
	}
	json.Unmarshal([]byte(run("recent", "list")), &rec)
	if len(rec.Recent) == 0 || rec.Recent[0].Mode != "manga" || rec.Recent[0].Pages != 4 || rec.Recent[0].Boards != 1 || !rec.Recent[0].Exists || rec.Recent[0].Thumbnail == "" {
		t.Errorf("recent: %+v", rec.Recent)
	}
	pdf := filepath.Join(dir, "out.pdf")
	run("export", "pages", "--pdf", "--out", pdf, "--dpi", "150", "--crop-marks", "--page-numbers")
	if data, _ := os.ReadFile(pdf); !strings.HasPrefix(string(data), "%PDF") {
		t.Error("pdf with options")
	}
}
