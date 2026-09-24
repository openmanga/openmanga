package sg

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"image"
	"math"
	"os"
	"path/filepath"

	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/story"
)

// HistoryLimit is the 3D undo depth (redux-undo limit 50).
const HistoryLimit = 50

// BoardData returns the board's 3D scene, or the default scene when it has none.
func BoardData(b *ojson.Object) (*ojson.Object, bool) {
	if d := b.ObjPath(false, "sg", "data"); d != nil {
		return d, true
	}
	return Initial(), false
}

// PutBoardData stores scene data on the board (board.sg = {version, data}).
func PutBoardData(b *ojson.Object, data *ojson.Object) {
	sg := b.Obj("sg")
	if sg == nil {
		sg = ojson.New()
		b.Set("sg", sg)
	}
	sg.Set("version", story.AppVersion)
	sg.Set("data", Serialize(data))
}

// History is the per-board undo/redo stack plus hand-off state, kept in
// userData/sg-history (not in the project folder).
type History struct {
	Past      []json.RawMessage `json:"past"`
	Future    []json.RawMessage `json:"future"`
	SavedHash string            `json:"savedHash,omitempty"`
	Explore   []Shot            `json:"explore,omitempty"`
	path      string
}

// LoadHistory reads the history of one board of one scene file.
func LoadHistory(sceneFile, uid string) *History {
	sum := sha1.Sum([]byte(sceneFile))
	p := filepath.Join(story.UserDataDir(), "sg-history", hex.EncodeToString(sum[:8])+"-"+uid+".json")
	h := &History{path: p}
	if data, err := os.ReadFile(p); err == nil {
		json.Unmarshal(data, h)
	}
	h.path = p
	return h
}

func (h *History) Save() error {
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(h.path, data, 0o644)
}

// Push records the state before an edit and clears redo.
func (h *History) Push(before *ojson.Object) {
	h.Past = append(h.Past, json.RawMessage(ojson.Stringify(before, "")))
	if len(h.Past) > HistoryLimit {
		h.Past = h.Past[len(h.Past)-HistoryLimit:]
	}
	h.Future = nil
}

func pop(list *[]json.RawMessage) *ojson.Object {
	if len(*list) == 0 {
		return nil
	}
	raw := (*list)[len(*list)-1]
	*list = (*list)[:len(*list)-1]
	o, _ := ojson.ParseObject(raw)
	return o
}

// Undo returns the previous state (nil when there is none) and remembers current for redo.
func (h *History) Undo(current *ojson.Object) *ojson.Object {
	prev := pop(&h.Past)
	if prev != nil {
		h.Future = append(h.Future, json.RawMessage(ojson.Stringify(current, "")))
	}
	return prev
}

// Redo is the inverse of Undo.
func (h *History) Redo(current *ojson.Object) *ojson.Object {
	next := pop(&h.Future)
	if next != nil {
		h.Past = append(h.Past, json.RawMessage(ojson.Stringify(current, "")))
	}
	return next
}

// SaveShot is saveToBoardFromShotGenerator: the UI renders the camera view and
// the 900x900 plot; this writes the shot-generator layer, its thumbnail, the
// camera plot, the board thumbnail and posterframe, and stores board.sg.
func SaveShot(s *story.Scene, b *ojson.Object, data *ojson.Object, cameraPNG, plotPNG string) error {
	camImg, err := render.Load(cameraPNG)
	if err != nil {
		return err
	}
	var plot image.Image
	if plotPNG != "" {
		if plot, err = render.Load(plotPNG); err != nil {
			return err
		}
	}
	layers := story.Layers(b, true)
	l := layers.Obj("shot-generator")
	if l == nil {
		l = ojson.New()
		layers.Set("shot-generator", l)
	}
	l.Set("url", story.LayerFile(b, "shot-generator"))
	l.Set("opacity", 1.0)
	l.Set("thumbnail", story.LayerThumbnailFile(b, "shot-generator"))
	PutBoardData(b, data)

	os.MkdirAll(s.ImagesDir(), 0o755)
	// fit to the board size, drawn at 0,0 with the original's 3 px padding
	draw := func(w, h int) *image.RGBA {
		dst := render.New(w, h)
		cb := camImg.Bounds()
		_, _, fw, fh := render.Fit(float64(w), float64(h), float64(cb.Dx()), float64(cb.Dy()))
		render.DrawScaled(dst, image.Rect(0, 0, int(math.Ceil(fw))+3, int(math.Ceil(fh))+3), camImg, 1)
		return dst
	}
	w, h := story.SizeOf(s, b)
	if err := render.SavePNG(s.ImagePath(l.Str("url")), draw(w, h)); err != nil {
		return err
	}
	if plot != nil {
		canvas := render.New(900, 900)
		render.DrawOver(canvas, image.Point{}, plot, 1)
		if err := render.SavePNG(s.ImagePath(story.CameraPlotFile(b)), canvas); err != nil {
			return err
		}
	}
	tw, th := s.LayerThumbnailSize()
	if err := render.SaveJPEG(s.ImagePath(l.Str("thumbnail")), draw(tw, th), render.JPEGQuality); err != nil {
		return err
	}
	_, err = story.RefreshArt(s, b)
	return err
}
