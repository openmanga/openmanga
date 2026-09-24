// Package pdf reproduces exporters/pdf/generate.js (pdfkit) with gopdf, and
// renders the same layout to PNG for page previews.
package pdf

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/image/font"

	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/story"
)

// Config is the print-project context the generator reads.
type Config struct {
	PaperSizeKey     string  `json:"paperSizeKey"`
	Orientation      string  `json:"orientation"`
	GridDim          [2]int  `json:"gridDim"`
	Direction        string  `json:"direction"`
	EnableDialogue   bool    `json:"enableDialogue"`
	EnableAction     bool    `json:"enableAction"`
	EnableNotes      bool    `json:"enableNotes"`
	EnableShotNumber bool    `json:"enableShotNumber"`
	BoardTimeDisplay string  `json:"boardTimeDisplay"`
	BoardTextSize    float64 `json:"boardTextSize"`
	BoardBorderStyle string  `json:"boardBorderStyle"`
	Header           struct {
		Stats struct {
			Boards        bool `json:"boards"`
			Shots         bool `json:"shots"`
			SceneDuration bool `json:"sceneDuration"`
			AspectRatio   bool `json:"aspectRatio"`
			DateExported  bool `json:"dateExported"`
		} `json:"stats"`
	} `json:"header"`
}

// Preset is one of the 15 built-in layouts.
type Preset struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Data  Config `json:"data"`
}

//go:embed presets.json
var presetsJSON []byte

// Presets returns the built-in presets in the app's order.
func Presets() []Preset {
	var p []Preset
	if err := json.Unmarshal(presetsJSON, &p); err != nil {
		panic(err)
	}
	return p
}

// FindPreset matches an id ("preset-bcdaf15", "bcdaf15") or a title ignoring case.
func FindPreset(key string) (Preset, bool) {
	k := strings.ToLower(strings.TrimSpace(key))
	norm := func(s string) string {
		return strings.NewReplacer("×", "x", " ", "", "-", "").Replace(strings.ToLower(s))
	}
	for _, p := range Presets() {
		if p.ID == k || strings.TrimPrefix(p.ID, "preset-") == k || norm(p.Title) == norm(k) {
			return p, true
		}
	}
	return Preset{}, false
}

// PaperSize is getPaperSize in the print window (points).
func PaperSize(key, orientation string) (float64, float64, error) {
	var a, b float64
	switch key {
	case "", "a4":
		a, b = 841.89, 595.28
	case "letter":
		a, b = 792, 612
	default:
		return 0, 0, fmt.Errorf("paper must be a4 or letter")
	}
	if orientation == "portrait" {
		return b, a, nil
	}
	return a, b, nil
}

// SceneData is one scene of the printed project.
type SceneData struct {
	Title string
	Scene *story.Scene
}

// Project is the data the generator prints; Title is the script title or "".
type Project struct {
	Title  string
	Scenes []SceneData
}

type page struct {
	index  int
	scene  SceneData
	boards []*ojson.Object
}

// groupByPage is exporters/pdf/group-by-page.js.
func groupByPage(scenes []SceneData, perPage int) []page {
	var pages []page
	for _, sc := range scenes {
		boards := sc.Scene.Boards()
		for start := 0; start < len(boards); start += perPage {
			end := min(start+perPage, len(boards))
			pages = append(pages, page{index: len(pages), scene: sc, boards: boards[start:end]})
		}
	}
	return pages
}

// PageCount returns how many pages a config produces.
func PageCount(p Project, cfg Config) int {
	return len(groupByPage(p.Scenes, max(1, cfg.GridDim[0]*cfg.GridDim[1])))
}

type rect struct{ X, Y, W, H float64 }

func (r rect) inset(dx, dy float64) rect { return rect{r.X + dx, r.Y + dy, r.W - 2*dx, r.H - 2*dy} }

// fit keeps the aspect ratio of [wi, hi] inside [ws, hs].
func fit(wi, hi, ws, hs float64) (float64, float64) {
	if ws/hs > wi/hi {
		return wi * hs / hi, hs
	}
	return ws, hi * ws / wi
}

var black = color.RGBA{0, 0, 0, 255}

const ellipsis = "[…]"

// doc is a tiny pdfkit-like text state on top of a Canvas.
type doc struct {
	c       Canvas
	pageW   float64
	margin  float64
	opacity float64
}

type textOpts struct {
	width, height float64
	align         string // left | center | right
	baseline      string // top (default) | middle | bottom | alphabetic
	ellipsis      string
}

func (d *doc) font(f *Font, s string) *Font {
	if ContainsForeign(s) {
		if fb := fallbackFont(); fb != nil {
			return fb
		}
	}
	return f
}

var reWords = regexp.MustCompile(`\S+\s*|\s+`)

// wrap splits text into lines of at most width (words keep trailing spaces).
func wrap(f *Font, size float64, text string, width float64) []string {
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		line, lw := "", 0.0
		words := reWords.FindAllString(para, -1)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		for _, w := range words {
			ww := f.Width(w, size)
			if lw+ww <= width || line == "" && ww <= width {
				line += w
				lw += ww
				continue
			}
			if line != "" && ww <= width {
				lines = append(lines, line)
				line, lw = w, ww
				continue
			}
			// the word is wider than a line: chop it
			if line != "" {
				lines = append(lines, line)
				line, lw = "", 0
			}
			r := []rune(w)
			for len(r) > 0 {
				n := len(r)
				for n > 1 && f.Width(string(r[:n]), size) > width {
					n--
				}
				if n == len(r) {
					line, lw = string(r), f.Width(string(r), size)
					break
				}
				lines = append(lines, string(r[:n]))
				r = r[n:]
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// text draws like pdfkit doc.text(s, x, y, {width, height, align, baseline, ellipsis}).
// It returns the y below the last line drawn.
func (d *doc) text(base *Font, size float64, s string, x, y float64, o textOpts, col color.RGBA) float64 {
	if s == "" {
		return y
	}
	f := d.font(base, s)
	if o.width <= 0 {
		o.width = math.Max(0, d.pageW-x-d.margin)
	}
	lh := f.LineHeight(size)
	var dy float64
	m := f.Metrics
	switch o.baseline {
	case "middle":
		dy = 0.5 * (m.Descender + m.Ascender)
	case "bottom":
		dy = m.Descender
	case "alphabetic":
		dy = 0
	default:
		dy = m.Ascender
	}
	dy = dy / 1000 * size
	lines := wrap(f, size, s, o.width)
	maxY := math.Inf(1)
	if o.height > 0 {
		maxY = y + o.height
	}
	n := len(lines)
	for i := 1; i < len(lines); i++ {
		if y+float64(i)*lh+lh > maxY {
			n = i
			break
		}
	}
	if n < len(lines) && o.ellipsis != "" {
		buf := strings.TrimRight(lines[n-1], " \t")
		for buf != "" && f.Width(buf+o.ellipsis, size) > o.width {
			r := []rune(buf)
			buf = strings.TrimRight(string(r[:len(r)-1]), " \t")
		}
		if f.Width(buf+o.ellipsis, size) <= o.width {
			buf += o.ellipsis
		}
		lines[n-1] = buf
	}
	for _, line := range lines[:n] {
		xx := x
		switch o.align {
		case "right":
			xx += o.width - f.Width(strings.TrimRight(line, " \t"), size)
		case "center":
			xx += o.width/2 - f.Width(line, size)/2
		}
		d.c.Text(f, size, line, xx, y+dy, col, d.opacity)
		y += lh
	}
	return y
}

// FormatMsecs is exporters/pdf/format-msecs.js.
func FormatMsecs(ms float64) string {
	if ms < 0 || math.IsNaN(ms) {
		ms = 0
	}
	t := math.Round(ms / 1000)
	h := math.Floor(t / 3600)
	m := math.Mod(math.Floor(t/60), 60)
	s := math.Mod(t, 60)
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", int(h), int(m), int(s))
	}
	return fmt.Sprintf("%d:%02d", int(m), int(s))
}

var humanAspect = map[string]string{"2.390": "2.39:1", "2.000": "2.00:1", "1.850": "1.85:1", "1.778": "16:9", "0.563": "9:16", "1.000": "1:1", "1.334": "4:3"}

func humanizeAspect(a float64) string {
	k := fmt.Sprintf("%.3f", a)
	if v, ok := humanAspect[k]; ok {
		return v
	}
	return k
}

func (d *doc) drawHeader(r rect, projectTitle, sceneTitle string, sc *story.Scene, pageIdx, total int, cfg Config, now time.Time) {
	// titles: "<project> / " thin + scene bold, baseline bottom at y+20
	x := r.X
	if projectTitle != "" {
		t := projectTitle + " / "
		d.text(Thin, 20, t, x, r.Y+20, textOpts{width: r.W, baseline: "bottom"}, black)
		x += d.font(Thin, t).Width(t, 20)
	}
	d.text(Bold, 20, sceneTitle, x, r.Y+20, textOpts{width: r.W - (x - r.X), baseline: "bottom"}, black)
	y := r.Y + 20 + 0.25*Thin.LineHeight(7)

	// stats
	type entry struct{ name, value string }
	var entries []entry
	st := cfg.Header.Stats
	boards := sc.Boards()
	shots := 0
	for i, b := range boards {
		if i == 0 || b.Bool("newShot") {
			shots++
		}
	}
	if st.Boards {
		entries = append(entries, entry{"Boards", fmt.Sprint(len(boards))})
	}
	if st.Shots {
		entries = append(entries, entry{"Shots", fmt.Sprint(shots)})
	}
	if st.SceneDuration {
		entries = append(entries, entry{"Duration", FormatMsecs(story.SceneDuration(sc))})
	}
	if st.AspectRatio {
		entries = append(entries, entry{"Aspect Ratio", humanizeAspect(sc.AspectRatio())})
	}
	d.opacity = 0.8
	run := func(parts [][2]any, y float64) {
		x := r.X
		for _, p := range parts {
			f, s := p[0].(*Font), p[1].(string)
			d.text(f, 6, s, x, y, textOpts{width: r.W}, black)
			x += d.font(f, s).Width(s, 6)
		}
	}
	if len(entries) > 0 {
		var parts [][2]any
		for i, e := range entries {
			parts = append(parts, [2]any{Thin, e.name + " "}, [2]any{Regular, e.value})
			if i < len(entries)-1 {
				parts = append(parts, [2]any{Thin, " / "})
			}
		}
		run(parts, y)
		y += Regular.LineHeight(6)
	}
	if st.DateExported {
		if len(entries) > 0 {
			y += 0.25 * Thin.LineHeight(6)
		}
		run([][2]any{{Thin, "DRAFT "}, {Regular, strings.ToUpper(now.Format("2 Jan 2006"))}}, y)
	}
	d.opacity = 1
	d.text(Thin, 7, fmt.Sprintf("%d / %d", pageIdx+1, total), r.X, r.Y, textOpts{width: r.W, align: "right"}, black)
}

func (d *doc) drawFooter(r rect) {
	d.opacity = 0.6
	// the original draws "Storyboarder by \\ wonder unit" with a font ligature
	d.text(Thin, 8, "Storyboarder by wonder unit", r.X, r.Y+r.H, textOpts{width: r.W, align: "right", baseline: "alphabetic"}, black)
	d.opacity = 1
}

func (d *doc) drawImageOrPlaceholder(path string, r rect, cfg Config) {
	if f, err := os.Open(path); err == nil {
		ic, _, err := image.DecodeConfig(f)
		f.Close()
		if err == nil {
			w, h := fit(float64(ic.Width), float64(ic.Height), r.W, r.H)
			if d.c.Image(path, r.X, r.Y, w, h) == nil {
				return
			}
		}
	}
	d.c.FillRect(r.X, r.Y, r.W, r.H, color.RGBA{255, 0, 0, 255}, 1)
	msg := "Error: Missing Posterframe"
	d.text(Thin, cfg.BoardTextSize, msg, r.X, r.Y+(r.H-Thin.LineHeight(cfg.BoardTextSize))/2, textOpts{width: r.W, align: "center"}, black)
}

type boardArgs struct {
	cell, container rect
	sc              *story.Scene
	b               *ojson.Object
}

func timeText(cfg Config, sc *story.Scene, b *ojson.Object) (string, bool) {
	switch cfg.BoardTimeDisplay {
	case "duration":
		return FormatMsecs(story.Duration(sc, b)), true
	case "sceneTime":
		return FormatMsecs(b.NumOr("time", 0)), true
	}
	return "", false
}

// drawBoardRow places the text to the right of the image.
func (d *doc) drawBoardRow(a boardArgs, cfg Config) {
	ts := cfg.BoardTextSize
	inner := a.cell
	inner.W -= 10
	iw, ih := story.ImageSize(a.sc.AspectRatio())
	imageR := inner.inset(1, 1)
	imageR.W, imageR.H = fit(float64(iw), float64(ih), imageR.W*0.6, imageR.H)
	cellA := inner
	cellA.W = math.Min(ts*4, cellA.W*0.1) - 1
	imageR.X += cellA.W + 1
	imageB := imageR.inset(-1, -1)
	cellB := inner
	cellB.X = imageR.X + imageR.W + 1
	cellB.W -= cellA.W + 1 + imageR.W + 1
	cellAinner := cellA.inset(5, 5)
	cellBinner := cellB.inset(5, 5)
	cellBinner.W += 5

	d.c.FillRect(imageB.X, imageB.Y, imageB.W, imageB.H, black, 1)
	d.drawImageOrPlaceholder(a.sc.ImagePath(story.PosterframeFile(a.b)), imageR, cfg)

	if a.b.Bool("newShot") {
		m := inner
		m.W = 2
		m.X -= 1 + 0.1
		d.c.FillRect(m.X, m.Y, m.W, m.H, black, 1)
		d.c.StrokeRect(m.X, m.Y, m.W, m.H, 0.1, black, 1)
	}
	if cfg.EnableShotNumber {
		f := Thin
		if a.b.Bool("newShot") {
			f = Bold
		}
		d.text(f, ts, a.b.Str("shot"), cellAinner.X, cellAinner.Y, textOpts{}, black)
	}

	type entry struct {
		text  string
		font  *Font
		align string
		size  float64
	}
	var entries []entry
	if cfg.EnableDialogue {
		entries = append(entries, entry{a.b.Str("dialogue"), Bold, "left", ts})
	}
	if cfg.EnableAction {
		entries = append(entries, entry{a.b.Str("action"), Regular, "left", ts})
	}
	if cfg.EnableNotes {
		entries = append(entries, entry{a.b.Str("notes"), Thin, "left", ts})
	}
	if t, ok := timeText(cfg, a.sc, a.b); ok {
		entries = append(entries, entry{t, Thin, "right", ts - 1})
	}
	maxNarrow := ts * 4
	hasNarrow := len(entries) > 1 && entries[len(entries)-1].align == "right"
	container := cellBinner
	if hasNarrow {
		container.W -= maxNarrow
	}
	rects := make([]rect, len(entries))
	xpos := 0.0
	for e := range entries {
		r := container
		if e == len(entries)-1 && hasNarrow {
			r.W = maxNarrow
		} else {
			k := len(entries)
			if hasNarrow {
				k--
			}
			r.W = container.W / float64(k)
		}
		r.X = container.X + xpos
		xpos += r.W
		r.W -= 5
		rects[e] = r
	}
	for e, en := range entries {
		r := &rects[e]
		r.H += 5
		if en.text != "" {
			d.c.PushClip(r.X, r.Y, r.W, r.H)
			d.text(en.font, en.size, en.text, r.X, r.Y, textOpts{align: en.align, width: r.W, height: r.H, ellipsis: ellipsis}, black)
			d.c.PopClip()
		}
		if cfg.BoardBorderStyle != "minimal" && e != len(entries)-1 {
			b := *r
			b.Y, b.H = cellB.Y, cellB.H
			b.X += 2.5
			d.c.Line(b.X+b.W, b.Y, b.X+b.W, b.Y+b.H, 0.1, black, 0.25)
		}
	}
}

// drawBoardColumn places the text below the image.
func (d *doc) drawBoardColumn(a boardArgs, cfg Config) {
	ts := cfg.BoardTextSize
	minimal := cfg.BoardBorderStyle == "minimal"
	inner := a.cell
	inner.W -= 15
	inner.H -= 10
	iw, ih := story.ImageSize(a.sc.AspectRatio())
	imageR := inner
	imageR.W, imageR.H = fit(float64(iw), float64(ih), inner.W, inner.H*0.6)
	inner.W = imageR.W
	remaining := inner.H - imageR.H
	upperR := inner
	upperR.H = math.Min(ts*3, remaining*0.3)
	if !minimal {
		upperR = upperR.inset(5, 0)
	}
	lowerR := inner
	lowerR.H = remaining - upperR.H
	upperR.Y = inner.Y
	imageR.Y = upperR.Y + upperR.H
	lowerR.Y = imageR.Y + imageR.H
	lowerR = lowerR.inset(3, 3)

	d.drawImageOrPlaceholder(a.sc.ImagePath(story.PosterframeFile(a.b)), imageR, cfg)

	if a.b.Bool("newShot") {
		m := imageR
		if !minimal {
			m = inner
		}
		m.W = 2
		m.X -= 2
		d.c.FillRect(m.X, m.Y, m.W, m.H, black, 1)
		d.c.StrokeRect(m.X, m.Y, m.W, m.H, 0.1, black, 1)
	}

	baseline, py := "middle", upperR.Y+upperR.H*0.5
	if minimal {
		baseline, py = "bottom", upperR.Y+upperR.H*0.875
	}
	if cfg.EnableShotNumber {
		f := Thin
		if a.b.Bool("newShot") {
			f = Bold
		}
		d.text(f, ts, a.b.Str("shot"), upperR.X, py, textOpts{baseline: baseline}, black)
	}
	if t, ok := timeText(cfg, a.sc, a.b); ok {
		d.text(Thin, ts-1, t, upperR.X, py, textOpts{width: upperR.W, align: "right", baseline: baseline}, black)
	}

	type entry struct {
		text string
		font *Font
	}
	all := []*entry{nil, nil, nil}
	if cfg.EnableDialogue && a.b.Str("dialogue") != "" {
		all[0] = &entry{a.b.Str("dialogue"), Bold}
	}
	if cfg.EnableAction && a.b.Str("action") != "" {
		all[1] = &entry{a.b.Str("action"), Regular}
	}
	if cfg.EnableNotes && a.b.Str("notes") != "" {
		all[2] = &entry{a.b.Str("notes"), Thin}
	}

	if minimal {
		// one multi-line centered text field as wide as the grid cell
		tf := rect{lowerR.X + lowerR.W/2, lowerR.Y, a.container.W - 10, lowerR.H + 5}
		tf.X -= tf.W / 2
		tw := Thin.Width("M", ts)
		th := Thin.LineHeight(ts)
		tf.W = math.Ceil(tf.W/tw) * tw
		tf.H = math.Ceil(tf.H/th) * th
		y := tf.Y
		for _, e := range all {
			if e == nil {
				continue
			}
			er := rect{tf.X, y, tf.W, math.Max(0, tf.H-(y-tf.Y))}
			d.c.PushClip(er.X, er.Y, er.W, er.H)
			y = d.text(e.font, ts, e.text, er.X, er.Y, textOpts{width: er.W, height: er.H, align: "center"}, black)
			d.c.PopClip()
		}
	} else {
		d.c.PushClip(lowerR.X, lowerR.Y, lowerR.W, lowerR.H)
		for i, e := range all {
			cell := lowerR
			cell.H /= float64(len(all))
			cell.Y += cell.H * float64(i)
			if e != nil {
				d.text(e.font, ts, e.text, cell.X, cell.Y, textOpts{width: cell.W, height: cell.H, ellipsis: ellipsis}, black)
			}
		}
		d.c.PopClip()
		d.c.StrokeRect(inner.X, inner.Y, inner.W, inner.H, 0.1, black, 1)
	}
	d.c.StrokeRect(imageR.X, imageR.Y, imageR.W, imageR.H, 0.1, black, 1)
}

func (d *doc) drawBordersRow(n, j, last int, r rect, cfg Config) {
	if cfg.BoardBorderStyle == "minimal" {
		return
	}
	first := j == 0
	isLast := j == cfg.GridDim[1]-1 || n == last
	if first {
		d.c.Line(r.X, r.Y, r.X+r.W, r.Y, 0.1, black, 0.25)
	}
	if isLast {
		d.c.Line(r.X, r.Y+r.H, r.X+r.W, r.Y+r.H, 0.1, black, 0.25)
	}
	d.c.Line(r.X+r.W, r.Y, r.X+r.W, r.Y+r.H, 0.1, black, 0.25)
	d.c.Line(r.X, r.Y, r.X, r.Y+r.H, 0.1, black, 0.25)
}

// generate draws pages [from, to] (0-based, inclusive) of the layout.
func generate(c Canvas, p Project, cfg Config, from, to int, now time.Time) error {
	if cfg.GridDim[0] < 1 || cfg.GridDim[1] < 1 {
		return fmt.Errorf("grid must be at least 1x1")
	}
	pw, ph, err := PaperSize(cfg.PaperSizeKey, cfg.Orientation)
	if err != nil {
		return err
	}
	for _, s := range p.Scenes {
		story.UpdateTiming(s.Scene)
	}
	pages := groupByPage(p.Scenes, cfg.GridDim[0]*cfg.GridDim[1])
	if len(pages) == 0 {
		return fmt.Errorf("nothing to print: no boards")
	}
	to = min(to, len(pages)-1)
	if from < 0 || from > to {
		return fmt.Errorf("page out of range (1-%d)", len(pages))
	}
	const margin = 22.0
	d := &doc{c: c, pageW: pw, margin: margin, opacity: 1}
	for _, pg := range pages[from : to+1] {
		c.NewPage(pw, ph)
		full := rect{margin, margin, pw - 2*margin, ph - 2*margin}
		header := rect{full.X, full.Y, full.W, full.H / 12}
		footer := rect{full.X, full.Y, full.W, full.H / 24}
		grid := full
		grid.H = full.H - header.H - footer.H
		grid.Y = header.Y + header.H
		footer.Y = grid.Y + grid.H

		d.drawHeader(header, p.Title, pg.scene.Title, pg.scene.Scene, pg.index, len(pages), cfg, now)
		d.drawFooter(footer)

		gd := [2]float64{float64(cfg.GridDim[0]), float64(cfg.GridDim[1])}
		tmpl := rect{grid.X, grid.Y, grid.W / gd[0], grid.H / gd[1]}
		iw, ih := story.ImageSize(pg.scene.Scene.AspectRatio())
		var boardW float64
		if cfg.Direction == "column" {
			boardW, _ = fit(float64(iw), float64(ih), tmpl.W-15, (tmpl.H-10)*0.6)
		} else {
			boardW = tmpl.W - 10
		}
		tmpl.X += (grid.W - (tmpl.W*(gd[0]-1) + boardW)) / 2

		for n, b := range pg.boards {
			var i, j int
			if cfg.Direction == "row" {
				i, j = n/cfg.GridDim[1], n%cfg.GridDim[1]
			} else {
				i, j = n%cfg.GridDim[0], n/cfg.GridDim[0]
			}
			cell := rect{tmpl.X + tmpl.W*float64(i), tmpl.Y + tmpl.H*float64(j), tmpl.W, tmpl.H}
			args := boardArgs{cell: cell, container: tmpl, sc: pg.scene.Scene, b: b}
			if cfg.Direction == "row" {
				d.drawBoardRow(args, cfg)
				d.drawBordersRow(n, j, len(pg.boards)-1, rect{cell.X + 1, cell.Y, cell.W - 10, cell.H}, cfg)
			} else {
				d.drawBoardColumn(args, cfg)
			}
		}
	}
	return nil
}

// Write generates the PDF file for pages [from, to] (0-based, inclusive; to < 0 means all).
func Write(path string, p Project, cfg Config, from, to int) error {
	title := p.Title
	if title == "" && len(p.Scenes) > 0 {
		title = p.Scenes[0].Title
	}
	c := newPDFCanvas(title, "Storyboarder v"+story.AppVersion)
	if to < 0 {
		to = math.MaxInt32
	}
	if err := generate(c, p, cfg, from, to, time.Now()); err != nil {
		return err
	}
	return c.pdf.WritePdf(path)
}

// Preview renders one page (0-based) to a PNG at scale (1 = 72 dpi).
func Preview(path string, p Project, cfg Config, pageIdx int, scale float64) error {
	c := &rasterCanvas{scale: scale, faces: map[string]font.Face{}, want: 0}
	if err := generate(c, p, cfg, pageIdx, pageIdx, time.Now()); err != nil {
		return err
	}
	return render.SavePNG(path, c.img)
}
