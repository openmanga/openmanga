package story

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"sb/internal/ojson"
	"sb/internal/render"
)

// DeleteBoards removes boards by index, always keeping at least one (deleteBoards).
// Files stay on disk.
func DeleteBoards(s *Scene, idx []int) int {
	sort.Sort(sort.Reverse(sort.IntSlice(idx)))
	n := 0
	last := -1
	for _, i := range idx {
		boards := s.Boards()
		if i == last || i < 0 || i >= len(boards) || len(boards) <= 1 {
			continue
		}
		last = i
		s.SetBoards(append(boards[:i], boards[i+1:]...))
		n++
	}
	return n
}

// DuplicateBoard copies board i to i+1 under a new uid with its layers,
// thumbnail, posterframe and linked file; text and audio are cleared.
func DuplicateBoard(s *Scene, i int) (*ojson.Object, error) {
	boards := s.Boards()
	src := boards[i]
	dst := src.Clone()
	MigrateBoards([]*ojson.Object{dst}, i+1)
	dst.Set("audio", nil)
	dst.Set("newShot", false)
	dst.Set("dialogue", "")
	dst.Set("action", "")
	dst.Set("notes", "")
	if !src.Has("duration") {
		dst.Delete("duration")
	}
	var pairs [][2]string
	for _, n := range OrderedLayers(src) {
		pairs = append(pairs, [2]string{Layer(src, n).Str("url"), Layer(dst, n).Str("url")})
	}
	if l, ok := src.Get("link").(string); ok {
		pairs = append(pairs, [2]string{l, LinkFile(dst)})
	}
	for _, p := range pairs {
		if !exists(s.ImagePath(p[0])) {
			return nil, fmt.Errorf("could not duplicate board: missing %s", p[0])
		}
	}
	for _, p := range pairs {
		if err := CopyFile(s.ImagePath(p[0]), s.ImagePath(p[1])); err != nil {
			return nil, err
		}
	}
	for _, p := range [][2]string{{ThumbnailFile(src), ThumbnailFile(dst)}, {PosterframeFile(src), PosterframeFile(dst)}} {
		if exists(s.ImagePath(p[0])) {
			if err := CopyFile(s.ImagePath(p[0]), s.ImagePath(p[1])); err != nil {
				return nil, err
			}
		}
	}
	boards = append(boards[:i+1], append([]*ojson.Object{dst}, boards[i+1:]...)...)
	s.SetBoards(boards)
	if !exists(s.ImagePath(PosterframeFile(dst))) || !exists(s.ImagePath(ThumbnailFile(dst))) {
		if _, err := RefreshArt(s, dst); err != nil {
			return nil, err
		}
	}
	return dst, nil
}

// MoveBoards moves the given boards (kept in their order) so the first lands at index to.
func MoveBoards(s *Scene, idx []int, to int) {
	sel := map[int]bool{}
	for _, i := range idx {
		sel[i] = true
	}
	var moved, rest []*ojson.Object
	for i, b := range s.Boards() {
		if sel[i] {
			moved = append(moved, b)
		} else {
			rest = append(rest, b)
		}
	}
	to = max(0, min(to, len(rest)))
	out := append(append(append([]*ojson.Object{}, rest[:to]...), moved...), rest[to:]...)
	s.SetBoards(out)
}

// FramesToMs is framesToMsecs: Math.round(frames / fps * 1000).
func FramesToMs(frames, fps float64) float64 { return math.Round(frames / fps * 1000) }

// MsToFrames is msecsToFrames: Math.round(ms / 1000 * fps).
func MsToFrames(ms, fps float64) float64 { return math.Round(ms / 1000 * fps) }

// ---- script lines ----

// ScriptLine is one clickable line of the script panel (renderScript).
type ScriptLine struct {
	Line     int    `json:"line"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	Duration int    `json:"duration,omitempty"`
	// what double-clicking the line writes to the board
	Dialogue string `json:"dialogue,omitempty"`
	Action   string `json:"action,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

var reStripHTML = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string { return reStripHTML.ReplaceAllString(s, " ") }

// ApplyScriptLine copies a script line to a board, overwriting existing values.
func ApplyScriptLine(b *ojson.Object, l ScriptLine) []string {
	var changed []string
	if l.Duration != 0 {
		b.Set("duration", l.Duration)
		changed = append(changed, "duration")
	}
	for _, kv := range [][2]string{{"dialogue", l.Dialogue}, {"action", l.Action}, {"notes", l.Notes}} {
		if kv[1] != "" {
			b.Set(kv[0], kv[1])
			changed = append(changed, kv[0])
		}
	}
	return changed
}

// ---- clipboard JSON ----

// ExportClip builds {boards, layerDataByBoardIndex} with base64 PNG layers (copyBoards).
func ExportClip(s *Scene, idx []int) (*ojson.Object, error) {
	var boards, layerData []any
	for _, i := range idx {
		b := s.Boards()[i]
		boards = append(boards, b.Clone())
		data := ojson.New()
		if l := Layers(b, false); l != nil {
			for _, n := range l.Keys() {
				lo := l.Obj(n)
				if lo == nil {
					continue
				}
				raw, err := os.ReadFile(s.ImagePath(lo.Str("url")))
				if err != nil {
					continue // the original logs and skips unreadable layers
				}
				if filepath.Ext(lo.Str("url")) != ".png" {
					img, err := render.Load(s.ImagePath(lo.Str("url")))
					if err != nil {
						continue
					}
					raw = render.EncodePNG(img)
				}
				data.Set(n, render.DataURL(raw))
			}
		}
		layerData = append(layerData, data)
	}
	return ojson.Obj("boards", boards, "layerDataByBoardIndex", layerData), nil
}

// ReadClip loads a clip JSON file, or wraps an image file like an external paste.
func ReadClip(path string) (*ojson.Object, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		mime := "image/png"
		if ext != ".png" {
			mime = "image/jpeg"
		}
		url := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
		return ojson.Obj(
			"boards", []any{ojson.Obj("layers", ojson.Obj("reference", ojson.Obj("url", nil)))},
			"layerDataByBoardIndex", []any{ojson.Obj("reference", url)},
		), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	clip, err := ojson.ParseObject(data)
	if err != nil {
		return nil, fmt.Errorf("not a clip JSON file: %w", err)
	}
	if len(clip.Arr("boards")) == 0 || len(clip.Arr("layerDataByBoardIndex")) == 0 {
		return nil, fmt.Errorf("clip has no boards or no layer data")
	}
	return clip, nil
}

// PasteClip inserts clip boards at insertAt (pasteBoards + insertBoards).
func PasteClip(s *Scene, clip *ojson.Object, insertAt int) ([]*ojson.Object, error) {
	var olds, news []*ojson.Object
	for _, v := range clip.Arr("boards") {
		if b, ok := v.(*ojson.Object); ok {
			olds = append(olds, b)
			news = append(news, b.Clone())
		}
	}
	MigrateBoards(news, insertAt)
	for n, dst := range news {
		if l, ok := olds[n].Get("link").(string); ok && exists(s.ImagePath(l)) {
			CopyFile(s.ImagePath(l), s.ImagePath(LinkFile(dst)))
		}
	}
	data := clip.Arr("layerDataByBoardIndex")
	for i, b := range news {
		w, h := SizeOf(s, b)
		if i < len(data) {
			if ld, ok := data[i].(*ojson.Object); ok {
				for _, name := range ld.Keys() {
					if !ValidLayer(name) {
						continue
					}
					img, err := render.DecodeDataURL(ld.Str(name))
					if err != nil {
						return nil, fmt.Errorf("board %d layer %s: %w", i+1, name, err)
					}
					layers := Layers(b, true)
					if layers.Obj(name) == nil {
						layers.Set(name, ojson.Obj("url", LayerFile(b, name)))
					}
					if err := render.SavePNG(s.ImagePath(layers.Obj(name).Str("url")), render.FitImage(img, w, h)); err != nil {
						return nil, err
					}
				}
			}
		}
		boards := s.Boards()
		pos := min(insertAt+i, len(boards))
		s.SetBoards(append(boards[:pos], append([]*ojson.Object{b}, boards[pos:]...)...))
		if _, err := RefreshArt(s, b); err != nil {
			return nil, err
		}
	}
	return news, nil
}

// ReplaceFromClip replaces one board's layers with a single-board clip (pasteAndReplace).
func ReplaceFromClip(s *Scene, b *ojson.Object, clip *ojson.Object) error {
	if len(clip.Arr("boards")) > 1 {
		return fmt.Errorf("can't paste: expected one board, but found %d", len(clip.Arr("boards")))
	}
	ld, _ := clip.Arr("layerDataByBoardIndex")[0].(*ojson.Object)
	if ld == nil {
		return fmt.Errorf("clip has no layer data")
	}
	refOpacity := ReferenceOpacityForBoard(b)
	b.Set("layers", ojson.New())
	w, h := SizeOf(s, b)
	for _, name := range DrawingLayers {
		if !ld.Has(name) {
			continue
		}
		img, err := render.DecodeDataURL(ld.Str(name))
		if err != nil {
			return fmt.Errorf("layer %s: %w", name, err)
		}
		l := ojson.Obj("url", LayerFile(b, name))
		if name == "reference" {
			l.Set("opacity", refOpacity)
		}
		Layers(b, true).Set(name, l)
		if err := WriteLayerFile(s, l.Str("url"), render.FitImage(img, w, h)); err != nil {
			return err
		}
	}
	_, err := RefreshArt(s, b)
	return err
}

// ---- images ----

// CollectImages expands folders recursively, keeping png/jpg/jpeg files.
func CollectImages(paths []string) ([]string, error) {
	var out []string
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	for _, p := range sorted {
		st, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			out = append(out, p)
			continue
		}
		filepath.Walk(p, func(f string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				switch strings.ToLower(filepath.Ext(f)) {
				case ".png", ".jpg", ".jpeg":
					out = append(out, f)
				}
			}
			return nil
		})
	}
	return out, nil
}

// ImportImages makes one new board per image, image fit into the reference layer
// at opacity 1 (insertNewBoardsWithFiles). Returns the new boards and failures.
func ImportImages(s *Scene, files []string, insertAt int) ([]*ojson.Object, []string, error) {
	w, h := s.ImageSize()
	var added []*ojson.Object
	var failed []string
	os.MkdirAll(s.ImagesDir(), 0o755)
	for _, f := range files {
		img, err := render.Load(f)
		if err != nil {
			failed = append(failed, f+": "+err.Error())
			continue
		}
		boards := s.Boards()
		pos := min(insertAt, len(boards))
		b := NewBoard(pos)
		s.SetBoards(append(boards[:pos], append([]*ojson.Object{b}, boards[pos:]...)...))
		ref := ojson.Obj("url", LayerFile(b, "reference"), "opacity", 1.0)
		Layers(b, true).Set("reference", ref)
		if err := render.SavePNG(s.ImagePath(ref.Str("url")), render.FitImage(img, w, h)); err != nil {
			return added, failed, err
		}
		if _, err := RefreshArt(s, b); err != nil {
			return added, failed, err
		}
		added = append(added, b)
		insertAt++
	}
	return added, failed, nil
}

// ReplaceLayerImage fits an image into a layer; the reference layer gets opacity 1
// (replaceReferenceLayerImage).
func ReplaceLayerImage(s *Scene, b *ojson.Object, layer, file string) error {
	img, err := render.Load(file)
	if err != nil {
		return err
	}
	w, h := SizeOf(s, b)
	if err := SaveLayer(s, b, layer, render.FitImage(img, w, h)); err != nil {
		return err
	}
	if layer == "reference" {
		Layer(b, "reference").Set("opacity", 1.0)
	}
	_, err = RefreshArt(s, b)
	return err
}

// EditLayers loads the given existing layers, applies fn to each and saves them.
func EditLayers(s *Scene, b *ojson.Object, names []string, fn func(name string, img *image.RGBA) *image.RGBA) error {
	for _, n := range names {
		if Layer(b, n) == nil {
			continue
		}
		out := fn(n, LoadLayer(s, b, n))
		if err := SaveLayer(s, b, n, out); err != nil {
			return err
		}
	}
	_, err := RefreshArt(s, b)
	return err
}

// IsBoardEmpty reports whether all existing layers are fully transparent.
func IsBoardEmpty(s *Scene, b *ojson.Object) bool {
	for _, n := range OrderedLayers(b) {
		if !render.IsBlank(LoadLayer(s, b, n)) {
			return false
		}
	}
	return true
}

// ClearLayers writes blank layers (files and entries stay, like the sketch pane clear).
func ClearLayers(s *Scene, b *ojson.Object, names []string) error {
	return EditLayers(s, b, names, func(_ string, img *image.RGBA) *image.RGBA {
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		return render.New(w, h)
	})
}

// MergeLayers composites reference and fill (reference at its opacity) into dest
// and clears the other one (layers-editor mergeDown / mergeUp).
func MergeLayers(s *Scene, b *ojson.Object, dest string) error {
	if dest != "reference" && dest != "fill" {
		return fmt.Errorf("merge destination must be reference or fill")
	}
	w, h := SizeOf(s, b)
	comp := render.New(w, h)
	render.DrawOver(comp, image.Point{}, LoadLayer(s, b, "reference"), ReferenceOpacityForBoard(b))
	render.DrawOver(comp, image.Point{}, LoadLayer(s, b, "fill"), 1)
	other := "fill"
	if dest == "fill" {
		other = "reference"
	}
	if err := SaveLayer(s, b, dest, comp); err != nil {
		return err
	}
	if err := SaveLayer(s, b, other, render.New(w, h)); err != nil {
		return err
	}
	_, err := RefreshArt(s, b)
	return err
}

// SetReferenceOpacity sets reference (and shot-generator) layer opacity, as the slider does.
func SetReferenceOpacity(s *Scene, b *ojson.Object, v float64) error {
	for _, n := range []string{"reference", "shot-generator"} {
		if l := Layer(b, n); l != nil {
			l.Set("opacity", v)
		}
	}
	_, err := RefreshArt(s, b)
	return err
}

// SetLayerOpacity sets any drawing layer's opacity (reference also sets the
// 3D layer, as the slider does). A layer without a file gets a blank one, so
// the original app never meets a layer entry without an image.
func SetLayerOpacity(s *Scene, b *ojson.Object, layer string, v float64) error {
	if layer == "reference" && !IsPage(b) && Layer(b, "reference") != nil {
		return SetReferenceOpacity(s, b, v)
	}
	if Layer(b, layer) == nil {
		w, h := SizeOf(s, b)
		if err := os.MkdirAll(s.ImagesDir(), 0o755); err != nil {
			return err
		}
		file := LayerFile(b, layer)
		if err := render.SavePNG(s.ImagePath(file), render.New(w, h)); err != nil {
			return err
		}
		Layers(b, true).Set(layer, ojson.Obj("url", file))
	}
	Layer(b, layer).Set("opacity", v)
	if layer == "reference" && Layer(b, "shot-generator") != nil {
		Layer(b, "shot-generator").Set("opacity", v)
	}
	_, err := RefreshArt(s, b)
	return err
}

// ParsePolygon reads "x1,y1 x2,y2 ..." or a JSON [[x,y],...] list.
func ParsePolygon(s string) ([][2]float64, error) {
	var pts [][2]float64
	if strings.HasPrefix(strings.TrimSpace(s), "[") {
		if err := json.Unmarshal([]byte(s), &pts); err != nil {
			return nil, err
		}
	} else {
		for _, pair := range strings.Fields(strings.ReplaceAll(s, ";", " ")) {
			xy := strings.Split(pair, ",")
			if len(xy) != 2 {
				return nil, fmt.Errorf("bad point %q (use x,y)", pair)
			}
			x, err1 := strconv.ParseFloat(xy[0], 64)
			y, err2 := strconv.ParseFloat(xy[1], 64)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("bad point %q", pair)
			}
			pts = append(pts, [2]float64{x, y})
		}
	}
	if len(pts) < 3 {
		return nil, fmt.Errorf("a polygon needs at least 3 points")
	}
	return pts, nil
}

// EraseRegion clears a polygon on all layers (lasso erase).
func EraseRegion(s *Scene, b *ojson.Object, pts [][2]float64) error {
	w, h := SizeOf(s, b)
	mask := render.PolygonMask(w, h, pts)
	return EditLayers(s, b, DrawingLayers, func(_ string, img *image.RGBA) *image.RGBA {
		render.Erase(img, mask)
		return img
	})
}

// FillRegion paints a polygon on the fill layer (lasso fill), creating the layer if needed.
func FillRegion(s *Scene, b *ojson.Object, pts [][2]float64, c color.Color, opacity float64) error {
	w, h := SizeOf(s, b)
	mask := render.PolygonMask(w, h, pts)
	img := LoadLayer(s, b, "fill")
	render.FillMask(img, mask, c, opacity)
	if err := SaveLayer(s, b, "fill", img); err != nil {
		return err
	}
	_, err := RefreshArt(s, b)
	return err
}

// MoveRegion cuts a polygon from all layers and pastes it offset by dx, dy.
func MoveRegion(s *Scene, b *ojson.Object, pts [][2]float64, dx, dy float64) error {
	w, h := SizeOf(s, b)
	mask := render.PolygonMask(w, h, pts)
	return EditLayers(s, b, DrawingLayers, func(_ string, img *image.RGBA) *image.RGBA {
		piece := render.Cut(img, mask)
		render.DrawOver(img, image.Pt(int(math.Round(dx)), int(math.Round(dy))), piece, 1)
		return img
	})
}

// ---- audio ----

// AudioDurationMs measures an audio file with ffprobe, falling back to the WAV header.
func AudioDurationMs(path string) (float64, error) {
	if probe, err := exec.LookPath("ffprobe"); err == nil {
		out, err := exec.Command(probe, "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", path).Output()
		if err == nil {
			if f, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
				return f * 1000, nil
			}
		}
	}
	return wavDurationMs(path)
}

func wavDurationMs(path string) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return 0, fmt.Errorf("could not measure %s: install ffprobe (ffmpeg) for non-WAV audio", filepath.Base(path))
	}
	var byteRate uint32
	for i := 12; i+8 <= len(data); {
		id := string(data[i : i+4])
		size := uint32(data[i+4]) | uint32(data[i+5])<<8 | uint32(data[i+6])<<16 | uint32(data[i+7])<<24
		body := i + 8
		if id == "fmt " && body+12 <= len(data) {
			byteRate = uint32(data[body+8]) | uint32(data[body+9])<<8 | uint32(data[body+10])<<16 | uint32(data[body+11])<<24
		}
		if id == "data" && byteRate > 0 {
			return float64(size) / float64(byteRate) * 1000, nil
		}
		i = body + int(size) + int(size%2)
	}
	return 0, fmt.Errorf("could not read WAV header of %s", filepath.Base(path))
}

// SetAudio copies an audio file as `<uid>-<name>` into images/ and sets board.audio.
func SetAudio(s *Scene, b *ojson.Object, file string, overwrite bool) (string, error) {
	name := UID(b) + "-" + filepath.Base(file)
	dst := s.ImagePath(name)
	if exists(dst) && !overwrite {
		return "", fmt.Errorf("a file named %s already exists in this project (use --force to overwrite)", name)
	}
	abs, _ := filepath.Abs(file)
	if abs != dst {
		if err := CopyFile(file, dst); err != nil {
			return "", err
		}
	}
	a := b.Obj("audio")
	if a == nil {
		a = ojson.New()
		b.Set("audio", a)
	}
	a.Set("filename", name)
	if d, err := AudioDurationMs(dst); err == nil {
		a.Set("duration", d)
	}
	return name, nil
}

// RefreshAudioDurations recomputes audio.duration for all boards (updateAudioDurations).
func RefreshAudioDurations(s *Scene) (map[string]float64, []string) {
	out := map[string]float64{}
	var failed []string
	for _, b := range s.Boards() {
		a := b.Obj("audio")
		if a == nil {
			continue
		}
		d, err := AudioDurationMs(s.ImagePath(a.Str("filename")))
		if err != nil {
			failed = append(failed, a.Str("filename"))
			continue
		}
		a.Set("duration", d)
		out[UID(b)] = d
	}
	return out, failed
}
