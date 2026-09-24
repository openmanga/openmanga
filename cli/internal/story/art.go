package story

import (
	"fmt"
	"image"
	"os"

	"sb/internal/ojson"
	"sb/internal/render"
)

// Flatten composites the board's layers in order onto white at w x h,
// drawing each layer scaled to the full size (exporters/common flattenBoardToCanvas).
// Missing layer files are skipped and reported.
func Flatten(s *Scene, b *ojson.Object, w, h int) (*image.RGBA, []string) {
	dst := render.White(w, h)
	var missing []string
	for _, name := range OrderedLayers(b) {
		url := Layer(b, name).Str("url")
		img, err := render.Load(s.ImagePath(url))
		if err != nil {
			missing = append(missing, url)
			continue
		}
		render.DrawScaled(dst, dst.Bounds(), img, LayerOpacity(b, name))
	}
	return dst, missing
}

// SavePosterframe writes the flattened full-size -posterframe.jpg.
func SavePosterframe(s *Scene, b *ojson.Object) ([]string, error) {
	w, h := SizeOf(s, b)
	img, missing := Flatten(s, b, w, h)
	return missing, render.SaveJPEG(s.ImagePath(PosterframeFile(b)), img, render.JPEGQuality)
}

// SaveThumbnail writes the 120 px high -thumbnail.png used in the strip.
func SaveThumbnail(s *Scene, b *ojson.Object) ([]string, error) {
	w, h := ThumbSizeOf(s, b)
	img, missing := Flatten(s, b, w, h)
	return missing, render.SavePNG(s.ImagePath(ThumbnailFile(b)), img)
}

// BoardArtHook runs after a board's art changed (manga pages that place it re-render).
var BoardArtHook func(s *Scene, uid string) error

// PageArtHook re-renders a manga page's derived files; set by the manga package.
var PageArtHook func(s *Scene, page *ojson.Object) error

// RefreshArt regenerates posterframe and thumbnail from the layer files
// (for a manga page: its whole page composite).
func RefreshArt(s *Scene, b *ojson.Object) ([]string, error) {
	if IsPage(b) {
		if PageArtHook == nil {
			return nil, nil
		}
		return nil, PageArtHook(s, b)
	}
	missing, err := SavePosterframe(s, b)
	if err != nil {
		return missing, err
	}
	if _, err = SaveThumbnail(s, b); err != nil {
		return missing, err
	}
	if BoardArtHook != nil {
		err = BoardArtHook(s, UID(b))
	}
	return missing, err
}

// InsertBlankBoard adds a new board at position with blank posterframe and thumbnail (newBoard).
func InsertBlankBoard(s *Scene, position int) (*ojson.Object, error) {
	return InsertSizedBoard(s, position, 0, 0)
}

// InsertSizedBoard is InsertBlankBoard with an own canvas size (0 = project default).
func InsertSizedBoard(s *Scene, position, width, height int) (*ojson.Object, error) {
	if err := os.MkdirAll(s.ImagesDir(), 0o755); err != nil {
		return nil, err
	}
	boards := s.Boards()
	position = max(0, min(position, len(boards)))
	b := NewBoard(position)
	boards = append(boards[:position], append([]*ojson.Object{b}, boards[position:]...)...)
	s.SetBoards(boards)
	if width > 0 && height > 0 {
		b.Set("size", ojson.Obj("width", width, "height", height))
	}
	w, h := SizeOf(s, b)
	if err := render.SaveJPEG(s.ImagePath(PosterframeFile(b)), render.White(w, h), render.JPEGQuality); err != nil {
		return nil, err
	}
	tw, th := ThumbSizeOf(s, b)
	if err := render.SavePNG(s.ImagePath(ThumbnailFile(b)), render.White(tw, th)); err != nil {
		return nil, err
	}
	return b, nil
}

// LoadLayer returns a layer as a full-size canvas (transparent when absent).
func LoadLayer(s *Scene, b *ojson.Object, name string) *image.RGBA {
	w, h := SizeOf(s, b)
	if l := Layer(b, name); l != nil {
		if img, err := render.Load(s.ImagePath(l.Str("url"))); err == nil {
			return render.Resize(img, w, h)
		}
	}
	return render.New(w, h)
}

// ReferenceOpacityForBoard is layersEditor.loadReferenceOpacity.
func ReferenceOpacityForBoard(b *ojson.Object) float64 {
	if l := Layer(b, "reference"); l != nil {
		if f, ok := l.Num("opacity"); ok {
			return f
		}
	}
	if l := Layer(b, "shot-generator"); l != nil {
		if f, ok := l.Num("opacity"); ok {
			return f
		}
	}
	return DefaultReferenceOpacity
}

// SaveLayer writes a layer PNG and registers it on the board like saveImageFile.
func SaveLayer(s *Scene, b *ojson.Object, name string, img image.Image) error {
	if !ValidLayer(name) {
		return fmt.Errorf("unknown layer %q (layers: %v)", name, DrawingLayers)
	}
	layers := Layers(b, true)
	file := LayerFile(b, name)
	if layers.Obj(name) == nil {
		l := ojson.Obj("url", file)
		if name == "reference" {
			l.Set("opacity", ReferenceOpacityForBoard(b))
		}
		layers.Set(name, l)
	}
	return WriteLayerFile(s, layers.Obj(name).Str("url"), img)
}

// ValidLayer reports whether name is one of the seven drawing layers.
func ValidLayer(name string) bool {
	for _, n := range DrawingLayers {
		if n == name {
			return true
		}
	}
	return false
}

// WriteLayerFile saves a layer PNG, keeping the previous version in images/.history.
func WriteLayerFile(s *Scene, file string, img image.Image) error {
	if err := os.MkdirAll(s.ImagesDir(), 0o755); err != nil {
		return err
	}
	if err := snapshot(s, file); err != nil {
		return err
	}
	return render.SavePNG(s.ImagePath(file), img)
}
