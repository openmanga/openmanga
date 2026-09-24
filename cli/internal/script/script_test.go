package script

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// oracle files were produced by running the original JS parsers on the fixtures.
type oracle struct {
	Tokens     []map[string]any `json:"tokens"`
	ScriptData []map[string]any `json:"scriptData"`
	Locations  [][]any          `json:"locations"`
	Characters [][]any          `json:"characters"`
}

func loadOracle(t *testing.T, name string) oracle {
	var o oracle
	data, err := os.ReadFile("../../testdata/oracle/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &o); err != nil {
		t.Fatal(err)
	}
	return o
}

func num(v any) int {
	f, _ := v.(float64)
	return int(f)
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func compareCounts(t *testing.T, label string, got []Count, want [][]any) {
	if len(got) != len(want) {
		t.Fatalf("%s: got %d entries, want %d (%v vs %v)", label, len(got), len(want), got, want)
	}
	for i := range got {
		if got[i].Name != str(want[i][0]) || got[i].Count != num(want[i][1]) {
			t.Errorf("%s[%d]: got %v want %v", label, i, got[i], want[i])
		}
	}
}

func compareNodes(t *testing.T, label string, got []Node, want []map[string]any) {
	if len(got) != len(want) {
		t.Fatalf("%s: got %d nodes, want %d", label, len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.Type != str(w["type"]) || g.Time != num(w["time"]) || g.Duration != num(w["duration"]) {
			t.Errorf("%s node %d: got %s t=%d d=%d, want %v t=%v d=%v", label, i, g.Type, g.Time, g.Duration, w["type"], w["time"], w["duration"])
		}
		if g.Type != "scene" {
			continue
		}
		if g.SceneNumber != num(w["scene_number"]) || g.SceneID != str(w["scene_id"]) || g.Slugline != str(w["slugline"]) || g.WordCount != num(w["word_count"]) || g.Synopsis != str(w["synopsis"]) {
			t.Errorf("%s scene %d: got %+v want %v", label, i, g, w)
		}
		items, _ := w["script"].([]any)
		if len(items) != len(g.Script) {
			t.Fatalf("%s scene %d: %d items, want %d", label, i, len(g.Script), len(items))
		}
		for j, raw := range items {
			wi := raw.(map[string]any)
			gi := g.Script[j]
			if gi.Type != str(wi["type"]) || gi.Text != str(wi["text"]) || gi.Time != num(wi["time"]) || gi.Duration != num(wi["duration"]) || gi.Character != str(wi["character"]) {
				t.Errorf("%s scene %d item %d: got %+v want %v", label, i, j, gi, wi)
			}
			if wi["page"] != nil && gi.Page != num(wi["page"]) {
				t.Errorf("%s scene %d item %d: page %d want %v", label, i, j, gi.Page, wi["page"])
			}
		}
	}
}

func TestFountainMatchesOriginal(t *testing.T) {
	for _, f := range []string{"fountain/eol-lf.fountain", "fountain/eol-crlf.fountain", "projects/multi-scene/multi-scene.fountain", "projects/printable/printable.fountain", "extra.fountain"} {
		name := f[strings.LastIndex(f, "/")+1:]
		t.Run(name, func(t *testing.T) {
			o := loadOracle(t, name)
			p, err := Load("../../testdata/" + f)
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Tokens) != len(o.Tokens) {
				t.Fatalf("tokens: got %d want %d", len(p.Tokens), len(o.Tokens))
			}
			for i, w := range o.Tokens {
				g := p.Tokens[i]
				if g.Type != str(w["type"]) || g.Text != str(w["text"]) || g.SceneNumber != str(w["scene_number"]) {
					t.Errorf("token %d: got %+v want %v", i, g, w)
				}
			}
			compareNodes(t, name, p.Nodes, o.ScriptData)
			compareCounts(t, "locations", p.Locations, o.Locations)
			compareCounts(t, "characters", p.Characters, o.Characters)
		})
	}
}

func TestFDXMatchesOriginal(t *testing.T) {
	o := loadOracle(t, "test.fdx")
	p, err := Load("../../testdata/final-draft/test.fdx")
	if err != nil {
		t.Fatal(err)
	}
	compareNodes(t, "fdx", p.Nodes, o.ScriptData)
	compareCounts(t, "locations", p.Locations, o.Locations)
	compareCounts(t, "characters", p.Characters, o.Characters)
	// expectations from the original test suite
	if p.Nodes[1].Script[1].Duration != 3900 || p.Nodes[1].Script[0].Time != 8400 {
		t.Errorf("unexpected timing %+v", p.Nodes[1].Script[:2])
	}
}

func TestInsertSceneIDs(t *testing.T) {
	n := 0
	gen := func() string { n++; return fmt.Sprintf("ID%03d", n) }
	in := "INT. A - DAY\r\n\r\nEXT. B - NIGHT #2-XYZ#\r\n\r\n.FORCED\r\n"
	out, changed := InsertFountainSceneIDs(in, gen)
	want := "INT. A - DAY #1-ID001#\r\n\r\nEXT. B - NIGHT #2-XYZ#\r\n\r\n.FORCED #3-ID002#\r\n"
	if !changed || out != want {
		t.Errorf("got %q", out)
	}
	if _, changed := InsertFountainSceneIDs(out, gen); changed {
		t.Error("second pass should not change anything")
	}

	fdx := []byte(`<Paragraph Bookmark="Start" Type="Scene Heading"><Text>X</Text></Paragraph><Paragraph Number="2" Type="Scene Heading"/>`)
	res, added := InsertFDXSceneIDs(fdx, gen)
	if len(added) != 1 || !strings.Contains(string(res), `<Paragraph Bookmark="Start" Type="Scene Heading" Number="ID003">`) {
		t.Errorf("fdx ids: %s %v", res, added)
	}
}

func TestSceneFolderName(t *testing.T) {
	got := SceneFolderName(Node{SceneNumber: 1, SceneID: "1-ZX3ZM", Slugline: "EXT. A PLACE - DAY"})
	if got != "Scene-1-EXT-A-PLACE-DAY-1-ZX3ZM" {
		t.Errorf("got %s", got)
	}
}
