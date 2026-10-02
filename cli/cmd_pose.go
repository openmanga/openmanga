package main

import (
	"math"
	"strconv"
	"strings"

	"sb/internal/draw"
	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/sg"
)

func init() {
	register(
		&Cmd{Path: "pose list", Short: "The pose presets (342 built-in plus yours) with the index pose draw accepts",
			Flags: []string{"q=search words (name and keywords)"}, Run: cmdPoseList},
		&Cmd{Path: "pose draw", Args: "<pose> [board]", Short: "Draw a pose preset as a traceable mannequin (bones, joints, head) from its skeleton, no 3D render",
			Long: "<pose> is an index from `sb pose list`, a preset name or an id (prefix). Forward kinematics on the model's skeleton\n" +
				"(internal/sg/data/skeletons.json) with the preset's bone rotations, projected orthographically. --x/--y is the\n" +
				"floor point under the character (area-local), --height the standing height in px (a crouch comes out shorter).\n" +
				"The far-side limbs are drawn lighter. The JSON result lists every joint's position for placing details.",
			Flags: append([]string{"model=adult-male, adult-female, teen-male, teen-female, child or baby (default adult-male)",
				"view=front, 3q, side or back (default front)", "x=floor point x in px (default: the middle)", "y=floor point y in px (default: 95% down)",
				"height=standing height in px (default: 85% of the board/panel height)", "size=bone line width px (default height/80)",
				"color=#rrggbb (default #4A9FE8, light blue)", "replace clear the layer (or the panel area) first"}, targetFlags...), Run: cmdPoseDraw},
	)
}

func cmdPoseList(c *Ctx) (any, error) {
	all, err := sg.ListPresets(sg.KindPoses, "")
	if err != nil {
		return nil, err
	}
	words := strings.Fields(strings.ToLower(c.Flag("q")))
	rows := []map[string]any{}
	for i, p := range all {
		hay := strings.ToLower(p.Name + " " + p.Keywords)
		match := true
		for _, w := range words {
			match = match && strings.Contains(hay, w)
		}
		if match {
			rows = append(rows, map[string]any{"index": i + 1, "id": p.ID, "name": p.Name, "keywords": p.Keywords, "builtIn": p.BuiltIn})
			c.Printf("%4d  %s\n", i+1, p.Name)
		}
	}
	return map[string]any{"poses": rows, "count": len(rows)}, nil
}

// findPose resolves a pose list index, name or id.
func findPose(ref string) (*ojson.Object, error) {
	if n, err := strconv.Atoi(ref); err == nil {
		all, err := sg.ListPresets(sg.KindPoses, "")
		if err != nil {
			return nil, err
		}
		if n < 1 || n > len(all) {
			return nil, usagef("pose index %d out of range (1-%d, see `sb pose list`)", n, len(all))
		}
		ref = all[n-1].ID
	}
	return sg.FindPreset(sg.KindPoses, ref)
}

var poseViews = map[string]float64{"front": 0, "3q": 45, "side": 90, "back": 180}

func cmdPoseDraw(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	preset, err := findPose(c.Pos[0])
	if err != nil {
		return nil, err
	}
	c.Pos = c.Pos[1:]
	t, err := resolveTarget(c)
	if err != nil {
		return nil, err
	}
	layer := c.Flag("layer")
	if layer == "" {
		layer = "reference"
	}
	if err := t.checkLayer(layer); err != nil {
		return nil, err
	}
	viewName := c.Flag("view")
	if viewName == "" {
		viewName = "front"
	}
	yaw, ok := poseViews[viewName]
	if !ok {
		return nil, usagef("--view must be front, 3q, side or back")
	}
	model := c.Flag("model")
	if model == "" {
		model = "adult-male"
	}
	if sg.IsCustom(model) {
		return nil, usagef("--model must be a built-in character")
	}
	fig, err := sg.PoseFigure(model, preset, yaw)
	if err != nil {
		return nil, usagef("%v", err)
	}
	x, y, height := t.area.W/2, t.area.H*0.95, t.area.H*0.85
	size := 0.0
	for name, dst := range map[string]*float64{"x": &x, "y": &y, "height": &height, "size": &size} {
		if f, ok, err := c.Float(name); err != nil {
			return nil, err
		} else if ok {
			*dst = f
		}
	}
	if height <= 0 {
		return nil, usagef("--height must be positive")
	}
	if size <= 0 {
		size = math.Max(2, height/80)
	}
	col := "#4A9FE8"
	if v := c.Flag("color"); v != "" {
		if _, err := render.ParseColor(v); err != nil {
			return nil, usagef("%v", err)
		}
		col = v
	}
	k := height / fig.Height
	px := func(name string) []float64 {
		j := fig.Joints[name]
		return []float64{x + j[0]*k, y + j[1]*k}
	}
	// the side farther from the viewer is drawn lighter
	far := ""
	if d := sideDepth(fig, "Left") - sideDepth(fig, "Right"); math.Abs(d) > 0.02 {
		far = map[bool]string{true: "Left", false: "Right"}[d < 0]
	}
	opacity := func(name string) *float64 {
		o := 0.9
		if far != "" && strings.HasPrefix(name, far) {
			o = 0.4
		}
		return &o
	}
	doc := draw.StrokeDoc{Color: col, Smooth: new(bool)}
	for _, b := range fig.Bones {
		a, e := px(b[0]), px(b[1])
		doc.Strokes = append(doc.Strokes, draw.Stroke{Size: size, Opacity: opacity(b[1]), Points: [][]float64{append(a, 1), append(e, 0.6)}})
	}
	for name := range fig.Joints {
		if name == fig.Head[1] || strings.HasPrefix(name, "leaf") || strings.HasSuffix(name, "Eye") {
			continue
		}
		doc.Strokes = append(doc.Strokes, draw.Stroke{Size: size * 1.8, Opacity: opacity(name), Points: [][]float64{append(px(name), 1)}})
	}
	// head: an ellipse along the Head bone -> head tip axis, sized from its 3D length
	h0, h1 := fig.Joints[fig.Head[0]], fig.Joints[fig.Head[1]]
	l3 := math.Sqrt(math.Pow(h1[0]-h0[0], 2) + math.Pow(h1[1]-h0[1], 2) + math.Pow(fig.Depth[fig.Head[1]]-fig.Depth[fig.Head[0]], 2))
	ang := math.Atan2(h1[1]-h0[1], h1[0]-h0[0])
	if math.Hypot(h1[0]-h0[0], h1[1]-h0[1]) < 0.3*l3 { // head pointing at the viewer
		ang = -math.Pi / 2
	}
	cx, cy := x+(h0[0]+h1[0])/2*k, y+(h0[1]+h1[1])/2*k
	ry, rx := l3*k*0.62, l3*k*0.5
	var ring [][]float64
	for i := 0; i <= 48; i++ {
		a := 2 * math.Pi * float64(i) / 48
		ex, ey := rx*math.Sin(a), ry*math.Cos(a) // ry along the head axis
		ring = append(ring, []float64{cx + ey*math.Cos(ang) - ex*math.Sin(ang), cy + ey*math.Sin(ang) + ex*math.Cos(ang), 1})
	}
	doc.Strokes = append(doc.Strokes, draw.Stroke{Size: size, Opacity: opacity("Head"), Points: ring})
	for _, e := range fig.Eyes {
		doc.Strokes = append(doc.Strokes, draw.Stroke{Size: size * 1.2, Opacity: opacity(e), Points: [][]float64{append(px(e), 1)}})
	}
	tool := draw.Tool{Name: "pose", Size: size, Opacity: 0.9}
	overlay, err := draw.Strokes(t.w, t.h, t.area, doc, tool)
	if err != nil {
		return nil, err
	}
	res, err := t.write(c, layer, overlay, false, c.Bool("replace"))
	if err != nil {
		return nil, err
	}
	joints := map[string][]float64{}
	for name := range fig.Joints {
		if !strings.HasPrefix(name, "leaf") {
			p := px(name)
			joints[name] = []float64{math.Round(p[0]), math.Round(p[1])}
		}
	}
	res["pose"], res["model"], res["view"], res["joints"] = preset.Str("name"), model, viewName, joints
	c.Printf("\nPose %q (%s, %s view)", preset.Str("name"), model, viewName)
	return res, nil
}

// sideDepth is the mean depth of one side's joints (Left or Right).
func sideDepth(f *sg.Figure, side string) float64 {
	s, n := 0.0, 0
	for name, d := range f.Depth {
		if strings.HasPrefix(name, side) {
			s += d
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return s / float64(n)
}
