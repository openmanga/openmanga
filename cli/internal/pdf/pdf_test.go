package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"sb/internal/render"
	"sb/internal/story"
)

func exampleProject(t *testing.T) Project {
	s, err := story.LoadScene("../../testdata/example/example.storyboarder")
	if err != nil {
		t.Fatal(err)
	}
	s.Boards()[0].Set("dialogue", "Hello there ГДЕ 你好")
	return Project{Scenes: []SceneData{{Title: "example", Scene: s}}}
}

func TestPDFIsValidForEveryPreset(t *testing.T) {
	p := exampleProject(t)
	if len(Presets()) != 15 {
		t.Fatalf("expected 15 presets, got %d", len(Presets()))
	}
	for _, pr := range Presets() {
		out := filepath.Join(t.TempDir(), pr.ID+".pdf")
		cfg := pr.Data
		cfg.PaperSizeKey = "letter"
		if err := Write(out, p, cfg, 0, -1); err != nil {
			t.Fatalf("%s: %v", pr.ID, err)
		}
		data, _ := os.ReadFile(out)
		if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data, []byte("%%EOF")) {
			t.Fatalf("%s: not a PDF", pr.ID)
		}
		pages := len(regexp.MustCompile(`/Type /Page\b`).FindAll(data, -1))
		if want := PageCount(p, cfg); pages != want {
			t.Errorf("%s: %d pages, want %d", pr.ID, pages, want)
		}
	}
}

func TestPreviewRendersPage(t *testing.T) {
	p := exampleProject(t)
	out := filepath.Join(t.TempDir(), "p.png")
	cfg := Presets()[0].Data
	if err := Preview(out, p, cfg, 0, 1); err != nil {
		t.Fatal(err)
	}
	img, err := render.Load(out)
	if err != nil {
		t.Fatal(err)
	}
	// A4 landscape at 72 dpi
	if b := img.Bounds(); b.Dx() != 842 || b.Dy() != 595 {
		t.Errorf("preview size %v", b)
	}
}

func TestGroupByPageAndFormat(t *testing.T) {
	p := exampleProject(t)
	cfg := Presets()[0].Data
	cfg.GridDim = [2]int{2, 1}
	n := len(p.Scenes[0].Scene.Boards())
	if got := PageCount(p, cfg); got != (n+1)/2 {
		t.Errorf("pages %d for %d boards", got, n)
	}
	for ms, want := range map[float64]string{0: "0:00", 1499: "0:01", 61000: "1:01", 3723000: "1:02:03"} {
		if got := FormatMsecs(ms); got != want {
			t.Errorf("FormatMsecs(%v)=%s want %s", ms, got, want)
		}
	}
	if humanizeAspect(1.7777777777777777) != "16:9" || humanizeAspect(2.39) != "2.39:1" {
		t.Error("aspect names")
	}
	if pr, ok := FindPreset("Hey-oh!"); !ok || pr.ID != "preset-8949e51" {
		t.Error("find preset by title")
	}
}
