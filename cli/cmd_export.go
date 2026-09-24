package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"sb/internal/ojson"
	"sb/internal/pdf"
	"sb/internal/story"
)

var pdfFlags = []string{
	"preset=layout preset id or title (see `sb export pdf-presets`); default: the last used layout, else Landscape Minimal 3x5",
	"paper=a4 or letter",
	"orientation=landscape or portrait",
	"grid=AxB: A boards across, B boards down (1-10 each)",
	"direction=column (text below image) or row (text right of image)",
	"dialogue=true|false show dialogue",
	"action=true|false show action",
	"notes=true|false show notes",
	"shot-number=true|false show shot numbers",
	"time=duration, sceneTime or none",
	"text-size=board text size 6-16",
	"border=minimal or full",
	"stats=header stats to show: comma list of boards,shots,duration,aspect,date (or none)",
	"all-scenes every scene of a script project in one PDF",
}

func init() {
	register(
		&Cmd{Path: "export gif", Short: "Animated GIF (888 px wide) with per-board timing and dialogue captions, in exports/",
			Flags: []string{"boards=boards to include, e.g. 1-4 (default: all)", "width=frame width in px (default 888)", "out=output .gif path"}, Run: cmdExportGIF},
		&Cmd{Path: "export pdf", Short: "PDF of the boards with header stats and footer (15 presets or a custom layout)",
			Flags: append(append([]string{}, pdfFlags...), "out=output .pdf path (default: exports/<name> <date>.pdf)", "pages=page range to include, e.g. 2-3", "page=with --preview: page number to rasterize", "preview=write page --page as a PNG here instead of a PDF", "scale=preview scale, 1 = 72 dpi (default 2)", "save remember this layout for next time (prefs printProjectState)"),
			Run:   cmdExportPDF},
		&Cmd{Path: "export pdf-presets", Short: "The 15 PDF layout presets", Run: cmdPDFPresets},
		&Cmd{Path: "print pdf", Short: "Print the PDF layout through lpr (macOS) or lp (Linux)",
			Flags: append(append([]string{}, pdfFlags...), "copies=number of copies (default 1)", "dry-run show the print command without printing"), Run: cmdPrintPDF},
		&Cmd{Path: "export video", Short: "MP4 of the flattened boards with the board audio mixed in, via ffmpeg (H.264 at scene fps)",
			Flags: []string{"out=output .mp4 path (default: exports/<name> Exported <date>.mp4)", "dry-run print the ffconcat file and ffmpeg arguments without encoding", "verbose stream ffmpeg output to stderr"}, Run: cmdExportVideo},
		&Cmd{Path: "export images", Short: "Flattened <name>-board-00001.png per board, in exports/<file> Images <date>/",
			Flags: []string{"out=output folder"}, Run: cmdExportImages},
	)
}

func cmdExportGIF(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	story.UpdateTiming(s)
	spec := c.Flag("boards")
	if spec == "" {
		spec = "all"
	}
	idx, err := boardIndexes(s, spec)
	if err != nil {
		return nil, err
	}
	var boards []*ojson.Object
	for _, i := range sortedInts(idx) {
		boards = append(boards, s.Boards()[i])
	}
	width := 888
	if v := c.Flag("width"); v != "" {
		if width, err = strconv.Atoi(v); err != nil || width < 16 {
			return nil, usagef("--width expects a pixel width")
		}
	}
	out, err := story.ExportGIF(s, boards, width, c.Flag("out"))
	if err != nil {
		return nil, err
	}
	c.Printf("Exported %d boards to %s", len(boards), out)
	return map[string]any{"path": out, "boards": len(boards)}, nil
}

func cmdExportImages(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	files, err := story.ExportImages(s, c.Flag("out"))
	if err != nil {
		return nil, err
	}
	c.Printf("Exported %d images:\n  %s", len(files), strings.Join(files, "\n  "))
	return map[string]any{"files": files}, nil
}

func parseBoolFlag(c *Ctx, name string, dst *bool) error {
	v, ok := c.Flags[name]
	if !ok {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return usagef("--%s expects true or false", name)
	}
	*dst = b
	return nil
}

// pdfConfig starts from the preset or remembered layout and applies flags.
func pdfConfig(c *Ctx) (pdf.Config, error) {
	cfg := pdf.Presets()[0].Data
	cfg.PaperSizeKey = "a4"
	prefs, _ := story.LoadPrefs()
	if prefs != nil && c.Flag("preset") == "" {
		if st := prefs.Obj("printProjectState"); st != nil {
			json.Unmarshal(ojson.Stringify(st, ""), &cfg)
		}
	}
	if v := c.Flag("preset"); v != "" {
		p, ok := pdf.FindPreset(v)
		if !ok {
			return cfg, usagef("unknown preset %q (see `sb export pdf-presets`)", v)
		}
		paper := cfg.PaperSizeKey
		cfg = p.Data
		cfg.PaperSizeKey = paper
	}
	if v := c.Flag("paper"); v != "" {
		if v != "a4" && v != "letter" {
			return cfg, usagef("--paper must be a4 or letter")
		}
		cfg.PaperSizeKey = v
	}
	if v := c.Flag("orientation"); v != "" {
		if v != "landscape" && v != "portrait" {
			return cfg, usagef("--orientation must be landscape or portrait")
		}
		cfg.Orientation = v
	}
	if v := c.Flag("grid"); v != "" {
		var a, b int
		if _, err := fmt.Sscanf(strings.ToLower(strings.ReplaceAll(v, "×", "x")), "%dx%d", &a, &b); err != nil || a < 1 || b < 1 || a > 10 || b > 10 {
			return cfg, usagef("--grid expects AxB with 1-10 each, e.g. 3x5")
		}
		cfg.GridDim = [2]int{a, b}
	}
	if v := c.Flag("direction"); v != "" {
		if v != "row" && v != "column" {
			return cfg, usagef("--direction must be row or column")
		}
		cfg.Direction = v
	}
	for name, dst := range map[string]*bool{"dialogue": &cfg.EnableDialogue, "action": &cfg.EnableAction, "notes": &cfg.EnableNotes, "shot-number": &cfg.EnableShotNumber} {
		if err := parseBoolFlag(c, name, dst); err != nil {
			return cfg, err
		}
	}
	if v := c.Flag("time"); v != "" {
		if v != "duration" && v != "sceneTime" && v != "none" {
			return cfg, usagef("--time must be duration, sceneTime or none")
		}
		cfg.BoardTimeDisplay = v
	}
	if f, ok, err := c.Float("text-size"); err != nil {
		return cfg, err
	} else if ok {
		if f < 6 || f > 16 {
			return cfg, usagef("--text-size must be 6-16")
		}
		cfg.BoardTextSize = f
	}
	if v := c.Flag("border"); v != "" {
		if v != "minimal" && v != "full" {
			return cfg, usagef("--border must be minimal or full")
		}
		cfg.BoardBorderStyle = v
	}
	if v, ok := c.Flags["stats"]; ok {
		st := &cfg.Header.Stats
		*st = struct {
			Boards        bool `json:"boards"`
			Shots         bool `json:"shots"`
			SceneDuration bool `json:"sceneDuration"`
			AspectRatio   bool `json:"aspectRatio"`
			DateExported  bool `json:"dateExported"`
		}{}
		for _, k := range strings.Split(v, ",") {
			switch strings.TrimSpace(k) {
			case "boards":
				st.Boards = true
			case "shots":
				st.Shots = true
			case "duration":
				st.SceneDuration = true
			case "aspect":
				st.AspectRatio = true
			case "date":
				st.DateExported = true
			case "none", "":
			default:
				return cfg, usagef("unknown stat %q", k)
			}
		}
	}
	if c.Bool("save") && prefs != nil {
		st := ojson.From(cfg)
		prefs.Set("printProjectState", st)
		story.SavePrefs(prefs)
	}
	return cfg, nil
}

// pdfProject collects the scenes to print and a base name for the output.
func pdfProject(c *Ctx) (pdf.Project, string, string, error) {
	p, err := openProject()
	if err != nil {
		return pdf.Project{}, "", "", err
	}
	if !p.IsScript() {
		s, err := story.LoadScene(p.File)
		if err != nil {
			return pdf.Project{}, "", "", err
		}
		return pdf.Project{Scenes: []pdf.SceneData{{Title: s.Name(), Scene: s}}}, s.Name(), s.Dir(), nil
	}
	sc, err := p.Script()
	if err != nil {
		return pdf.Project{}, "", "", err
	}
	proj := pdf.Project{Title: sc.Title}
	if c.Bool("all-scenes") {
		refs, _, err := p.SceneRefs()
		if err != nil {
			return proj, "", "", err
		}
		for _, r := range refs {
			if !r.Exists {
				continue
			}
			s, err := story.LoadScene(r.File)
			if err != nil {
				return proj, "", "", err
			}
			proj.Scenes = append(proj.Scenes, pdf.SceneData{Title: r.Slugline, Scene: s})
		}
		if len(proj.Scenes) == 0 {
			return proj, "", "", fmt.Errorf("no scene has storyboards yet (run `sb scene open <n>`)")
		}
		base := strings.TrimSuffix(filepath.Base(p.File), filepath.Ext(p.File))
		if len(proj.Scenes) == 1 {
			base = proj.Scenes[0].Scene.Name()
		}
		return proj, base, p.Root(), nil
	}
	s, ref, err := p.OpenScene(sceneArg, prefsFps(), false)
	if err != nil {
		return proj, "", "", err
	}
	proj.Scenes = []pdf.SceneData{{Title: ref.Slugline, Scene: s}}
	return proj, s.Name(), p.Root(), nil
}

func cmdPDFPresets(c *Ctx) (any, error) {
	ps := pdf.Presets()
	for _, p := range ps {
		d := p.Data
		c.Printf("%-15s %-26s %-9s %dx%d %-6s %s\n", p.ID, p.Title, d.Orientation, d.GridDim[0], d.GridDim[1], d.Direction, d.BoardBorderStyle)
	}
	return map[string]any{"presets": ps}, nil
}

func cmdExportPDF(c *Ctx) (any, error) {
	cfg, err := pdfConfig(c)
	if err != nil {
		return nil, err
	}
	proj, base, root, err := pdfProject(c)
	if err != nil {
		return nil, err
	}
	total := pdf.PageCount(proj, cfg)
	if out := c.Flag("preview"); out != "" {
		pg := 1
		if v := c.Flag("page"); v != "" {
			if pg, err = strconv.Atoi(v); err != nil || pg < 1 || pg > total {
				return nil, usagef("--page must be 1-%d", total)
			}
		}
		scale := 2.0
		if f, ok, err := c.Float("scale"); err != nil {
			return nil, err
		} else if ok {
			scale = f
		}
		if err := pdf.Preview(out, proj, cfg, pg-1, scale); err != nil {
			return nil, err
		}
		c.Printf("Wrote page %d of %d to %s", pg, total, out)
		return map[string]any{"preview": out, "page": pg, "pages": total}, nil
	}
	from, to := 0, -1
	if v := c.Flag("pages"); v != "" {
		a, b, _ := strings.Cut(v, "-")
		x, err1 := strconv.Atoi(a)
		y := x
		var err2 error
		if b != "" {
			y, err2 = strconv.Atoi(b)
		}
		if err1 != nil || err2 != nil || x < 1 || y < x {
			return nil, usagef("--pages expects N or A-B")
		}
		from, to = x-1, y-1
	}
	out := c.Flag("out")
	if out == "" {
		dir := filepath.Join(root, "exports")
		os.MkdirAll(dir, 0o755)
		out = filepath.Join(dir, base+" "+story.Stamp()+".pdf")
	}
	if err := pdf.Write(out, proj, cfg, from, to); err != nil {
		return nil, err
	}
	c.Printf("Wrote %s (%d pages)", out, total)
	return map[string]any{"path": out, "pages": total, "config": cfg}, nil
}

func cmdPrintPDF(c *Ctx) (any, error) {
	cfg, err := pdfConfig(c)
	if err != nil {
		return nil, err
	}
	proj, _, _, err := pdfProject(c)
	if err != nil {
		return nil, err
	}
	copies := 1
	if v := c.Flag("copies"); v != "" {
		if copies, err = strconv.Atoi(v); err != nil || copies < 1 {
			return nil, usagef("--copies expects a positive number")
		}
	}
	tmp, err := os.MkdirTemp("", "storyboarder-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	file := filepath.Join(tmp, "export.pdf")
	if err := pdf.Write(file, proj, cfg, 0, -1); err != nil {
		return nil, err
	}
	var args []string
	bin := "lpr"
	switch runtime.GOOS {
	case "darwin":
		args = append(args, "-o", "media="+cfg.PaperSizeKey)
		if cfg.Orientation == "landscape" {
			args = append(args, "-o", "orientation-requested=4")
		}
		args = append(args, "-#", strconv.Itoa(copies), file)
	case "linux":
		bin = "lp"
		args = []string{"-n", strconv.Itoa(copies), file}
	default:
		return nil, fmt.Errorf("printing is supported on macOS (lpr) and Linux (lp)")
	}
	if c.Bool("dry-run") {
		c.Printf("%s %s", bin, strings.Join(args, " "))
		return map[string]any{"command": append([]string{bin}, args...), "printed": false}, nil
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %v %s", bin, err, out)
	}
	c.Printf("Sent %d copies to the printer", copies)
	return map[string]any{"printed": true, "copies": copies, "output": string(out)}, nil
}

func cmdExportVideo(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	out := c.Flag("out")
	if out == "" {
		d, err := story.ExportsDir(s)
		if err != nil {
			return nil, err
		}
		out = filepath.Join(d, s.Name()+" Exported "+story.Stamp()+".mp4")
	}
	out, _ = filepath.Abs(out)
	job, err := story.PrepareVideo(s, out)
	if err != nil {
		return nil, err
	}
	if c.Bool("dry-run") {
		defer os.RemoveAll(job.Dir)
		c.Printf("%s\n\nffmpeg %s", job.Concat, strings.Join(job.Args, " "))
		return map[string]any{"concat": job.Concat, "args": job.Args, "output": out}, nil
	}
	var log *os.File
	if c.Bool("verbose") {
		log = os.Stderr
	}
	if err := job.Run(log); err != nil {
		return nil, err
	}
	c.Printf("Exported %s", out)
	return map[string]any{"path": out, "missingLayers": job.Missing}, nil
}
