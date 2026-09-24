package sg

import (
	"math"
	"os"
	"testing"

	"sb/internal/ojson"
	"sb/internal/story"
)

func newDoc() *Doc {
	d := NewDoc(Initial(), "")
	return d
}

func TestCreationDefaults(t *testing.T) {
	d := newDoc()
	ch, err := d.AddCharacter("")
	if err != nil {
		t.Fatal(err)
	}
	if ch.NumOr("height", 0) != 1.8 || ch.Str("model") != "adult-male" || ch.Str("posePresetId") != DefaultPoseID ||
		ch.ObjPath(false, "skeleton", "RightArm", "rotation") == nil || !ch.Bool("visible") || ch.Str("tintColor") != "#000000" {
		t.Errorf("character defaults: %s", ojson.Stringify(ch, ""))
	}
	// 5 m in front of the default camera (at y=6 looking toward -y), on the floor, facing it
	if y := ch.NumOr("y", 0); y < 0.6 || y > 1.4 || ch.NumOr("z", -1) != 0 || math.Abs(ch.NumOr("x", 9)) > 0.31 {
		t.Errorf("character position %v %v %v", ch.Get("x"), ch.Get("y"), ch.Get("z"))
	}
	if r := ch.NumOr("rotation", 9); math.Abs(r) > 0.2 {
		t.Errorf("character should face the camera, rotation %v", r)
	}
	f, _ := d.AddCharacter("adult-female")
	if f.NumOr("height", 0) != 1.65 {
		t.Error("model height")
	}
	l := d.AddLight()
	if l.NumOr("intensity", 0) != 0.8 || l.NumOr("z", 0) != 2 || l.NumOr("penumbra", 0) != 1 || l.NumOr("distance", 0) != 5 {
		t.Errorf("light: %s", ojson.Stringify(l, ""))
	}
	v, _ := d.AddVolume("")
	if v.NumOr("numberOfLayers", 0) != 4 || v.NumOr("color", 0) != 0x777777 || len(v.Arr("volumeImageAttachmentIds")) != 2 {
		t.Errorf("volume: %s", ojson.Stringify(v, ""))
	}
	o, _ := d.AddObject("")
	if o.Str("model") != "box" || o.Obj("rotation") == nil || o.NumOr("width", 0) != 1 {
		t.Errorf("object: %s", ojson.Stringify(o, ""))
	}
	c := d.AddCamera()
	if c.NumOr("x", 0) != -0.91 || Active(d.Data) != c.Str("id") || c.Str("displayName") != "Camera 2" {
		t.Errorf("camera: %s", ojson.Stringify(c, ""))
	}
	if ch.Str("displayName") != "Adult-male 1" || f.Str("displayName") != "Adult-female 1" || o.Str("displayName") != "Box 1" {
		t.Errorf("display names %s %s %s", ch.Str("displayName"), f.Str("displayName"), o.Str("displayName"))
	}
	if _, err := d.AddObject("no-such-model"); err == nil {
		t.Error("unknown model should fail")
	}
}

func TestUpdateRules(t *testing.T) {
	d := newDoc()
	o, _ := d.AddObject("")
	if err := d.Update(o, ojson.Obj("rotation", ojson.Obj("z", 1.5))); err != nil {
		t.Fatal(err)
	}
	if r := o.Obj("rotation"); r.NumOr("z", 0) != 1.5 || !r.Has("y") {
		t.Errorf("object rotation should merge: %s", ojson.Stringify(r, ""))
	}
	d.Update(o, ojson.Obj("locked", true))
	if err := d.Update(o, ojson.Obj("x", 3)); err == nil {
		t.Error("locked object accepted an edit")
	}
	ch, _ := d.AddCharacter("")
	d.Update(ch, ojson.Obj("model", "child"))
	if ch.NumOr("height", 0) != 1.2 {
		t.Errorf("model change should reset height, got %v", ch.Get("height"))
	}
	if err := d.SetBone(ch, "RightArm", 0.1, 0.2, 0.3); err != nil {
		t.Fatal(err)
	}
	if ch.Has("posePresetId") {
		t.Error("changing a bone should drop the pose preset id")
	}
	if err := d.Delete([]string{Active(d.Data)}); err == nil {
		t.Error("active camera must be protected")
	}
	a, err := d.AddAttachable(ch, "glasses", "", "")
	if err != nil || a.Str("bindBone") != "Head" {
		t.Fatalf("attachable: %v", err)
	}
	gid, _ := d.Group([]string{o.Str("id")})
	d.Delete([]string{ch.Str("id")})
	if d.Objects().Has(a.Str("id")) {
		t.Error("deleting a character must delete its attachables")
	}
	d.Update(o, ojson.Obj("locked", false))
	d.Delete([]string{o.Str("id")})
	if d.Objects().Has(gid) {
		t.Error("empty group should be removed")
	}
}

func TestDuplicateAndMirror(t *testing.T) {
	d := newDoc()
	ch, _ := d.AddCharacter("")
	d.Update(ch, ojson.Obj("name", "Bob"))
	ids := d.Duplicate([]string{ch.Str("id")})
	cp := d.Objects().Obj(ids[0])
	if cp.Str("name") != "Bob copy" || cp.NumOr("x", 0) != ch.NumOr("x", 0)+0.5 {
		t.Errorf("duplicate: %s", ojson.Stringify(cp, ""))
	}
	before := ch.ObjPath(false, "skeleton", "RightArm", "rotation").Clone()
	if err := d.MirrorPose(ch); err != nil {
		t.Fatal(err)
	}
	left := ch.ObjPath(false, "skeleton", "LeftArm", "rotation")
	// mirroring flips the sign of y and z of the XYZ euler
	if math.Abs(left.NumOr("x", 0)-before.NumOr("x", 0)) > 1e-9 || math.Abs(left.NumOr("y", 0)+before.NumOr("y", 0)) > 1e-9 {
		t.Errorf("mirror: %s vs %s", ojson.Stringify(left, ""), ojson.Stringify(before, ""))
	}
}

func TestHashMatchesOriginal(t *testing.T) {
	s, err := story.LoadScene("../../testdata/example/example.storyboarder")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := BoardData(s.Boards()[0])
	// value computed with Node from the original getSerializedState + getHash
	if got := Hash(data); got != "/Dzu0A8i1l1UK6kUDdMylHlZRw4=" {
		t.Errorf("hash %s", got)
	}
}

func TestFramingLooksAtHead(t *testing.T) {
	d := newDoc()
	ch, _ := d.AddCharacter("")
	ch.Set("x", 0)
	ch.Set("y", 0)
	ch.Set("rotation", 0)
	cam := d.ActiveCamera()
	for _, size := range []string{"Close up", "Medium", "Long", "Establishing"} {
		if err := d.Frame(cam, ch.Str("id"), size, "Eye", 1.78); err != nil {
			t.Fatal(err)
		}
		c := CameraFromObject(cam, 1.78)
		sk, _ := ModelSkeleton("adult-male", "")
		p := Pose(ch, sk)
		head := p.BonePos(p.Index("Head"))
		if !c.Visible(head) {
			t.Errorf("%s: head not visible from %v", size, c.Pos)
		}
		if c.Pos.Z <= 0 {
			t.Errorf("%s: camera should stay in front of the character, got %v", size, c.Pos)
		}
	}
	// head height sanity: adult male ~1.8 m, head bone well above 1.4 m
	sk, _ := ModelSkeleton("adult-male", "")
	p := Pose(ch, sk)
	if y := p.BonePos(p.Index("Head")).Y; y < 1.4 || y > 1.85 {
		t.Errorf("head height %v", y)
	}
}

func TestExploreIsDeterministic(t *testing.T) {
	d := newDoc()
	d.AddCharacter("")
	a, err := d.Explore(4, 7, 1.78)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := d.Explore(4, 7, 1.78)
	for i := range a {
		if a[i].Description != b[i].Description || a[i].Camera["x"] != b[i].Camera["x"] {
			t.Errorf("shot %d differs", i)
		}
		for _, k := range []string{"x", "y", "z", "rotation", "tilt", "roll", "fov"} {
			if f := a[i].Camera[k].(float64); math.IsNaN(f) {
				t.Errorf("shot %d %s is NaN", i, k)
			}
		}
	}
	if _, err := newDoc().Explore(1, 1, 1.78); err == nil {
		t.Error("explore without characters should fail")
	}
}

func TestPresetsAndHistory(t *testing.T) {
	t.Setenv("SB_USER_DATA", t.TempDir())
	poses, _ := ListPresets(KindPoses, "")
	hands, _ := ListPresets(KindHandPoses, "")
	emo, _ := ListPresets(KindEmotions, "")
	if len(poses) != 342 || len(hands) != 32 || len(emo) != 6 || len(Models()) != 60 {
		t.Errorf("counts: poses %d hands %d emotions %d models %d", len(poses), len(hands), len(emo), len(Models()))
	}
	p := ojson.Obj("id", "X1", "name", "Mine", "keywords", "Mine", "state", ojson.Obj("skeleton", ojson.New()), "priority", 0)
	if err := AddUserPreset(KindPoses, p); err != nil {
		t.Fatal(err)
	}
	if got, _ := FindPreset(KindPoses, "mine"); got == nil || got.Str("id") != "X1" {
		t.Error("user preset lookup")
	}
	if err := DeleteUserPreset(KindPoses, DefaultPoseID); err == nil {
		t.Error("built-in delete must fail")
	}
	h := LoadHistory("/x.storyboarder", "AAAAA")
	a, b := Initial(), Initial()
	b.Set("activeCamera", "other")
	h.Push(a)
	if prev := h.Undo(b); prev == nil || prev.Str("activeCamera") != DefaultCameraID {
		t.Error("undo")
	}
	if next := h.Redo(a); next == nil || next.Str("activeCamera") != "other" {
		t.Error("redo")
	}
	if err := h.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(h.path); err != nil {
		t.Error("history not saved")
	}
}
