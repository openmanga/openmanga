package render

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

var (
	fallbackOnce sync.Once
	fallbackFont *opentype.Font
)

func loadFallback() *opentype.Font {
	fallbackOnce.Do(func() {
		p := FallbackFontPath()
		if p == "" {
			return
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return
		}
		if strings.EqualFold(filepath.Ext(p), ".ttc") {
			if c, err := opentype.ParseCollection(data); err == nil {
				fallbackFont, _ = c.Font(0)
			}
			return
		}
		fallbackFont, _ = opentype.Parse(data)
	})
	return fallbackFont
}

// fallbackFace draws runes the primary font lacks (e.g. Japanese) with the
// system fallback font.
type fallbackFace struct {
	primary font.Face
	pf      *sfnt.Font
	fb      font.Face
	buf     sfnt.Buffer
	mu      sync.Mutex
}

// FallbackFace returns a face of ttf at size that falls back per rune.
func FallbackFace(ttf []byte, size float64) font.Face {
	pf, err := ParseFont(ttf)
	if err != nil {
		panic(err)
	}
	f := &fallbackFace{primary: Face(ttf, size), pf: pf}
	if fb := loadFallback(); fb != nil {
		f.fb, _ = opentype.NewFace(fb, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	}
	return f
}

func (f *fallbackFace) pick(r rune) font.Face {
	if f.fb == nil {
		return f.primary
	}
	f.mu.Lock()
	idx, err := f.pf.GlyphIndex(&f.buf, r)
	f.mu.Unlock()
	if err == nil && idx != 0 {
		return f.primary
	}
	return f.fb
}

func (f *fallbackFace) Close() error { return nil }
func (f *fallbackFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return f.pick(r).Glyph(dot, r)
}
func (f *fallbackFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.pick(r).GlyphBounds(r)
}
func (f *fallbackFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) { return f.pick(r).GlyphAdvance(r) }
func (f *fallbackFace) Kern(a, b rune) fixed.Int26_6 {
	if f.pick(a) == f.primary && f.pick(b) == f.primary {
		return f.primary.Kern(a, b)
	}
	return 0
}
func (f *fallbackFace) Metrics() font.Metrics { return f.primary.Metrics() }
