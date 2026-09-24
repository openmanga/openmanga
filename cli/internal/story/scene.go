// Package story reads and writes Storyboarder scenes (.storyboarder), script
// projects and the user-data files, following the original app's rules.
package story

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"sb/internal/ojson"
)

// AppVersion is written to `version` on save, as the original writes pkg.version.
const AppVersion = "3.0.0"

// DefaultReferenceOpacity is used when a reference layer has no opacity (exporters/common.js).
const DefaultReferenceOpacity = 0.75

// DrawingLayers are the seven layers of the original app, back to front
// (boardOrderedLayerFilenames).
var DrawingLayers = []string{"shot-generator", "reference", "fill", "tone", "pencil", "ink", "notes"}

// LayerOrder is the composite order: manga panel borders ("frames") and
// balloons sit between ink and notes. Those two are rendered from page data.
var LayerOrder = []string{"shot-generator", "reference", "fill", "tone", "pencil", "ink", "frames", "balloons", "notes"}

// DerivedLayers are rendered from data and never drawn on directly.
var DerivedLayers = []string{"frames", "balloons"}

// Scene is one .storyboarder file.
type Scene struct {
	Path string
	Data *ojson.Object
}

// LoadScene reads a .storyboarder file.
func LoadScene(path string) (*Scene, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	o, err := ojson.ParseObject(data)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s. The file may be inaccessible or corrupt: %w", filepath.Base(abs), err)
	}
	if o.Arr("boards") == nil {
		o.Set("boards", []any{})
	}
	return &Scene{Path: abs, Data: o}, nil
}

func (s *Scene) Dir() string       { return filepath.Dir(s.Path) }
func (s *Scene) ImagesDir() string { return filepath.Join(s.Dir(), "images") }
func (s *Scene) Name() string {
	return trimExt(filepath.Base(s.Path))
}

func trimExt(name string) string { return name[:len(name)-len(filepath.Ext(name))] }

// ImagePath is the absolute path of a file in images/.
func (s *Scene) ImagePath(name string) string { return filepath.Join(s.ImagesDir(), name) }

// Boards returns the board objects (shared, mutations affect the scene).
func (s *Scene) Boards() []*ojson.Object {
	arr := s.Data.Arr("boards")
	out := make([]*ojson.Object, 0, len(arr))
	for _, v := range arr {
		if b, ok := v.(*ojson.Object); ok {
			out = append(out, b)
		}
	}
	return out
}

// Pages returns the manga pages (shared objects).
func (s *Scene) Pages() []*ojson.Object {
	var out []*ojson.Object
	for _, v := range s.Data.Arr("pages") {
		if p, ok := v.(*ojson.Object); ok {
			out = append(out, p)
		}
	}
	return out
}

func (s *Scene) SetPages(pages []*ojson.Object) {
	arr := make([]any, len(pages))
	for i, p := range pages {
		arr[i] = p
	}
	s.Data.Set("pages", arr)
}

func (s *Scene) SetBoards(boards []*ojson.Object) {
	arr := make([]any, len(boards))
	for i, b := range boards {
		arr[i] = b
	}
	s.Data.Set("boards", arr)
}

func (s *Scene) AspectRatio() float64 { return s.Data.NumOr("aspectRatio", 1.7777777777777777) }
func (s *Scene) Fps() float64         { return s.Data.NumOr("fps", 24) }

// DefaultBoardTiming accepts the string form old projects use ("2001").
func (s *Scene) DefaultBoardTiming() float64 { return s.Data.NumOr("defaultBoardTiming", 2000) }

// ImageSize is boardFileImageSize: 900 px on the short side, truncated like a canvas.
func (s *Scene) ImageSize() (int, int) { return ImageSize(s.AspectRatio()) }

func ImageSize(aspect float64) (int, int) {
	if aspect >= 1 {
		return int(900 * aspect), 900
	}
	return 900, int(900 / aspect)
}

// IsManga reports a manga-mode project (pages next to boards).
func (s *Scene) IsManga() bool { return s.Data.Str("mode") == "manga" }

// PageSize is the manga page size in pixels (default 1414x2000, 1:1.414).
func (s *Scene) PageSize() (int, int) {
	p := s.Data.Obj("page")
	if p == nil {
		return 1414, 2000
	}
	return int(p.NumOr("width", 1414)), int(p.NumOr("height", 2000))
}

// IsPage reports whether an object is a manga page (it has a panels list).
func IsPage(o *ojson.Object) bool { return o.Get("panels") != nil }

// SizeOf is the pixel size of a board or a page. A board may set its own
// canvas with "size": {"width", "height"}; otherwise 900 px high x aspect.
func SizeOf(s *Scene, o *ojson.Object) (int, int) {
	if IsPage(o) {
		return s.PageSize()
	}
	if sz := o.Obj("size"); sz != nil {
		if w, h := int(sz.NumOr("width", 0)), int(sz.NumOr("height", 0)); w > 0 && h > 0 {
			return w, h
		}
	}
	return s.ImageSize()
}

// ThumbnailSize is getThumbnailSize: [floor(60*aspect)*2, 120].
func (s *Scene) ThumbnailSize() (int, int) {
	return int(60*s.AspectRatio()) * 2, 120
}

// ThumbSizeOf is the thumbnail size for a board's own canvas (120 px high).
func ThumbSizeOf(s *Scene, b *ojson.Object) (int, int) {
	if b.Obj("size") == nil {
		return s.ThumbnailSize()
	}
	w, h := SizeOf(s, b)
	return int(60*float64(w)/float64(h)) * 2, 120
}

// LayerThumbnailSize is getLayerThumbnailSize: ceil([320, 320/aspect] * 2).
func (s *Scene) LayerThumbnailSize() (int, int) {
	a := s.AspectRatio()
	return 640, int(math.Ceil(320 / a * 2))
}

// Save updates timing, stamps the version and writes atomically via a
// `.backup-<ms>` file renamed over the original (main-window saveBoardFile).
func (s *Scene) Save() error {
	UpdateTiming(s)
	s.Data.Set("version", AppVersion)
	return WriteJSONAtomic(s.Path, s.Data, "  ")
}

// WriteJSONAtomic writes JSON.stringify(v, null, indent) through a backup file.
func WriteJSONAtomic(path string, v any, indent string) error {
	tmp := path + ".backup-" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	if err := os.WriteFile(tmp, ojson.Stringify(v, indent), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// WriteJSON writes JSON directly (for files the original writes non-atomically).
func WriteJSON(path string, v any, indent string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, ojson.Stringify(v, indent), 0o644)
}

// NowMs is Date.now().
func NowMs() int64 { return time.Now().UnixMilli() }
