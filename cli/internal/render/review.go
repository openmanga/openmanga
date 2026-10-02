package render

import (
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	"math"
	"strings"

	"golang.org/x/image/font"
)

// Grid overlays thin lines every step pixels (image coordinates, scaled by
// scale for smaller renders) with coordinate labels along the top and left,
// so drawing instructions can be placed precisely.
func Grid(dst *image.RGBA, step, scale float64) { GridAt(dst, step, scale, 0, 0) }

// GridAt is Grid for an image showing source pixels from ox, oy on (a crop):
// lines and labels stay in source coordinates.
func GridAt(dst *image.RGBA, step, scale, ox, oy float64) {
	if step <= 0 {
		return
	}
	b := dst.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	line := color.NRGBA{255, 0, 140, 90}
	major := color.NRGBA{255, 0, 140, 170}
	face := Face(FontRegular, math.Max(11, 14*math.Min(1, scale*2)))
	for i := int(math.Ceil(ox / step)); (float64(i)*step-ox)*scale <= w; i++ {
		x := int(math.Round((float64(i)*step - ox) * scale))
		c := line
		if i%5 == 0 {
			c = major
		}
		FillRect(dst, image.Rect(x, 0, x+1, b.Dy()), c)
		if x > 0 {
			label := fmt.Sprint(i * int(step))
			FillRect(dst, image.Rect(x+2, 2, x+4+int(Measure(face, label)), 18), color.NRGBA{255, 255, 255, 200})
			DrawText(dst, face, label, float64(x+3), 15, major, "left")
		}
	}
	for i := int(math.Ceil(oy / step)); (float64(i)*step-oy)*scale <= h; i++ {
		y := int(math.Round((float64(i)*step - oy) * scale))
		c := line
		if i%5 == 0 {
			c = major
		}
		FillRect(dst, image.Rect(0, y, b.Dx(), y+1), c)
		if y > 0 {
			label := fmt.Sprint(i * int(step))
			FillRect(dst, image.Rect(2, y+2, 4+int(Measure(face, label)), y+18), color.NRGBA{255, 255, 255, 200})
			DrawText(dst, face, label, 3, float64(y+15), major, "left")
		}
	}
}

// Cell is one tile of a contact sheet.
type Cell struct {
	Image   image.Image
	Title   string // e.g. "3  2A"
	Caption string // e.g. dialogue
}

// ContactSheet lays cells out in a grid: each image scaled to cellW wide,
// with a bold title and up to two caption lines under it.
func ContactSheet(cells []Cell, cols int, cellW int) *image.RGBA {
	if cols < 1 {
		cols = 4
	}
	cols = max(1, min(cols, len(cells)))
	pad := 24
	imgH := 0
	for _, c := range cells {
		b := c.Image.Bounds()
		imgH = max(imgH, int(math.Round(float64(cellW)*float64(b.Dy())/float64(b.Dx()))))
	}
	titleFace := Face(FontBold, 20)
	capFace := Face(FontRegular, 16)
	textH := 70
	rows := (len(cells) + cols - 1) / cols
	W := pad + cols*(cellW+pad)
	H := pad + rows*(imgH+textH+pad)
	dst := NewFilled(W, H, color.RGBA{242, 242, 242, 255})
	for i, c := range cells {
		x := pad + (i%cols)*(cellW+pad)
		y := pad + (i/cols)*(imgH+textH+pad)
		b := c.Image.Bounds()
		h := int(math.Round(float64(cellW) * float64(b.Dy()) / float64(b.Dx())))
		FillRect(dst, image.Rect(x-1, y-1, x+cellW+1, y+h+1), color.RGBA{160, 160, 160, 255})
		stddraw.Draw(dst, image.Rect(x, y, x+cellW, y+h), Resize(c.Image, cellW, h), image.Point{}, stddraw.Src)
		DrawText(dst, titleFace, c.Title, float64(x), float64(y+h+24), color.Black, "left")
		lines := wrapSimple(capFace, c.Caption, float64(cellW))
		for k, l := range lines {
			if k == 2 {
				break
			}
			if k == 1 && len(lines) > 2 {
				l = strings.TrimRight(l, " ") + " …"
			}
			DrawText(dst, capFace, l, float64(x), float64(y+h+46+k*20), color.RGBA{60, 60, 60, 255}, "left")
		}
	}
	return dst
}

func wrapSimple(face font.Face, text string, width float64) []string {
	var out []string
	line := ""
	for _, w := range strings.Fields(text) {
		try := strings.TrimSpace(line + " " + w)
		if line != "" && Measure(face, try) > width {
			out = append(out, line)
			line = w
			continue
		}
		line = try
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}
