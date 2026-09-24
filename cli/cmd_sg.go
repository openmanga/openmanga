package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/sg"
	"sb/internal/story"
)

var boardFlag = "board=board number or uid (default: the board picked with `sg load`)"

// sgCtx is an open 3D scene of one board.
type sgCtx struct {
	s       *story.Scene
	idx     int
	b       *ojson.Object
	doc     *sg.Doc
	existed bool
	before  *ojson.Object
	hist    *sg.History
}

func sgStatePath() string { return filepath.Join(story.UserDataDir(), "sg-current.json") }

func rememberSGBoard(scene, uid string) {
	m := map[string]string{}
	if data, err := os.ReadFile(sgStatePath()); err == nil {
		json.Unmarshal(data, &m)
	}
	m[scene] = uid
	data, _ := json.MarshalIndent(m, "", "  ")
	os.MkdirAll(filepath.Dir(sgStatePath()), 0o755)
	os.WriteFile(sgStatePath(), data, 0o644)
}

func currentSGBoard(scene string) string {
	m := map[string]string{}
	if data, err := os.ReadFile(sgStatePath()); err == nil {
		json.Unmarshal(data, &m)
	}
	return m[scene]
}

// openSG resolves the board (positional ref, --board, or the last `sg load`).
func openSG(c *Ctx, ref string) (*sgCtx, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	if ref == "" {
		ref = c.Flag("board")
	}
	if ref == "" {
		ref = currentSGBoard(s.Path)
	}
	if ref == "" {
		return nil, usagef("which board? pass --board <i> or run `sb sg load <i>` first")
	}
	i, b, err := oneBoard(s, ref)
	if err != nil {
		return nil, err
	}
	data, existed := sg.BoardData(b)
	data = data.Clone()
	return &sgCtx{s: s, idx: i, b: b, doc: sg.NewDoc(data, s.Dir()), existed: existed, before: data.Clone(), hist: sg.LoadHistory(s.Path, story.UID(b))}, nil
}

// commit writes the scene back when it changed, recording undo history.
func (x *sgCtx) commit() error {
	if x.existed && sg.Hash(x.before) == sg.Hash(x.doc.Data) && string(ojson.Stringify(x.before, "")) == string(ojson.Stringify(x.doc.Data, "")) {
		return nil
	}
	x.hist.Push(sg.Serialize(x.before))
	sg.PutBoardData(x.b, x.doc.Data)
	if err := x.s.Save(); err != nil {
		return err
	}
	return x.hist.Save()
}

// sgRun opens the scene, runs fn, and commits the result.
func sgRun(c *Ctx, ref string, fn func(x *sgCtx) (any, error)) (any, error) {
	x, err := openSG(c, ref)
	if err != nil {
		return nil, err
	}
	res, err := fn(x)
	if err != nil {
		return nil, err
	}
	if err := x.commit(); err != nil {
		return nil, err
	}
	return res, nil
}

func objSummary(o *ojson.Object) map[string]any {
	m := map[string]any{"id": o.Str("id"), "type": o.Str("type"), "name": o.Str("displayName")}
	for _, k := range []string{"x", "y", "z"} {
		m[k], _ = o.Num(k)
	}
	switch r := o.Get("rotation").(type) {
	case *ojson.Object:
		m["rotation"] = r
	default:
		m["rotation"], _ = ojson.Num(r)
	}
	for _, k := range []string{"model", "fov", "visible", "locked", "attachToId", "group", "bindBone"} {
		if o.Has(k) {
			m[k] = o.Get(k)
		}
	}
	return m
}

func fmtNum(v any) string {
	f, ok := ojson.Num(v)
	if !ok {
		return "-"
	}
	return strconv.FormatFloat(math.Round(f*100)/100, 'f', -1, 64)
}

// parseAssignments reads key=value pairs; values are JSON-parsed, dotted keys nest.
func parseAssignments(args []string) (*ojson.Object, error) {
	o := ojson.New()
	for _, a := range args {
		k, v, ok := strings.Cut(a, "=")
		if !ok || k == "" {
			return nil, usagef("expected key=value, got %q", a)
		}
		story.PrefsSetPath(o, k, story.ParseValue(v))
	}
	return o, nil
}

func readJSONArg(path string) (any, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	return ojson.Parse(data)
}

func deg(c *Ctx, name string) (float64, bool, error) {
	f, ok, err := c.Float(name)
	return sg.Rad(f), ok, err
}

func argFloat(c *Ctx, i int, what string) (float64, error) {
	if i >= len(c.Pos) {
		return 0, usagef("missing %s", what)
	}
	return parseFloatArg(c.Pos[i], what)
}

func inRange(v, lo, hi float64, what string) error {
	if v < lo || v > hi {
		return usagef("%s must be between %g and %g", what, lo, hi)
	}
	return nil
}

// cameraFor returns --camera or the active camera.
func cameraFor(c *Ctx, d *sg.Doc) (*ojson.Object, error) {
	if id := c.Flag("camera"); id != "" {
		return d.ObjOfType(id, "camera")
	}
	cam := d.ActiveCamera()
	if cam == nil {
		return nil, fmt.Errorf("the scene has no camera")
	}
	return cam, nil
}

// update applies props to one object.
func update(x *sgCtx, id string, types []string, props *ojson.Object) (any, error) {
	var o *ojson.Object
	var err error
	if types == nil {
		o, err = x.doc.Obj(id)
	} else {
		o, err = x.doc.ObjOfType(id, types...)
	}
	if err != nil {
		return nil, err
	}
	if err := x.doc.Update(o, props); err != nil {
		return nil, err
	}
	return o, nil
}

// setter builds "sg <noun> set-<prop> <id> <value>" commands.
func setter(types []string, key string, conv func(float64) float64, lo, hi float64) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		if err := c.Need(2); err != nil {
			return nil, err
		}
		v, err := parseFloatArg(c.Arg(1), key)
		if err != nil {
			return nil, err
		}
		if err := inRange(v, lo, hi, key); err != nil {
			return nil, err
		}
		if conv != nil {
			v = conv(v)
		}
		return sgRun(c, "", func(x *sgCtx) (any, error) { return update(x, c.Arg(0), types, ojson.Obj(key, v)) })
	}
}

// assigner builds "sg <noun> set <id> k=v..." commands.
func assigner(types []string) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		if err := c.Need(2); err != nil {
			return nil, err
		}
		props, err := parseAssignments(c.Pos[1:])
		if err != nil {
			return nil, err
		}
		return sgRun(c, "", func(x *sgCtx) (any, error) {
			o, err := x.doc.Obj(c.Arg(0))
			if err != nil {
				return nil, err
			}
			if types != nil {
				if _, err := x.doc.ObjOfType(c.Arg(0), types...); err != nil {
					return nil, err
				}
			}
			if v, ok := props.Get("visible").(bool); ok {
				x.doc.SetVisible(o, v)
				props.Delete("visible")
			}
			if v := props.Get("visible-to-camera"); v != nil {
				props.Delete("visible-to-camera")
				props.Set("visibleToCam", v)
			}
			if err := x.doc.Update(o, props); err != nil {
				return nil, err
			}
			return o, nil
		})
	}
}

func d2r(v float64) float64 { return sg.Rad(v) }

func init() {
	B := []string{boardFlag}
	obj := []string{"object"}
	light := []string{"light"}
	char := []string{"character"}
	img := []string{"image"}
	register(
		// scene I/O
		&Cmd{Path: "sg load", Args: "<i>", Short: "Show a board's 3D scene (world, cameras, objects) and make it the board later sg commands use", Run: cmdSGLoad},
		&Cmd{Path: "sg reset", Short: "Replace the board's 3D scene with the default one (one camera, ground)", Flags: B, Run: cmdSGReset},
		&Cmd{Path: "sg get", Short: "Print the raw scene JSON (world, sceneObjects, activeCamera)", Flags: B, Run: cmdSGGet},
		&Cmd{Path: "sg replace", Short: "Replace the whole scene with JSON from a file (or - for stdin)", Flags: append([]string{"json=scene JSON file or -"}, B...), Run: cmdSGReplace},
		&Cmd{Path: "sg status", Short: "SHA-1 of the scene vs the last `sg save` (dirty = the saved shot images are stale)", Flags: B, Run: cmdSGStatus},
		&Cmd{Path: "sg save", Short: "Hand-off from the 3D renderer: store the scene and write the shot-generator layer, its thumbnail, camera plot, thumbnail and posterframe",
			Long:  "The UI renders the active camera view (any size; it is fit to the board) and the 900x900 top-down plot,\nthen calls this. Files and board JSON match saveToBoardFromShotGenerator in the original app.",
			Flags: append([]string{"camera-image=PNG/JPG render of the active camera (required)", "plot-image=900x900 PNG camera plot", "json=scene JSON to store instead of the current one"}, B...), Run: cmdSGSave},
		&Cmd{Path: "sg insert-board", Short: "Like sg save, into a new board inserted after --board",
			Flags: append([]string{"camera-image=camera render (required)", "plot-image=900x900 plot", "json=scene JSON (default: the source board's scene)"}, B...), Run: cmdSGInsertBoard},
		&Cmd{Path: "sg undo", Short: "Undo the last 3D edit of the board (50 steps, kept in userData/sg-history)", Flags: B, Run: cmdSGUndoRedo(true)},
		&Cmd{Path: "sg redo", Short: "Redo an undone 3D edit", Flags: B, Run: cmdSGUndoRedo(false)},

		// objects
		&Cmd{Path: "sg list", Args: "[i]", Short: "All scene objects with type, name and transform", Flags: B, Run: cmdSGList},
		&Cmd{Path: "sg camera add", Short: "New camera 0.91 m left of the active one (or from an `sg explore` shot); it becomes active",
			Flags: append([]string{"from-shot=number of a shot from the last `sg explore` of this board"}, B...), Run: cmdSGCameraAdd},
		&Cmd{Path: "sg object add", Short: "1x1x1 box (or a built-in/custom model) 5 m in front of the camera", Flags: append([]string{"model=built-in object id (see sg models list --type object)", "file=custom .glb, copied to models/objects"}, B...), Run: cmdSGAdd("object")},
		&Cmd{Path: "sg character add", Short: "Adult male 1.8 m in the stand pose, 5 m in front of the camera", Flags: append([]string{"model=adult-male, adult-female, teen-male, teen-female, child, baby", "file=custom character .glb"}, B...), Run: cmdSGAdd("character")},
		&Cmd{Path: "sg light add", Short: "Spot light, intensity 0.8, 2 m high", Flags: B, Run: cmdSGAdd("light")},
		&Cmd{Path: "sg volume add", Short: "5x5x5 atmospherics volume (rain by default)", Flags: append([]string{"preset=rain, fog or explosion"}, B...), Run: cmdSGAdd("volume")},
		&Cmd{Path: "sg image add", Short: "Image plane with a placeholder or a file copied to models/images", Flags: append([]string{"file=image file"}, B...), Run: cmdSGAdd("image")},
		&Cmd{Path: "sg objects create", Short: "Create many objects from JSON: an array of scene objects or {id: object}", Flags: append([]string{"json=file or -"}, B...), Run: cmdSGCreateMany},
		&Cmd{Path: "sg delete", Args: "<id..>", Short: "Delete objects (a character's attachables too); the active camera is protected", Flags: B, Run: cmdSGDelete},
		&Cmd{Path: "sg rename", Args: "<id> <name>", Short: "Set an object's name (display names are recomputed)", Flags: B, Run: cmdSGRename},
		&Cmd{Path: "sg set", Args: "<id> <key=value..>", Short: "Set any object property, e.g. visible=false (groups pass it to children), locked=true, x=1.5",
			Long: "Values are JSON (numbers, true/false, \"text\", {..}); dotted keys nest (morphTargets.mesomorphic=0.5).\nObject ids can be the full id, a 4+ character prefix, or the display name (\"Camera 1\").", Flags: B, Run: assigner(nil)},
		&Cmd{Path: "sg lock", Args: "<id..>", Short: "Lock objects (locked objects ignore edits)", Flags: B, Run: cmdSGLock(true)},
		&Cmd{Path: "sg unlock", Args: "<id..>", Short: "Unlock objects", Flags: B, Run: cmdSGLock(false)},
		&Cmd{Path: "sg duplicate", Args: "<id..>", Short: "Copy objects with +0.5 m offset and \" copy\" names (deep for groups)", Flags: B, Run: cmdSGDuplicate},
		&Cmd{Path: "sg group", Args: "<id..>", Short: "Group objects", Flags: B, Run: cmdSGGroup},
		&Cmd{Path: "sg ungroup", Args: "<group-id> [id..]", Short: "Remove objects (default: all) from a group; an empty group is deleted", Flags: B, Run: cmdSGUngroup},
		&Cmd{Path: "sg group merge", Args: "<group-id..>", Short: "Merge groups (and --add objects) into the first group", Flags: append([]string{"add=comma list of object ids to add"}, B...), Run: cmdSGGroupMerge},
		&Cmd{Path: "sg group move", Args: "<group-id>", Short: "Move all children of a group", Flags: append([]string{"dx=m", "dy=m", "dz=m (height)"}, B...), Run: cmdSGGroupMove},
		&Cmd{Path: "sg move", Short: "Batch move: JSON {id: {x, y, z, rotation}}", Flags: append([]string{"json=file or -"}, B...), Run: cmdSGMove},
		&Cmd{Path: "sg drop", Args: "<id..>", Short: "Drop objects/characters onto the ground or a box below (no WebGL; other models are not surfaces)", Flags: B, Run: cmdSGDrop},

		// camera
		&Cmd{Path: "sg camera set", Args: "<id> <key=value..>", Short: "Set camera properties, e.g. x=0 y=6 z=1.5 (z is height, metres)", Flags: B, Run: assigner([]string{"camera"})},
		&Cmd{Path: "sg camera set-pan", Args: "<deg>", Short: "Pan (rotation) -180..180°", Flags: append([]string{"camera=camera id (default: active)"}, B...), Run: cmdSGCamAngle("rotation", -180, 180)},
		&Cmd{Path: "sg camera set-tilt", Args: "<deg>", Short: "Tilt -90..90°", Flags: append([]string{"camera=camera id"}, B...), Run: cmdSGCamAngle("tilt", -90, 90)},
		&Cmd{Path: "sg camera set-roll", Args: "<deg>", Short: "Roll -45..45°", Flags: append([]string{"camera=camera id"}, B...), Run: cmdSGCamAngle("roll", -45, 45)},
		&Cmd{Path: "sg camera set-fov", Args: "<deg>", Short: "Vertical field of view 1..120°", Flags: append([]string{"camera=camera id"}, B...), Run: cmdSGCamFov},
		&Cmd{Path: "sg camera set-lens", Args: "<mm>", Short: "Lens: 12 16 18 22 24 35 50 85 100 120 200 300 500 mm (any value accepted)", Flags: append([]string{"camera=camera id"}, B...), Run: cmdSGCamLens},
		&Cmd{Path: "sg camera move", Short: "Move relative to the camera heading", Flags: append([]string{"forward=m", "right=m", "camera=camera id"}, B...), Run: cmdSGCamMove},
		&Cmd{Path: "sg camera elevate", Args: "<m>", Short: "Raise (or lower) the camera", Flags: append([]string{"camera=camera id"}, B...), Run: cmdSGCamElevate},
		&Cmd{Path: "sg camera orbit", Short: "Orbit around a character/object or a point", Flags: append([]string{"target=object id, or x,y (floor) or x,y,z", "deg=degrees", "camera=camera id"}, B...), Run: cmdSGCamOrbit},
		&Cmd{Path: "sg camera dolly-zoom", Short: "Change fov while moving so the target keeps its size", Flags: append([]string{"target=object id or x,y,z", "fov=new fov in degrees", "camera=camera id"}, B...), Run: cmdSGCamDolly},
		&Cmd{Path: "sg camera activate", Args: "<id|1-9>", Short: "Make a camera active (by id, name or 1-based index)", Flags: B, Run: cmdSGCamActivate},
		&Cmd{Path: "sg camera frame", Short: "Auto-frame a character by shot size and/or camera angle (uses the character skeletons)",
			Long:  "Sizes: ecu, vcu, cu, mcu, bust, medium, mls, long, els, establishing, ots-left, ots-right.\nAngles: birds-eye, high, eye, low, worms-eye. Without --character the closest visible character is used.",
			Flags: append([]string{"size=shot size", "angle=camera angle", "character=character id", "camera=camera id"}, B...), Run: cmdSGCamFrame},

		// characters
		&Cmd{Path: "sg character set", Args: "<id> <key=value..>", Short: "Set character properties (x, y, z, ...)", Flags: B, Run: assigner(char)},
		&Cmd{Path: "sg character set-rotation", Args: "<id> <deg>", Short: "Rotation -180..180°", Flags: B, Run: setter(char, "rotation", d2r, -180, 180)},
		&Cmd{Path: "sg character set-height", Args: "<id> <m>", Short: "Height (adult 1.47–2.13 m, child 1.00–1.38, baby 0.49–0.94; custom models: scale 0.3–3.05)", Flags: B, Run: cmdSGCharHeight},
		&Cmd{Path: "sg character set-head-scale", Args: "<id> <pct>", Short: "Head size 80–120 %", Flags: B, Run: setter(char, "headScale", func(v float64) float64 { return v / 100 }, 80, 120)},
		&Cmd{Path: "sg character set-tint", Args: "<id> <#hex>", Short: "Skin tint color", Flags: B, Run: cmdSGTint(char)},
		&Cmd{Path: "sg character set-morph", Args: "<id> <muscular|skinny|obese> <pct>", Short: "Body shape 0–100 %", Flags: B, Run: cmdSGMorph},
		&Cmd{Path: "sg character set-model", Args: "<id> [model]", Short: "Change model (adult/teen male/female, child, baby) or --file a custom .glb", Flags: append([]string{"file=custom .glb, copied to models/characters"}, B...), Run: cmdSGSetModel(char)},
		&Cmd{Path: "sg character pose", Args: "<id> <pose>", Short: "Apply a pose preset (id or name, see sg poses list)", Flags: B, Run: cmdSGPose},
		&Cmd{Path: "sg character mirror-pose", Args: "<id>", Short: "Mirror the pose left/right", Flags: B, Run: cmdSGMirror},
		&Cmd{Path: "sg character hand-pose", Args: "<id> <preset>", Short: "Apply a hand pose preset", Flags: append([]string{"hand=left, right or both (default both)"}, B...), Run: cmdSGHandPose},
		&Cmd{Path: "sg character emotion", Args: "<id> <emotion|none>", Short: "Face emotion preset (see sg emotions list) or none", Flags: B, Run: cmdSGEmotion},
		&Cmd{Path: "sg character hair", Args: "<id> <default|curly|none>", Short: "Hair attachable, or --file a custom .glb", Flags: append([]string{"file=custom hair .glb"}, B...), Run: cmdSGHair},
		&Cmd{Path: "sg character apply-preset", Args: "<id> <preset>", Short: "Apply a character preset: model, height, head, tint, morphs, attachables", Flags: B, Run: cmdSGApplyCharPreset},
		&Cmd{Path: "sg bone set", Args: "<id> <bone>", Short: "Set a bone rotation in degrees", Flags: append([]string{"x=deg", "y=deg", "z=deg"}, B...), Run: cmdSGBoneSet},
		&Cmd{Path: "sg bone list", Args: "<id>", Short: "Skeleton bone names and current rotations (degrees)", Flags: B, Run: cmdSGBoneList},
		&Cmd{Path: "sg attachable add", Args: "<character-id> [model]", Short: "Bind glasses, mask, moustache, backpack or pistol (or --file with --bone)", Flags: append([]string{"file=custom .glb", "bone=bind bone"}, B...), Run: cmdSGAttachAdd},
		&Cmd{Path: "sg attachable set-bone", Args: "<id> <bone>", Short: "Rebind to another bone", Flags: B, Run: cmdSGAttachBone},
		&Cmd{Path: "sg attachable set-size", Args: "<id> <n>", Short: "Size 0.7–2", Flags: B, Run: setter([]string{"attachable"}, "size", nil, 0.7, 2)},
		&Cmd{Path: "sg attachable set", Args: "<id> <key=value..>", Short: "Offset in bone space: x= y= z=", Flags: B, Run: assigner([]string{"attachable"})},
		&Cmd{Path: "sg attachable remove", Args: "<id>", Short: "Delete an attachable", Flags: B, Run: cmdSGDelete},

		// objects, lights, volumes, images
		&Cmd{Path: "sg object set", Args: "<id> <key=value..>", Short: "Set object properties (x, y, z, ...)", Flags: B, Run: assigner(obj)},
		&Cmd{Path: "sg object set-size", Args: "<id>", Short: "Box: --w --h --d; models: --size (uniform), 0.025–5 m", Flags: append([]string{"w=width", "h=height", "d=depth", "size=uniform size"}, B...), Run: cmdSGObjSize},
		&Cmd{Path: "sg object rotate", Args: "<id>", Short: "Rotation in degrees", Flags: append([]string{"x=deg", "y=deg", "z=deg"}, B...), Run: cmdSGObjRotate},
		&Cmd{Path: "sg object set-tint", Args: "<id> <#hex>", Short: "Object tint color", Flags: B, Run: cmdSGTint(obj)},
		&Cmd{Path: "sg object set-model", Args: "<id> [model]", Short: "One of the 47 built-in models, or --file a custom .glb", Flags: append([]string{"file=custom .glb, copied to models/objects"}, B...), Run: cmdSGSetModel(obj)},
		&Cmd{Path: "sg light set", Args: "<id> <key=value..>", Short: "Set light properties (x, y, z, ...)", Flags: B, Run: assigner(light)},
		&Cmd{Path: "sg light set-intensity", Args: "<id> <v>", Short: "Intensity 0.025–1", Flags: B, Run: setter(light, "intensity", nil, 0.025, 1)},
		&Cmd{Path: "sg light set-angle", Args: "<id> <deg>", Short: "Cone angle 1–90°", Flags: B, Run: setter(light, "angle", d2r, 1, 90)},
		&Cmd{Path: "sg light set-distance", Args: "<id> <m>", Short: "Distance 0.025–100", Flags: B, Run: setter(light, "distance", nil, 0.025, 100)},
		&Cmd{Path: "sg light set-penumbra", Args: "<id> <v>", Short: "Penumbra 0–1", Flags: B, Run: setter(light, "penumbra", nil, 0, 1)},
		&Cmd{Path: "sg light set-decay", Args: "<id> <v>", Short: "Decay 1–2", Flags: B, Run: setter(light, "decay", nil, 1, 2)},
		&Cmd{Path: "sg light set-rotation", Args: "<id> <deg>", Short: "Aim: rotation -180..180°", Flags: B, Run: setter(light, "rotation", d2r, -180, 180)},
		&Cmd{Path: "sg light set-tilt", Args: "<id> <deg>", Short: "Aim: tilt -90..90°", Flags: B, Run: setter(light, "tilt", d2r, -90, 90)},
		&Cmd{Path: "sg volume set", Args: "<id> <key=value..>", Short: "Volume transform and look: x= y= z= width= height= depth= rotation= layers=1-10 opacity= color=<grey 0-1>", Flags: B, Run: cmdSGVolumeSet},
		&Cmd{Path: "sg volume set-preset", Args: "<id> <rain|fog|explosion>", Short: "Built-in volume textures", Flags: B, Run: cmdSGVolumePreset},
		&Cmd{Path: "sg volume set-images", Args: "<id> <files..>", Short: "Custom layer textures, copied to models/volumes", Flags: B, Run: cmdSGVolumeImages},
		&Cmd{Path: "sg image set", Args: "<id> <key=value..>", Short: "Image plane: x= y= z= height= rotation.x=... visible-to-camera=false", Flags: B, Run: assigner(img)},
		&Cmd{Path: "sg image set-file", Args: "<id> <path>", Short: "Replace the texture (copied to models/images)", Flags: B, Run: cmdSGImageFile},
		&Cmd{Path: "sg image set-opacity", Args: "<id> <v>", Short: "Opacity 0.1–1", Flags: B, Run: setter(img, "opacity", nil, 0.1, 1)},

		// world
		&Cmd{Path: "sg world set", Args: "<key=value..>", Short: "Set world properties, e.g. ground=false", Flags: B, Run: cmdSGWorldSet},
		&Cmd{Path: "sg world set-bg", Args: "<0-1>", Short: "Background grey", Flags: B, Run: cmdSGWorldBg},
		&Cmd{Path: "sg world set-shading", Args: "<mode>", Short: "Outline, Wireframe, Flat or Depth", Flags: B, Run: cmdSGWorldShading},
		&Cmd{Path: "sg world set-ambient", Args: "<0-1>", Short: "Ambient light intensity", Flags: B, Run: cmdSGWorldAmbient},
		&Cmd{Path: "sg world set-sun", Short: "Directional light", Flags: append([]string{"intensity=0-1", "rotation=deg", "tilt=deg"}, B...), Run: cmdSGWorldSun},
		&Cmd{Path: "sg world set-fog", Short: "Fog on/off and distance", Flags: append([]string{"visible=true|false", "far=m"}, B...), Run: cmdSGWorldFog},
		&Cmd{Path: "sg room set", Short: "Room: visible, width, length, height (1.83–76.2 m, height up to 12.19)", Flags: append([]string{"visible=true|false", "w=width m", "l=length m", "h=height m"}, B...), Run: cmdSGRoom},
		&Cmd{Path: "sg env set-file", Args: "<glb>", Short: "Environment model (set/location), copied to models/environments", Flags: B, Run: cmdSGEnvFile},
		&Cmd{Path: "sg env set", Args: "<key=value..>", Short: "Environment: x= y= z= scale= rotation= visible= grayscale=", Flags: B, Run: cmdSGEnvSet},

		// presets and libraries
		&Cmd{Path: "sg models list", Short: "Built-in models: characters (6), objects (47), attachables (7)", Flags: []string{"type=character, object or attachable", "q=search words"}, Run: cmdSGModels},
		&Cmd{Path: "sg poses list", Short: "Pose presets (342 built-in plus yours)", Flags: []string{"q=search words"}, Run: cmdSGPresetList(sg.KindPoses)},
		&Cmd{Path: "sg hand-poses list", Short: "Hand pose presets (32 built-in plus yours)", Flags: []string{"q=search words"}, Run: cmdSGPresetList(sg.KindHandPoses)},
		&Cmd{Path: "sg emotions list", Short: "Emotion presets (6 built-in plus yours)", Flags: []string{"q=search words"}, Run: cmdSGPresetList(sg.KindEmotions)},
		&Cmd{Path: "sg preset pose create", Args: "<character-id> <name>", Short: "Save a character's skeleton as a pose preset (no network call)", Flags: B, Run: cmdSGPosePresetCreate},
		&Cmd{Path: "sg preset pose delete", Args: "<id>", Short: "Delete a user pose preset", Run: cmdSGPresetDelete(sg.KindPoses)},
		&Cmd{Path: "sg preset pose rename", Args: "<id> <name>", Short: "Rename a user pose preset", Run: cmdSGPresetRename(sg.KindPoses)},
		&Cmd{Path: "sg preset hand-pose create", Args: "<character-id> <name>", Short: "Save one hand's finger bones as a hand pose preset", Flags: append([]string{"hand=left or right (default right)"}, B...), Run: cmdSGHandPresetCreate},
		&Cmd{Path: "sg preset hand-pose delete", Args: "<id>", Short: "Delete a user hand pose preset", Run: cmdSGPresetDelete(sg.KindHandPoses)},
		&Cmd{Path: "sg preset character create", Args: "<character-id> <name>", Short: "Save a character's look and attachables as a preset", Flags: B, Run: cmdSGCharPresetCreate},
		&Cmd{Path: "sg preset character list", Short: "Character presets", Flags: []string{"q=search words"}, Run: cmdSGPresetList(sg.KindCharacters)},
		&Cmd{Path: "sg preset character delete", Args: "<id>", Short: "Delete a user character preset", Run: cmdSGPresetDelete(sg.KindCharacters)},
		&Cmd{Path: "sg preset emotion create", Args: "<name>", Short: "Face texture preset from an image (its thumbnail needs a 3D render, so none is made)", Flags: []string{"image=PNG face texture (required)", "character=also apply it to this character", boardFlag}, Run: cmdSGEmotionCreate},
		&Cmd{Path: "sg preset emotion delete", Args: "<id>", Short: "Delete a user emotion: removes its files and clears references in this board", Flags: B, Run: cmdSGEmotionDelete},
		&Cmd{Path: "sg preset scene list", Short: "Scene presets", Run: cmdSGPresetList(sg.KindScenes)},
		&Cmd{Path: "sg preset scene create", Args: "<name>", Short: "Save the board's 3D scene as a preset", Flags: B, Run: cmdSGScenePresetCreate},
		&Cmd{Path: "sg preset scene apply", Args: "<id>", Short: "Load a scene preset into the board", Flags: B, Run: cmdSGScenePresetApply},
		&Cmd{Path: "sg preset scene rename", Args: "<id> <name>", Short: "Rename a user scene preset", Run: cmdSGPresetRename(sg.KindScenes)},
		&Cmd{Path: "sg preset scene delete", Args: "<id>", Short: "Delete a user scene preset", Run: cmdSGPresetDelete(sg.KindScenes)},

		// explorer and shot list
		&Cmd{Path: "sg explore", Args: "[i]", Short: "Generate camera angles (size, angle, character, lens + orbit/roll/thirds rules); data only, no renders",
			Flags: append([]string{"count=number of shots (default 9)", "seed=random seed for repeatable results"}, B...), Run: cmdSGExplore},
		&Cmd{Path: "shotlist", Short: "Shot list: boards grouped into camera setups with lens, height and angles", Flags: []string{"all-scenes every scene of a script project"}, Run: cmdShotlist},
	)
}

func cmdSGLoad(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	x, err := openSG(c, c.Arg(0))
	if err != nil {
		return nil, err
	}
	rememberSGBoard(x.s.Path, story.UID(x.b))
	res := sgSummary(c, x)
	res["hasScene"] = x.existed
	if !x.existed {
		c.Printf("(board %d has no 3D scene yet; showing the default scene)\n", x.idx+1)
	}
	return res, nil
}

func sgSummary(c *Ctx, x *sgCtx) map[string]any {
	d := x.doc
	var objs []map[string]any
	c.Printf("Board %d (%s), active camera %s\n", x.idx+1, story.UID(x.b), activeName(d))
	for _, id := range d.Objects().Keys() {
		o := d.Objects().Obj(id)
		if o == nil {
			continue
		}
		objs = append(objs, objSummary(o))
		extra := ""
		if m := o.Str("model"); m != "" {
			extra = " " + m
		}
		if o.Bool("locked") {
			extra += " [locked]"
		}
		if v, ok := o.Get("visible").(bool); ok && !v {
			extra += " [hidden]"
		}
		c.Printf("  %-36s %-10s %-18s x=%s y=%s z=%s%s\n", id, o.Str("type"), o.Str("displayName"), fmtNum(o.Get("x")), fmtNum(o.Get("y")), fmtNum(o.Get("z")), extra)
	}
	return map[string]any{"board": x.idx + 1, "uid": story.UID(x.b), "activeCamera": sg.Active(d.Data), "world": sg.World(d.Data), "objects": objs}
}

func activeName(d *sg.Doc) string {
	if cam := d.ActiveCamera(); cam != nil {
		return cam.Str("displayName")
	}
	return "(none)"
}

func cmdSGList(c *Ctx) (any, error) {
	x, err := openSG(c, c.Arg(0))
	if err != nil {
		return nil, err
	}
	return sgSummary(c, x), nil
}

func cmdSGReset(c *Ctx) (any, error) {
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		x.doc.Data = sg.Initial()
		c.Printf("Reset the 3D scene of board %d", x.idx+1)
		return x.doc.Data, nil
	})
}

func cmdSGGet(c *Ctx) (any, error) {
	x, err := openSG(c, "")
	if err != nil {
		return nil, err
	}
	c.Printf("%s", ojson.Stringify(x.doc.Data, "  "))
	return x.doc.Data, nil
}

func cmdSGReplace(c *Ctx) (any, error) {
	if c.Flag("json") == "" {
		return nil, usagef("--json <file|-> is required")
	}
	v, err := readJSONArg(c.Flag("json"))
	if err != nil {
		return nil, err
	}
	o, ok := v.(*ojson.Object)
	if !ok || o.Obj("sceneObjects") == nil || o.Obj("world") == nil {
		return nil, fmt.Errorf("scene JSON needs world and sceneObjects")
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		x.doc = sg.NewDoc(o, x.s.Dir())
		sg.DisplayNames(sg.Objects(o))
		return map[string]any{"objects": sg.Objects(o).Len()}, nil
	})
}

func cmdSGStatus(c *Ctx) (any, error) {
	x, err := openSG(c, "")
	if err != nil {
		return nil, err
	}
	h := sg.Hash(x.doc.Data)
	dirty := h != x.hist.SavedHash
	c.Printf("hash %s, last save %s, dirty: %v", h, x.hist.SavedHash, dirty)
	return map[string]any{"hash": h, "savedHash": x.hist.SavedHash, "dirty": dirty, "hasScene": x.existed, "undo": len(x.hist.Past), "redo": len(x.hist.Future)}, nil
}

func sceneFromFlag(c *Ctx, def *ojson.Object) (*ojson.Object, error) {
	if c.Flag("json") == "" {
		return def, nil
	}
	v, err := readJSONArg(c.Flag("json"))
	if err != nil {
		return nil, err
	}
	o, ok := v.(*ojson.Object)
	if !ok {
		return nil, fmt.Errorf("--json must be a scene object")
	}
	return o, nil
}

func cmdSGSave(c *Ctx) (any, error) {
	if c.Flag("camera-image") == "" {
		return nil, usagef("--camera-image is required")
	}
	x, err := openSG(c, "")
	if err != nil {
		return nil, err
	}
	data, err := sceneFromFlag(c, x.doc.Data)
	if err != nil {
		return nil, err
	}
	if err := sg.SaveShot(x.s, x.b, data, c.Flag("camera-image"), c.Flag("plot-image")); err != nil {
		return nil, err
	}
	if err := x.s.Save(); err != nil {
		return nil, err
	}
	x.hist.SavedHash = sg.Hash(data)
	x.hist.Save()
	c.Printf("Saved the shot to board %d", x.idx+1)
	return boardRow(x.s, x.idx, x.b), nil
}

func cmdSGInsertBoard(c *Ctx) (any, error) {
	if c.Flag("camera-image") == "" {
		return nil, usagef("--camera-image is required")
	}
	x, err := openSG(c, "")
	if err != nil {
		return nil, err
	}
	data, err := sceneFromFlag(c, x.doc.Data)
	if err != nil {
		return nil, err
	}
	nb, err := story.InsertBlankBoard(x.s, x.idx+1)
	if err != nil {
		return nil, err
	}
	if err := sg.SaveShot(x.s, nb, data, c.Flag("camera-image"), c.Flag("plot-image")); err != nil {
		return nil, err
	}
	if err := x.s.Save(); err != nil {
		return nil, err
	}
	h := sg.LoadHistory(x.s.Path, story.UID(nb))
	h.SavedHash = sg.Hash(data)
	h.Save()
	c.Printf("Inserted board %d with the shot", x.idx+2)
	return boardRow(x.s, x.idx+1, nb), nil
}

func cmdSGUndoRedo(undo bool) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		x, err := openSG(c, "")
		if err != nil {
			return nil, err
		}
		var next *ojson.Object
		if undo {
			next = x.hist.Undo(sg.Serialize(x.doc.Data))
		} else {
			next = x.hist.Redo(sg.Serialize(x.doc.Data))
		}
		if next == nil {
			return nil, fmt.Errorf("nothing to %s", map[bool]string{true: "undo", false: "redo"}[undo])
		}
		sg.PutBoardData(x.b, next)
		if err := x.s.Save(); err != nil {
			return nil, err
		}
		if err := x.hist.Save(); err != nil {
			return nil, err
		}
		c.Printf("%s: %d undo / %d redo steps left", map[bool]string{true: "Undone", false: "Redone"}[undo], len(x.hist.Past), len(x.hist.Future))
		return map[string]any{"undo": len(x.hist.Past), "redo": len(x.hist.Future)}, nil
	}
}

func cmdSGCameraAdd(c *Ctx) (any, error) {
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		if v := c.Flag("from-shot"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > len(x.hist.Explore) {
				return nil, fmt.Errorf("no shot %s: run `sb sg explore` on this board first (%d shots stored)", v, len(x.hist.Explore))
			}
			o := x.doc.AddCameraFromShot(x.hist.Explore[n-1])
			c.Printf("Added %s from shot %d (%s)", o.Str("displayName"), n, x.hist.Explore[n-1].Description)
			return objSummary(o), nil
		}
		o := x.doc.AddCamera()
		c.Printf("Added %s (%s), now active", o.Str("displayName"), o.Str("id"))
		return objSummary(o), nil
	})
}

func cmdSGAdd(kind string) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		return sgRun(c, "", func(x *sgCtx) (any, error) {
			var o *ojson.Object
			var err error
			model := c.Flag("model")
			if f := c.Flag("file"); f != "" && (kind == "object" || kind == "character") {
				if model, err = x.doc.CopyAsset(f, kind+"s"); err != nil {
					return nil, err
				}
			}
			switch kind {
			case "object":
				o, err = x.doc.AddObject(model)
			case "character":
				o, err = x.doc.AddCharacter(model)
			case "light":
				o = x.doc.AddLight()
			case "volume":
				o, err = x.doc.AddVolume(c.Flag("preset"))
			case "image":
				o, err = x.doc.AddImage(c.Flag("file"))
			}
			if err != nil {
				return nil, err
			}
			c.Printf("Added %s (%s)", o.Str("displayName"), o.Str("id"))
			return objSummary(o), nil
		})
	}
}

func cmdSGCreateMany(c *Ctx) (any, error) {
	if c.Flag("json") == "" {
		return nil, usagef("--json <file|-> is required")
	}
	v, err := readJSONArg(c.Flag("json"))
	if err != nil {
		return nil, err
	}
	var list []*ojson.Object
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			if o, ok := e.(*ojson.Object); ok {
				list = append(list, o)
			}
		}
	case *ojson.Object:
		for _, k := range t.Keys() {
			if o := t.Obj(k); o != nil {
				if o.Str("id") == "" {
					o.Set("id", k)
				}
				list = append(list, o)
			}
		}
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		var ids []string
		for _, o := range list {
			if o.Str("type") == "" {
				return nil, fmt.Errorf("every object needs a type")
			}
			ids = append(ids, x.doc.Create(o).Str("id"))
		}
		c.Printf("Created %d objects", len(ids))
		return map[string]any{"ids": ids}, nil
	})
}

func resolveIDs(x *sgCtx, keys []string) ([]string, error) {
	var ids []string
	for _, k := range keys {
		o, err := x.doc.Obj(k)
		if err != nil {
			return nil, err
		}
		ids = append(ids, o.Str("id"))
	}
	return ids, nil
}

func cmdSGDelete(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		ids, err := resolveIDs(x, c.Pos)
		if err != nil {
			return nil, err
		}
		if err := x.doc.Delete(ids); err != nil {
			return nil, err
		}
		c.Printf("Deleted %d object(s)", len(ids))
		return map[string]any{"deleted": ids}, nil
	})
}

func cmdSGRename(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		return update(x, c.Arg(0), nil, ojson.Obj("name", strings.Join(c.Pos[1:], " ")))
	})
}

func cmdSGLock(lock bool) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		if err := c.Need(1); err != nil {
			return nil, err
		}
		return sgRun(c, "", func(x *sgCtx) (any, error) {
			ids, err := resolveIDs(x, c.Pos)
			if err != nil {
				return nil, err
			}
			for _, id := range ids {
				x.doc.Objects().Obj(id).Set("locked", lock)
			}
			return map[string]any{"ids": ids, "locked": lock}, nil
		})
	}
}

func cmdSGDuplicate(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		ids, err := resolveIDs(x, c.Pos)
		if err != nil {
			return nil, err
		}
		n := x.doc.Duplicate(ids)
		c.Printf("Duplicated: %s", strings.Join(n, ", "))
		return map[string]any{"ids": n}, nil
	})
}

func cmdSGGroup(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		ids, err := resolveIDs(x, c.Pos)
		if err != nil {
			return nil, err
		}
		gid, err := x.doc.Group(ids)
		if err != nil {
			return nil, err
		}
		c.Printf("Group %s", gid)
		return map[string]any{"group": gid}, nil
	})
}

func cmdSGUngroup(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		all, err := resolveIDs(x, c.Pos)
		if err != nil {
			return nil, err
		}
		return map[string]any{"group": all[0]}, x.doc.Ungroup(all[0], all[1:])
	})
}

func cmdSGGroupMerge(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		groups, err := resolveIDs(x, c.Pos)
		if err != nil {
			return nil, err
		}
		var add []string
		if v := c.Flag("add"); v != "" {
			if add, err = resolveIDs(x, strings.Split(v, ",")); err != nil {
				return nil, err
			}
		}
		return map[string]any{"group": groups[0]}, x.doc.MergeGroups(groups, add)
	})
}

func cmdSGGroupMove(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	dx, _, err := c.Float("dx")
	if err != nil {
		return nil, err
	}
	dy, _, err := c.Float("dy")
	if err != nil {
		return nil, err
	}
	dz, _, err := c.Float("dz")
	if err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		g, err := x.doc.Obj(c.Arg(0))
		if err != nil {
			return nil, err
		}
		return map[string]any{"group": g.Str("id")}, x.doc.GroupMove(g.Str("id"), dx, dy, dz)
	})
}

func cmdSGMove(c *Ctx) (any, error) {
	if c.Flag("json") == "" {
		return nil, usagef("--json <file|-> is required")
	}
	v, err := readJSONArg(c.Flag("json"))
	if err != nil {
		return nil, err
	}
	o, ok := v.(*ojson.Object)
	if !ok {
		return nil, usagef("--json must be {id: {x, y, z, rotation}}")
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		moved, err := x.doc.BatchMove(o)
		return map[string]any{"moved": moved}, err
	})
}

func cmdSGDrop(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		ids, err := resolveIDs(x, c.Pos)
		if err != nil {
			return nil, err
		}
		z, err := x.doc.Drop(ids)
		return map[string]any{"z": z}, err
	})
}

func cmdSGCamAngle(key string, lo, hi float64) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		v, err := argFloat(c, 0, "degrees")
		if err != nil {
			return nil, err
		}
		if err := inRange(v, lo, hi, key); err != nil {
			return nil, err
		}
		return sgRun(c, "", func(x *sgCtx) (any, error) {
			cam, err := cameraFor(c, x.doc)
			if err != nil {
				return nil, err
			}
			return cam, x.doc.Update(cam, ojson.Obj(key, sg.Rad(v)))
		})
	}
}

func cmdSGCamFov(c *Ctx) (any, error) {
	v, err := argFloat(c, 0, "fov")
	if err != nil {
		return nil, err
	}
	if err := inRange(v, 1, 120, "fov"); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		cam, err := cameraFor(c, x.doc)
		if err != nil {
			return nil, err
		}
		return cam, x.doc.Update(cam, ojson.Obj("fov", v))
	})
}

func cmdSGCamLens(c *Ctx) (any, error) {
	mm, err := argFloat(c, 0, "focal length")
	if err != nil || mm <= 0 {
		return nil, usagef("focal length in mm expected")
	}
	fov := sg.FovForLens(mm, sg.LensAspect)
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		cam, err := cameraFor(c, x.doc)
		if err != nil {
			return nil, err
		}
		c.Printf("%gmm = fov %.2f°", mm, fov)
		return map[string]any{"camera": cam.Str("id"), "fov": fov, "lens": mm}, x.doc.Update(cam, ojson.Obj("fov", fov))
	})
}

func cmdSGCamMove(c *Ctx) (any, error) {
	f, _, err := c.Float("forward")
	if err != nil {
		return nil, err
	}
	r, _, err := c.Float("right")
	if err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		cam, err := cameraFor(c, x.doc)
		if err != nil {
			return nil, err
		}
		if cam.Bool("locked") {
			return nil, sg.ErrLocked{Name: cam.Str("displayName")}
		}
		sg.MoveRelative(cam, r, f)
		return objSummary(cam), nil
	})
}

func cmdSGCamElevate(c *Ctx) (any, error) {
	m, err := argFloat(c, 0, "metres")
	if err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		cam, err := cameraFor(c, x.doc)
		if err != nil {
			return nil, err
		}
		return objSummary(cam), x.doc.Update(cam, ojson.Obj("z", cam.NumOr("z", 0)+m))
	})
}

// targetPoint resolves an object id or "x,y[,z]" into three.js space.
func targetPoint(d *sg.Doc, s string, defHeight float64) (sg.Vec3, error) {
	var x, y, z float64
	if n, _ := fmt.Sscanf(s, "%g,%g,%g", &x, &y, &z); n == 3 {
		return sg.Vec3{X: x, Y: z, Z: y}, nil
	}
	if n, _ := fmt.Sscanf(s, "%g,%g", &x, &y); n == 2 {
		return sg.Vec3{X: x, Y: defHeight, Z: y}, nil
	}
	o, err := d.Obj(s)
	if err != nil {
		return sg.Vec3{}, err
	}
	h := o.NumOr("z", 0)
	if o.Str("type") == "character" {
		h += o.NumOr("height", 1.8) * 0.9 // eye level
	}
	return sg.Vec3{X: o.NumOr("x", 0), Y: h, Z: o.NumOr("y", 0)}, nil
}

func cmdSGCamOrbit(c *Ctx) (any, error) {
	d, ok, err := c.Float("deg")
	if err != nil {
		return nil, err
	}
	if !ok || c.Flag("target") == "" {
		return nil, usagef("--target and --deg are required")
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		cam, err := cameraFor(c, x.doc)
		if err != nil {
			return nil, err
		}
		if cam.Bool("locked") {
			return nil, sg.ErrLocked{Name: cam.Str("displayName")}
		}
		t, err := targetPoint(x.doc, c.Flag("target"), cam.NumOr("z", 0))
		if err != nil {
			return nil, err
		}
		sg.Orbit(cam, t, d)
		return objSummary(cam), nil
	})
}

func cmdSGCamDolly(c *Ctx) (any, error) {
	fov, ok, err := c.Float("fov")
	if err != nil {
		return nil, err
	}
	if !ok || c.Flag("target") == "" {
		return nil, usagef("--target and --fov are required")
	}
	if err := inRange(fov, 1, 120, "fov"); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		cam, err := cameraFor(c, x.doc)
		if err != nil {
			return nil, err
		}
		if cam.Bool("locked") {
			return nil, sg.ErrLocked{Name: cam.Str("displayName")}
		}
		t, err := targetPoint(x.doc, c.Flag("target"), cam.NumOr("z", 0))
		if err != nil {
			return nil, err
		}
		sg.DollyZoom(cam, t, fov)
		return objSummary(cam), nil
	})
}

func cmdSGCamActivate(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		cam, err := x.doc.Activate(c.Arg(0))
		if err != nil {
			return nil, err
		}
		c.Printf("Active camera: %s", cam.Str("displayName"))
		return objSummary(cam), nil
	})
}

func cmdSGCamFrame(c *Ctx) (any, error) {
	size, angle := "", ""
	var err error
	if v := c.Flag("size"); v != "" {
		if size, err = sg.ParseShotSize(v); err != nil {
			return nil, usagef("%v", err)
		}
	}
	if v := c.Flag("angle"); v != "" {
		if angle, err = sg.ParseShotAngle(v); err != nil {
			return nil, usagef("%v", err)
		}
	}
	if size == "" && angle == "" {
		return nil, usagef("give --size and/or --angle")
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		cam, err := cameraFor(c, x.doc)
		if err != nil {
			return nil, err
		}
		if err := x.doc.Frame(cam, c.Flag("character"), size, angle, x.s.AspectRatio()); err != nil {
			return nil, err
		}
		c.Printf("%s framed: x=%s y=%s z=%s pan=%.1f° tilt=%.1f°", cam.Str("displayName"), fmtNum(cam.Get("x")), fmtNum(cam.Get("y")), fmtNum(cam.Get("z")), sg.Deg(cam.NumOr("rotation", 0)), sg.Deg(cam.NumOr("tilt", 0)))
		return objSummary(cam), nil
	})
}

var heightRanges = map[string][2]float64{"child": {1.003, 1.384}, "baby": {0.492, 0.94}}

func cmdSGCharHeight(c *Ctx) (any, error) {
	v, err := argFloat(c, 1, "height")
	if err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		r := [2]float64{1.4732, 2.1336}
		if sg.IsCustom(o.Str("model")) {
			r = [2]float64{0.3, 3.05}
		} else if hr, ok := heightRanges[o.Str("model")]; ok {
			r = hr
		}
		if err := inRange(v, r[0], r[1], "height"); err != nil {
			return nil, err
		}
		return o, x.doc.Update(o, ojson.Obj("height", v))
	})
}

func cmdSGTint(types []string) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		if err := c.Need(2); err != nil {
			return nil, err
		}
		col, err := render.ParseColor(c.Arg(1))
		if err != nil {
			return nil, usagef("%v", err)
		}
		hex := fmt.Sprintf("#%02x%02x%02x", col.R, col.G, col.B)
		return sgRun(c, "", func(x *sgCtx) (any, error) { return update(x, c.Arg(0), types, ojson.Obj("tintColor", hex)) })
	}
}

var morphNames = map[string]string{"muscular": "mesomorphic", "skinny": "ectomorphic", "obese": "endomorphic", "mesomorphic": "mesomorphic", "ectomorphic": "ectomorphic", "endomorphic": "endomorphic"}

func cmdSGMorph(c *Ctx) (any, error) {
	if err := c.Need(3); err != nil {
		return nil, err
	}
	key, ok := morphNames[c.Arg(1)]
	if !ok {
		return nil, usagef("morph must be muscular, skinny or obese")
	}
	v, err := argFloat(c, 2, "percent")
	if err != nil {
		return nil, err
	}
	if err := inRange(v, 0, 100, "percent"); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		if m, ok := sg.FindModel(o.Str("model")); ok {
			valid := false
			for _, t := range m.ValidMorphTargets {
				valid = valid || t == key
			}
			if !valid {
				return nil, fmt.Errorf("the %s model has no %s body shape", m.ID, c.Arg(1))
			}
		}
		return o, x.doc.Update(o, ojson.Obj("morphTargets", ojson.Obj(key, v/100)))
	})
}

func cmdSGSetModel(types []string) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		if err := c.Need(1); err != nil {
			return nil, err
		}
		return sgRun(c, "", func(x *sgCtx) (any, error) {
			o, err := x.doc.ObjOfType(c.Arg(0), types...)
			if err != nil {
				return nil, err
			}
			if f := c.Flag("file"); f != "" {
				return o, x.doc.SetModelFile(o, f)
			}
			m, ok := sg.FindModel(c.Arg(1))
			if !ok || m.Type != types[0] {
				return nil, usagef("unknown %s model %q (see `sb sg models list --type %s`)", types[0], c.Arg(1), types[0])
			}
			return o, x.doc.Update(o, ojson.Obj("model", m.ID))
		})
	}
}

func cmdSGPose(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	p, err := sg.FindPreset(sg.KindPoses, strings.Join(c.Pos[1:], " "))
	if err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		c.Printf("%s: pose %s", o.Str("displayName"), p.Str("name"))
		return map[string]any{"character": o.Str("id"), "pose": p.Str("id"), "name": p.Str("name")}, x.doc.ApplyPose(o, p)
	})
}

func cmdSGMirror(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		return map[string]any{"character": o.Str("id")}, x.doc.MirrorPose(o)
	})
}

func cmdSGHandPose(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	hand := map[string]string{"": "BothHands", "both": "BothHands", "left": "LeftHand", "right": "RightHand"}[c.Flag("hand")]
	if hand == "" {
		return nil, usagef("--hand must be left, right or both")
	}
	p, err := sg.FindPreset(sg.KindHandPoses, strings.Join(c.Pos[1:], " "))
	if err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		return map[string]any{"character": o.Str("id"), "handPose": p.Str("id"), "hand": hand}, x.doc.ApplyHandPose(o, p, hand)
	})
}

func cmdSGEmotion(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	var id any
	if c.Arg(1) != "none" {
		p, err := sg.FindPreset(sg.KindEmotions, strings.Join(c.Pos[1:], " "))
		if err != nil {
			return nil, err
		}
		id = p.Str("id")
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		return update(x, c.Arg(0), []string{"character"}, ojson.Obj("emotionPresetId", id))
	})
}

func cmdSGHair(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		h, err := x.doc.SetHair(o, c.Arg(1), c.Flag("file"))
		if err != nil {
			return nil, err
		}
		if h == nil {
			return map[string]any{"hair": nil}, nil
		}
		return objSummary(h), nil
	})
}

func cmdSGApplyCharPreset(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	p, err := sg.FindPreset(sg.KindCharacters, strings.Join(c.Pos[1:], " "))
	if err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		return o, x.doc.ApplyCharacterPreset(o, p)
	})
}

func cmdSGBoneSet(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		sk, err := sg.ModelSkeleton(o.Str("model"), x.s.Dir())
		if err != nil {
			return nil, err
		}
		p := sg.Pose(o, sk)
		i := p.Index(c.Arg(1))
		if i < 0 {
			return nil, fmt.Errorf("no bone %q (see `sb sg bone list %s`)", c.Arg(1), c.Arg(0))
		}
		e := sg.EulerFromQuat(p.Rot[i], "XYZ")
		vals := []float64{e.X, e.Y, e.Z}
		for k, name := range []string{"x", "y", "z"} {
			v, ok, err := deg(c, name)
			if err != nil {
				return nil, err
			}
			if ok {
				vals[k] = v
			}
		}
		return map[string]any{"bone": c.Arg(1), "x": sg.Deg(vals[0]), "y": sg.Deg(vals[1]), "z": sg.Deg(vals[2])}, x.doc.SetBone(o, c.Arg(1), vals[0], vals[1], vals[2])
	})
}

func cmdSGBoneList(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	x, err := openSG(c, "")
	if err != nil {
		return nil, err
	}
	o, err := x.doc.ObjOfType(c.Arg(0), "character")
	if err != nil {
		return nil, err
	}
	sk, err := sg.ModelSkeleton(o.Str("model"), x.s.Dir())
	if err != nil {
		return nil, err
	}
	rows := sg.Pose(o, sk).BoneRotations()
	for _, r := range rows {
		for _, k := range []string{"x", "y", "z"} {
			r[k] = math.Round(sg.Deg(r[k].(float64))*100) / 100
		}
		mark := ""
		if r["modified"].(bool) {
			mark = " *"
		}
		c.Printf("%-24s %8.2f %8.2f %8.2f%s\n", r["name"], r["x"], r["y"], r["z"], mark)
	}
	return map[string]any{"bones": rows}, nil
}

func cmdSGAttachAdd(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		ch, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		a, err := x.doc.AddAttachable(ch, c.Arg(1), c.Flag("file"), c.Flag("bone"))
		if err != nil {
			return nil, err
		}
		c.Printf("Added %s on %s (%s)", a.Str("displayName"), a.Str("bindBone"), a.Str("id"))
		return objSummary(a), nil
	})
}

func cmdSGAttachBone(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		a, err := x.doc.ObjOfType(c.Arg(0), "attachable")
		if err != nil {
			return nil, err
		}
		if ch := x.doc.Objects().Obj(a.Str("attachToId")); ch != nil {
			if sk, err := sg.ModelSkeleton(ch.Str("model"), x.s.Dir()); err == nil && sg.Pose(ch, sk).Index(c.Arg(1)) < 0 {
				return nil, fmt.Errorf("no bone %q on %s", c.Arg(1), ch.Str("displayName"))
			}
		}
		return a, x.doc.Update(a, ojson.Obj("bindBone", c.Arg(1)))
	})
}

func cmdSGObjSize(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	props := ojson.New()
	if s, ok, err := c.Float("size"); err != nil {
		return nil, err
	} else if ok {
		if err := inRange(s, 0.025, 5, "size"); err != nil {
			return nil, err
		}
		props = ojson.Obj("width", s, "height", s, "depth", s)
	}
	for flag, key := range map[string]string{"w": "width", "h": "height", "d": "depth"} {
		if v, ok, err := c.Float(flag); err != nil {
			return nil, err
		} else if ok {
			if err := inRange(v, 0.025, 5, key); err != nil {
				return nil, err
			}
			props.Set(key, v)
		}
	}
	if props.Len() == 0 {
		return nil, usagef("give --w/--h/--d or --size")
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) { return update(x, c.Arg(0), []string{"object"}, props) })
}

func cmdSGObjRotate(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	r := ojson.New()
	for _, k := range []string{"x", "y", "z"} {
		v, ok, err := deg(c, k)
		if err != nil {
			return nil, err
		}
		if ok {
			r.Set(k, v)
		}
	}
	if r.Len() == 0 {
		return nil, usagef("give --x, --y and/or --z in degrees")
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		return update(x, c.Arg(0), []string{"object", "image"}, ojson.Obj("rotation", r))
	})
}

func greyColor(v float64) int {
	g := int(0xFF * v)
	return g<<16 | g<<8 | g
}

func cmdSGVolumeSet(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	props, err := parseAssignments(c.Pos[1:])
	if err != nil {
		return nil, err
	}
	if v := props.Get("layers"); v != nil {
		n, _ := ojson.Num(v)
		if n < 1 || n > 10 {
			return nil, usagef("layers must be 1-10")
		}
		props.Delete("layers")
		props.Set("numberOfLayers", n)
	}
	if v := props.Get("color"); v != nil {
		if f, ok := ojson.Num(v); ok && f <= 1 {
			props.Set("color", greyColor(f))
		}
	}
	if v := props.Get("rotation"); v != nil {
		if f, ok := ojson.Num(v); ok {
			props.Set("rotation", sg.Rad(f))
		}
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) { return update(x, c.Arg(0), []string{"volume"}, props) })
}

func cmdSGVolumePreset(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	ids, ok := sg.VolumePresets[c.Arg(1)]
	if !ok {
		return nil, usagef("preset must be rain, fog or explosion")
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		return update(x, c.Arg(0), []string{"volume"}, ojson.Obj("volumeImageAttachmentIds", append([]any{}, ids...)))
	})
}

func cmdSGVolumeImages(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		var ids []any
		for _, f := range c.Pos[1:] {
			rel, err := x.doc.CopyAsset(f, "volumes")
			if err != nil {
				return nil, err
			}
			ids = append(ids, rel)
		}
		return update(x, c.Arg(0), []string{"volume"}, ojson.Obj("volumeImageAttachmentIds", ids))
	})
}

func cmdSGImageFile(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		rel, err := x.doc.CopyAsset(c.Arg(1), "images")
		if err != nil {
			return nil, err
		}
		return update(x, c.Arg(0), []string{"image"}, ojson.Obj("imageAttachmentIds", []any{rel}))
	})
}

func worldEdit(c *Ctx, fn func(w *ojson.Object) error) (any, error) {
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		w := sg.World(x.doc.Data)
		if err := fn(w); err != nil {
			return nil, err
		}
		return w, nil
	})
}

func cmdSGWorldSet(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	props, err := parseAssignments(c.Pos)
	if err != nil {
		return nil, err
	}
	return worldEdit(c, func(w *ojson.Object) error {
		for _, k := range props.Keys() {
			story.PrefsSetPath(w, k, props.Get(k))
		}
		return nil
	})
}

func cmdSGWorldBg(c *Ctx) (any, error) {
	v, err := argFloat(c, 0, "grey")
	if err != nil {
		return nil, err
	}
	if err := inRange(v, 0, 1, "background"); err != nil {
		return nil, err
	}
	return worldEdit(c, func(w *ojson.Object) error { w.Set("backgroundColor", greyColor(v)); return nil })
}

func cmdSGWorldShading(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	for _, m := range sg.ShadingModes {
		if strings.EqualFold(m, c.Arg(0)) {
			return worldEdit(c, func(w *ojson.Object) error { w.Set("shadingMode", m); return nil })
		}
	}
	return nil, usagef("shading must be Outline, Wireframe, Flat or Depth")
}

func cmdSGWorldAmbient(c *Ctx) (any, error) {
	v, err := argFloat(c, 0, "intensity")
	if err != nil {
		return nil, err
	}
	if err := inRange(v, 0, 1, "intensity"); err != nil {
		return nil, err
	}
	return worldEdit(c, func(w *ojson.Object) error { w.ObjPath(true, "ambient").Set("intensity", v); return nil })
}

func cmdSGWorldSun(c *Ctx) (any, error) {
	props := map[string]float64{}
	for _, k := range []string{"intensity", "rotation", "tilt"} {
		v, ok, err := c.Float(k)
		if err != nil {
			return nil, err
		}
		if ok {
			if k == "intensity" {
				if err := inRange(v, 0, 1, k); err != nil {
					return nil, err
				}
			} else {
				v = sg.Rad(v)
			}
			props[k] = v
		}
	}
	if len(props) == 0 {
		return nil, usagef("give --intensity, --rotation and/or --tilt")
	}
	return worldEdit(c, func(w *ojson.Object) error {
		for k, v := range props {
			w.ObjPath(true, "directional").Set(k, v)
		}
		return nil
	})
}

func cmdSGWorldFog(c *Ctx) (any, error) {
	return worldEdit(c, func(w *ojson.Object) error {
		fog := w.ObjPath(true, "fog")
		if v := c.Flag("visible"); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return usagef("--visible expects true or false")
			}
			fog.Set("visible", b)
		}
		if f, ok, err := c.Float("far"); err != nil {
			return err
		} else if ok {
			fog.Set("far", f)
		}
		return nil
	})
}

func cmdSGRoom(c *Ctx) (any, error) {
	return worldEdit(c, func(w *ojson.Object) error {
		room := w.ObjPath(true, "room")
		if v := c.Flag("visible"); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return usagef("--visible expects true or false")
			}
			room.Set("visible", b)
		}
		for flag, spec := range map[string]struct {
			key    string
			lo, hi float64
		}{"w": {"width", 1.83, 76.2}, "l": {"length", 1.83, 76.2}, "h": {"height", 1.83, 12.19}} {
			if v, ok, err := c.Float(flag); err != nil {
				return err
			} else if ok {
				if err := inRange(v, spec.lo, spec.hi, spec.key); err != nil {
					return err
				}
				room.Set(spec.key, v)
			}
		}
		return nil
	})
}

func cmdSGEnvFile(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		rel, err := x.doc.CopyAsset(c.Arg(0), "environments")
		if err != nil {
			return nil, err
		}
		env := sg.World(x.doc.Data).ObjPath(true, "environment")
		env.Set("file", rel)
		return env, nil
	})
}

func cmdSGEnvSet(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	props, err := parseAssignments(c.Pos)
	if err != nil {
		return nil, err
	}
	return worldEdit(c, func(w *ojson.Object) error {
		env := w.ObjPath(true, "environment")
		for _, k := range props.Keys() {
			switch k {
			case "x", "y", "z", "scale", "rotation", "visible", "grayscale":
				env.Set(k, props.Get(k))
			default:
				return usagef("environment keys: x y z scale rotation visible grayscale")
			}
		}
		return nil
	})
}

func cmdSGModels(c *Ctx) (any, error) {
	typ := c.Flag("type")
	words := strings.Fields(strings.ToLower(c.Flag("q")))
	var out []sg.Model
	for _, m := range sg.Models() {
		if typ != "" && m.Type != typ {
			continue
		}
		hay := strings.ToLower(m.ID + " " + m.Name)
		ok := true
		for _, w := range words {
			ok = ok && strings.Contains(hay, w)
		}
		if ok {
			out = append(out, m)
			c.Printf("%-11s %-26s %s\n", m.Type, m.ID, m.Name)
		}
	}
	return map[string]any{"models": out, "count": len(out)}, nil
}

func cmdSGPresetList(kind string) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		list, err := sg.ListPresets(kind, c.Flag("q"))
		if err != nil {
			return nil, err
		}
		for _, p := range list {
			tag := ""
			if !p.BuiltIn {
				tag = " (user)"
			}
			c.Printf("%-38s %s%s\n", p.ID, p.Name, tag)
		}
		if list == nil {
			list = []sg.PresetSummary{}
		}
		return map[string]any{"presets": list, "count": len(list)}, nil
	}
}

func cmdSGPresetDelete(kind string) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		if err := c.Need(1); err != nil {
			return nil, err
		}
		p, err := sg.FindPreset(kind, c.Arg(0))
		if err != nil {
			return nil, err
		}
		if err := sg.DeleteUserPreset(kind, p.Str("id")); err != nil {
			return nil, err
		}
		c.Printf("Deleted %s", p.Str("name"))
		return map[string]any{"deleted": p.Str("id")}, nil
	}
}

func cmdSGPresetRename(kind string) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		if err := c.Need(2); err != nil {
			return nil, err
		}
		p, err := sg.FindPreset(kind, c.Arg(0))
		if err != nil {
			return nil, err
		}
		if sg.IsBuiltinPreset(kind, p.Str("id")) {
			return nil, fmt.Errorf("built-in presets cannot be renamed")
		}
		p.Set("name", strings.Join(c.Pos[1:], " "))
		if err := sg.AddUserPreset(kind, p); err != nil {
			return nil, err
		}
		return p, nil
	}
}

func cmdSGPosePresetCreate(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	name := strings.Join(c.Pos[1:], " ")
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		sk := o.Obj("skeleton")
		if sk == nil {
			sk = ojson.New()
		}
		p := ojson.Obj("id", story.UUID(), "name", name, "keywords", name, "state", ojson.Obj("skeleton", sk.Clone()), "priority", 0)
		if err := sg.AddUserPreset(sg.KindPoses, p); err != nil {
			return nil, err
		}
		o.Set("posePresetId", p.Str("id"))
		c.Printf("Created pose preset %s (%s)", name, p.Str("id"))
		return map[string]any{"id": p.Str("id"), "name": name}, nil
	})
}

func cmdSGHandPresetCreate(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	hand := map[string]string{"": "RightHand", "right": "RightHand", "left": "LeftHand"}[c.Flag("hand")]
	if hand == "" {
		return nil, usagef("--hand must be left or right")
	}
	name := strings.Join(c.Pos[1:], " ")
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		skel, err := sg.ModelSkeleton(o.Str("model"), x.s.Dir())
		if err != nil {
			return nil, err
		}
		posed := sg.Pose(o, skel)
		hs := ojson.New()
		for i, b := range skel.Bones {
			if strings.Contains(b.Name, hand) && b.Name != hand {
				e := sg.EulerFromQuat(posed.Rot[i], "XYZ")
				hs.Set(b.Name, ojson.Obj("rotation", ojson.Obj("x", e.X, "y", e.Y, "z", e.Z)))
			}
		}
		p := ojson.Obj("id", story.UUID(), "name", name, "keywords", name, "state", ojson.Obj("handSkeleton", hs), "priority", 0)
		if err := sg.AddUserPreset(sg.KindHandPoses, p); err != nil {
			return nil, err
		}
		o.Set("handPosePresetId", p.Str("id"))
		c.Printf("Created hand pose preset %s from the %s (%d bones)", name, hand, hs.Len())
		return map[string]any{"id": p.Str("id"), "bones": hs.Len()}, nil
	})
}

func cmdSGCharPresetCreate(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	name := strings.Join(c.Pos[1:], " ")
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		o, err := x.doc.ObjOfType(c.Arg(0), "character")
		if err != nil {
			return nil, err
		}
		p := x.doc.CharacterPresetFrom(o, name)
		if err := sg.AddUserPreset(sg.KindCharacters, p); err != nil {
			return nil, err
		}
		o.Set("characterPresetId", p.Str("id"))
		if o.Str("name") == "" {
			o.Set("name", name)
		}
		sg.DisplayNames(x.doc.Objects())
		return map[string]any{"id": p.Str("id"), "name": name}, nil
	})
}

func cmdSGEmotionCreate(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	src := c.Flag("image")
	if src == "" {
		return nil, usagef("--image <png> is required")
	}
	if _, err := render.Load(src); err != nil {
		return nil, err
	}
	name := strings.Join(c.Pos, " ")
	id := story.UUID()
	if err := story.CopyFile(src, filepath.Join(sg.EmotionsDir(), id+"-texture.png")); err != nil {
		return nil, err
	}
	p := ojson.Obj("id", id, "name", name, "priority", 0)
	if err := sg.AddUserPreset(sg.KindEmotions, p); err != nil {
		return nil, err
	}
	res := map[string]any{"id": id, "name": name, "thumbnail": "not created: it needs a 3D render (ui-3d)"}
	if ch := c.Flag("character"); ch != "" {
		if _, err := sgRun(c, "", func(x *sgCtx) (any, error) {
			return update(x, ch, []string{"character"}, ojson.Obj("emotionPresetId", id))
		}); err != nil {
			return nil, err
		}
		res["appliedTo"] = ch
	}
	c.Printf("Created emotion %s (%s)", name, id)
	return res, nil
}

func cmdSGEmotionDelete(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	p, err := sg.FindPreset(sg.KindEmotions, c.Arg(0))
	if err != nil {
		return nil, err
	}
	id := p.Str("id")
	if err := sg.DeleteUserPreset(sg.KindEmotions, id); err != nil {
		return nil, err
	}
	os.Remove(filepath.Join(sg.EmotionsDir(), id+"-texture.png"))
	os.Remove(filepath.Join(sg.EmotionsDir(), id+"-thumbnail.jpg"))
	cleared := 0
	if x, err := openSG(c, ""); err == nil {
		for _, k := range x.doc.Objects().Keys() {
			if o := x.doc.Objects().Obj(k); o != nil && o.Str("emotionPresetId") == id {
				o.Delete("emotionPresetId")
				cleared++
			}
		}
		if cleared > 0 {
			if err := x.commit(); err != nil {
				return nil, err
			}
		}
	}
	c.Printf("Deleted emotion %s", p.Str("name"))
	return map[string]any{"deleted": id, "clearedReferences": cleared}, nil
}

func cmdSGScenePresetCreate(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	x, err := openSG(c, "")
	if err != nil {
		return nil, err
	}
	s := sg.Serialize(x.doc.Data)
	p := ojson.Obj("id", story.UUID(), "name", strings.Join(c.Pos, " "), "state", ojson.Obj("world", s.Get("world"), "sceneObjects", s.Get("sceneObjects"), "activeCamera", s.Get("activeCamera")))
	if err := sg.AddUserPreset(sg.KindScenes, p); err != nil {
		return nil, err
	}
	c.Printf("Created scene preset %s (%s)", p.Str("name"), p.Str("id"))
	return map[string]any{"id": p.Str("id"), "name": p.Str("name")}, nil
}

func cmdSGScenePresetApply(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	p, err := sg.FindPreset(sg.KindScenes, strings.Join(c.Pos, " "))
	if err != nil {
		return nil, err
	}
	return sgRun(c, "", func(x *sgCtx) (any, error) {
		st := p.Obj("state").Clone()
		x.doc = sg.NewDoc(st, x.s.Dir())
		sg.DisplayNames(sg.Objects(st))
		c.Printf("Applied scene preset %s", p.Str("name"))
		return map[string]any{"preset": p.Str("id"), "objects": sg.Objects(st).Len()}, nil
	})
}

func cmdSGExplore(c *Ctx) (any, error) {
	count := 9
	if v := c.Flag("count"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			return nil, usagef("--count expects 1-500")
		}
		count = n
	}
	seed := uint64(story.NowMs())
	if v := c.Flag("seed"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return nil, usagef("--seed expects a non-negative integer")
		}
		seed = n
	}
	x, err := openSG(c, c.Arg(0))
	if err != nil {
		return nil, err
	}
	shots, err := x.doc.Explore(count, seed, x.s.AspectRatio())
	if err != nil {
		return nil, err
	}
	x.hist.Explore = shots
	if err := x.hist.Save(); err != nil {
		return nil, err
	}
	for _, s := range shots {
		c.Printf("%3d  %s\n", s.Index, s.Description)
	}
	c.Printf("Add one with `sb sg camera add --from-shot <n>`.\n")
	return map[string]any{"seed": seed, "shots": shots}, nil
}

func cmdShotlist(c *Ctx) (any, error) {
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	if p.IsScript() && c.Bool("all-scenes") {
		refs, _, err := p.SceneRefs()
		if err != nil {
			return nil, err
		}
		var scenes []map[string]any
		for _, r := range refs {
			if !r.Exists {
				continue
			}
			s, err := story.LoadScene(r.File)
			if err != nil {
				return nil, err
			}
			l := sg.ShotListForScene(s)
			c.Printf("SCENE %d %s: %d setups\n", r.Number, r.Slugline, len(l.Setups))
			scenes = append(scenes, map[string]any{"number": r.Number, "id": r.ID, "slugline": r.Slugline, "synopsis": r.Synopsis, "characters": []string{}, "setups": l.Setups, "shots": l.Shots})
		}
		return map[string]any{"scenes": scenes}, nil
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	l := sg.ShotListForScene(s)
	for _, st := range l.Setups {
		c.Printf("Setup %d: %s, height %s, boards %s\n", st.Number, st.Fov, st.Height, strings.Join(st.Shots, " "))
	}
	if len(l.Setups) == 0 {
		c.Printf("No boards with 3D shots.")
	}
	return l, nil
}
