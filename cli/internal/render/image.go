// Package render holds the 2D raster operations the app did with canvas:
// loading, fitting, compositing with opacity, flipping, transforms, lasso
// masks, and PNG/JPEG encoding.
package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/vector"
)

// JPEGQuality matches canvas.toDataURL('image/jpeg') default (0.92).
const JPEGQuality = 92

// Load decodes a PNG, JPEG or GIF file.
func Load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("could not load image %s: %w", filepath.Base(path), err)
	}
	return img, nil
}

// DecodeDataURL decodes a data:image/...;base64 URL (clipboard JSON format).
func DecodeDataURL(u string) (image.Image, error) {
	i := strings.Index(u, ",")
	if !strings.HasPrefix(u, "data:") || i < 0 {
		return nil, fmt.Errorf("not a data URL")
	}
	raw, err := base64.StdEncoding.DecodeString(u[i+1:])
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	return img, err
}

// DataURL encodes a PNG file's bytes as a data URL.
func DataURL(pngBytes []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
}

// New returns a transparent canvas.
func New(w, h int) *image.RGBA { return image.NewRGBA(image.Rect(0, 0, w, h)) }

// NewFilled returns a canvas filled with c.
func NewFilled(w, h int, c color.Color) *image.RGBA {
	img := New(w, h)
	stddraw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, stddraw.Src)
	return img
}

// White returns a white canvas.
func White(w, h int) *image.RGBA { return NewFilled(w, h, color.White) }

// ToRGBA copies any image into an RGBA canvas of the same size at origin 0,0.
func ToRGBA(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := New(b.Dx(), b.Dy())
	stddraw.Draw(dst, dst.Bounds(), src, b.Min, stddraw.Src)
	return dst
}

// Resize scales src to exactly w x h.
func Resize(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	if b.Dx() == w && b.Dy() == h {
		return ToRGBA(src)
	}
	dst := New(w, h)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}

// DrawScaled draws src into rect r of dst (like context.drawImage(img, x, y, w, h)).
func DrawScaled(dst *image.RGBA, r image.Rectangle, src image.Image, opacity float64) {
	if r.Empty() {
		return
	}
	tmp := Resize(src, r.Dx(), r.Dy())
	DrawOver(dst, r.Min, tmp, opacity)
}

// DrawOver composites src at pt with a global alpha, source-over.
func DrawOver(dst *image.RGBA, pt image.Point, src image.Image, opacity float64) {
	sb := src.Bounds()
	r := image.Rectangle{pt, pt.Add(sb.Size())}
	if opacity >= 1 {
		stddraw.Draw(dst, r, src, sb.Min, stddraw.Over)
		return
	}
	a := uint8(math.Round(math.Max(0, opacity) * 255))
	stddraw.DrawMask(dst, r, src, sb.Min, &image.Uniform{color.Alpha{a}}, image.Point{}, stddraw.Over)
}

// Fit is util.fitToDst: the rect that fits src inside dst keeping aspect, centered.
func Fit(dw, dh, sw, sh float64) (x, y, w, h float64) {
	if dw/dh > sw/sh {
		w, h = sw*dh/sh, dh
	} else {
		w, h = dw, sh*dw/sw
	}
	return (dw - w) / 2, (dh - h) / 2, w, h
}

// FitImage returns a transparent w x h canvas with src fit inside (fitImageData).
// An image that already has the exact size is returned unchanged.
func FitImage(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	if b.Dx() == w && b.Dy() == h {
		return ToRGBA(src)
	}
	dst := New(w, h)
	x, y, fw, fh := Fit(float64(w), float64(h), float64(b.Dx()), float64(b.Dy()))
	r := image.Rect(int(math.Round(x)), int(math.Round(y)), int(math.Round(x))+int(math.Round(fw)), int(math.Round(y))+int(math.Round(fh)))
	DrawScaled(dst, r, src, 1)
	return dst
}

// CoverImage scales src to cover w x h (centered, cropped).
func CoverImage(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	k := math.Max(float64(w)/float64(b.Dx()), float64(h)/float64(b.Dy()))
	sw, sh := int(math.Round(float64(b.Dx())*k)), int(math.Round(float64(b.Dy())*k))
	dst := New(w, h)
	x, y := (w-sw)/2, (h-sh)/2
	DrawScaled(dst, image.Rect(x, y, x+sw, y+sh), src, 1)
	return dst
}

// Flip mirrors horizontally, or vertically when vertical is true.
func Flip(src *image.RGBA, vertical bool) *image.RGBA {
	b := src.Bounds()
	dst := New(b.Dx(), b.Dy())
	w, h := b.Dx(), b.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := w-1-x, y
			if vertical {
				sx, sy = x, h-1-y
			}
			i := dst.PixOffset(x, y)
			j := src.PixOffset(sx+b.Min.X, sy+b.Min.Y)
			copy(dst.Pix[i:i+4], src.Pix[j:j+4])
		}
	}
	return dst
}

// Transform scales around an anchor then translates, keeping the canvas size.
func Transform(src *image.RGBA, dx, dy, scale, ax, ay float64) *image.RGBA {
	b := src.Bounds()
	dst := New(b.Dx(), b.Dy())
	// new position of the image's top-left corner
	x0 := ax - ax*scale + dx
	y0 := ay - ay*scale + dy
	w := float64(b.Dx()) * scale
	h := float64(b.Dy()) * scale
	r := image.Rect(int(math.Round(x0)), int(math.Round(y0)), int(math.Round(x0+w)), int(math.Round(y0+h)))
	if r.Empty() {
		return dst
	}
	tmp := New(r.Dx(), r.Dy())
	draw.CatmullRom.Scale(tmp, tmp.Bounds(), src, b, draw.Src, nil)
	stddraw.Draw(dst, r, tmp, image.Point{}, stddraw.Src)
	return dst
}

// PolygonMask rasterizes a closed polygon (anti-aliased) into an alpha mask.
func PolygonMask(w, h int, pts [][2]float64) *image.Alpha {
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	if len(pts) < 3 {
		return mask
	}
	z := vector.NewRasterizer(w, h)
	z.MoveTo(float32(pts[0][0]), float32(pts[0][1]))
	for _, p := range pts[1:] {
		z.LineTo(float32(p[0]), float32(p[1]))
	}
	z.ClosePath()
	z.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	return mask
}

// Erase clears pixels under the mask (destination-out).
func Erase(img *image.RGBA, mask *image.Alpha) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			m := mask.AlphaAt(x, y).A
			if m == 0 {
				continue
			}
			i := img.PixOffset(x, y)
			k := 255 - uint32(m)
			for c := 0; c < 4; c++ {
				img.Pix[i+c] = uint8(uint32(img.Pix[i+c]) * k / 255)
			}
		}
	}
}

// FillMask paints c with opacity through the mask (source-over).
func FillMask(img *image.RGBA, mask *image.Alpha, c color.Color, opacity float64) {
	src := &image.Uniform{c}
	m := image.NewAlpha(mask.Bounds())
	for i, a := range mask.Pix {
		m.Pix[i] = uint8(math.Round(float64(a) * opacity))
	}
	stddraw.DrawMask(img, img.Bounds(), src, image.Point{}, m, image.Point{}, stddraw.Over)
}

// Cut returns the masked pixels as their own layer and erases them from img.
func Cut(img *image.RGBA, mask *image.Alpha) *image.RGBA {
	b := img.Bounds()
	piece := New(b.Dx(), b.Dy())
	stddraw.DrawMask(piece, piece.Bounds(), img, b.Min, mask, image.Point{}, stddraw.Src)
	Erase(img, mask)
	return piece
}

// IsBlank reports whether every pixel is fully transparent.
func IsBlank(img *image.RGBA) bool {
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 {
			return false
		}
	}
	return true
}

// ParseColor accepts #rgb, #rrggbb or rrggbb.
func ParseColor(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	var r, g, b uint8
	if len(s) != 6 {
		return color.RGBA{}, fmt.Errorf("invalid color %q", s)
	}
	if _, err := fmt.Sscanf(s, "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.RGBA{}, fmt.Errorf("invalid color %q", s)
	}
	return color.RGBA{r, g, b, 255}, nil
}

// EncodePNG returns PNG bytes.
func EncodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// SavePNG writes a PNG file.
func SavePNG(path string, img image.Image) error {
	return os.WriteFile(path, EncodePNG(img), 0o644)
}

// SaveJPEG writes a JPEG file on white (JPEG has no alpha).
func SaveJPEG(path string, img image.Image, quality int) error {
	b := img.Bounds()
	flat := White(b.Dx(), b.Dy())
	stddraw.Draw(flat, flat.Bounds(), img, b.Min, stddraw.Over)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, flat, &jpeg.Options{Quality: quality}); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
