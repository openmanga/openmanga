package draw

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/tdewolff/canvas"
)

// Taper modes for PathStrokes.
var Tapers = []string{"both", "start", "end", "none"}

// PathSpec is how SVG paths become pressure strokes.
type PathSpec struct {
	Taper       string  // both | start | end | none
	MinPressure float64 // pressure at a tapered end (0-1)
}

// taperRamp is the share of a stroke's length over which the pressure ramps.
const taperRamp = 0.3

// Pressure at t (0-1 along the stroke): min at the tapered ends, 1 in the
// middle, eased with a half cosine.
func (s PathSpec) Pressure(t float64) float64 {
	ramp := func(u float64) float64 {
		u = math.Min(1, u/taperRamp)
		return s.MinPressure + (1-s.MinPressure)*(1-math.Cos(u*math.Pi))/2
	}
	p := 1.0
	if s.Taper == "both" || s.Taper == "start" {
		p = math.Min(p, ramp(t))
	}
	if s.Taper == "both" || s.Taper == "end" {
		p = math.Min(p, ramp(1-t))
	}
	return p
}

// PathStrokes turns every <path d> of an SVG (or a bare path "d" string) into
// pressure strokes, one per subpath: curves and arcs are flattened, points are
// resampled every 2 px and pressure follows the taper. Per-path attributes
// override the spec: data-taper, data-min-pressure, stroke-width (size) and a
// #rrggbb stroke (color). Transforms are not applied.
func PathStrokes(data []byte, spec PathSpec) ([]Stroke, error) {
	type pathElem struct {
		d     string
		attrs map[string]string
	}
	var paths []pathElem
	if src := strings.TrimSpace(string(data)); !strings.HasPrefix(src, "<") {
		paths = append(paths, pathElem{d: src})
	} else {
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("invalid SVG: %w", err)
			}
			if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "path" {
				attrs := map[string]string{}
				for _, a := range se.Attr {
					attrs[a.Name.Local] = a.Value
				}
				paths = append(paths, pathElem{attrs["d"], attrs})
			}
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no <path> elements")
	}
	var out []Stroke
	for i, pe := range paths {
		ps := spec
		if v := pe.attrs["data-taper"]; v != "" {
			ps.Taper = v
		}
		if !validTaper(ps.Taper) {
			return nil, fmt.Errorf("path %d: taper must be %s", i+1, strings.Join(Tapers, ", "))
		}
		st := Stroke{}
		for name, dst := range map[string]*float64{"data-min-pressure": &ps.MinPressure, "stroke-width": &st.Size} {
			if v := pe.attrs[name]; v != "" {
				f, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
				if err != nil {
					return nil, fmt.Errorf("path %d: %s must be a number, got %q", i+1, name, v)
				}
				*dst = f
			}
		}
		if v := pe.attrs["stroke"]; strings.HasPrefix(v, "#") {
			st.Color = v
		}
		p, err := canvas.ParseSVGPath(pe.d)
		if err != nil {
			return nil, fmt.Errorf("path %d: %v", i+1, err)
		}
		for _, sub := range p.Flatten(0.2).Split() {
			pts := resample(sub.Coords(), 2)
			if len(pts) == 0 {
				continue
			}
			s := st
			for k, q := range pts {
				t := 0.5
				if len(pts) > 1 {
					t = float64(k) / float64(len(pts)-1)
				}
				s.Points = append(s.Points, []float64{q.X, q.Y, ps.Pressure(t)})
			}
			out = append(out, s)
		}
	}
	return out, nil
}

func validTaper(t string) bool {
	for _, x := range Tapers {
		if t == x {
			return true
		}
	}
	return false
}

// resample walks a polyline and returns points every step px (plus the end).
func resample(in []canvas.Point, step float64) []canvas.Point {
	if len(in) < 2 {
		return in
	}
	out := []canvas.Point{in[0]}
	carry := 0.0 // distance walked since the last emitted point
	for i := 1; i < len(in); i++ {
		a, b := in[i-1], in[i]
		l := math.Hypot(b.X-a.X, b.Y-a.Y)
		for d := step - carry; d < l; d += step {
			out = append(out, canvas.Point{X: a.X + (b.X-a.X)*d/l, Y: a.Y + (b.Y-a.Y)*d/l})
		}
		carry = math.Mod(carry+l, step)
	}
	if last := in[len(in)-1]; out[len(out)-1] != last {
		out = append(out, last)
	}
	return out
}
