// Package sg edits Shot Generator scenes (board.sg.data) as plain JSON, following
// shared/reducers/shot-generator.js and the scene object creators. Rendering is
// left to the UI; see Save for the hand-off.
package sg

import (
	"crypto/sha1"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"sb/internal/ojson"
)

const DefaultCameraID = "6BC46A44-7965-43B5-B290-E3D2B9D15EEE"

// DefaultPoseID is the 'stand' pose given to new characters.
const DefaultPoseID = "79BBBD0D-6BA2-4D84-9B71-EE661AB6E5AE"

// ShadingModes in the order CYCLE_SHADING_MODE walks them.
var ShadingModes = []string{"Outline", "Wireframe", "Flat", "Depth"}

// InitialWorld is initialScene.world.
func InitialWorld() *ojson.Object {
	return ojson.Obj(
		"ground", true,
		"backgroundColor", 0xE5E5E5,
		"room", ojson.Obj("visible", false, "width", 10, "length", 10, "height", 3),
		"environment", ojson.Obj("x", 0, "y", 0, "z", 0, "rotation", 0, "scale", 1, "visible", true, "grayscale", true),
		"ambient", ojson.Obj("intensity", 0.5),
		"directional", ojson.Obj("intensity", 0.5, "rotation", -0.9, "tilt", 0.75),
		"fog", ojson.Obj("visible", true, "far", 40),
		"shadingMode", "Outline",
	)
}

// Initial is the default scene: one camera and the ground (resetScene).
func Initial() *ojson.Object {
	cam := ojson.Obj("id", DefaultCameraID, "type", "camera", "fov", 22.25, "x", 0, "y", 6, "z", 1, "rotation", 0, "tilt", 0, "roll", 0)
	d := ojson.Obj("world", InitialWorld(), "sceneObjects", ojson.Obj(DefaultCameraID, cam), "activeCamera", DefaultCameraID)
	DisplayNames(d.Obj("sceneObjects"))
	return d
}

func Objects(d *ojson.Object) *ojson.Object { return d.ObjPath(true, "sceneObjects") }
func World(d *ojson.Object) *ojson.Object   { return d.ObjPath(true, "world") }
func Active(d *ojson.Object) string         { return d.Str("activeCamera") }

// Migrate applies the LOAD_SCENE migrations (older beta rotations and missing world keys).
func Migrate(d *ojson.Object) {
	objs := Objects(d)
	for _, id := range objs.Keys() {
		o := objs.Obj(id)
		if o == nil {
			continue
		}
		if o.Str("type") == "object" {
			if r, ok := o.Num("rotation"); ok && o.Obj("rotation") == nil {
				o.Set("rotation", ojson.Obj("x", 0, "y", r, "z", 0))
			}
		}
	}
	w := World(d)
	def := InitialWorld()
	for _, k := range []string{"ambient", "directional", "fog", "shadingMode"} {
		if w.Get(k) == nil {
			w.Set(k, def.Get(k))
		}
	}
	if env := w.Obj("environment"); env != nil && env.Get("grayscale") == nil {
		env.Set("grayscale", false)
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// DisplayNames is withDisplayNames: "<Key> <n>" counted per name/model/type.
func DisplayNames(objs *ojson.Object) {
	count := map[string]int{}
	for _, id := range objs.Keys() {
		o := objs.Obj(id)
		if o == nil {
			continue
		}
		key := o.Str("name")
		if key == "" {
			key = o.Str("model")
		}
		if key == "" {
			key = o.Str("type")
		}
		parts := strings.Split(key, "/")
		key = parts[len(parts)-1]
		count[key]++
		o.Set("displayName", capitalize(fmt.Sprintf("%s %d", key, count[key])))
	}
}

// Serialize is getSerializedState: drops `loaded`, `blocked`, and bone position/quaternion.
func Serialize(d *ojson.Object) *ojson.Object {
	c := d.Clone()
	objs := Objects(c)
	for _, id := range objs.Keys() {
		o := objs.Obj(id)
		if o == nil {
			continue
		}
		o.Delete("loaded")
		o.Delete("blocked")
		if o.Str("type") == "character" {
			if sk := o.Obj("skeleton"); sk != nil {
				for _, b := range sk.Keys() {
					if bo := sk.Obj(b); bo != nil {
						bo.Delete("quaternion")
						bo.Delete("position")
					}
				}
			}
		}
	}
	return ojson.Obj("world", c.Get("world"), "sceneObjects", c.Get("sceneObjects"), "activeCamera", c.Get("activeCamera"))
}

// Hash is getHash: base64 SHA-1 of the serialized state.
func Hash(d *ojson.Object) string {
	sum := sha1.Sum(ojson.Stringify(Serialize(d), ""))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// ---- models ----

// Model describes a built-in model.
type Model struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Type              string          `json:"type"`
	Height            float64         `json:"height,omitempty"`
	ValidMorphTargets []string        `json:"validMorphTargets,omitempty"`
	BindBone          string          `json:"bindBone,omitempty"`
	AttachableType    string          `json:"attachableType,omitempty"`
	X                 float64         `json:"x,omitempty"`
	Y                 float64         `json:"y,omitempty"`
	Z                 float64         `json:"z,omitempty"`
	Rotation          json.RawMessage `json:"rotation,omitempty"`
}

var (
	//go:embed data/objects.json
	objectsJSON []byte
	//go:embed data/attachables.json
	attachablesJSON []byte
)

var characterModels = []Model{
	{ID: "adult-female", Name: "Adult Female", Type: "character", ValidMorphTargets: []string{"ectomorphic", "mesomorphic", "endomorphic"}, Height: 1.65},
	{ID: "adult-male", Name: "Adult Male", Type: "character", ValidMorphTargets: []string{"ectomorphic", "mesomorphic", "endomorphic"}, Height: 1.8},
	{ID: "teen-female", Name: "Teen Female", Type: "character", ValidMorphTargets: []string{"ectomorphic", "mesomorphic", "endomorphic"}, Height: 1.6},
	{ID: "teen-male", Name: "Teen Male", Type: "character", ValidMorphTargets: []string{"ectomorphic", "mesomorphic", "endomorphic"}, Height: 1.6},
	{ID: "child", Name: "Child", Type: "character", ValidMorphTargets: []string{"ectomorphic", "endomorphic"}, Height: 1.2},
	{ID: "baby", Name: "Baby", Type: "character", ValidMorphTargets: []string{}, Height: 0.75},
}

func loadModelMap(data []byte) []Model {
	var m map[string]Model
	if err := json.Unmarshal(data, &m); err != nil {
		panic(err)
	}
	out := make([]Model, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Models lists built-in models: characters (6), objects (47 incl. box), attachables (7).
func Models() []Model {
	out := append([]Model{}, characterModels...)
	out = append(out, Model{ID: "box", Name: "Box", Type: "object", Height: 1})
	out = append(out, loadModelMap(objectsJSON)...)
	return append(out, loadModelMap(attachablesJSON)...)
}

// FindModel looks up a built-in model by id.
func FindModel(id string) (Model, bool) {
	for _, m := range Models() {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// IsCustom reports a user model path ("models/objects/x.glb").
func IsCustom(model string) bool { return strings.Contains(model, "/") }
