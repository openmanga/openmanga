package sg

import (
	"fmt"
	"math"
	"math/rand/v2"

	"sb/internal/ojson"
)

// Shot is one generated camera angle (shot-explorer ShotMaker).
type Shot struct {
	Index       int            `json:"index"`
	Size        string         `json:"size"`
	Angle       string         `json:"angle"`
	CharacterID string         `json:"characterId"`
	Character   string         `json:"character"`
	Lens        float64        `json:"lens"`
	Description string         `json:"description"`
	Camera      map[string]any `json:"camera"`
}

var explorerLenses = []float64{12, 16, 18, 22, 24, 35, 50}
var describeLenses = []float64{12, 16, 18, 22, 24, 35, 50, 85, 100}

// indexIn returns the closest index in a sorted list (subcollider indexIn).
func indexIn(arr []float64, v float64) int {
	j := -1
	for i, a := range arr {
		if a > v {
			j = i
			break
		}
	}
	if j == -1 {
		return len(arr) - 1
	}
	if j == 0 {
		return 0
	}
	if v-arr[j-1] < arr[j]-v {
		return j - 1
	}
	return j
}

func orbitingAngle(r *rand.Rand) float64 {
	n := r.IntN(100)
	switch {
	case n < 40:
		return float64(r.IntN(110)) - 55
	case n < 65:
		return -float64(r.IntN(70)) - 55
	case n < 75:
		a := float64(r.IntN(110)) - 55
		if a >= 0 {
			return 180 - a
		}
		return -(180 + a)
	default:
		return float64(r.IntN(70)) + 55
	}
}

func clampDeg(v, lim float64) float64 {
	if v == lim || v == -lim {
		return v
	}
	c := math.Max(-lim, math.Min(lim, v))
	if c != v {
		return (v - c) - c
	}
	return v
}

func angleBetween(a, b Vec3) float64 {
	c := a.Dot(b) / (a.Len() * b.Len())
	return math.Acos(clamp(c, -1, 1))
}

func rotateLocal(q Quat, axis Vec3, a float64) Quat { return q.Mul(AxisAngle(axis, a)) }

// Explore generates count shots from the active camera, like the Shot Explorer:
// random size, angle, character and lens, then orbit, roll and rule-of-thirds rules.
func (d *Doc) Explore(count int, seed uint64, aspect float64) ([]Shot, error) {
	chars, err := d.PosedCharacters(true)
	if err != nil {
		return nil, err
	}
	if len(chars) == 0 {
		return nil, fmt.Errorf("the Shot Explorer needs at least one built-in character in the scene")
	}
	camObj := d.ActiveCamera()
	if camObj == nil {
		return nil, fmt.Errorf("the scene has no camera")
	}
	r := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
	var shots []Shot
	for i := 0; i < count; i++ {
		cam := CameraFromObject(camObj, aspect)
		angle := ShotAngles[r.IntN(len(ShotAngles))]
		size := ShotSizes[r.IntN(len(ShotSizes)-2)]
		ch := chars[r.IntN(len(chars))]
		mm := explorerLenses[r.IntN(len(explorerLenses))]
		cam.Fov = FovForLens(mm, aspect)
		box := SetShot(&cam, chars, ch, size, angle)
		focused := box.Center()

		// rules (RulesGenerator.js)
		n := r.IntN(100)
		head := ch.Index("Head")
		pts := []Vec3{}
		if head >= 0 {
			pts = append(pts, ch.BonePos(head))
			for j, b := range ch.Sk.Bones {
				if b.Parent == head && len(b.Name) >= 4 && contains(b.Name, "leaf") {
					pts = append(pts, ch.BonePos(j))
				}
			}
		}
		hb := EmptyBox()
		for _, p := range pts {
			hb.Expand(p)
		}
		headCenter := focused
		if !hb.Empty() {
			headCenter = hb.Center()
		}
		orbit := Rad(orbitingAngle(r)) + ch.Char.NumOr("rotation", 0)
		type rule func()
		rules := []rule{func() {
			q := AxisAngle(Vec3{0, 1, 0}, orbit)
			cam.Pos = headCenter.Add(q.Rotate(cam.Pos.Sub(headCenter)))
			cam.Q = q.Mul(cam.Q)
		}}
		if n < 10 {
			roll := Rad(float64(r.IntN(30) + 5))
			rules = append(rules, func() {
				e := EulerFromQuat(cam.Q, "YXZ")
				cam.Q = Euler{e.X, e.Y, roll, "YXZ"}.Quat()
			})
		}
		if n < 70 && head >= 0 {
			_, hq, _ := ch.World[head].Decompose()
			charRot := Deg(EulerFromQuat(hq, "XYZ").Y)
			facing := clampDeg(Deg(orbit)-charRot, 180)
			right := facing >= 0
			rules = append(rules, func() {
				height := 2 * math.Tan(Rad(cam.Fov)/2) * headCenter.Dist(cam.Pos)
				width := height * cam.Aspect
				desired := Vec3{headCenter.X - width/2 + width*2/3, headCenter.Y, headCenter.Z}
				a := angleBetween(focused.Sub(cam.Pos), desired.Sub(cam.Pos))
				if !right {
					a = -a
				}
				cam.Q = rotateLocal(cam.Q, Vec3{0, 1, 0}, a)
			})
		}
		rules = append(rules, func() {
			height := 2 * math.Tan(Rad(cam.Fov)/2) * focused.Dist(cam.Pos)
			desired := Vec3{focused.X, focused.Y - height/2 + height*2/3, focused.Z}
			a := angleBetween(headCenter.Sub(cam.Pos), desired.Sub(cam.Pos))
			if desired.Y > headCenter.Y {
				a = -a
			}
			cam.Q = rotateLocal(cam.Q, Vec3{1, 0, 0}, a)
		})
		if size != "Establishing" {
			for _, apply := range rules {
				apply()
			}
		}

		e := EulerFromQuat(cam.Q, "YXZ")
		lens := describeLenses[indexIn(describeLenses, FocalLength(cam.Fov, aspect))]
		name := ch.Char.Str("displayName")
		shots = append(shots, Shot{
			Index: i + 1, Size: size, Angle: angle, CharacterID: ch.Char.Str("id"), Character: name, Lens: lens,
			Description: fmt.Sprintf("%s, %s on %s, %gmm", size, angle, name, lens),
			Camera:      map[string]any{"x": cam.Pos.X, "y": cam.Pos.Z, "z": cam.Pos.Y, "rotation": e.Y, "tilt": e.X, "roll": e.Z, "fov": cam.Fov},
		})
	}
	return shots, nil
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// AddCameraFromShot inserts a camera from a generated shot and activates it.
func (d *Doc) AddCameraFromShot(s Shot) *ojson.Object {
	c := s.Camera
	o := ojson.Obj("id", "", "type", "camera", "fov", c["fov"], "x", c["x"], "y", c["y"], "z", c["z"], "rotation", c["rotation"], "tilt", c["tilt"], "roll", c["roll"])
	o.Delete("id")
	d.Create(o)
	d.Data.Set("activeCamera", o.Str("id"))
	return o
}
