package pdf

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"sync"

	"golang.org/x/image/font"

	"sb/internal/render"
)

// FontMetrics are the per-1000 units pdfkit uses for layout (hhea values),
// plus the OS/2 typo ascender gopdf uses to place text.
type FontMetrics struct {
	Ascender, Descender, LineGap float64 // per 1000 em, descender negative
	TypoAscender                 float64
}

func readMetrics(ttf []byte) (FontMetrics, error) {
	var m FontMetrics
	if len(ttf) < 12 {
		return m, fmt.Errorf("not a TrueType font")
	}
	n := int(binary.BigEndian.Uint16(ttf[4:6]))
	tables := map[string][]byte{}
	for i := 0; i < n; i++ {
		rec := ttf[12+16*i:]
		off, ln := binary.BigEndian.Uint32(rec[8:12]), binary.BigEndian.Uint32(rec[12:16])
		if int(off+ln) <= len(ttf) {
			tables[string(rec[0:4])] = ttf[off : off+ln]
		}
	}
	head, hhea, os2 := tables["head"], tables["hhea"], tables["OS/2"]
	if len(head) < 20 || len(hhea) < 10 {
		return m, fmt.Errorf("font has no head/hhea table")
	}
	upem := float64(binary.BigEndian.Uint16(head[18:20]))
	s := 1000 / upem
	m.Ascender = float64(int16(binary.BigEndian.Uint16(hhea[4:6]))) * s
	m.Descender = float64(int16(binary.BigEndian.Uint16(hhea[6:8]))) * s
	m.LineGap = float64(int16(binary.BigEndian.Uint16(hhea[8:10]))) * s
	m.TypoAscender = m.Ascender
	if len(os2) >= 70 {
		m.TypoAscender = float64(int16(binary.BigEndian.Uint16(os2[68:70]))) * s
	}
	return m, nil
}

// Font is one typeface used by the layout.
type Font struct {
	Name    string
	TTF     []byte
	Metrics FontMetrics
	faces   map[float64]font.Face
	mu      sync.Mutex
}

func newFont(name string, ttf []byte) *Font {
	m, err := readMetrics(ttf)
	if err != nil {
		panic(err)
	}
	return &Font{Name: name, TTF: ttf, Metrics: m, faces: map[float64]font.Face{}}
}

// Width of s at size (points), in points.
func (f *Font) Width(s string, size float64) float64 {
	f.mu.Lock()
	face, ok := f.faces[size]
	if !ok {
		face = render.Face(f.TTF, size)
		f.faces[size] = face
	}
	f.mu.Unlock()
	return render.Measure(face, s)
}

// Positions returns the kerned x offset of each rune of s at size.
func (f *Font) Positions(s string, size float64) []float64 {
	f.Width("", size) // ensure the face exists
	f.mu.Lock()
	face := f.faces[size]
	f.mu.Unlock()
	var out []float64
	x := 0.0
	prev := rune(-1)
	for _, r := range s {
		if prev >= 0 {
			x += float64(face.Kern(prev, r)) / 64
		}
		out = append(out, x)
		a, _ := face.GlyphAdvance(r)
		x += float64(a) / 64
		prev = r
	}
	return out
}

// LineHeight is pdfkit currentLineHeight(true).
func (f *Font) LineHeight(size float64) float64 {
	return (f.Metrics.Ascender + f.Metrics.LineGap - f.Metrics.Descender) / 1000 * size
}

var (
	Thin    = newFont("thin", render.FontThin)
	Regular = newFont("regular", render.FontRegular)
	Bold    = newFont("bold", render.FontBold)
)

// foreign is exporters/pdf/string-contains-foreign.js: any char outside Latin & common symbols.
var foreign = regexp.MustCompile(`[^AÁĂÂÄÀĀĄÅÃÆBCĆČÇĊDÐĎĐEÉĚÊËĖÈĒĘFGĞĢĠHĦIÍÎÏİÌĪĮJKĶLĹĽĻŁMNŃŇŅŊÑOÓÔÖÒŐŌØÕŒPÞQRŔŘŖSŚŠŞȘTŦŤŢȚUÚÛÜÙŰŪŲŮVWẂŴẄẀXYÝŶŸỲZŹŽŻaáăâäàāąåãæbcćčçċdðďđeéěêëėèēęfgğģġhħiıíîïìīįjkķlĺľļłmnńňņŋñoóôöòőōøõœpþqrŕřŗsśšşșßtŧťţțuúûüùűūųůvwẃŵẅẁxyýŷÿỳzźžż0123456789.,/#!$%^&*;:{}=\-_` + "`" + `~()\s?¿—–\-€₪¢₡¤$ƒ₣₤₧₨£¥⋅+−×÷=≠><≥≤±≈~¬∞∫Ω∆∏∑√µ∂%‰⊳⊲↑→↓←●◊■▲▼★☐♦✓@&¶§©®℗™°|¦†ℓ‡№℮^⌘'"„“”‘‛’´˘ˇ¸ˆ¨˙` + "`" + `˝¯˛˚˜]`)

// ContainsForeign reports whether text needs the Unicode fallback font.
func ContainsForeign(s string) bool { return foreign.MatchString(s) }
