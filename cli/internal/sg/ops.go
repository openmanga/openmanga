package sg

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"sb/internal/ojson"
	"sb/internal/story"
)

// Doc is one board's 3D scene being edited.
type Doc struct {
	Data       *ojson.Object
	ProjectDir string // folder of the .storyboarder, for custom models and images
	Rand       *rand.Rand
}

// NewDoc wraps scene data, applying the load migrations.
func NewDoc(data *ojson.Object, projectDir string) *Doc {
	Migrate(data)
	return &Doc{Data: data, ProjectDir: projectDir, Rand: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))}
}

func (d *Doc) Objects() *ojson.Object { return Objects(d.Data) }

// Obj finds a scene object by id, id prefix (>= 4 chars) or display name.
func (d *Doc) Obj(key string) (*ojson.Object, error) {
	objs := d.Objects()
	if o := objs.Obj(key); o != nil {
		return o, nil
	}
	var found []*ojson.Object
	for _, id := range objs.Keys() {
		o := objs.Obj(id)
		if o == nil {
			continue
		}
		if strings.EqualFold(o.Str("displayName"), key) || (len(key) >= 4 && strings.HasPrefix(strings.ToUpper(id), strings.ToUpper(key))) {
			found = append(found, o)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return nil, fmt.Errorf("no scene object %q (see `sg list`)", key)
	}
	return nil, fmt.Errorf("%q matches %d objects; use the full id", key, len(found))
}

// ObjOfType finds an object and checks its type.
func (d *Doc) ObjOfType(key string, types ...string) (*ojson.Object, error) {
	o, err := d.Obj(key)
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		if o.Str("type") == t {
			return o, nil
		}
	}
	return nil, fmt.Errorf("%s is a %s, expected %s", o.Str("displayName"), o.Str("type"), strings.Join(types, " or "))
}

// ActiveCamera returns the active camera object.
func (d *Doc) ActiveCamera() *ojson.Object {
	if o := d.Objects().Obj(Active(d.Data)); o != nil {
		return o
	}
	for _, id := range d.Objects().Keys() {
		if o := d.Objects().Obj(id); o != nil && o.Str("type") == "camera" {
			return o
		}
	}
	return nil
}

func (d *Doc) finish() { DisplayNames(d.Objects()) }

// ---- creation ----

// genPosition is generatePositionAndRotation: 5 m in front of the active camera
// (halfway to the room wall when closer), on the floor, +/-0.3 m jitter, facing the camera.
func (d *Doc) genPosition() (x, y, z, rot float64) {
	cam := d.ActiveCamera()
	if cam == nil {
		return 0, 0, 0, 0
	}
	c := CameraFromObject(cam, 1)
	dir := c.Direction()
	dist := 5.0
	if room := World(d.Data).Obj("room"); room != nil && room.Bool("visible") {
		if t := rayExitBox(c.Pos, dir, room.NumOr("width", 10), room.NumOr("height", 3), room.NumOr("length", 10)); t > 0 && t < dist {
			dist = t / 2
		}
	}
	center := c.Pos.Add(dir.Mul(dist))
	p := Vec3{center.X + (d.Rand.Float64()*2-1)*0.3, 0, center.Z + (d.Rand.Float64()*2-1)*0.3}
	to := c.Pos.Sub(p)
	return p.X, p.Z, p.Y, math.Atan2(to.X, to.Z)
}

// rayExitBox returns the distance at which a ray leaves the room box
// (floor at 0, centered on the origin), or -1.
func rayExitBox(o, dir Vec3, w, h, l float64) float64 {
	lo := Vec3{-w / 2, 0, -l / 2}
	hi := Vec3{w / 2, h, l / 2}
	best := math.Inf(1)
	for _, ax := range []struct{ o, d, lo, hi float64 }{{o.X, dir.X, lo.X, hi.X}, {o.Y, dir.Y, lo.Y, hi.Y}, {o.Z, dir.Z, lo.Z, hi.Z}} {
		if ax.d > 0 {
			best = math.Min(best, (ax.hi-ax.o)/ax.d)
		} else if ax.d < 0 {
			best = math.Min(best, (ax.lo-ax.o)/ax.d)
		}
	}
	if math.IsInf(best, 1) {
		return -1
	}
	return best
}

// Create adds an object (CREATE_OBJECT), assigning an id when missing.
func (d *Doc) Create(o *ojson.Object) *ojson.Object {
	if o.Str("id") == "" {
		o.Set("id", story.UUID())
	}
	d.Objects().Set(o.Str("id"), o)
	d.finish()
	return o
}

// AddCamera puts a new camera ~0.91 m left of the active one and activates it.
func (d *Doc) AddCamera() *ojson.Object {
	cam := d.ActiveCamera()
	o := ojson.Obj("id", story.UUID(), "type", "camera", "fov", 22.25, "x", -0.91, "y", 6.0, "z", 1.0, "rotation", 0.0, "tilt", 0.0, "roll", 0.0)
	if cam != nil {
		o = ojson.Obj("id", story.UUID(), "type", "camera", "fov", cam.NumOr("fov", 22.25),
			"x", cam.NumOr("x", 0)-0.91, "y", cam.NumOr("y", 0), "z", cam.NumOr("z", 0),
			"rotation", cam.NumOr("rotation", 0), "tilt", cam.NumOr("tilt", 0), "roll", cam.NumOr("roll", 0))
	}
	d.Create(o)
	d.Data.Set("activeCamera", o.Str("id"))
	return o
}

// AddObject creates a 1x1x1 box or a built-in/custom model in front of the camera.
func (d *Doc) AddObject(model string) (*ojson.Object, error) {
	if model == "" {
		model = "box"
	}
	if !IsCustom(model) {
		if m, ok := FindModel(model); !ok || m.Type != "object" {
			return nil, fmt.Errorf("unknown object model %q (see `sg models list --type object`)", model)
		}
	}
	x, y, z, r := d.genPosition()
	return d.Create(ojson.Obj("id", story.UUID(), "type", "object", "model", model, "tintColor", "#000000",
		"width", 1, "height", 1, "depth", 1, "x", x, "y", y, "z", z,
		"rotation", ojson.Obj("x", 0, "y", r, "z", 0), "visible", true)), nil
}

func defaultPoseSkeleton() *ojson.Object {
	p := builtin(KindPoses).Obj(DefaultPoseID)
	return p.ObjPath(false, "state", "skeleton").Clone()
}

// AddCharacter creates an adult male (1.8 m) in the stand pose, or another model.
func (d *Doc) AddCharacter(model string) (*ojson.Object, error) {
	height := 1.8
	if model == "" {
		model = "adult-male"
	} else if m, ok := FindModel(model); ok && m.Type == "character" {
		height = m.Height
	} else if !IsCustom(model) {
		return nil, fmt.Errorf("unknown character model %q", model)
	}
	x, y, z, r := d.genPosition()
	return d.Create(ojson.Obj("id", story.UUID(), "type", "character", "height", height, "model", model,
		"x", x, "y", y, "z", z, "rotation", r, "headScale", 1, "tintColor", "#000000",
		"morphTargets", ojson.Obj("mesomorphic", 0, "ectomorphic", 0, "endomorphic", 0),
		"posePresetId", DefaultPoseID, "skeleton", defaultPoseSkeleton(), "visible", true)), nil
}

// AddLight creates a spot light, intensity 0.8, 2 m high.
func (d *Doc) AddLight() *ojson.Object {
	x, y, _, r := d.genPosition()
	return d.Create(ojson.Obj("id", story.UUID(), "type", "light", "x", x, "y", y, "z", 2,
		"rotation", r, "tilt", 0, "roll", 0, "intensity", 0.8, "visible", true,
		"angle", 1.04, "distance", 5, "penumbra", 1.0, "decay", 1))
}

// VolumePresets are the built-in volume textures.
var VolumePresets = map[string][]any{"rain": {"rain1", "rain2"}, "fog": {"fog1", "fog2"}, "explosion": {"debris", "explosion"}}

// AddVolume creates a 5x5x5 volume (rain by default).
func (d *Doc) AddVolume(preset string) (*ojson.Object, error) {
	ids := []any{"rain2", "rain1"}
	if preset != "" {
		p, ok := VolumePresets[preset]
		if !ok {
			return nil, fmt.Errorf("volume preset must be rain, fog or explosion")
		}
		ids = p
	}
	x, y, z, r := d.genPosition()
	return d.Create(ojson.Obj("id", story.UUID(), "type", "volume", "x", x, "y", y, "z", z,
		"width", 5, "height", 5, "depth", 5, "rotation", r, "visible", true, "opacity", 0.3,
		"color", 0x777777, "numberOfLayers", 4, "distanceBetweenLayers", 1.5,
		"volumeImageAttachmentIds", ids)), nil
}

// CopyAsset copies a file into <project>/models/<kind>/ and returns the relative path.
func (d *Doc) CopyAsset(file, kind string) (string, error) {
	if d.ProjectDir == "" {
		return "", fmt.Errorf("no project folder to copy %s into", file)
	}
	rel := filepath.ToSlash(filepath.Join("models", kind, filepath.Base(file)))
	dst := filepath.Join(d.ProjectDir, filepath.FromSlash(rel))
	abs, _ := filepath.Abs(file)
	if abs != dst {
		if err := story.CopyFile(file, dst); err != nil {
			return "", err
		}
	}
	return rel, nil
}

// AddImage creates an image plane with a placeholder or a copied file.
func (d *Doc) AddImage(file string) (*ojson.Object, error) {
	img := "placeholder"
	if file != "" {
		rel, err := d.CopyAsset(file, "images")
		if err != nil {
			return nil, err
		}
		img = rel
	}
	x, y, _, r := d.genPosition()
	return d.Create(ojson.Obj("id", story.UUID(), "type", "image", "tintColor", "#000000", "height", 1,
		"x", x, "y", y, "z", 1, "rotation", ojson.Obj("x", 0, "y", r, "z", 0),
		"visible", true, "opacity", 1, "visibleToCam", true, "imageAttachmentIds", []any{img})), nil
}

// ---- updates ----

// ErrLocked is returned when editing a locked object.
type ErrLocked struct{ Name string }

func (e ErrLocked) Error() string {
	return e.Name + " is locked (run `sb sg unlock` first)"
}

// Update is UPDATE_OBJECT followed by the preset-change checks.
func (d *Doc) Update(o *ojson.Object, props *ojson.Object) error {
	props = props.Clone()
	if props.Has("locked") {
		o.Set("locked", props.Bool("locked"))
		props.Delete("locked")
	}
	if props.Len() == 0 {
		return nil
	}
	if o.Bool("locked") || o.Bool("blocked") {
		return ErrLocked{o.Str("displayName")}
	}
	typ := o.Str("type")
	if r := props.Get("rotation"); r != nil {
		if ro, ok := r.(*ojson.Object); ok && (typ == "object" || typ == "image") {
			cur := o.Obj("rotation")
			if cur == nil {
				cur = ojson.Obj("x", 0, "y", 0, "z", 0)
			} else {
				cur = cur.Clone()
			}
			for _, k := range ro.Keys() {
				cur.Set(k, ro.Get(k))
			}
			o.Set("rotation", cur)
		} else {
			o.Set("rotation", r)
		}
		props.Delete("rotation")
	}
	if m, ok := props.Get("model").(string); ok {
		o.Set("model", m)
		if typ == "character" {
			h := 1.6
			if mm, ok := FindModel(m); ok {
				h = mm.Height
			}
			o.Set("height", h)
		}
		props.Delete("model")
	}
	if mt := props.Obj("morphTargets"); mt != nil {
		cur := o.ObjPath(true, "morphTargets")
		for _, k := range mt.Keys() {
			cur.Set(k, mt.Get(k))
		}
		props.Delete("morphTargets")
	}
	if props.Has("posePresetId") {
		o.Set("posePresetId", props.Get("posePresetId"))
		if o.Str("handPosePresetId") != "" {
			o.Set("handPosePresetId", nil)
			o.Set("handSkeleton", []any{})
		}
	}
	for _, k := range props.Keys() {
		if k == "posePresetId" {
			continue
		}
		v := props.Get(k)
		if v == nil && (k == "emotionPresetId" || k == "characterPresetId") {
			o.Delete(k) // undefined in the original
			continue
		}
		o.Set(k, v)
	}
	if typ == "character" {
		if !props.Has("characterPresetId") {
			d.checkCharacterPreset(o)
		}
		if !props.Has("posePresetId") {
			d.checkPosePreset(o)
		}
		if !props.Has("handPosePresetId") {
			d.checkHandPosePreset(o)
		}
	}
	d.finish()
	return nil
}

func sameRotations(a, b *ojson.Object) bool {
	if a == nil || b == nil || a.Len() != b.Len() {
		return false
	}
	for _, name := range b.Keys() {
		pa, pb := a.ObjPath(false, name, "rotation"), b.ObjPath(false, name, "rotation")
		if pa == nil || pb == nil {
			return false
		}
		for _, k := range []string{"x", "y", "z"} {
			x, _ := pa.Num(k)
			y, _ := pb.Num(k)
			if x != y {
				return false
			}
		}
	}
	return true
}

func (d *Doc) checkCharacterPreset(o *ojson.Object) {
	id := o.Str("characterPresetId")
	if id == "" {
		return
	}
	p, err := FindPreset(KindCharacters, id)
	if err != nil || p.Str("id") != id {
		o.Delete("characterPresetId")
		return
	}
	st := p.Obj("state")
	for _, k := range st.Keys() {
		switch v := st.Get(k).(type) {
		case string:
			if o.Str(k) != v {
				o.Delete("characterPresetId")
				return
			}
		case *ojson.Object, []any:
		default:
			a, _ := ojson.Num(v)
			b, _ := o.Num(k)
			if a != b {
				o.Delete("characterPresetId")
				return
			}
		}
	}
	for _, k := range []string{"mesomorphic", "ectomorphic", "endomorphic"} {
		a, _ := st.ObjPath(true, "morphTargets").Num(k)
		b, _ := o.ObjPath(true, "morphTargets").Num(k)
		if a != b {
			o.Delete("characterPresetId")
			return
		}
	}
}

func (d *Doc) checkPosePreset(o *ojson.Object) {
	id := o.Str("posePresetId")
	if id == "" {
		return
	}
	p, err := FindPreset(KindPoses, id)
	if err != nil || p.Str("id") != id || !sameRotations(o.Obj("skeleton"), p.ObjPath(false, "state", "skeleton")) {
		o.Delete("posePresetId")
	}
}

func (d *Doc) checkHandPosePreset(o *ojson.Object) {
	id := o.Str("handPosePresetId")
	if id == "" {
		return
	}
	p, err := FindPreset(KindHandPoses, id)
	if err != nil || p.Str("id") != id || !sameRotations(o.Obj("handSkeleton"), p.ObjPath(false, "state", "handSkeleton")) {
		o.Delete("handPosePresetId")
	}
}

// ---- delete, duplicate, groups ----

// Delete removes objects, their group membership and a character's attachables.
// The active camera cannot be deleted.
func (d *Doc) Delete(ids []string) error {
	objs := d.Objects()
	for _, id := range ids {
		if id == Active(d.Data) {
			return fmt.Errorf("the active camera cannot be deleted; activate another camera first")
		}
	}
	for _, id := range ids {
		o := objs.Obj(id)
		if o == nil {
			continue
		}
		if g := objs.Obj(o.Str("group")); g != nil {
			var keep []any
			for _, c := range g.Arr("children") {
				if c != id {
					keep = append(keep, c)
				}
			}
			if len(keep) == 0 {
				objs.Delete(g.Str("id"))
			} else {
				g.Set("children", keep)
			}
		}
		if o.Str("type") == "character" {
			for _, k := range objs.Keys() {
				if a := objs.Obj(k); a != nil && a.Str("attachToId") == id {
					objs.Delete(k)
				}
			}
		}
		objs.Delete(id)
	}
	d.finish()
	return nil
}

// Duplicate copies objects with a +0.5 m x/y offset and " copy" names (deep for groups).
func (d *Doc) Duplicate(ids []string) []string {
	objs := d.Objects()
	var out []string
	for _, id := range ids {
		src := objs.Obj(id)
		if src == nil {
			continue
		}
		dup := func(s *ojson.Object, newID string) *ojson.Object {
			c := s.Clone()
			if s.Get("name") == nil {
				c.Set("name", nil)
			} else {
				c.Set("name", s.Str("name")+" copy")
			}
			if s.Has("x") {
				c.Set("x", s.NumOr("x", 0)+0.5)
				c.Set("y", s.NumOr("y", 0)+0.5)
			}
			c.Set("id", newID)
			return c
		}
		dstID := story.UUID()
		dst := dup(src, dstID)
		objs.Set(dstID, dst)
		if g := objs.Obj(src.Str("group")); g != nil {
			g.Set("children", append(g.Arr("children"), dstID))
		}
		if src.Arr("children") != nil {
			var kids []any
			for _, cid := range src.Arr("children") {
				child := objs.Obj(fmt.Sprint(cid))
				if child == nil {
					continue
				}
				nid := story.UUID()
				c := dup(child, nid)
				c.Set("group", dstID)
				objs.Set(nid, c)
				kids = append(kids, nid)
			}
			dst.Set("children", kids)
		}
		out = append(out, dstID)
	}
	d.finish()
	return out
}

// Group creates a group of objects.
func (d *Doc) Group(ids []string) (string, error) {
	if len(ids) == 0 {
		return "", fmt.Errorf("nothing to group")
	}
	var kids []any
	for _, id := range ids {
		o := d.Objects().Obj(id)
		if o == nil {
			return "", fmt.Errorf("no object %s", id)
		}
		kids = append(kids, id)
	}
	gid := story.UUID()
	d.Objects().Set(gid, ojson.Obj("id", gid, "name", "Group", "type", "group", "visible", true, "children", kids))
	for _, id := range ids {
		d.Objects().Obj(id).Set("group", gid)
	}
	d.finish()
	return gid, nil
}

// Ungroup removes objects (all when ids is empty) from a group; an empty group is deleted.
func (d *Doc) Ungroup(gid string, ids []string) error {
	g := d.Objects().Obj(gid)
	if g == nil || g.Str("type") != "group" {
		return fmt.Errorf("%s is not a group", gid)
	}
	sel := map[string]bool{}
	for _, id := range ids {
		sel[id] = true
	}
	var keep []any
	for _, c := range g.Arr("children") {
		cid := fmt.Sprint(c)
		if child := d.Objects().Obj(cid); child != nil && (len(ids) == 0 || sel[cid]) {
			child.Set("group", nil)
			continue
		}
		keep = append(keep, c)
	}
	if len(keep) == 0 {
		d.Objects().Delete(gid)
	} else {
		g.Set("children", keep)
	}
	d.finish()
	return nil
}

// MergeGroups moves every child of the other groups and the loose ids into the first group.
func (d *Doc) MergeGroups(groupIDs, ids []string) error {
	dest := d.Objects().Obj(groupIDs[0])
	if dest == nil {
		return fmt.Errorf("no group %s", groupIDs[0])
	}
	has := func(id string) bool {
		for _, c := range dest.Arr("children") {
			if c == id {
				return true
			}
		}
		return false
	}
	add := func(id string) {
		if o := d.Objects().Obj(id); o != nil {
			o.Set("group", dest.Str("id"))
			if !has(id) {
				dest.Set("children", append(dest.Arr("children"), id))
			}
		}
	}
	for _, gid := range groupIDs[1:] {
		g := d.Objects().Obj(gid)
		if g == nil {
			continue
		}
		for _, c := range g.Arr("children") {
			add(fmt.Sprint(c))
		}
		d.Objects().Delete(gid)
	}
	for _, id := range ids {
		add(id)
	}
	d.finish()
	return nil
}

// GroupMove moves all children of a group by a delta.
func (d *Doc) GroupMove(gid string, dx, dy, dz float64) error {
	g := d.Objects().Obj(gid)
	if g == nil || g.Str("type") != "group" {
		return fmt.Errorf("%s is not a group", gid)
	}
	for _, c := range g.Arr("children") {
		if o := d.Objects().Obj(fmt.Sprint(c)); o != nil && !o.Bool("locked") {
			o.Set("x", o.NumOr("x", 0)+dx)
			o.Set("y", o.NumOr("y", 0)+dy)
			o.Set("z", o.NumOr("z", 0)+dz)
		}
	}
	return nil
}

// BatchMove sets x/y/z/rotation for many ids (UPDATE_OBJECTS); locked objects are skipped.
// Unlike the original, 0 is a valid value.
func (d *Doc) BatchMove(changes *ojson.Object) ([]string, error) {
	var moved []string
	for _, id := range changes.Keys() {
		o := d.Objects().Obj(id)
		if o == nil {
			return moved, fmt.Errorf("no object %s", id)
		}
		if o.Bool("locked") {
			continue
		}
		v := changes.Obj(id)
		if v == nil {
			continue
		}
		for _, k := range []string{"x", "y", "z", "rotation"} {
			if v.Has(k) {
				o.Set(k, v.Get(k))
			}
		}
		moved = append(moved, id)
	}
	return moved, nil
}

// SetVisible shows or hides an object; groups pass it on to their children.
func (d *Doc) SetVisible(o *ojson.Object, v bool) {
	o.Set("visible", v)
	for _, c := range o.Arr("children") {
		if child := d.Objects().Obj(fmt.Sprint(c)); child != nil {
			child.Set("visible", v)
		}
	}
}

// ---- drop to floor ----

// surfaceBelow finds the highest ground or box top under point p (three.js space), or false.
func (d *Doc) surfaceBelow(p Vec3, skip string) (float64, bool) {
	best, ok := math.Inf(-1), false
	if p.Y > 0 { // the ground plane
		best, ok = 0, true
	}
	for _, id := range d.Objects().Keys() {
		o := d.Objects().Obj(id)
		if o == nil || id == skip || o.Str("type") != "object" || o.Str("model") != "box" {
			continue
		}
		// box footprint in its own frame (Y rotation only)
		ry := 0.0
		if r := o.Obj("rotation"); r != nil {
			ry = r.NumOr("y", 0)
		}
		local := AxisAngle(Vec3{0, 1, 0}, -ry).Rotate(Vec3{p.X - o.NumOr("x", 0), 0, p.Z - o.NumOr("y", 0)})
		w, dep := o.NumOr("width", 1), o.NumOr("depth", 1)
		if math.Abs(local.X) > w/2 || math.Abs(local.Z) > dep/2 {
			continue
		}
		top := o.NumOr("z", 0) + o.NumOr("height", 1)
		if top < p.Y && top > best {
			best, ok = top, true
		}
	}
	return best, ok
}

// Drop lowers objects and characters onto the ground or a box below them
// (dropToObjects.js; box models only, other models are not raycast targets here).
func (d *Doc) Drop(ids []string) (map[string]float64, error) {
	out := map[string]float64{}
	for _, id := range ids {
		o := d.Objects().Obj(id)
		if o == nil {
			return out, fmt.Errorf("no object %s", id)
		}
		if o.Bool("locked") {
			continue
		}
		switch o.Str("type") {
		case "object":
			base := Vec3{o.NumOr("x", 0), o.NumOr("z", 0), o.NumOr("y", 0)}
			if top, ok := d.surfaceBelow(base, id); ok {
				o.Set("z", top)
				out[id] = top
			}
		case "character":
			sk, err := ModelSkeleton(o.Str("model"), d.ProjectDir)
			if err != nil {
				return out, err
			}
			p := Pose(o, sk)
			low := 0
			for i := range p.World {
				if p.BonePos(i).Y < p.BonePos(low).Y {
					low = i
				}
			}
			lp := p.BonePos(low)
			if top, ok := d.surfaceBelow(lp, id); ok {
				z := o.NumOr("z", 0) + (top - lp.Y)
				o.Set("z", z)
				out[id] = z
			}
		}
	}
	return out, nil
}

// ---- characters ----

// ApplyPose sets a pose preset's skeleton (clears any hand pose).
func (d *Doc) ApplyPose(ch *ojson.Object, preset *ojson.Object) error {
	return d.Update(ch, ojson.Obj("posePresetId", preset.Str("id"), "skeleton", preset.ObjPath(true, "state", "skeleton").Clone()))
}

func mirrorRotation(r *ojson.Object) *ojson.Object {
	q := Euler{r.NumOr("x", 0), r.NumOr("y", 0), r.NumOr("z", 0), "XYZ"}.Quat()
	q.X, q.W = -q.X, -q.W
	e := EulerFromQuat(q, "XYZ")
	return ojson.Obj("x", e.X, "y", e.Y, "z", e.Z)
}

func swapSide(name string) string {
	switch {
	case strings.Contains(name, "Left"):
		return strings.Replace(name, "Left", "Right", 1)
	case strings.Contains(name, "Right"):
		return strings.Replace(name, "Right", "Left", 1)
	}
	return name
}

// MirrorPose mirrors the skeleton left/right (PosePresetsInspector mirrorSkeleton).
func (d *Doc) MirrorPose(ch *ojson.Object) error {
	if ch.Bool("locked") {
		return ErrLocked{ch.Str("displayName")}
	}
	sk := ch.Obj("skeleton")
	if sk == nil || sk.Len() == 0 {
		return fmt.Errorf("character has no posed bones to mirror")
	}
	out := sk.Clone()
	for _, name := range sk.Keys() {
		b := sk.Obj(name)
		if b == nil || b.Obj("rotation") == nil {
			continue
		}
		target := swapSide(name)
		dst := out.Obj(target)
		if dst == nil {
			dst = ojson.New()
			out.Set(target, dst)
		}
		dst.Set("rotation", mirrorRotation(b.Obj("rotation")))
		dst.Set("name", target)
	}
	ch.Set("skeleton", out)
	d.checkPosePreset(ch)
	return nil
}

// SetBone is UPDATE_CHARACTER_SKELETON for one bone (radians).
func (d *Doc) SetBone(ch *ojson.Object, bone string, x, y, z float64) error {
	if ch.Bool("locked") {
		return ErrLocked{ch.Str("displayName")}
	}
	sk := ch.ObjPath(true, "skeleton")
	b := sk.Obj(bone)
	rot := ojson.Obj("x", x, "y", y, "z", z)
	if b == nil {
		sk.Set(bone, ojson.Obj("rotation", rot))
	} else {
		b.Set("rotation", rot)
	}
	if hs := ch.Obj("handSkeleton"); hs != nil && hs.Has(bone) {
		hs.Set(bone, ojson.Obj("rotation", rot.Clone()))
	}
	d.checkPosePreset(ch)
	return nil
}

// ApplyHandPose applies a hand pose preset to "LeftHand", "RightHand" or "BothHands"
// (HandPresetsEditorItem onPointerDown).
func (d *Doc) ApplyHandPose(ch, preset *ojson.Object, hand string) error {
	hs := preset.ObjPath(true, "state", "handSkeleton")
	cur := ch.Obj("handSkeleton")
	if cur == nil {
		cur = ojson.New()
	}
	result := hs.Clone()
	if hs.Len() > 0 {
		presetHand := "LeftHand"
		if strings.Contains(hs.Keys()[0], "RightHand") {
			presetHand = "RightHand"
		}
		other := "RightHand"
		if presetHand == "RightHand" {
			other = "LeftHand"
		}
		mirrored := ojson.New()
		for _, k := range hs.Keys() {
			if r := hs.ObjPath(false, k, "rotation"); r != nil {
				mirrored.Set(strings.Replace(k, presetHand, other, 1), ojson.Obj("rotation", mirrorRotation(r)))
			}
		}
		apply := func(base, changes *ojson.Object) *ojson.Object {
			out := base.Clone()
			for _, k := range changes.Keys() {
				out.Set(k, changes.Get(k))
			}
			return out
		}
		hasBones := func(side string) bool {
			for _, k := range cur.Keys() {
				if strings.Contains(k, side) {
					return true
				}
			}
			return false
		}
		switch {
		case hand == "BothHands":
			result = apply(mirrored, hs)
		case hand != presetHand:
			if hasBones(presetHand) {
				result = apply(cur, mirrored)
			} else {
				result = mirrored
			}
		default:
			if hasBones(other) {
				result = apply(cur, hs)
			}
		}
	}
	return d.Update(ch, ojson.Obj("handPosePresetId", preset.Str("id"), "handSkeleton", result))
}

// SetHair replaces the character's hair attachable ("none" removes it).
func (d *Doc) SetHair(ch *ojson.Object, hair, file string) (*ojson.Object, error) {
	for _, k := range d.Objects().Keys() {
		if a := d.Objects().Obj(k); a != nil && a.Str("type") == "attachable" && a.Str("attachableType") == "hair" && a.Str("attachToId") == ch.Str("id") {
			d.Objects().Delete(k)
		}
	}
	if hair == "none" && file == "" {
		d.finish()
		return nil, nil
	}
	o := ojson.Obj("id", story.UUID(), "type", "attachable", "attachableType", "hair", "bindBone", "Head",
		"x", 0, "y", 0, "z", 0, "rotation", ojson.Obj("x", 0, "y", 0, "z", 0), "size", 1, "status", "PENDING")
	if file != "" {
		rel, err := d.CopyAsset(file, "attachables")
		if err != nil {
			return nil, err
		}
		o.Set("model", rel)
		o.Set("attachToId", ch.Str("id"))
		o.Set("name", strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)))
		o.Set("x", -0.0013)
		o.Set("y", 0.15)
		o.Set("z", 0.014)
	} else {
		if hair == "default" {
			hair = "hair"
		}
		if hair == "curly" {
			hair = "hair-curly"
		}
		m, ok := FindModel(hair)
		if !ok || m.AttachableType != "hair" {
			return nil, fmt.Errorf("hair must be default, curly, none or --file")
		}
		o.Set("model", m.ID)
		o.Set("attachToId", ch.Str("id"))
		o.Set("name", m.Name)
		o.Set("x", m.X)
		o.Set("y", m.Y)
		o.Set("z", m.Z)
		o.Set("rotation", ojson.From(m.Rotation))
	}
	return d.Create(o), nil
}

// AddAttachable binds a built-in attachable (or a custom file with a bone) to a character.
func (d *Doc) AddAttachable(ch *ojson.Object, model, file, bone string) (*ojson.Object, error) {
	o := ojson.Obj("id", story.UUID(), "type", "attachable")
	if file != "" {
		if bone == "" {
			return nil, fmt.Errorf("a custom attachable needs --bone")
		}
		rel, err := d.CopyAsset(file, "attachables")
		if err != nil {
			return nil, err
		}
		o.Set("x", 0)
		o.Set("y", 0)
		o.Set("z", 0)
		o.Set("model", rel)
		o.Set("name", rel)
		o.Set("bindBone", bone)
		o.Set("rotation", ojson.Obj("x", 0, "y", 0, "z", 0))
	} else {
		m, ok := FindModel(model)
		if !ok || m.Type != "attachable" || m.AttachableType == "hair" {
			return nil, fmt.Errorf("unknown attachable %q (see `sg models list --type attachable`)", model)
		}
		if bone == "" {
			bone = m.BindBone
		}
		o.Set("x", m.X)
		o.Set("y", m.Y)
		o.Set("z", m.Z)
		o.Set("model", m.ID)
		o.Set("name", m.Name)
		o.Set("bindBone", bone)
		o.Set("rotation", ojson.From(m.Rotation))
	}
	o.Set("attachToId", ch.Str("id"))
	o.Set("size", 1)
	o.Set("status", "PENDING")
	return d.Create(o), nil
}

// ApplyCharacterPreset sets model, height, head, tint, morphs and the stand pose,
// and recreates the preset's attachables with their bone-space offsets.
func (d *Doc) ApplyCharacterPreset(ch, preset *ojson.Object) error {
	st := preset.Obj("state")
	for _, k := range d.Objects().Keys() {
		if a := d.Objects().Obj(k); a != nil && a.Str("attachToId") == ch.Str("id") {
			d.Objects().Delete(k)
		}
	}
	name := ch.Get("name")
	if ch.Str("name") == "" {
		name = preset.Str("name")
	}
	props := ojson.Obj("characterPresetId", preset.Str("id"), "height", st.Get("height"), "model", st.Get("model"),
		"headScale", st.Get("headScale"), "tintColor", st.Get("tintColor"), "rotation", 0,
		"morphTargets", st.ObjPath(true, "morphTargets").Clone(), "name", name,
		"posePresetId", DefaultPoseID, "skeleton", defaultPoseSkeleton())
	if err := d.Update(ch, props); err != nil {
		return err
	}
	// the model change reset the height; the preset wins
	ch.Set("height", st.Get("height"))
	for _, v := range st.Arr("attachables") {
		a, ok := v.(*ojson.Object)
		if !ok {
			continue
		}
		n := ojson.Obj("id", story.UUID(), "type", "attachable", "attachToId", ch.Str("id"), "model", a.Get("model"),
			"name", a.Get("name"), "size", a.Get("size"), "bindBone", a.Get("bindBone"),
			"x", a.Get("x"), "y", a.Get("y"), "z", a.Get("z"), "rotation", a.Get("rotation"))
		d.Objects().Set(n.Str("id"), n)
	}
	ch.Set("characterPresetId", preset.Str("id"))
	d.finish()
	return nil
}

// CharacterPresetFrom builds a preset from a character and its attachables.
func (d *Doc) CharacterPresetFrom(ch *ojson.Object, name string) *ojson.Object {
	st := ojson.Obj("height", ch.Get("height"), "model", ch.Get("model"), "headScale", ch.Get("headScale"), "tintColor", ch.Get("tintColor"),
		"morphTargets", ch.ObjPath(true, "morphTargets").Clone())
	var att []any
	for _, k := range d.Objects().Keys() {
		if a := d.Objects().Obj(k); a != nil && a.Str("attachToId") == ch.Str("id") {
			att = append(att, a.Clone())
		}
	}
	if len(att) > 0 {
		st.Set("attachables", att)
		st.Set("presetPosition", ojson.Obj("x", ch.Get("x"), "y", ch.Get("y"), "z", ch.Get("z")))
		st.Set("presetRotation", ch.Get("rotation"))
	}
	return ojson.Obj("id", story.UUID(), "name", name, "state", st)
}

// SetModelFile copies a custom .glb into models/<kind>s and sets it on the object.
func (d *Doc) SetModelFile(o *ojson.Object, file string) error {
	kind := map[string]string{"character": "characters", "object": "objects"}[o.Str("type")]
	if kind == "" {
		return fmt.Errorf("only characters and objects take a model file")
	}
	if strings.ToLower(filepath.Ext(file)) != ".glb" {
		return fmt.Errorf("model files must be .glb")
	}
	if _, err := os.Stat(file); err != nil {
		return err
	}
	rel, err := d.CopyAsset(file, kind)
	if err != nil {
		return err
	}
	return d.Update(o, ojson.Obj("model", rel))
}
