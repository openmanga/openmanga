package sg

import (
	"math"
	"testing"
)

func TestPoseFigureForwardKinematics(t *testing.T) {
	stand, err := FindPreset(KindPoses, DefaultPoseID)
	if err != nil {
		t.Fatal(err)
	}
	f, err := PoseFigure("adult-male", stand, 0)
	if err != nil {
		t.Fatal(err)
	}
	j := f.Joints
	// upright: head tip near -height, feet on the floor (y down, floor 0)
	if tip := j[f.Head[1]][1]; math.Abs(tip+f.Height) > 0.2 {
		t.Errorf("head tip at %.2f, height %.2f", tip, f.Height)
	}
	if foot := j["LeftFoot"][1]; foot > 0 || foot < -0.2 {
		t.Errorf("left foot at %.2f", foot)
	}
	// front view: the character's left is on the viewer's right
	if j["LeftHand"][0] <= 0 || j["RightHand"][0] >= 0 {
		t.Errorf("hands: left %v right %v", j["LeftHand"], j["RightHand"])
	}
	// the preset's arm rotations are applied: standing, the hands hang lower
	// than in the bind pose (arms raised outward)
	def, err := FindPreset(KindPoses, "Default Pose")
	if err != nil {
		t.Fatal(err)
	}
	bind, _ := PoseFigure("adult-male", def, 0)
	drop := func(f *Figure) float64 { return f.Joints["LeftHand"][1] - f.Joints["LeftArm"][1] }
	if drop(f)-drop(bind) < 0.1 {
		t.Errorf("hand below shoulder: stand %.2f, bind pose %.2f", drop(f), drop(bind))
	}
	// side view: the shoulders line up in depth, not across the screen
	side, _ := PoseFigure("adult-male", stand, 90)
	front := j["LeftArm"][0] - j["RightArm"][0]
	if across := math.Abs(side.Joints["LeftArm"][0] - side.Joints["RightArm"][0]); front < 0.2 || across > 0.05 {
		t.Errorf("shoulder spread: front %.2f, side %.2f", front, across)
	}
}
