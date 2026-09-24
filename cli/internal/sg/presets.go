package sg

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sb/internal/ojson"
	"sb/internal/story"
)

var (
	//go:embed data/poses.json
	posesJSON []byte
	//go:embed data/hand-poses.json
	handPosesJSON []byte
	//go:embed data/emotions.json
	emotionsJSON []byte
)

// Preset kinds and their user files in userData/presets (presetsStorage.js).
const (
	KindPoses      = "poses"
	KindHandPoses  = "hand-poses"
	KindEmotions   = "emotions"
	KindCharacters = "characters"
	KindScenes     = "scenes"
)

func builtin(kind string) *ojson.Object {
	var data []byte
	switch kind {
	case KindPoses:
		data = posesJSON
	case KindHandPoses:
		data = handPosesJSON
	case KindEmotions:
		data = emotionsJSON
	case KindCharacters:
		return ojson.Obj("7C694D0F-9D45-4B74-BA70-38479E520091", ojson.Obj("id", "7C694D0F-9D45-4B74-BA70-38479E520091", "name", "Default Character", "state", defaultCharacterState()))
	case KindScenes:
		return ojson.Obj("C181CF19-AF44-4348-8BCB-FB3EE582FC5D", ojson.Obj("id", "C181CF19-AF44-4348-8BCB-FB3EE582FC5D", "name", "Default Scene", "state", defaultScenePreset()))
	default:
		panic("unknown preset kind " + kind)
	}
	o, err := ojson.ParseObject(data)
	if err != nil {
		panic(err)
	}
	return o
}

func defaultCharacterState() *ojson.Object {
	return ojson.Obj("height", 1.6256, "model", "adult-female", "headScale", 1, "tintColor", "#000000",
		"morphTargets", ojson.Obj("mesomorphic", 0, "ectomorphic", 0, "endomorphic", 0))
}

// defaultScenePreset is the reducer's defaultScenePreset.
func defaultScenePreset() *ojson.Object {
	w := InitialWorld()
	w.Set("ground", false)
	w.Obj("room").Set("visible", true)
	w.Obj("ambient").Set("intensity", 0.1)
	objs := ojson.New()
	add := func(o *ojson.Object) { objs.Set(o.Str("id"), o) }
	add(ojson.Obj("id", "C2062AFC-D710-4C7D-942D-A3BAF8A76D5C", "type", "object", "model", "box", "width", 1, "height", 1, "depth", 1, "x", 0, "y", 0, "z", 0, "rotation", ojson.Obj("x", 0, "y", 0, "z", 0), "visible", true))
	add(ojson.Obj("id", "D8B95127-6C04-40A9-B592-8870EEAF43A8", "type", "object", "model", "chair", "width", 1, "height", 1, "depth", 1, "x", 2, "y", 0.5, "z", 0, "rotation", ojson.Obj("x", 0, "y", 0, "z", 0), "visible", true))
	add(ojson.Obj("id", "94FA0F9D-E1E8-436B-8041-D831BD06CB33", "type", "object", "model", "box", "width", 0.5, "height", 0.5, "depth", 0.5, "x", -2, "y", -2, "z", 0, "rotation", ojson.Obj("x", 0, "y", 2, "z", 0), "visible", true))
	ch := ojson.Obj("id", "A1A35319-82D1-4A24-98FE-136836750A61", "type", "character", "x", 1, "y", 0, "z", 0, "rotation", 0, "visible", true, "characterPresetId", "7C694D0F-9D45-4B74-BA70-38479E520091")
	st := defaultCharacterState()
	for _, k := range st.Keys() {
		ch.Set(k, st.Get(k))
	}
	ch.Set("posePresetId", DefaultPoseID)
	add(ch)
	add(ojson.Obj("id", "4F0FF9B8-BBB4-4D83-9E87-6EFE16A01D6F", "type", "light", "x", 1, "y", 1.5, "z", 2, "rotation", 10, "tilt", 10, "roll", 0, "intensity", 0.7, "visible", true, "angle", 1.04, "distance", 3, "penumbra", 0, "decay", 1))
	add(ojson.Obj("id", DefaultCameraID, "type", "camera", "fov", 22.25, "x", 0, "y", 6, "z", 1, "rotation", 0, "tilt", 0, "roll", 0))
	return ojson.Obj("world", w, "sceneObjects", objs, "activeCamera", DefaultCameraID)
}

func presetsPath(kind string) string {
	return filepath.Join(story.UserDataDir(), "presets", kind+".json")
}

// UserPresets reads userData/presets/<kind>.json (empty when missing).
func UserPresets(kind string) (*ojson.Object, error) {
	data, err := os.ReadFile(presetsPath(kind))
	if os.IsNotExist(err) {
		return ojson.New(), nil
	}
	if err != nil {
		return nil, err
	}
	o, err := ojson.ParseObject(data)
	if err != nil {
		return nil, err
	}
	if kind == KindPoses || kind == KindHandPoses {
		for _, k := range o.Keys() {
			if p := o.Obj(k); p != nil && p.Get("priority") == nil {
				p.Set("priority", 0)
			}
		}
	}
	return o, nil
}

// SaveUserPresets writes only user presets, 2-space indented.
func SaveUserPresets(kind string, o *ojson.Object) error {
	return story.WriteJSON(presetsPath(kind), o, "  ")
}

// AllPresets merges built-in and user presets (user wins on id clash).
func AllPresets(kind string) (*ojson.Object, error) {
	all := builtin(kind)
	user, err := UserPresets(kind)
	if err != nil {
		return nil, err
	}
	for _, k := range user.Keys() {
		all.Set(k, user.Get(k))
	}
	return all, nil
}

// IsBuiltinPreset reports whether an id ships with the app.
func IsBuiltinPreset(kind, id string) bool { return builtin(kind).Has(id) }

// PresetSummary is a list row.
type PresetSummary struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Keywords string  `json:"keywords,omitempty"`
	Priority float64 `json:"priority"`
	BuiltIn  bool    `json:"builtIn"`
}

// ListPresets returns presets sorted by priority then name, filtered by query words.
func ListPresets(kind, query string) ([]PresetSummary, error) {
	all, err := AllPresets(kind)
	if err != nil {
		return nil, err
	}
	b := builtin(kind)
	words := strings.Fields(strings.ToLower(query))
	var out []PresetSummary
	for _, id := range all.Keys() {
		p := all.Obj(id)
		if p == nil {
			continue
		}
		hay := strings.ToLower(p.Str("name") + " " + p.Str("keywords"))
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, PresetSummary{ID: id, Name: p.Str("name"), Keywords: p.Str("keywords"), Priority: p.NumOr("priority", 0), BuiltIn: b.Has(id)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return strings.ToUpper(out[i].Name) < strings.ToUpper(out[j].Name)
	})
	return out, nil
}

// FindPreset resolves an id, an id prefix or an exact name (case-insensitive).
func FindPreset(kind, key string) (*ojson.Object, error) {
	all, err := AllPresets(kind)
	if err != nil {
		return nil, err
	}
	if p := all.Obj(key); p != nil {
		return p, nil
	}
	var matches []*ojson.Object
	for _, id := range all.Keys() {
		p := all.Obj(id)
		if p != nil && (strings.EqualFold(p.Str("name"), key) || strings.HasPrefix(strings.ToLower(id), strings.ToLower(key))) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no %s preset %q", kind, key)
	}
	return matches[0], nil
}

// AddUserPreset stores a new user preset.
func AddUserPreset(kind string, p *ojson.Object) error {
	user, err := UserPresets(kind)
	if err != nil {
		return err
	}
	user.Set(p.Str("id"), p)
	return SaveUserPresets(kind, user)
}

// DeleteUserPreset removes a user preset (built-ins cannot be deleted).
func DeleteUserPreset(kind, id string) error {
	if IsBuiltinPreset(kind, id) {
		return fmt.Errorf("%s is a built-in preset and cannot be deleted", id)
	}
	user, err := UserPresets(kind)
	if err != nil {
		return err
	}
	if !user.Has(id) {
		return fmt.Errorf("no user %s preset %q", kind, id)
	}
	user.Delete(id)
	return SaveUserPresets(kind, user)
}

// EmotionsDir is where custom emotion textures live (userData/presets/emotions).
func EmotionsDir() string { return filepath.Join(story.UserDataDir(), "presets", "emotions") }
