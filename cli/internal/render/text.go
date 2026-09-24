package render

import (
	_ "embed"
	"image"
	"image/color"
	"math"
	"os"
	"runtime"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The app's THICCCBOI weights (PDF uses Thin/Regular/Bold, GIF captions use Light).
var (
	//go:embed fonts/THICCCBOI-Thin.ttf
	FontThin []byte
	//go:embed fonts/THICCCBOI-Light.ttf
	FontLight []byte
	//go:embed fonts/THICCCBOI-Regular.ttf
	FontRegular []byte
	//go:embed fonts/THICCCBOI-Bold.ttf
	FontBold []byte
)

var (
	parsedMu sync.Mutex
	parsed   = map[*byte]*opentype.Font{}
)

// ParseFont parses (and caches) TTF bytes.
func ParseFont(ttf []byte) (*opentype.Font, error) {
	parsedMu.Lock()
	defer parsedMu.Unlock()
	if f, ok := parsed[&ttf[0]]; ok {
		return f, nil
	}
	f, err := opentype.Parse(ttf)
	if err != nil {
		return nil, err
	}
	parsed[&ttf[0]] = f
	return f, nil
}

// Face returns a font face at a pixel size.
func Face(ttf []byte, size float64) font.Face {
	f, err := ParseFont(ttf)
	if err != nil {
		panic(err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	return face
}

// FallbackFontPath is a Unicode font for text THICCCBOI cannot show
// (the original bundles unicore.ttf, 10 MB, which we do not embed).
// SB_FALLBACK_FONT overrides the system default.
func FallbackFontPath() string {
	if p := os.Getenv("SB_FALLBACK_FONT"); p != "" {
		return p
	}
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{"/System/Library/Fonts/Supplemental/Arial Unicode.ttf", "/Library/Fonts/Arial Unicode.ttf"}
	case "linux":
		candidates = []string{"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "/usr/share/fonts/TTF/DejaVuSans.ttf"}
	case "windows":
		candidates = []string{`C:\Windows\Fonts\arialuni.ttf`, `C:\Windows\Fonts\arial.ttf`}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// Measure is the advance width of s in pixels.
func Measure(face font.Face, s string) float64 {
	return float64(font.MeasureString(face, s)) / 64
}

// DrawText draws s with its baseline at y; align is "left", "center" or "right" around x.
func DrawText(dst *image.RGBA, face font.Face, s string, x, y float64, c color.Color, align string) {
	switch align {
	case "center":
		x -= Measure(face, s) / 2
	case "right":
		x -= Measure(face, s)
	}
	d := &font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: face, Dot: fixed.P(0, 0)}
	d.Dot = fixed.Point26_6{X: fixed.Int26_6(math.Round(x * 64)), Y: fixed.Int26_6(math.Round(y * 64))}
	d.DrawString(s)
}

// StrokeText approximates canvas strokeText by stamping the glyphs around a
// circle of radius width/2.
func StrokeText(dst *image.RGBA, face font.Face, s string, x, y float64, c color.Color, width float64, align string) {
	r := width / 2
	mask := New(dst.Bounds().Dx(), dst.Bounds().Dy())
	steps := 12
	for i := 0; i < steps; i++ {
		a := 2 * math.Pi * float64(i) / float64(steps)
		DrawText(mask, face, s, x+r*math.Cos(a), y+r*math.Sin(a), color.White, align)
	}
	alpha := image.NewAlpha(mask.Bounds())
	for i := 3; i < len(mask.Pix); i += 4 {
		alpha.Pix[i/4] = mask.Pix[i]
	}
	drawMaskUniform(dst, alpha, c)
}
