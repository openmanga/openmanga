package sg

import (
	"fmt"
	"math"
	"strings"

	"sb/internal/ojson"
)

// LensAspect is the aspect of the fake camera the lens buttons use (CameraPanelInspector).
const LensAspect = 2.348927875243665

// Camera is a three.js perspective camera state.
type Camera struct {
	Pos    Vec3
	Q      Quat
	Fov    float64
	Aspect float64
}

// CameraFromObject is CameraUpdate: position (x, z, y), rotation Y then local X (tilt) then Z (roll).
func CameraFromObject(o *ojson.Object, aspect float64) Camera {
	return Camera{
		Pos:    Vec3{o.NumOr("x", 0), o.NumOr("z", 0), o.NumOr("y", 0)},
		Q:      Euler{o.NumOr("tilt", 0), o.NumOr("rotation", 0), o.NumOr("roll", 0), "YXZ"}.Quat(),
		Fov:    o.NumOr("fov", 22.25),
		Aspect: aspect,
	}
}

// Apply writes the camera back as x, y, z, rotation, tilt, roll (Euler YXZ).
func (c Camera) Apply(o *ojson.Object) {
	e := EulerFromQuat(c.Q, "YXZ")
	o.Set("x", c.Pos.X)
	o.Set("y", c.Pos.Z)
	o.Set("z", c.Pos.Y)
	o.Set("rotation", e.Y)
	o.Set("tilt", e.X)
	o.Set("roll", e.Z)
}

func (c Camera) Direction() Vec3 { return c.Q.Rotate(Vec3{0, 0, -1}) }

// FocalLength is PerspectiveCamera.getFocalLength (35 mm film gauge).
func FocalLength(fov, aspect float64) float64 {
	film := 35 / math.Max(aspect, 1)
	return 0.5 * film / math.Tan(Rad(fov)/2)
}

// FovForLens is PerspectiveCamera.setFocalLength.
func FovForLens(mm, aspect float64) float64 {
	film := 35 / math.Max(aspect, 1)
	return Deg(2 * math.Atan(0.5*film/mm))
}

// Visible reports whether p is inside the camera frustum.
func (c Camera) Visible(p Vec3) bool {
	inv := Compose(c.Pos, c.Q, Vec3{1, 1, 1}).Inverse()
	l := inv.Apply(p)
	depth := -l.Z
	if depth < 0.1 || depth > 1000 {
		return false
	}
	ty := math.Tan(Rad(c.Fov) / 2)
	return math.Abs(l.Y/depth) <= ty && math.Abs(l.X/depth) <= ty*c.Aspect
}

// MoveRelative is CameraControls.getMovedState: right/forward in the camera's heading.
func MoveRelative(o *ojson.Object, right, forward float64) {
	x, y, r := o.NumOr("x", 0), o.NumOr("y", 0), o.NumOr("rotation", 0)
	a := -r
	dx, dy := right, -forward
	o.Set("x", x+dx*math.Cos(a)-dy*math.Sin(a))
	o.Set("y", y+dx*math.Sin(a)+dy*math.Cos(a))
}

// Orbit rotates the camera position and heading around a vertical axis through target.
func Orbit(o *ojson.Object, target Vec3, deg float64) {
	c := CameraFromObject(o, 1)
	q := AxisAngle(Vec3{0, 1, 0}, Rad(deg))
	c.Pos = target.Add(q.Rotate(c.Pos.Sub(target)))
	c.Q = q.Mul(c.Q)
	c.Apply(o)
}

// DollyZoom changes the fov while moving along the view axis so an object at
// target keeps its size on screen.
func DollyZoom(o *ojson.Object, target Vec3, fov float64) {
	c := CameraFromObject(o, 1)
	d := c.Pos.Dist(target)
	nd := d * math.Tan(Rad(c.Fov)/2) / math.Tan(Rad(fov)/2)
	c.Pos = c.Pos.Add(c.Direction().Mul(d - nd))
	c.Apply(o)
	o.Set("fov", fov)
}

// Cameras lists camera objects in scene order.
func (d *Doc) Cameras() []*ojson.Object {
	var out []*ojson.Object
	for _, id := range d.Objects().Keys() {
		if o := d.Objects().Obj(id); o != nil && o.Str("type") == "camera" {
			out = append(out, o)
		}
	}
	return out
}

// Activate sets the active camera by id or by 1-based index.
func (d *Doc) Activate(key string) (*ojson.Object, error) {
	cams := d.Cameras()
	var n int
	if _, err := fmt.Sscan(key, &n); err == nil && n >= 1 && n <= len(cams) && len(key) <= 2 {
		d.Data.Set("activeCamera", cams[n-1].Str("id"))
		return cams[n-1], nil
	}
	o, err := d.ObjOfType(key, "camera")
	if err != nil {
		return nil, err
	}
	d.Data.Set("activeCamera", o.Str("id"))
	return o, nil
}

// ---- framing (shot-generator/utils/cameraUtils.js) ----

// Shot sizes and angles, with the names the app uses.
var (
	ShotSizes  = []string{"Extremely close up", "Very close up", "Close up", "Medium close up", "Bust", "Medium", "Medium long", "Long", "Extremely long", "Establishing", "OTS Left", "OTS Right"}
	ShotAngles = []string{"Birds", "High", "Eye", "Low", "Worms"}
)

var angleRad = map[string]float64{"Birds": Rad(-30), "High": Rad(-15), "Eye": 0, "Low": Rad(30), "Worms": Rad(45)}

type sizeInfo struct {
	bones      []string
	yReduction []float64
	relative   float64
	backSide   bool
	pan        float64
}

var head3 = []string{"Head", "leaf", "Neck"}

var sizeInfos = map[string]sizeInfo{
	"Extremely close up": {bones: head3, yReduction: []float64{0.08, 0}},
	"Very close up":      {bones: []string{"Head", "Neck", "leaf"}, yReduction: []float64{0.02, 0}},
	"Close up":           {bones: []string{"Head", "Neck", "leaf"}, yReduction: []float64{-0.02, -0.04}},
	"Medium close up":    {bones: []string{"Head", "Neck", "leaf", "RightShoulder", "LeftShoulder", "Spine2"}, yReduction: []float64{0, -0.04}},
	"Bust":               {bones: []string{"Head", "Neck", "leaf", "RightShoulder", "LeftShoulder", "Spine1"}, yReduction: []float64{0, -0.04}},
	"Medium":             {bones: []string{"Head", "Neck", "leaf", "RightShoulder", "LeftShoulder", "Hips"}, yReduction: []float64{0, -0.04}},
	"Medium long":        {bones: []string{"Head", "Neck", "leaf", "RightShoulder", "LeftShoulder", "Hips", "LeftUpLeg", "RightUpLeg", "LeftLeg", "RightLeg"}, yReduction: []float64{0, -0.04}},
	"Long":               {bones: []string{"Head", "Neck", "leaf", "RightShoulder", "LeftShoulder", "Hips", "LeftUpLeg", "RightUpLeg", "LeftLeg", "RightLeg", "leaf011", "leaf012"}, yReduction: []float64{0, -0.04}},
	"Extremely long":     {bones: []string{"Head", "Neck", "leaf", "RightShoulder", "LeftShoulder", "Hips", "LeftUpLeg", "RightUpLeg", "LeftLeg", "RightLeg", "leaf011", "leaf012"}, relative: 1.0 / 3.0},
	"OTS Left":           {bones: []string{"Head", "Neck", "leaf", "LeftShoulder"}, backSide: true, pan: 0.5},
	"OTS Right":          {bones: []string{"Head", "Neck", "leaf", "RightShoulder"}, backSide: true, pan: -0.5},
}

var sizeAliases = map[string]string{
	"ecu": "Extremely close up", "extreme-close-up": "Extremely close up", "vcu": "Very close up", "very-close-up": "Very close up",
	"cu": "Close up", "close-up": "Close up", "mcu": "Medium close up", "medium-close-up": "Medium close up",
	"bust": "Bust", "medium": "Medium", "ms": "Medium", "mls": "Medium long", "medium-long": "Medium long",
	"long": "Long", "ls": "Long", "wide": "Long", "els": "Extremely long", "extreme-long": "Extremely long",
	"establishing": "Establishing", "ots-left": "OTS Left", "ots-right": "OTS Right", "ots": "OTS Left",
}

var angleAliases = map[string]string{"birds-eye": "Birds", "birds": "Birds", "bird": "Birds", "high": "High", "eye": "Eye", "low": "Low", "worms-eye": "Worms", "worms": "Worms", "worm": "Worms"}

// ParseShotSize accepts app names ("Medium close up") or short forms (mcu, close-up).
func ParseShotSize(s string) (string, error) {
	k := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "-"))
	if v, ok := sizeAliases[k]; ok {
		return v, nil
	}
	for _, n := range ShotSizes {
		if strings.EqualFold(n, s) {
			return n, nil
		}
	}
	return "", fmt.Errorf("unknown shot size %q (ecu, vcu, cu, mcu, bust, medium, mls, long, els, establishing, ots-left, ots-right)", s)
}

// ParseShotAngle accepts birds-eye, high, eye, low, worms-eye.
func ParseShotAngle(s string) (string, error) {
	k := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "-"))
	k = strings.ReplaceAll(k, "'", "")
	if v, ok := angleAliases[k]; ok {
		return v, nil
	}
	for _, n := range ShotAngles {
		if strings.EqualFold(n, s) {
			return n, nil
		}
	}
	return "", fmt.Errorf("unknown camera angle %q (birds-eye, high, eye, low, worms-eye)", s)
}

// shotBox is getShotBox: bounds of the chosen bones and their tips.
func shotBox(p *Posed, size string) Box {
	box := EmptyBox()
	info, ok := sizeInfos[size]
	if !ok {
		if i := p.Index("leaf"); i >= 0 {
			box.Expand(p.BonePos(i))
			box.Expand(p.BonePos(i).Add(p.BoneLocalPos(i)))
		}
		return box
	}
	want := map[string]bool{}
	for _, b := range info.bones {
		want[b] = true
	}
	for i, b := range p.Sk.Bones {
		if want[b.Name] {
			box.Expand(p.BonePos(i))
			box.Expand(p.BonePos(i).Add(p.BoneLocalPos(i)))
		}
	}
	if box.Empty() {
		return box
	}
	if info.yReduction != nil {
		box.Min.Y += info.yReduction[0]
		box.Max.Y -= info.yReduction[len(info.yReduction)-1]
	} else if info.relative > 0 {
		h := box.Size().Y / info.relative
		box.Min.Y -= h * 0.5
		box.Max.Y += h * 0.5
	}
	return box
}

// closestCharacter is getClosestCharacter.
func closestCharacter(chars []*Posed, cam Camera) *Posed {
	var best *Posed
	bestDist := math.Inf(1)
	dir := cam.Direction()
	for _, c := range chars {
		dist := cam.Pos.Dist(c.Position())
		angle := c.Direction().Dot(dir)
		if (angle >= 1 || angle <= 0) && dist <= bestDist && cam.Visible(c.Position()) {
			best, bestDist = c, dist
		}
	}
	if best == nil {
		return chars[0]
	}
	return best
}

// SetShot is cameraUtils setShot: frame `selected` (or the closest character) for a
// shot size and/or angle. It returns the framed box.
func SetShot(cam *Camera, chars []*Posed, selected *Posed, size, angle string) Box {
	if selected == nil {
		selected = closestCharacter(chars, *cam)
	}
	direction := cam.Direction().Mul(-1)
	box := shotBox(selected, size)
	var pos, target Vec3
	info, known := sizeInfos[size]
	switch {
	case size == "Establishing":
		for _, c := range chars {
			mb := c.MeshBox()
			box.Expand(mb.Min)
			box.Expand(mb.Max)
		}
		if len(chars) > 1 {
			sum := Vec3{}
			for i := 0; i+1 < len(chars); i += 2 {
				sum = sum.Add(chars[i+1].Position().Sub(chars[i].Position()))
			}
			sum = sum.Mul(1 / float64(len(chars)))
			direction = cam.Pos.Sub(sum)
			// the original assigns camera.y (undefined); the camera height is used here
			direction.Y = cam.Pos.Y
		}
		fallthrough
	case known:
		if info.backSide {
			direction = direction.Mul(-1)
		}
		center := box.Center()
		radius := box.Size().Len() / 2
		h := radius / math.Tan(Rad(cam.Fov)/2)
		pos = center.Add(direction.SetLen(h))
		target = center
		if info.pan != 0 {
			panV := direction.Cross(Vec3{0, 1, 0})
			pos = pos.Add(panV.SetLen(info.pan))
			target = target.Sub(direction.SetLen(math.Abs(info.pan)))
		}
		direction = target.Sub(pos).Norm()
	default:
		pos = cam.Pos
		target = box.Center()
		direction = direction.Mul(-1)
	}
	if a, ok := angleRad[angle]; ok {
		dist := pos.Dist(target)
		direction.Y = 0
		direction = direction.Norm()
		axis := Vec3{0, 1, 0}.Cross(direction)
		direction = AxisAngle(axis, -a).Rotate(direction).SetLen(dist)
		pos = target.Sub(direction)
	}
	if pos.Y < 0 {
		flat := Vec3{direction.X, 0, direction.Z}
		pos = pos.Sub(flat.SetLen(pos.Y))
		pos.Y = 0
	}
	cam.Pos = pos
	cam.Q = LookAtQuat(pos, target, true)
	return box
}

// PosedCharacters poses every built-in (or loadable custom) character.
func (d *Doc) PosedCharacters(builtinOnly bool) ([]*Posed, error) {
	var out []*Posed
	for _, id := range d.Objects().Keys() {
		o := d.Objects().Obj(id)
		if o == nil || o.Str("type") != "character" {
			continue
		}
		if builtinOnly && IsCustom(o.Str("model")) {
			continue
		}
		sk, err := ModelSkeleton(o.Str("model"), d.ProjectDir)
		if err != nil {
			return nil, err
		}
		out = append(out, Pose(o, sk))
	}
	return out, nil
}

// Frame points a camera at a character by shot size and/or angle.
func (d *Doc) Frame(camObj *ojson.Object, charID, size, angle string, aspect float64) error {
	chars, err := d.PosedCharacters(false)
	if err != nil {
		return err
	}
	if len(chars) == 0 {
		return fmt.Errorf("the scene has no characters to frame")
	}
	var sel *Posed
	if charID != "" {
		o, err := d.ObjOfType(charID, "character")
		if err != nil {
			return err
		}
		for _, c := range chars {
			if c.Char == o {
				sel = c
			}
		}
	}
	cam := CameraFromObject(camObj, aspect)
	SetShot(&cam, chars, sel, size, angle)
	if camObj.Bool("locked") {
		return ErrLocked{camObj.Str("displayName")}
	}
	cam.Apply(camObj)
	return nil
}
