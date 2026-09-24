package render

import (
	"image"
	"image/color"
	stddraw "image/draw"
	"image/gif"
	"math"
	"os"
	"sort"
)

func drawMaskUniform(dst *image.RGBA, mask *image.Alpha, c color.Color) {
	stddraw.DrawMask(dst, dst.Bounds(), &image.Uniform{c}, image.Point{}, mask, mask.Bounds().Min, stddraw.Over)
}

// FillRect paints a rectangle with alpha blending.
func FillRect(dst *image.RGBA, r image.Rectangle, c color.Color) {
	stddraw.Draw(dst, r, &image.Uniform{c}, image.Point{}, stddraw.Over)
}

// Palette builds a 256-color palette by popularity of 4-bit-per-channel buckets
// (the original used NeuQuant via gifencoder; this is a simpler quantizer).
func Palette(frames []*image.RGBA) color.Palette {
	type acc struct{ r, g, b, n int }
	buckets := map[int]*acc{}
	for _, f := range frames {
		for i := 0; i < len(f.Pix); i += 16 { // sample every 4th pixel
			r, g, b := int(f.Pix[i]), int(f.Pix[i+1]), int(f.Pix[i+2])
			k := (r>>4)<<8 | (g>>4)<<4 | b>>4
			a := buckets[k]
			if a == nil {
				a = &acc{}
				buckets[k] = a
			}
			a.r += r
			a.g += g
			a.b += b
			a.n++
		}
	}
	list := make([]*acc, 0, len(buckets))
	for _, a := range buckets {
		list = append(list, a)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
	pal := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}}
	for _, a := range list {
		if len(pal) == 256 {
			break
		}
		pal = append(pal, color.RGBA{uint8(a.r / a.n), uint8(a.g / a.n), uint8(a.b / a.n), 255})
	}
	return pal
}

// WriteGIF encodes frames with per-frame delays in ms, looping forever.
func WriteGIF(path string, frames []*image.RGBA, delaysMs []float64) error {
	pal := Palette(frames)
	out := &gif.GIF{LoopCount: 0}
	for i, f := range frames {
		p := image.NewPaletted(f.Bounds(), pal)
		stddraw.FloydSteinberg.Draw(p, f.Bounds(), f, image.Point{})
		out.Image = append(out.Image, p)
		out.Delay = append(out.Delay, int(math.Round(delaysMs[i]/10)))
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return gif.EncodeAll(file, out)
}
