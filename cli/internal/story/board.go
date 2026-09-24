package story

import (
	"fmt"
	"strconv"
	"strings"

	"sb/internal/ojson"
	"sb/internal/script"
)

// replaceFirst mirrors JS String#replace with a string pattern.
func replaceFirst(s, old, repl string) string { return strings.Replace(s, old, repl, 1) }

func URL(b *ojson.Object) string                 { return b.Str("url") }
func UID(b *ojson.Object) string                 { return b.Str("uid") }
func LayerFile(b *ojson.Object, n string) string { return replaceFirst(URL(b), ".png", "-"+n+".png") }
func ThumbnailFile(b *ojson.Object) string       { return replaceFirst(URL(b), ".png", "-thumbnail.png") }
func PosterframeFile(b *ojson.Object) string     { return replaceFirst(URL(b), ".png", "-posterframe.jpg") }
func LinkFile(b *ojson.Object) string            { return replaceFirst(URL(b), ".png", ".psd") }
func CameraPlotFile(b *ojson.Object) string      { return replaceFirst(URL(b), ".png", "-camera-plot.png") }
func LayerThumbnailFile(b *ojson.Object, n string) string {
	return replaceFirst(URL(b), ".png", "-"+n+"-thumbnail.jpg")
}

// ExportFile is boardFilenameForExport: <base>-board-00001.png
func ExportFile(index int, base string) string {
	return fmt.Sprintf("%s-board-%05d.png", base, index+1)
}

// Layers returns the board's layers object, creating it when asked.
func Layers(b *ojson.Object, create bool) *ojson.Object {
	l := b.Obj("layers")
	if l == nil && create {
		l = ojson.New()
		b.Set("layers", l)
	}
	return l
}

// Layer returns one layer object or nil.
func Layer(b *ojson.Object, name string) *ojson.Object {
	if l := Layers(b, false); l != nil {
		return l.Obj(name)
	}
	return nil
}

// OrderedLayers returns layer names present on the board in drawing order.
func OrderedLayers(b *ojson.Object) []string {
	out := []string{}
	for _, n := range LayerOrder {
		if Layer(b, n) != nil {
			out = append(out, n)
		}
	}
	return out
}

// LayerOpacity is the layer's stored opacity, else 0.75 for reference and 1
// for the rest. (The original app only reads opacity on reference.)
func LayerOpacity(b *ojson.Object, name string) float64 {
	if l := Layer(b, name); l != nil {
		if f, ok := l.Num("opacity"); ok {
			return f
		}
	}
	if name == "reference" {
		return DefaultReferenceOpacity
	}
	return 1
}

// Duration is boardDuration: the board's duration or the scene default.
func Duration(s *Scene, b *ojson.Object) float64 {
	if v := b.Get("duration"); v != nil {
		f, _ := ojson.Num(v)
		return f
	}
	return s.DefaultBoardTiming()
}

// AudioDuration is board.audio.duration or 0.
func AudioDuration(b *ojson.Object) float64 {
	if a := b.Obj("audio"); a != nil {
		f, _ := a.Num("duration")
		return f
	}
	return 0
}

func DurationWithAudio(s *Scene, b *ojson.Object) float64 {
	return max(AudioDuration(b), Duration(s, b))
}

// SceneDuration is models/scene sceneDuration: the latest board end time incl. audio.
func SceneDuration(s *Scene) float64 {
	best := 0.0
	for _, b := range s.Boards() {
		t, _ := b.Num("time")
		best = max(best, t+DurationWithAudio(s, b))
	}
	return best
}

// UpdateTiming is main-window updateSceneTiming: shot labels, numbers, start times.
func UpdateTiming(s *Scene) {
	boards := s.Boards()
	hasShots := false
	for _, b := range boards {
		if b.Bool("newShot") {
			hasShots = true
			break
		}
	}
	shot, sub, number := 0, 0, 1
	t := 0.0
	for _, b := range boards {
		if hasShots {
			if b.Bool("newShot") || shot == 0 {
				shot++
				sub = 0
			} else {
				sub++
			}
			b.Set("shot", strconv.Itoa(shot)+ShotSuffix(sub))
		} else {
			b.Set("shot", strconv.Itoa(number)+"A")
		}
		b.Set("number", number)
		number++
		b.Set("time", t)
		t += Duration(s, b)
	}
}

// ShotSuffix: A..Z, then letters wrap with a numeric suffix (1A, 1B, ... 1Z, 1A2 ...).
func ShotSuffix(sub int) string {
	s := string(rune('A' + sub%26))
	if n := (sub+24)/25 - 1; n > 0 { // Math.ceil(sub/25) - 1
		s += strconv.Itoa((sub + 24) / 25)
	}
	return s
}

// NewBoard is insertNewBoardDataAtPosition's board object.
func NewBoard(position int) *ojson.Object {
	uid := script.UID()
	return ojson.Obj(
		"uid", uid,
		"url", fmt.Sprintf("board-%d-%s.png", position+1, uid),
		"newShot", false,
		"lastEdited", NowMs(),
		"layers", ojson.New(),
	)
}

// UpdateURLsFromIndex renames url and layer urls for a board at index (board model).
func UpdateURLsFromIndex(b *ojson.Object, index int) {
	b.Set("url", fmt.Sprintf("board-%d-%s.png", index+1, UID(b)))
	if l := Layers(b, false); l != nil {
		for _, name := range l.Keys() {
			if lo := l.Obj(name); lo != nil {
				lo.Set("url", LayerFile(b, name))
			}
		}
	}
}

// MigrateBoards gives copied boards new uids and index-based filenames (main-window migrateBoards).
func MigrateBoards(boards []*ojson.Object, insertAt int) {
	for i, b := range boards {
		b.Set("uid", script.UID())
		if Layers(b, false) == nil {
			b.Set("layers", ojson.New())
		}
		if !b.Bool("newShot") {
			b.Set("newShot", false)
		}
		b.Set("lastEdited", NowMs())
		UpdateURLsFromIndex(b, insertAt+i)
		if b.Get("link") != nil {
			b.Set("link", LinkFile(b))
		}
	}
}

// MediaFiles lists every file in images/ a board uses (getMediaFilenames).
func MediaFiles(b *ojson.Object) []string {
	var out []string
	add := func(s string) {
		if s != "" {
			out = append(out, s)
		}
	}
	l := Layers(b, false)
	if l != nil {
		for _, n := range l.Keys() {
			if lo := l.Obj(n); lo != nil {
				add(lo.Str("url"))
			}
		}
	}
	add(ThumbnailFile(b))
	add(PosterframeFile(b))
	if v, ok := b.Get("link").(string); ok {
		add(v)
	}
	if a := b.Obj("audio"); a != nil {
		add(a.Str("filename"))
	}
	if l != nil {
		for _, n := range l.Keys() {
			if lo := l.Obj(n); lo != nil {
				if t, ok := lo.Get("thumbnail").(string); ok {
					add(t)
				}
			}
		}
	}
	if b.Get("sg") != nil {
		add(CameraPlotFile(b))
	}
	return out
}

// SuggestedDuration is the dialogue-based hint: words x 300 ms + 300.
func SuggestedDuration(b *ojson.Object) int {
	return script.DurationOfWords(b.Str("dialogue"), 300) + 300
}
