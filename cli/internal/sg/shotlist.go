package sg

import (
	"fmt"
	"math"

	"sb/internal/ojson"
	"sb/internal/story"
)

// Setup is one camera setup of the shot list (models/shot-list.js).
type Setup struct {
	Number int      `json:"number"`
	Fov    string   `json:"fov"`
	Height string   `json:"height"`
	Shots  []string `json:"shots"`
}

// ListShot is one shot of a setup, with following boards as beats.
type ListShot struct {
	SetupNumber int              `json:"setupNumber"`
	Number      any              `json:"number"`
	UID         string           `json:"uid"`
	Duration    any              `json:"duration,omitempty"`
	Fov         string           `json:"fov,omitempty"`
	X           string           `json:"x,omitempty"`
	Y           string           `json:"y,omitempty"`
	Height      string           `json:"height,omitempty"`
	Rotation    string           `json:"rotation,omitempty"`
	Tilt        string           `json:"tilt,omitempty"`
	Roll        string           `json:"roll,omitempty"`
	Beats       []map[string]any `json:"beats"`
}

// SceneShotList is getShotListForScene.
type SceneShotList struct {
	Setups []Setup      `json:"setups"`
	Shots  [][]ListShot `json:"shots"`
}

type setup struct {
	number int
	camera *ojson.Object
	shots  []*ojson.Object
}

func toFixed(v float64, n int) string { return fmt.Sprintf("%.*f", n, v) }

func signed(s string) string {
	var f float64
	fmt.Sscan(s, &f)
	if f > 0 {
		return "+" + s
	}
	return s
}

func cameraSetups(s *story.Scene) []*setup {
	tolRot, tolPos := Rad(60), 6.0
	near := func(a, b float64, tol float64) bool { return math.Abs(a-b) < tol }
	var setups []*setup
	for _, b := range s.Boards() {
		data := b.ObjPath(false, "sg", "data")
		if data == nil {
			continue
		}
		count := 0
		objs := Objects(data)
		for _, id := range objs.Keys() {
			cam := objs.Obj(id)
			if cam == nil || cam.Str("type") != "camera" {
				continue
			}
			var match *setup
			for _, st := range setups {
				c := st.camera
				if near(c.NumOr("roll", 0), cam.NumOr("roll", 0), tolRot) && near(c.NumOr("rotation", 0), cam.NumOr("rotation", 0), tolRot) &&
					near(c.NumOr("tilt", 0), cam.NumOr("tilt", 0), tolRot) && near(c.NumOr("x", 0), cam.NumOr("x", 0), tolPos) &&
					near(c.NumOr("y", 0), cam.NumOr("y", 0), tolPos) && near(c.NumOr("z", 0), cam.NumOr("z", 0), tolPos) {
					match = st
				}
			}
			if match != nil {
				match.shots = append(match.shots, b)
			} else {
				count++
				setups = append(setups, &setup{number: count, camera: cam, shots: []*ojson.Object{b}})
			}
		}
	}
	return setups
}

// ShotListForScene groups boards into camera setups with lens, height and angles.
func ShotListForScene(s *story.Scene) SceneShotList {
	story.UpdateTiming(s)
	aspect := s.AspectRatio()
	setups := cameraSetups(s)
	out := SceneShotList{Setups: []Setup{}, Shots: [][]ListShot{}}
	for n, st := range setups {
		ids := []string{}
		for _, b := range st.shots {
			ids = append(ids, story.UID(b))
		}
		out.Setups = append(out.Setups, Setup{Number: st.number, Fov: toFixed(FocalLength(st.camera.NumOr("fov", 22.25), aspect), 0) + "mm", Height: toFixed(st.camera.NumOr("z", 0), 2) + "m", Shots: ids})

		var values []ListShot
		orig := st.camera
		for _, b := range st.shots {
			if len(values) == 0 {
				values = append(values, ListShot{
					SetupNumber: n + 1, Number: b.Get("number"), UID: story.UID(b), Duration: b.Get("duration"),
					Fov: toFixed(FocalLength(orig.NumOr("fov", 22.25), aspect), 0) + "mm",
					X:   toFixed(orig.NumOr("x", 0), 2) + "m", Y: toFixed(orig.NumOr("y", 0), 2) + "m", Height: toFixed(orig.NumOr("z", 0), 2) + "m",
					Rotation: toFixed(Deg(orig.NumOr("rotation", 0)), 2) + "°", Tilt: toFixed(Deg(orig.NumOr("tilt", 0)), 2) + "°", Roll: toFixed(Deg(orig.NumOr("roll", 0)), 2) + "°",
					Beats: []map[string]any{},
				})
				continue
			}
			data := b.ObjPath(false, "sg", "data")
			cur := Objects(data).Obj(Active(data))
			beat := map[string]any{"uid": story.UID(b), "dialogue": b.Get("dialogue"), "action": b.Get("action"), "notes": b.Get("notes"), "number": b.Get("number")}
			if cur != nil {
				diff := map[string]string{}
				for _, k := range []string{"x", "y", "z", "rotation", "tilt", "roll"} {
					dv := orig.NumOr(k, 0) - cur.NumOr(k, 0)
					if dv == 0 || toFixed(dv, 4) == "0.0000" || toFixed(dv, 4) == "-0.0000" {
						continue
					}
					switch k {
					case "x", "y":
						diff[k] = signed(toFixed(dv, 2)) + "m"
					case "z":
						diff["height"] = signed(toFixed(dv, 2)) + "m"
					default:
						diff[k] = signed(toFixed(Deg(dv), 2)) + "°"
					}
				}
				if len(diff) > 0 {
					beat["camera"] = diff
				}
			}
			values[len(values)-1].Beats = append(values[len(values)-1].Beats, beat)
		}
		out.Shots = append(out.Shots, values)
	}
	return out
}
