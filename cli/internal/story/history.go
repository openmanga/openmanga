package story

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"sb/internal/ojson"
)

// HistoryDepth is how many previous versions each layer file keeps.
const HistoryDepth = 20

// absentMark marks a snapshot of a layer that did not exist yet.
const absentMark = ".absent"

// historyDir is images/.history/<layer file without .png>/.
func historyDir(s *Scene, file string) string {
	return filepath.Join(s.ImagesDir(), ".history", strings.TrimSuffix(file, filepath.Ext(file)))
}

// keep stores the current bytes of a layer file (or an "absent" marker) in dir.
func keep(s *Scene, file, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// strictly increasing names even within one millisecond
	name := strconv.FormatInt(time.Now().UnixNano(), 10)
	if data, err := os.ReadFile(s.ImagePath(file)); err == nil {
		return os.WriteFile(filepath.Join(dir, name+".png"), data, 0o644)
	} else if os.IsNotExist(err) {
		return os.WriteFile(filepath.Join(dir, name+absentMark), nil, 0o644)
	} else {
		return err
	}
}

func trim(dir string) {
	entries := historyEntries(dir)
	for len(entries) > HistoryDepth {
		os.Remove(filepath.Join(dir, entries[0]))
		entries = entries[1:]
	}
}

// snapshot keeps the current version before a new write, trims the history to
// HistoryDepth and drops the redo list (a new edit ends the redo chain).
func snapshot(s *Scene, file string) error {
	dir := historyDir(s, file)
	if err := keep(s, file, dir); err != nil {
		return err
	}
	trim(dir)
	return os.RemoveAll(filepath.Join(dir, "redo"))
}

func historyEntries(dir string) []string {
	des, _ := os.ReadDir(dir)
	var out []string
	for _, d := range des {
		if !d.IsDir() {
			out = append(out, d.Name())
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := strconv.ParseInt(strings.SplitN(out[i], ".", 2)[0], 10, 64)
		b, _ := strconv.ParseInt(strings.SplitN(out[j], ".", 2)[0], 10, 64)
		return a < b
	})
	return out
}

// HistoryEntry describes one kept version of a layer (newest last).
type HistoryEntry struct {
	Index  int    `json:"index"` // 1 = the version undo restores
	Time   string `json:"time"`
	Path   string `json:"path,omitempty"`
	Bytes  int64  `json:"bytes"`
	Absent bool   `json:"absent"` // the layer did not exist yet
}

// LayerHistory lists the kept versions of a board layer, most recent first.
func LayerHistory(s *Scene, b *ojson.Object, layer string) []HistoryEntry {
	dir := historyDir(s, LayerFile(b, layer))
	entries := historyEntries(dir)
	out := []HistoryEntry{}
	for i := len(entries) - 1; i >= 0; i-- {
		name := entries[i]
		ns, _ := strconv.ParseInt(strings.SplitN(name, ".", 2)[0], 10, 64)
		e := HistoryEntry{Index: len(out) + 1, Time: time.Unix(0, ns).Format(time.RFC3339), Absent: strings.HasSuffix(name, absentMark)}
		if !e.Absent {
			e.Path = filepath.Join(dir, name)
			if st, err := os.Stat(e.Path); err == nil {
				e.Bytes = st.Size()
			}
		}
		out = append(out, e)
	}
	return out
}

func layerURL(b *ojson.Object, layer string) string {
	if l := Layer(b, layer); l != nil {
		return l.Str("url")
	}
	return LayerFile(b, layer)
}

// step moves the newest entry of `from` into the layer file, keeping the
// current version in `to` (undo: history -> redo, redo: redo -> history).
func step(s *Scene, b *ojson.Object, layer, from, to, what string) (HistoryEntry, error) {
	file := layerURL(b, layer)
	entries := historyEntries(from)
	if len(entries) == 0 {
		return HistoryEntry{}, fmt.Errorf("nothing to %s for layer %s", what, layer)
	}
	last := entries[len(entries)-1]
	path := filepath.Join(from, last)
	if err := keep(s, file, to); err != nil {
		return HistoryEntry{}, err
	}
	trim(to)
	e := HistoryEntry{Index: 1, Absent: strings.HasSuffix(last, absentMark)}
	if e.Absent {
		os.Remove(s.ImagePath(file))
		if ls := Layers(b, false); ls != nil {
			ls.Delete(layer)
		}
	} else {
		data, err := os.ReadFile(path)
		if err != nil {
			return e, err
		}
		if err := os.WriteFile(s.ImagePath(file), data, 0o644); err != nil {
			return e, err
		}
		if Layer(b, layer) == nil {
			Layers(b, true).Set(layer, ojson.Obj("url", file))
		}
	}
	os.Remove(path)
	_, err := RefreshArt(s, b)
	return e, err
}

// UndoLayer restores the newest kept version of a layer; the current version
// goes to the redo list.
func UndoLayer(s *Scene, b *ojson.Object, layer string) (HistoryEntry, error) {
	dir := historyDir(s, layerURL(b, layer))
	return step(s, b, layer, dir, filepath.Join(dir, "redo"), "undo")
}

// RedoLayer re-applies the newest undone version (until a new edit is made).
func RedoLayer(s *Scene, b *ojson.Object, layer string) (HistoryEntry, error) {
	dir := historyDir(s, layerURL(b, layer))
	return step(s, b, layer, filepath.Join(dir, "redo"), dir, "redo")
}

// RedoCount is how many undone versions can be redone.
func RedoCount(s *Scene, b *ojson.Object, layer string) int {
	return len(historyEntries(filepath.Join(historyDir(s, layerURL(b, layer)), "redo")))
}
