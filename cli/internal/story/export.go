package story

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/image/font"

	"sb/internal/ojson"
	"sb/internal/render"
)

// ExportsDir is <scene dir>/exports, created on demand (ensureExportsPathExists).
func ExportsDir(s *Scene) (string, error) {
	d := filepath.Join(s.Dir(), "exports")
	return d, os.MkdirAll(d, 0o755)
}

// Stamp is moment().format('YYYY-MM-DD hh.mm.ss') — a 12-hour clock, as the original.
func Stamp() string { return time.Now().Format("2006-01-02 03.04.05") }

// ExportImages writes <name>-board-00001.png flattened at full size for every board.
func ExportImages(s *Scene, outDir string) ([]string, error) {
	if outDir == "" {
		d, err := ExportsDir(s)
		if err != nil {
			return nil, err
		}
		outDir = filepath.Join(d, filepath.Base(s.Path)+" Images "+Stamp())
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	var files []string
	for i, b := range s.Boards() {
		w, h := SizeOf(s, b)
		img, _ := Flatten(s, b, w, h)
		p := filepath.Join(outDir, ExportFile(i, s.Name()))
		if err := render.SavePNG(p, img); err != nil {
			return files, err
		}
		files = append(files, p)
	}
	return files, nil
}

// fragmentText is the GIF exporter's word wrap.
func fragmentText(face font.Face, text string, maxWidth float64) []string {
	if render.Measure(face, text) < maxWidth {
		return []string{text}
	}
	words := strings.Split(text, " ")
	var lines []string
	line := ""
	for len(words) > 0 {
		for render.Measure(face, words[0]) >= maxWidth && len([]rune(words[0])) > 1 {
			r := []rune(words[0])
			words[0] = string(r[:len(r)-1])
			last := string(r[len(r)-1:])
			if len(words) > 1 {
				words[1] = last + words[1]
			} else {
				words = append(words, last)
			}
		}
		if render.Measure(face, line+words[0]) < maxWidth {
			line += words[0] + " "
			words = words[1:]
		} else {
			if line == "" { // a single oversized glyph: place it anyway
				line = words[0] + " "
				words = words[1:]
			}
			lines = append(lines, line)
			line = ""
		}
		if len(words) == 0 {
			lines = append(lines, line)
		}
	}
	return lines
}

// drawCaption draws dialogue like exporter.js exportAnimatedGif.
func drawCaption(img *image.RGBA, text string) {
	const fontSize = 22.0
	face := render.Face(render.FontLight, fontSize)
	W, H := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	lines := fragmentText(face, text, 450)
	n := float64(len(lines))
	outline := render.New(img.Bounds().Dx(), img.Bounds().Dy())
	for i, line := range lines {
		y := (float64(i)+1)*(fontSize+6) + (H - (n+1)*(fontSize+6)) - 20
		tw := render.Measure(face, line) / 2
		pad := 35.0
		// fillRect plus a 15 px strokeRect: the box grows by 7.5 px on each side
		x0, y0 := W/2-tw-pad/2-7.5, y-6-pad/2-7.5
		render.FillRect(outline, image.Rect(int(x0), int(y0), int(x0+tw*2+pad+15), int(y0+pad+15)), color.Black)
	}
	render.DrawOver(img, image.Point{}, outline, 0.5)
	for i, line := range lines {
		y := (float64(i)+1)*(fontSize+6) + (H - (n+1)*(fontSize+6)) - 20
		t := strings.TrimSpace(line)
		render.StrokeText(img, face, t, W/2, y, color.NRGBA{0, 0, 0, 204}, 4, "center")
		render.StrokeText(img, face, t, W/2, y+2, color.NRGBA{0, 0, 0, 51}, 4, "center")
		render.DrawText(img, face, t, W/2, y, color.White, "center")
	}
}

// ExportGIF writes an animated GIF 888 px wide (by default) with per-board timing
// and dialogue captions. outPath "" means exports/<name> <date>.gif.
func ExportGIF(s *Scene, boards []*ojson.Object, width int, outPath string) (string, error) {
	if outPath == "" {
		d, err := ExportsDir(s)
		if err != nil {
			return "", err
		}
		outPath = filepath.Join(d, s.Name()+" "+Stamp()+".gif")
	}
	bw, bh := s.ImageSize()
	h := int(float64(width) * float64(bh) / float64(bw))
	var frames []*image.RGBA
	var delays []float64
	for _, b := range boards {
		img, _ := Flatten(s, b, width, h)
		if d := b.Str("dialogue"); d != "" {
			drawCaption(img, d)
		}
		frames = append(frames, img)
		delay := s.DefaultBoardTiming()
		if f, ok := b.Num("duration"); ok && f != 0 {
			delay = f
		}
		delays = append(delays, delay)
	}
	return outPath, render.WriteGIF(outPath, frames, delays)
}
