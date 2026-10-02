// Package draw turns instructions an agent can write (SVG, pressure strokes,
// text) into pixels on a board layer, optionally inside a clip polygon.
package draw

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"io"
	"math"
	"regexp"
	"strings"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers/rasterizer"
	"golang.org/x/image/font"

	"sb/internal/render"
)

// Area is where a drawing goes: an offset (panel-local 0,0) and size in layer
// pixels, and an optional clip polygon in layer pixels.
type Area struct {
	X, Y, W, H float64
	Clip       [][2]float64
}

// FullArea is the whole layer.
func FullArea(w, h int) Area { return Area{W: float64(w), H: float64(h)} }

// Apply composites overlay onto layer through the area's clip. With erase the
// overlay's alpha removes pixels instead (destination-out).
func Apply(layer, overlay *image.RGBA, a Area, erase bool) {
	if len(a.Clip) >= 3 {
		b := layer.Bounds()
		mask := render.PolygonMask(b.Dx(), b.Dy(), a.Clip)
		for i := 3; i < len(overlay.Pix); i += 4 {
			m := uint32(mask.Pix[i/4])
			for c := 0; c < 4; c++ {
				overlay.Pix[i-3+c] = uint8(uint32(overlay.Pix[i-3+c]) * m / 255)
			}
		}
	}
	if erase {
		alpha := image.NewAlpha(overlay.Bounds())
		for i := 3; i < len(overlay.Pix); i += 4 {
			alpha.Pix[i/4] = overlay.Pix[i]
		}
		render.Erase(layer, alpha)
		return
	}
	stddraw.Draw(layer, layer.Bounds(), overlay, image.Point{}, stddraw.Over)
}

// ClearArea empties the area (inside its clip, or its rectangle).
func ClearArea(layer *image.RGBA, a Area) {
	clip := a.Clip
	if len(clip) < 3 {
		clip = [][2]float64{{a.X, a.Y}, {a.X + a.W, a.Y}, {a.X + a.W, a.Y + a.H}, {a.X, a.Y + a.H}}
	}
	b := layer.Bounds()
	render.Erase(layer, render.PolygonMask(b.Dx(), b.Dy(), clip))
}

// svgRoot reports whether the root <svg> sets its own size.
func svgRoot(data []byte) (hasSize bool, err error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return false, fmt.Errorf("no <svg> element")
		}
		if err != nil {
			return false, fmt.Errorf("invalid SVG: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local != "svg" {
				return false, fmt.Errorf("the root element must be <svg>, got <%s>", se.Name.Local)
			}
			for _, a := range se.Attr {
				if a.Name.Local == "width" || a.Name.Local == "height" || a.Name.Local == "viewBox" {
					return true, nil
				}
			}
			return false, nil
		}
	}
}

// SVG rasterizes an SVG into the area. Without width/height/viewBox the SVG
// user units are layer pixels relative to the area origin; otherwise the SVG is
// scaled so its width fills the area width.
func SVG(data []byte, layerW, layerH int, a Area) (*image.RGBA, error) {
	has, err := svgRoot(data)
	if err != nil {
		return nil, err
	}
	src := string(data)
	if !has {
		i := strings.Index(src, "<svg")
		src = src[:i+4] + fmt.Sprintf(` width="%g" height="%g" viewBox="0 0 %g %g"`, a.W, a.H, a.W, a.H) + src[i+4:]
	}
	if err := checkFonts(src); err != nil {
		return nil, err
	}
	c, err := canvas.ParseSVG(strings.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("could not parse SVG: %w", err)
	}
	if c.W <= 0 || c.H <= 0 {
		return nil, fmt.Errorf("SVG has no size")
	}
	scale := a.W / c.W
	w, h := int(math.Ceil(c.W*scale)), int(math.Ceil(c.H*scale))
	piece := image.NewRGBA(image.Rect(0, 0, w, h))
	ras := rasterizer.FromImage(piece, canvas.DPMM(scale), canvas.DefaultColorSpace)
	c.RenderTo(ras)
	ras.Close()
	overlay := render.New(layerW, layerH)
	stddraw.Draw(overlay, image.Rect(int(math.Round(a.X)), int(math.Round(a.Y)), int(math.Round(a.X))+w, int(math.Round(a.Y))+h), piece, image.Point{}, stddraw.Over)
	return overlay, nil
}

// FontError is an SVG font-family that matches no installed font. The SVG
// renderer only loads system fonts by name and panics when one is missing, so
// fonts are checked before parsing.
type FontError struct{ Family string }

func (e FontError) Error() string {
	return fmt.Sprintf("no installed font matches font-family %q; use a font installed on this machine or a generic family (serif, sans-serif, monospace), or `sb draw text` (embedded THICCCBOI)", e.Family)
}

// Code is the JSON error code.
func (e FontError) Code() string { return "font_not_found" }

var fontFamilyRe = regexp.MustCompile(`font-family\s*(?:=\s*(?:"([^"]*)"|'([^']*)')|:\s*([^;"}<]+))`)

// checkFonts finds every font-family the SVG names (attributes, style
// attributes, <style> CSS; "serif" is the default for <text>) and reports the
// first that no system font matches. It may reject a family set on an element
// without text; that is cheaper than replaying style inheritance.
func checkFonts(src string) error {
	if !strings.Contains(src, "<text") {
		return nil
	}
	families := []string{"serif"}
	for _, m := range fontFamilyRe.FindAllStringSubmatch(src, -1) {
		families = append(families, strings.TrimSpace(m[1]+m[2]+m[3]))
	}
	for _, f := range families {
		if _, ok := canvas.FindSystemFont(f, canvas.FontRegular); !ok {
			return FontError{f}
		}
	}
	return nil
}

// Text draws a text block at x, y (area-local; y is the top of the first line).
func Text(layerW, layerH int, a Area, t TextSpec) *image.RGBA {
	overlay := render.New(layerW, layerH)
	t.X += a.X
	t.Y += a.Y
	DrawText(overlay, t)
	return overlay
}

// TextSpec describes a block of text.
type TextSpec struct {
	Text     string
	X, Y     float64
	Size     float64
	Width    float64 // wrap width (0 = no wrap)
	Font     string  // thin | light | regular | bold
	Color    color.Color
	Align    string // left | center | right (around X for center/right)
	Vertical bool   // top-to-bottom columns, right to left
}

// FontBytes maps a font name to the embedded THICCCBOI weight.
func FontBytes(name string) ([]byte, error) {
	switch name {
	case "", "regular":
		return render.FontRegular, nil
	case "thin":
		return render.FontThin, nil
	case "light":
		return render.FontLight, nil
	case "bold":
		return render.FontBold, nil
	}
	return nil, fmt.Errorf("font must be thin, light, regular or bold")
}

// DrawText lays out and draws text (horizontal lines or vertical columns).
func DrawText(dst *image.RGBA, t TextSpec) (w, h float64) {
	ttf, err := FontBytes(t.Font)
	if err != nil {
		ttf = render.FontRegular
	}
	if t.Size <= 0 {
		t.Size = 32
	}
	if t.Color == nil {
		t.Color = color.Black
	}
	face := render.FallbackFace(ttf, t.Size)
	lh := t.Size * 1.25
	if t.Vertical {
		// one column per paragraph (wrapped by height), columns right to left
		col := t.Size * 1.2
		x := t.X
		maxLen := 0
		cols := verticalColumns(t.Text, t.Width)
		for _, c := range cols {
			y := t.Y + t.Size
			for _, r := range c {
				render.DrawText(dst, face, string(r), x, y, t.Color, "center")
				y += t.Size * 1.05
			}
			maxLen = max(maxLen, len(c))
			x -= col
		}
		return col * float64(len(cols)), float64(maxLen) * t.Size * 1.05
	}
	lines := WrapText(face, t.Text, t.Width)
	y := t.Y + t.Size*0.95
	for _, l := range lines {
		render.DrawText(dst, face, l, t.X, y, t.Color, t.Align)
		w = math.Max(w, render.Measure(face, l))
		y += lh
	}
	return w, lh * float64(len(lines))
}

// verticalColumns splits text into columns of at most maxChars runes (0 = no limit).
func verticalColumns(text string, maxChars float64) [][]rune {
	var cols [][]rune
	for _, para := range strings.Split(text, "\n") {
		r := []rune(strings.ReplaceAll(para, " ", "　"))
		n := int(maxChars)
		if n <= 0 || n >= len(r) {
			cols = append(cols, r)
			continue
		}
		for len(r) > 0 {
			k := min(n, len(r))
			cols = append(cols, r[:k])
			r = r[k:]
		}
	}
	return cols
}

// WrapText breaks text at spaces (or anywhere for CJK) to fit width.
func WrapText(face font.Face, text string, width float64) []string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		if width <= 0 {
			out = append(out, para)
			continue
		}
		line := ""
		for _, word := range splitWords(para) {
			try := line + word
			if line != "" && render.Measure(face, strings.TrimRight(try, " ")) > width {
				out = append(out, strings.TrimRight(line, " "))
				line = strings.TrimLeft(word, " ")
				continue
			}
			line = try
		}
		out = append(out, strings.TrimRight(line, " "))
	}
	return out
}

// splitWords keeps spaces attached and treats each wide (CJK) rune as a word.
func splitWords(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		switch {
		case r == ' ':
			cur += " "
			out = append(out, cur)
			cur = ""
		case r >= 0x2E80:
			if cur != "" {
				out = append(out, cur)
			}
			out = append(out, string(r))
			cur = ""
		default:
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
