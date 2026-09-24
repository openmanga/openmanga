package sg

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"sb/internal/ojson"
)

//go:embed data/skeletons.json
var skeletonsJSON []byte

var (
	skeletonsOnce sync.Once
	skeletons     map[string]*Skeleton
)

// ModelSkeleton returns the skeleton of a built-in character, or parses a
// custom .glb relative to the project folder.
func ModelSkeleton(model, projectDir string) (*Skeleton, error) {
	skeletonsOnce.Do(func() {
		if err := json.Unmarshal(skeletonsJSON, &skeletons); err != nil {
			panic(err)
		}
	})
	if sk, ok := skeletons[model]; ok {
		return sk, nil
	}
	if IsCustom(model) {
		p := model
		if !filepath.IsAbs(p) {
			p = filepath.Join(projectDir, filepath.FromSlash(model))
		}
		sk, err := ParseSkeleton(p)
		if err != nil {
			return nil, err
		}
		sk.OriginalHeight = 1 // Character.js uses 1 for user models
		return sk, nil
	}
	return nil, fmt.Errorf("unknown character model %q", model)
}

// Posed is a character's skeleton in world space (three.js coordinates).
type Posed struct {
	Char   *ojson.Object
	Sk     *Skeleton
	Group  Mat4 // character group: position, Y rotation, body scale
	World  []Mat4
	Rot    []Quat // local rotation per bone as posed
	byName map[string]int
}

func (p *Posed) Index(name string) int {
	if i, ok := p.byName[name]; ok {
		return i
	}
	return -1
}

// Position is the character root in three.js space (x, z, y of the scene object).
func (p *Posed) Position() Vec3 { return p.Group.Pos() }

// Direction is the character's facing (+Z of its group).
func (p *Posed) Direction() Vec3 {
	r := p.Char.NumOr("rotation", 0)
	return AxisAngle(Vec3{0, 1, 0}, r).Rotate(Vec3{0, 0, 1})
}

// BonePos is a bone's world position.
func (p *Posed) BonePos(i int) Vec3 { return p.World[i].Pos() }

// BoneLocalPos is bone.position (local translation).
func (p *Posed) BoneLocalPos(i int) Vec3 {
	t := p.Sk.Bones[i].T
	return Vec3{t[0], t[1], t[2]}
}

func rotationOf(o *ojson.Object, name string) (Quat, bool) {
	if o == nil {
		return Quat{}, false
	}
	b := o.Obj(name)
	if b == nil {
		return Quat{}, false
	}
	r := b.Obj("rotation")
	if r == nil {
		return Quat{}, false
	}
	return Euler{r.NumOr("x", 0), r.NumOr("y", 0), r.NumOr("z", 0), "XYZ"}.Quat(), true
}

// Pose computes world bone matrices like Character.js: bind pose, user bone
// rotations (skeleton, then handSkeleton), head scale, body scale from height.
func Pose(ch *ojson.Object, sk *Skeleton) *Posed {
	h := ch.NumOr("height", 1.8)
	scale := h / sk.OriginalHeight
	if sk.OriginalHeight == 0 {
		scale = 1
	}
	group := Compose(Vec3{ch.NumOr("x", 0), ch.NumOr("z", 0), ch.NumOr("y", 0)}, AxisAngle(Vec3{0, 1, 0}, ch.NumOr("rotation", 0)), Vec3{scale, scale, scale})
	arm := Compose(Vec3{sk.ArmatureT[0], sk.ArmatureT[1], sk.ArmatureT[2]}, Quat{sk.ArmatureR[0], sk.ArmatureR[1], sk.ArmatureR[2], sk.ArmatureR[3]}, Vec3{sk.ArmatureS[0], sk.ArmatureS[1], sk.ArmatureS[2]})
	root := group.Mul(arm)
	p := &Posed{Char: ch, Sk: sk, Group: group, World: make([]Mat4, len(sk.Bones)), Rot: make([]Quat, len(sk.Bones)), byName: map[string]int{}}
	skel, hands := ch.Obj("skeleton"), ch.Obj("handSkeleton")
	head := ch.NumOr("headScale", 1)
	for i, b := range sk.Bones {
		p.byName[b.Name] = i
		r := Quat{b.R[0], b.R[1], b.R[2], b.R[3]}
		if q, ok := rotationOf(skel, b.Name); ok {
			r = q
		}
		if q, ok := rotationOf(hands, b.Name); ok {
			r = q
		}
		p.Rot[i] = r
		s := Vec3{b.S[0], b.S[1], b.S[2]}
		if b.Name == "Head" {
			s = Vec3{head, head, head}
		}
		local := Compose(Vec3{b.T[0], b.T[1], b.T[2]}, r, s)
		if b.Parent >= 0 {
			p.World[i] = p.World[b.Parent].Mul(local)
		} else {
			p.World[i] = root.Mul(local)
		}
	}
	return p
}

// Box is an axis-aligned bounding box.
type Box struct{ Min, Max Vec3 }

func EmptyBox() Box {
	inf := 1e308
	return Box{Vec3{inf, inf, inf}, Vec3{-inf, -inf, -inf}}
}

func (b *Box) Expand(p Vec3) {
	b.Min = Vec3{min(b.Min.X, p.X), min(b.Min.Y, p.Y), min(b.Min.Z, p.Z)}
	b.Max = Vec3{max(b.Max.X, p.X), max(b.Max.Y, p.Y), max(b.Max.Z, p.Z)}
}

func (b Box) Center() Vec3 { return b.Min.Add(b.Max).Mul(0.5) }
func (b Box) Size() Vec3   { return b.Max.Sub(b.Min) }
func (b Box) Empty() bool  { return b.Max.X < b.Min.X }

// MeshBox is the character's mesh bounds in world space (corners of the model box).
func (p *Posed) MeshBox() Box {
	box := EmptyBox()
	lo, hi := p.Sk.BoxMin, p.Sk.BoxMax
	for _, x := range []float64{lo[0], hi[0]} {
		for _, y := range []float64{lo[1], hi[1]} {
			for _, z := range []float64{lo[2], hi[2]} {
				box.Expand(p.Group.Apply(Vec3{x, y, z}))
			}
		}
	}
	return box
}

// BoneRotations returns every bone's current local rotation as Euler XYZ (for `sg bone list`).
func (p *Posed) BoneRotations() []map[string]any {
	var out []map[string]any
	for i, b := range p.Sk.Bones {
		e := EulerFromQuat(p.Rot[i], "XYZ")
		out = append(out, map[string]any{"name": b.Name, "x": e.X, "y": e.Y, "z": e.Z, "modified": p.Char.Obj("skeleton") != nil && p.Char.Obj("skeleton").Has(b.Name)})
	}
	return out
}
