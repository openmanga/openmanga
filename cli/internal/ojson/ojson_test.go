package ojson

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Every fixture written by the original app must re-serialize byte for byte.
func TestRoundTripFixtures(t *testing.T) {
	var files []string
	filepath.Walk("../../testdata", func(p string, info os.FileInfo, err error) error {
		if err == nil && (filepath.Ext(p) == ".storyboarder" || filepath.Ext(p) == ".settings") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) < 5 {
		t.Fatalf("expected fixtures, found %d", len(files))
	}
	for _, f := range files {
		data, _ := os.ReadFile(f)
		v, err := Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		indent := "  "
		if !bytes.Contains(data, []byte("\n")) {
			indent = ""
		}
		out := Stringify(v, indent)
		if !bytes.Equal(bytes.TrimRight(data, "\n"), out) {
			t.Errorf("%s: round trip differs", f)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	for in, want := range map[float64]string{
		1500: "1500", 0.1: "0.1", 1e21: "1e+21", 1e-7: "1e-7", -2.5: "-2.5", 123456789012: "123456789012", 0.000001: "0.000001",
	} {
		if got := FormatNumber(in); got != want {
			t.Errorf("FormatNumber(%v) = %s, want %s", in, got, want)
		}
	}
}

func TestStringEscapes(t *testing.T) {
	got := string(Stringify(Obj("a", "<b>&\"\n\u2028é"), ""))
	want := "{\"a\":\"<b>&\\\"\\n\u2028é\"}"
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}
