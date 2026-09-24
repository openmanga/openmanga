package pdf

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"sync"

	"github.com/signintech/gopdf"
	"golang.org/x/image/font"

	"sb/internal/render"
)

// Canvas is what the layout draws on: a PDF document or a raster preview.
// Coordinates are PDF points from the top-left corner.
type Canvas interface {
	NewPage(w, h float64)
	Text(f *Font, size float64, s string, x, baseline float64, c color.RGBA, opacity float64)
	FillRect(x, y, w, h float64, c color.RGBA, opacity float64)
	StrokeRect(x, y, w, h, lineWidth float64, c color.RGBA, opacity float64)
	Line(x1, y1, x2, y2, lineWidth float64, c color.RGBA, opacity float64)
	Image(path string, x, y, w, h float64) error
	PushClip(x, y, w, h float64)
	PopClip()
}

// ---- PDF ----

type pdfCanvas struct {
	pdf    *gopdf.GoPdf
	fonts  map[*Font]bool
	pageH  float64
	opaque bool
}

func newPDFCanvas(title, creator string) *pdfCanvas {
	p := &gopdf.GoPdf{}
	p.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	p.SetInfo(gopdf.PdfInfo{Title: title, Creator: creator, Producer: "sb (Storyboarder Next)"})
	return &pdfCanvas{pdf: p, fonts: map[*Font]bool{}, opaque: true}
}

func (c *pdfCanvas) NewPage(w, h float64) {
	c.pdf.AddPageWithOption(gopdf.PageOption{PageSize: &gopdf.Rect{W: w, H: h}})
	c.pageH = h
}

func (c *pdfCanvas) alpha(a float64) {
	if a >= 1 {
		if !c.opaque {
			c.pdf.ClearTransparency()
			c.opaque = true
		}
		return
	}
	c.pdf.SetTransparency(gopdf.Transparency{Alpha: a, BlendModeType: gopdf.NormalBlendMode})
	c.opaque = false
}

func (c *pdfCanvas) Text(f *Font, size float64, s string, x, baseline float64, col color.RGBA, opacity float64) {
	if s == "" {
		return
	}
	if !c.fonts[f] {
		if err := c.pdf.AddTTFFontDataWithOption(f.Name, f.TTF, gopdf.TtfOption{UseKerning: true}); err != nil {
			panic(fmt.Errorf("font %s: %w", f.Name, err))
		}
		c.fonts[f] = true
	}
	c.pdf.SetFont(f.Name, "", size)
	c.pdf.SetTextColor(col.R, col.G, col.B)
	c.alpha(opacity)
	// gopdf does not apply GPOS kerning, so each glyph goes at its kerned
	// position; a top-aligned cell puts the typo ascender at Y
	pos := f.Positions(s, size)
	top := baseline - f.Metrics.TypoAscender/1000*size
	i := 0
	for _, r := range s {
		if r != ' ' {
			c.pdf.SetXY(x+pos[i], top)
			c.pdf.CellWithOption(&gopdf.Rect{W: size * 2, H: size}, string(r), gopdf.CellOption{Align: gopdf.Left | gopdf.Top})
		}
		i++
	}
	c.alpha(1)
}

func (c *pdfCanvas) FillRect(x, y, w, h float64, col color.RGBA, opacity float64) {
	c.pdf.SetFillColor(col.R, col.G, col.B)
	c.alpha(opacity)
	c.pdf.RectFromUpperLeftWithStyle(x, y, w, h, "F")
	c.alpha(1)
}

func (c *pdfCanvas) StrokeRect(x, y, w, h, lw float64, col color.RGBA, opacity float64) {
	c.pdf.SetStrokeColor(col.R, col.G, col.B)
	c.pdf.SetLineWidth(lw)
	c.alpha(opacity)
	c.pdf.RectFromUpperLeftWithStyle(x, y, w, h, "D")
	c.alpha(1)
}

func (c *pdfCanvas) Line(x1, y1, x2, y2, lw float64, col color.RGBA, opacity float64) {
	c.pdf.SetStrokeColor(col.R, col.G, col.B)
	c.pdf.SetLineWidth(lw)
	c.alpha(opacity)
	c.pdf.Line(x1, y1, x2, y2)
	c.alpha(1)
}

func (c *pdfCanvas) Image(path string, x, y, w, h float64) error {
	return c.pdf.Image(path, x, y, &gopdf.Rect{W: w, H: h})
}

func (c *pdfCanvas) PushClip(x, y, w, h float64) {
	c.pdf.SaveGraphicsState()
	c.pdf.ClipPolygon([]gopdf.Point{{X: x, Y: y}, {X: x + w, Y: y}, {X: x + w, Y: y + h}, {X: x, Y: y + h}})
}

func (c *pdfCanvas) PopClip() {
	c.pdf.RestoreGraphicsState()
	c.opaque = true // the graphics state restore also reset transparency
}

// ---- raster preview ----

type rasterCanvas struct {
	scale float64
	img   *image.RGBA
	clips []image.Rectangle
	faces map[string]font.Face
	pages int
	want  int
}

func (c *rasterCanvas) NewPage(w, h float64) {
	c.pages++
	if c.pages-1 == c.want {
		c.img = render.White(int(math.Round(w*c.scale)), int(math.Round(h*c.scale)))
		c.clips = nil
	}
}

// target returns nil on pages we don't render.
func (c *rasterCanvas) target() *image.RGBA {
	if c.img == nil || c.pages-1 != c.want {
		return nil
	}
	if len(c.clips) == 0 {
		return c.img
	}
	return c.img.SubImage(c.clips[len(c.clips)-1]).(*image.RGBA)
}

func (c *rasterCanvas) rect(x, y, w, h float64) image.Rectangle {
	k := c.scale
	return image.Rect(int(math.Round(x*k)), int(math.Round(y*k)), int(math.Round((x+w)*k)), int(math.Round((y+h)*k)))
}

func withAlpha(col color.RGBA, a float64) color.NRGBA {
	return color.NRGBA{col.R, col.G, col.B, uint8(math.Round(math.Max(0, math.Min(1, a)) * 255))}
}

func (c *rasterCanvas) Text(f *Font, size float64, s string, x, baseline float64, col color.RGBA, opacity float64) {
	t := c.target()
	if t == nil || s == "" {
		return
	}
	key := fmt.Sprintf("%s/%g", f.Name, size)
	face, ok := c.faces[key]
	if !ok {
		face = render.Face(f.TTF, size*c.scale)
		c.faces[key] = face
	}
	render.DrawText(t, face, s, x*c.scale, baseline*c.scale, withAlpha(col, opacity), "left")
}

func (c *rasterCanvas) FillRect(x, y, w, h float64, col color.RGBA, opacity float64) {
	if t := c.target(); t != nil {
		render.FillRect(t, c.rect(x, y, w, h).Intersect(t.Bounds()), withAlpha(col, opacity))
	}
}

func (c *rasterCanvas) StrokeRect(x, y, w, h, lw float64, col color.RGBA, opacity float64) {
	c.Line(x, y, x+w, y, lw, col, opacity)
	c.Line(x, y+h, x+w, y+h, lw, col, opacity)
	c.Line(x, y, x, y+h, lw, col, opacity)
	c.Line(x+w, y, x+w, y+h, lw, col, opacity)
}

// Line draws the axis-aligned lines the layout uses, at least one pixel wide.
func (c *rasterCanvas) Line(x1, y1, x2, y2, lw float64, col color.RGBA, opacity float64) {
	t := c.target()
	if t == nil {
		return
	}
	half := math.Max(lw*c.scale, 1) / 2 / c.scale
	r := c.rect(math.Min(x1, x2)-half, math.Min(y1, y2)-half, math.Abs(x2-x1)+2*half, math.Abs(y2-y1)+2*half)
	if r.Dx() == 0 {
		r.Max.X++
	}
	if r.Dy() == 0 {
		r.Max.Y++
	}
	render.FillRect(t, r.Intersect(t.Bounds()), withAlpha(col, opacity))
}

func (c *rasterCanvas) Image(path string, x, y, w, h float64) error {
	t := c.target()
	if t == nil {
		return nil
	}
	img, err := render.Load(path)
	if err != nil {
		return err
	}
	r := c.rect(x, y, w, h)
	tmp := render.Resize(img, r.Dx(), r.Dy())
	render.DrawOver(t, r.Min, tmp, 1)
	return nil
}

func (c *rasterCanvas) PushClip(x, y, w, h float64) {
	r := c.rect(x, y, w, h)
	if len(c.clips) > 0 {
		r = r.Intersect(c.clips[len(c.clips)-1])
	} else if c.img != nil {
		r = r.Intersect(c.img.Bounds())
	}
	c.clips = append(c.clips, r)
}

func (c *rasterCanvas) PopClip() {
	if len(c.clips) > 0 {
		c.clips = c.clips[:len(c.clips)-1]
	}
}

// fallbackFont lazily loads the Unicode fallback font, or nil.
var (
	fallbackOnce sync.Once
	fallback     *Font
)

func fallbackFont() *Font {
	fallbackOnce.Do(func() {
		if p := render.FallbackFontPath(); p != "" {
			if data, err := os.ReadFile(p); err == nil {
				if m, err := readMetrics(data); err == nil {
					fallback = &Font{Name: "fallback", TTF: data, Metrics: m, faces: map[float64]font.Face{}}
				}
			}
		}
	})
	return fallback
}
