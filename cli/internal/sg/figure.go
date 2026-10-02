package sg

import (
	"math"
	"strings"

	"sb/internal/ojson"
)

// Figure is a posed character's main joints projected orthographically to 2D:
// meters, x to the right, y down, origin on the floor under the character.
type Figure struct {
	Joints map[string][2]float64 // by bone name
	Depth  map[string]float64    // toward the viewer (larger = nearer)
	Bones  [][2]string           // parent, child
	Head   [2]string             // Head bone and its tip ("leaf"): the head ellipse's axis
	Eyes   []string
	Height float64 // the model's standing height (m)
}

// figureBone reports whether a bone belongs to the stick figure: body,
// shoulders to hands (the middle finger base ends the hand), legs to toe tips.
func figureBone(name, parent string) bool {
	side := strings.TrimPrefix(strings.TrimPrefix(name, "Left"), "Right")
	switch side {
	case "Hips", "Spine", "Spine1", "Spine2", "Neck", "Head", "Shoulder", "Arm", "ForeArm", "Hand", "HandMiddle1", "UpLeg", "Leg", "Foot", "ToeBase":
		return true
	}
	return strings.HasPrefix(name, "leaf") && strings.HasSuffix(parent, "ToeBase")
}

// PoseFigure runs forward kinematics for a pose preset (its state.skeleton bone
// rotations over the model's bind pose, like Character.js) and projects the
// joints for a viewer turned yaw degrees around the character: 0 = front,
// 90 = the character's left side, 180 = back.
func PoseFigure(model string, preset *ojson.Object, yaw float64) (*Figure, error) {
	sk, err := ModelSkeleton(model, "")
	if err != nil {
		return nil, err
	}
	var skel *ojson.Object
	if st := preset.Obj("state"); st != nil {
		skel = st.Obj("skeleton")
	}
	if skel == nil {
		skel = ojson.New()
	}
	p := Pose(ojson.Obj("height", sk.OriginalHeight, "skeleton", skel), sk)
	turn := AxisAngle(Vec3{0, 1, 0}, -yaw*math.Pi/180)
	f := &Figure{Joints: map[string][2]float64{}, Depth: map[string]float64{}, Height: sk.OriginalHeight}
	add := func(i int) {
		v := turn.Rotate(p.BonePos(i))
		f.Joints[sk.Bones[i].Name] = [2]float64{v.X, -v.Y}
		f.Depth[sk.Bones[i].Name] = v.Z
	}
	for i, b := range sk.Bones {
		parent := ""
		if b.Parent >= 0 {
			parent = sk.Bones[b.Parent].Name
		}
		switch {
		case figureBone(b.Name, parent):
			add(i)
			if parent != "" && figureBone(parent, "") {
				f.Bones = append(f.Bones, [2]string{parent, b.Name})
			}
		case parent == "Head" && strings.HasPrefix(b.Name, "leaf"):
			add(i)
			f.Head = [2]string{"Head", b.Name}
		case strings.HasSuffix(b.Name, "Eye"):
			add(i)
			f.Eyes = append(f.Eyes, b.Name)
		}
	}
	return f, nil
}
