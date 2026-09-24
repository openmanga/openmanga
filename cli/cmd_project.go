package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sb/internal/manga"
	"sb/internal/ojson"
	"sb/internal/script"
	"sb/internal/story"
)

var aspectPresets = map[string]float64{
	"2.39": 2.39, "2.0": 2, "2": 2, "1.85": 1.85,
	"16:9": 1.7777777777777777, "9:16": 0.5625, "1:1": 1, "1": 1, "4:3": 1.3333333333333333,
}

var validFps = []float64{12, 15, 23.976, 24, 25, 29.97, 30, 50, 59.94, 60}

func parseAspect(s string) (float64, error) {
	if v, ok := aspectPresets[s]; ok {
		return v, nil
	}
	f, err := parseFloatArg(s, "--aspect")
	if err != nil || f <= 0 {
		return 0, usagef("--aspect must be one of 2.39, 2.0, 1.85, 16:9, 9:16, 1:1, 4:3 or a positive number")
	}
	return f, nil
}

func checkFps(f float64) error {
	for _, v := range validFps {
		if v == f {
			return nil
		}
	}
	return usagef("fps must be one of 12, 15, 23.976, 24, 25, 29.97, 30, 50, 59.94, 60")
}

// stamp is moment().format('YYYY-MM-DD hh.mm.ss') (12-hour clock, as the original).
func stamp() string { return time.Now().Format("2006-01-02 03.04.05") }

func sceneInfo(s *story.Scene) map[string]any {
	shots := 0
	for i, b := range s.Boards() {
		if i == 0 || b.Bool("newShot") {
			shots++
		}
	}
	return map[string]any{
		"file":               s.Path,
		"version":            s.Data.Get("version"),
		"aspectRatio":        s.AspectRatio(),
		"fps":                s.Fps(),
		"defaultBoardTiming": s.DefaultBoardTiming(),
		"boards":             len(s.Boards()),
		"shots":              shots,
		"duration":           story.SceneDuration(s),
		"mode":               map[bool]string{true: "manga", false: "film"}[s.IsManga()],
		"pages":              len(s.Pages()),
	}
}

func init() {
	register(
		&Cmd{Path: "project new", Args: "<dir>", Short: "Create <dir>/<name>.storyboarder and images/ with one blank board",
			Flags: []string{"aspect=aspect ratio: 2.39, 2.0, 1.85, 16:9, 9:16, 1:1, 4:3 or a number (default 16:9)", "fps=frames per second (default: prefs lastUsedFps or 24)", "timing=default board duration in ms (default: prefs defaultBoardTiming)", "force move an existing folder to the Trash first", "manga manga mode: pages[] with panels and balloons next to the boards (one empty page to start)", "page-size=with --manga: page size WxH px (default 1414x2000)", "reading=with --manga: rtl (default) or ltr", "template=with --manga: panel template for the new pages", "pages=with --manga: number of pages to create (default 1)", "first-page=with --manga: single (default) or paired"},
			Run:   cmdProjectNew},
		&Cmd{Path: "project open", Args: "<file>", Short: "Open a .storyboarder, .fountain or .fdx: adds to recents and reports the project (scripts get scene ids inserted)", Run: cmdProjectOpen},
		&Cmd{Path: "project info", Short: "Version, aspect ratio, fps, default timing, board count, file path", Run: cmdProjectInfo},
		&Cmd{Path: "project stats", Short: "Shot count, board count, total duration incl. audio, average line mileage", Run: cmdProjectStats},
		&Cmd{Path: "project migrate", Short: "Convert string durations to numbers and pre-1.6 single-PNG boards to the fill layer (backs up to <name>-backup)", Run: cmdProjectMigrate},
		&Cmd{Path: "project verify", Short: "Check for missing layer files, thumbnails, linked PSDs and posterframes",
			Flags: []string{"fix write placeholder PNGs, unlink missing PSDs, rebuild posterframes"}, Run: cmdProjectVerify},
		&Cmd{Path: "project files", Short: "Every file the project uses (layers, thumbnails, audio, links, 3D models) and whether it exists", Run: cmdProjectFiles},
		&Cmd{Path: "project copy", Args: "<dst-folder>", Short: "Save As: copy every used file into a new folder, renaming the project file after the folder",
			Flags: []string{"force move a non-empty destination folder to the Trash first"}, Run: cmdProjectCopy},
		&Cmd{Path: "project zip", Short: "Export the project as a ZIP in exports/, reporting missing files",
			Flags: []string{"out=zip file path (default: <scene dir>/exports/<name>-<date>.zip)"}, Run: cmdProjectZip},
		&Cmd{Path: "project cleanup", Short: "Rename files to board order, drop dead links and audio, trash unused files in images/",
			Flags: []string{"dry-run only report what would change"}, Run: cmdProjectCleanup},

		&Cmd{Path: "recent list", Short: "Recent projects with their metadata (prefs recentDocuments)", Run: cmdRecentList},
		&Cmd{Path: "recent add", Args: "<file>", Short: "Put a project at the top of the recent list", Run: cmdRecentAdd},
		&Cmd{Path: "recent prune", Short: "Remove recent entries whose files are gone", Run: cmdRecentPrune},

		&Cmd{Path: "scene list", Short: "Scenes with number, duration, slugline, synopsis and storyboard folder", Run: cmdSceneList},
		&Cmd{Path: "scene open", Args: "<n>", Short: "Map scene <n> to Scene-<n>-<slug>-<id>/, creating JSON, images/ and a first board if missing; remembers it as the current scene", Run: cmdSceneOpen},
		&Cmd{Path: "scene script", Args: "[n]", Short: "Numbered script lines (slugline, action, dialogue, transitions) of a scene, for board set-from-script", Run: cmdSceneScript},
		&Cmd{Path: "scene renumber", Short: "Recompute shot labels (1A, 1B…), board numbers and start times", Run: cmdSceneRenumber},
		&Cmd{Path: "scene set-fps", Args: "<fps>", Short: "Set scene fps (12, 15, 23.976, 24, 25, 29.97, 30, 50, 59.94, 60); also stored as prefs lastUsedFps", Run: cmdSceneSetFps},
		&Cmd{Path: "scene set-default-duration", Args: "<ms>", Short: "Set the scene's default board duration in ms", Run: cmdSceneSetDefault},

		&Cmd{Path: "script parse", Args: "<file>", Short: "Parse a .fountain or .fdx into scenes with estimated timing and word counts (read-only)", Run: cmdScriptParse},
		&Cmd{Path: "script info", Args: "[file]", Short: "Title from the title page, scene count, total estimated time", Run: cmdScriptInfo},
		&Cmd{Path: "script locations", Args: "[file]", Short: "Slugline locations with counts", Run: cmdScriptLocations},
		&Cmd{Path: "script characters", Args: "[file]", Short: "Speaking characters with counts", Run: cmdScriptCharacters},
		&Cmd{Path: "script add-scene-ids", Args: "[file]", Short: "Append stable ids (#n-XXXXX#) to scene headings without one and rewrite the script", Run: cmdScriptAddIDs},
		&Cmd{Path: "script init", Args: "<file>", Short: "Create storyboards/storyboard.settings next to the script (inserts scene ids first)",
			Flags: []string{"aspect=aspect ratio (default 16:9)"}, Run: cmdScriptInit},
		&Cmd{Path: "script watch", Args: "[file]", Short: "Watch the script; on change insert new scene ids and print the re-parsed scene list (one JSON line per change with --json)",
			Flags: []string{"interval=poll interval in ms (default 1000)", "once exit after the first change"}, Run: cmdScriptWatch},
	)
}

func cmdProjectNew(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	dir, _ := filepath.Abs(c.Arg(0))
	aspect := 1.7777777777777777
	if v := c.Flag("aspect"); v != "" {
		var err error
		if aspect, err = parseAspect(v); err != nil {
			return nil, err
		}
	}
	prefs, err := story.LoadPrefs()
	if err != nil {
		return nil, err
	}
	fps := prefs.NumOr("lastUsedFps", 24)
	if f, ok, err := c.Float("fps"); err != nil {
		return nil, err
	} else if ok {
		if err := checkFps(f); err != nil {
			return nil, err
		}
		fps = f
	}
	timing := prefs.NumOr("defaultBoardTiming", 2000)
	if f, ok, err := c.Float("timing"); err != nil {
		return nil, err
	} else if ok {
		timing = f
	}
	if st, err := os.Stat(dir); err == nil {
		if !st.IsDir() {
			return nil, fmt.Errorf("could not overwrite file %s: only folders can be overwritten", filepath.Base(dir))
		}
		if !c.Bool("force") {
			return nil, fmt.Errorf("%s already exists (use --force to move it to the Trash)", dir)
		}
		if err := story.Trash(dir); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0o755); err != nil {
		return nil, err
	}
	name := filepath.Base(dir)
	file := filepath.Join(dir, name+".storyboarder")
	data := ojson.Obj("version", story.AppVersion, "aspectRatio", aspect, "fps", fps, "defaultBoardTiming", timing, "boards", []any{})
	if err := story.WriteJSON(file, data, ""); err != nil {
		return nil, err
	}
	story.AddRecent(file, data.Clone())
	s, err := story.LoadScene(file)
	if err != nil {
		return nil, err
	}
	if err := story.EnsureBoardExists(s); err != nil {
		return nil, err
	}
	if c.Bool("manga") {
		pw, ph := 1414, 2000
		if v := c.Flag("page-size"); v != "" {
			if pw, ph, err = parseSize(v); err != nil {
				return nil, err
			}
		}
		rtl, err := readingFlag(c)
		if err != nil {
			return nil, err
		}
		manga.MakeManga(s, pw, ph, rtl)
		fp, err := firstPageFlag(c)
		if err != nil {
			return nil, err
		}
		if fp != nil {
			manga.Settings(s).Set("firstPageSingle", *fp)
		}
		n := 1
		if v := c.Flag("pages"); v != "" {
			if n, err = strconv.Atoi(v); err != nil || n < 0 || n > 500 {
				return nil, usagef("--pages expects 0-500")
			}
		}
		var pages []*ojson.Object
		for i := 0; i < n; i++ {
			pg := manga.NewPage()
			pages = append(pages, pg)
			s.SetPages(pages)
			if t := c.Flag("template"); t != "" {
				if err := applyTemplate(c, s, pg, t); err != nil {
					return nil, usagef("%v", err)
				}
			}
			if err := manga.RefreshPage(s, pg); err != nil {
				return nil, err
			}
		}
		s.SetPages(pages)
		if err := s.Save(); err != nil {
			return nil, err
		}
	}
	c.Printf("Created %s (aspect %v, %v fps, %v ms default)\n", file, aspect, fps, timing)
	if s.IsManga() {
		pw, ph := s.PageSize()
		c.Printf("Manga mode: %dx%d pages, %s, %d pages\n", pw, ph, manga.Settings(s).Str("readingDirection"), len(s.Pages()))
	}
	return sceneInfo(s), nil
}

func scriptMeta(p *story.Project, sc *script.Parsed) *ojson.Object {
	refs, _, _ := p.SceneRefs()
	boards := 0
	for _, r := range refs {
		if r.Exists {
			boards++
		}
	}
	return ojson.Obj("type", "script", "sceneBoardsCount", boards, "sceneCount", len(sc.Scenes()), "totalMovieTime", sc.TotalTime(), "title", sc.Title)
}

func cmdProjectOpen(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	p, err := story.FindProject(c.Arg(0))
	if err != nil {
		return nil, err
	}
	if !p.IsScript() {
		s, err := story.LoadScene(p.File)
		if err != nil {
			return nil, err
		}
		story.AddRecent(p.File, ojson.Obj("boards", 2, "time", 3000))
		c.Printf("Opened %s: %d boards\n", p.File, len(s.Boards()))
		return map[string]any{"type": "scene", "scene": sceneInfo(s)}, nil
	}
	added, err := p.EnsureSceneIDs()
	if err != nil {
		return nil, err
	}
	sc, err := p.Script()
	if err != nil {
		return nil, err
	}
	if _, err := p.Settings(); err != nil {
		return nil, err
	}
	story.AddRecent(p.File, scriptMeta(p, sc))
	refs, _, _ := p.SceneRefs()
	c.Printf("Opened script %s: %q, %d scenes\n", p.File, sc.Title, len(refs))
	if added {
		c.Printf("Added scene ids to the script.\n")
	}
	return map[string]any{"type": "script", "file": p.File, "title": sc.Title, "sceneIdsAdded": added, "scenes": refs}, nil
}

func cmdProjectInfo(c *Ctx) (any, error) {
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	res := map[string]any{"project": p.File, "type": "scene"}
	if p.IsScript() {
		sc, err := p.Script()
		if err != nil {
			return nil, err
		}
		settings, err := p.Settings()
		if err != nil {
			return nil, err
		}
		refs, _, _ := p.SceneRefs()
		res["type"] = "script"
		res["title"] = sc.Title
		res["aspectRatio"] = settings.Get("aspectRatio")
		res["scenes"] = len(refs)
		res["currentScene"] = settings.NumOr("lastScene", 0) + 1
	}
	s, err := openScene()
	if err == nil {
		res["scene"] = sceneInfo(s)
		c.Printf("%s\n  version %v, aspect %v, %v fps, default %v ms, %d boards\n", s.Path, s.Data.Get("version"), s.AspectRatio(), s.Fps(), s.DefaultBoardTiming(), len(s.Boards()))
	} else if !p.IsScript() {
		return nil, err
	}
	return res, nil
}

func cmdProjectStats(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	shots := 0
	miles := 0.0
	for _, b := range s.Boards() {
		if b.Bool("newShot") {
			shots++
		}
		miles += b.NumOr("lineMileage", 0)
	}
	if shots == 0 {
		shots = 1
	}
	n := len(s.Boards())
	avg := 0.0
	if n > 0 {
		avg = miles / float64(n) / 5280
	}
	story.UpdateTiming(s)
	d := story.SceneDuration(s)
	c.Printf("%d boards, %d shots, %s total, %.1f avg. line mileage\n", n, shots, msToTime(d), avg)
	return map[string]any{"boards": n, "shots": shots, "duration": d, "durationText": msToTime(d), "averageLineMiles": avg}, nil
}

func cmdProjectMigrate(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	rep, err := story.Migrate(s)
	if err != nil {
		return nil, err
	}
	c.Printf("%d string durations converted, %d boards moved to the fill layer", len(rep.StringDurations), len(rep.LayersMigrated))
	if rep.Backup != "" {
		c.Printf(" (backup: %s)", rep.Backup)
	}
	if rep.Skipped != "" {
		c.Printf("\n%s", rep.Skipped)
	}
	return rep, nil
}

func cmdProjectVerify(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	rep, err := story.Verify(s, c.Bool("fix"))
	if err != nil {
		return nil, err
	}
	c.Printf("missing files: %d, missing linked files: %d, missing posterframes: %d", len(rep.MissingFiles), len(rep.MissingLinks), len(rep.MissingPosterframes))
	if rep.Fixed {
		c.Printf(" (fixed)")
	}
	for _, f := range append(append(rep.MissingFiles, rep.MissingLinks...), rep.MissingPosterframes...) {
		c.Printf("\n  %s", f)
	}
	return rep, nil
}

func cmdProjectFiles(c *Ctx) (any, error) {
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	files, err := story.ProjectFiles(p, false)
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	missing := 0
	for _, f := range append([]string{p.File}, files...) {
		ok := story.Exists(f)
		if !ok {
			missing++
		}
		list = append(list, map[string]any{"path": f, "exists": ok})
		mark := " "
		if !ok {
			mark = "!"
		}
		c.Printf("%s %s\n", mark, f)
	}
	return map[string]any{"files": list, "missing": missing}, nil
}

func cmdProjectCopy(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	dst, _ := filepath.Abs(c.Arg(0))
	if filepath.Ext(dst) != "" {
		return nil, usagef("please choose a folder name, not a file name")
	}
	if entries, err := os.ReadDir(dst); err == nil && len(entries) > 0 {
		if !c.Bool("force") {
			return nil, fmt.Errorf("%s is not empty (use --force to move it to the Trash first)", dst)
		}
		if err := story.Trash(dst); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return nil, err
	}
	missing, err := story.CopyProject(p.File, dst, false, true)
	if err != nil {
		return nil, err
	}
	out := filepath.Join(dst, filepath.Base(dst)+filepath.Ext(p.File))
	story.AddRecent(out, nil)
	c.Printf("Copied to %s", out)
	for _, m := range missing {
		c.Printf("\n  missing: %s", m)
	}
	return map[string]any{"project": out, "missing": missing}, nil
}

func cmdProjectZip(c *Ctx) (any, error) {
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	out := c.Flag("out")
	if out == "" {
		dir := p.Root()
		if s, err := openScene(); err == nil {
			dir = s.Dir()
		}
		base := strings.TrimSuffix(filepath.Base(p.File), filepath.Ext(p.File))
		out = filepath.Join(dir, "exports", base+"-"+stamp()+".zip")
	}
	missing, err := story.ZipProject(p.File, out)
	if err != nil {
		return nil, err
	}
	c.Printf("Wrote %s", out)
	for _, m := range missing {
		c.Printf("\n  missing: %s", m)
	}
	return map[string]any{"zip": out, "missing": missing}, nil
}

func cmdProjectCleanup(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	plan, err := story.Cleanup(s, c.Bool("dry-run"))
	if err != nil {
		return nil, err
	}
	verb := "Renamed"
	if c.Bool("dry-run") {
		verb = "Would rename"
	}
	c.Printf("%s %d files, drop %d links and %d audio refs, trash %d unused files\n", verb, len(plan.Renames), len(plan.DroppedLinks), len(plan.DroppedAudio), len(plan.Trash))
	for _, r := range plan.Renames {
		c.Printf("  %s -> %s\n", r.From, r.To)
	}
	for _, t := range plan.Trash {
		c.Printf("  trash %s\n", t)
	}
	return map[string]any{"dryRun": c.Bool("dry-run"), "plan": plan}, nil
}

func cmdRecentList(c *Ctx) (any, error) {
	prefs, err := story.LoadPrefs()
	if err != nil {
		return nil, err
	}
	out := []any{}
	for _, v := range prefs.Arr("recentDocuments") {
		d, ok := v.(*ojson.Object)
		if !ok {
			continue
		}
		e := d.Clone()
		describeRecent(e)
		out = append(out, e)
		c.Printf("%-30s %-6s %s\n", e.Str("title"), e.Str("mode"), e.Str("filename"))
	}
	if len(out) == 0 {
		c.Printf("No recent projects.")
	}
	return map[string]any{"recent": out}, nil
}

// describeRecent adds live facts to a recent entry: exists, mode (manga, film,
// script), pages and boards counts, and a thumbnail path (first page for
// manga, else first board). The stored metadata is not changed.
func describeRecent(e *ojson.Object) {
	file := e.Str("filename")
	e.Set("exists", story.Exists(file))
	e.Set("pages", 0)
	e.Set("boards", 0)
	e.Set("thumbnail", nil)
	if !story.Exists(file) {
		e.Set("mode", nil)
		return
	}
	p := &story.Project{File: file}
	if p.IsScript() {
		e.Set("mode", "script")
		refs, _, err := p.SceneRefs()
		if err != nil {
			return
		}
		n := 0
		for _, r := range refs {
			if !r.Exists {
				continue
			}
			if s, err := story.LoadScene(r.File); err == nil {
				if n == 0 && len(s.Boards()) > 0 {
					e.Set("thumbnail", s.ImagePath(story.ThumbnailFile(s.Boards()[0])))
				}
				n += len(s.Boards())
			}
		}
		e.Set("boards", n)
		return
	}
	s, err := story.LoadScene(file)
	if err != nil {
		e.Set("mode", nil)
		return
	}
	e.Set("mode", map[bool]string{true: "manga", false: "film"}[s.IsManga()])
	e.Set("pages", len(s.Pages()))
	e.Set("boards", len(s.Boards()))
	switch {
	case s.IsManga() && len(s.Pages()) > 0:
		e.Set("thumbnail", s.ImagePath(story.ThumbnailFile(s.Pages()[0])))
	case len(s.Boards()) > 0:
		e.Set("thumbnail", s.ImagePath(story.ThumbnailFile(s.Boards()[0])))
	}
}

func cmdRecentAdd(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	p, err := story.FindProject(c.Arg(0))
	if err != nil {
		return nil, err
	}
	meta := ojson.Obj("boards", 2, "time", 3000)
	if p.IsScript() {
		if sc, err := p.Script(); err == nil {
			meta = scriptMeta(p, sc)
		}
	}
	if err := story.AddRecent(p.File, meta); err != nil {
		return nil, err
	}
	c.Printf("Added %s", p.File)
	return map[string]any{"added": p.File}, nil
}

func cmdRecentPrune(c *Ctx) (any, error) {
	removed, err := story.PruneRecent()
	if err != nil {
		return nil, err
	}
	if removed == nil {
		removed = []string{}
	}
	c.Printf("Removed %d entries", len(removed))
	return map[string]any{"removed": removed}, nil
}

func cmdSceneList(c *Ctx) (any, error) {
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	refs, err := p.Scenes()
	if err != nil {
		return nil, err
	}
	for _, r := range refs {
		folder := r.Folder
		if !r.Exists && p.IsScript() {
			folder = "(no storyboards yet)"
		}
		c.Printf("%3d  %-6s %-40s %s\n", r.Number, msToTime(float64(r.Duration)), r.Slugline, folder)
		if r.Synopsis != "" {
			c.Printf("       %s\n", r.Synopsis)
		}
	}
	return map[string]any{"scenes": refs}, nil
}

func cmdSceneOpen(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	if !p.IsScript() {
		return nil, fmt.Errorf("scene open needs a script project (.fountain/.fdx)")
	}
	var n int
	if _, err := fmt.Sscan(c.Arg(0), &n); err != nil || n < 1 {
		return nil, usagef("scene number expected")
	}
	if _, err := p.EnsureSceneIDs(); err != nil {
		return nil, err
	}
	s, ref, err := p.OpenScene(n, prefsFps(), true)
	if err != nil {
		return nil, err
	}
	settings, _ := p.Settings()
	settings.Set("lastScene", n-1)
	if err := p.SaveSettings(settings); err != nil {
		return nil, err
	}
	c.Printf("Scene %d: %s\n  %s (%d boards)", n, ref.Slugline, s.Path, len(s.Boards()))
	return map[string]any{"scene": ref, "info": sceneInfo(s)}, nil
}

// scriptLines builds the script panel lines of one scene (renderScript).
func scriptLines(n script.Node) []story.ScriptLine {
	notes := n.Slugline
	if n.Synopsis != "" {
		notes += "\n" + n.Synopsis
	}
	lines := []story.ScriptLine{{Line: 1, Type: "slugline", Text: n.Slugline, Duration: n.Duration, Notes: notes}}
	strip := func(s string) string { return strings.TrimSpace(reTags.ReplaceAllString(s, " ")) }
	for _, it := range n.Script {
		l := story.ScriptLine{Line: len(lines) + 1, Type: it.Type, Duration: it.Duration}
		switch it.Type {
		case "action":
			l.Text = strip(it.Text)
			l.Action = l.Text
		case "dialogue":
			l.Text = strip(it.Character) + ": " + strip(it.Text)
			l.Dialogue = l.Text
		case "transition":
			l.Text = strip(it.Text)
			l.Notes = l.Text
		default:
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

func currentScriptScene(p *story.Project, arg string) (script.Node, error) {
	sc, err := p.Script()
	if err != nil {
		return script.Node{}, err
	}
	n := sceneArg
	if arg != "" {
		if _, err := fmt.Sscan(arg, &n); err != nil {
			return script.Node{}, usagef("scene number expected")
		}
	}
	if n <= 0 {
		settings, err := p.Settings()
		if err != nil {
			return script.Node{}, err
		}
		n = int(settings.NumOr("lastScene", 0)) + 1
	}
	node, ok := sc.Scene(n)
	if !ok {
		return node, fmt.Errorf("scene %d not found", n)
	}
	return node, nil
}

func cmdSceneScript(c *Ctx) (any, error) {
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	node, err := currentScriptScene(p, c.Arg(0))
	if err != nil {
		return nil, err
	}
	lines := scriptLines(node)
	c.Printf("SCENE %d - %s\n", node.SceneNumber, msToTime(float64(node.Duration)))
	for _, l := range lines {
		c.Printf("%3d  %-10s %s\n", l.Line, l.Type, l.Text)
	}
	return map[string]any{"scene": node.SceneNumber, "slugline": node.Slugline, "lines": lines}, nil
}

func cmdSceneRenumber(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	return boardList(c, s), nil
}

func cmdSceneSetFps(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	f, err := parseFloatArg(c.Arg(0), "fps")
	if err != nil {
		return nil, err
	}
	if err := checkFps(f); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	s.Data.Set("fps", f)
	if err := s.Save(); err != nil {
		return nil, err
	}
	if prefs, err := story.LoadPrefs(); err == nil {
		prefs.Set("lastUsedFps", f)
		story.SavePrefs(prefs)
	}
	c.Printf("fps set to %v", f)
	return map[string]any{"fps": f}, nil
}

func cmdSceneSetDefault(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	ms, err := parseFloatArg(c.Arg(0), "duration")
	if err != nil || ms <= 0 {
		return nil, usagef("duration must be a positive number of ms")
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	s.Data.Set("defaultBoardTiming", ms)
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("default board duration set to %v ms", ms)
	return map[string]any{"defaultBoardTiming": ms}, nil
}

func scriptArg(c *Ctx) (*story.Project, error) {
	if c.Arg(0) != "" {
		return story.FindProject(c.Arg(0))
	}
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	if !p.IsScript() {
		return nil, usagef("give a script file or run in a script project")
	}
	return p, nil
}

func cmdScriptParse(c *Ctx) (any, error) {
	p, err := scriptArg(c)
	if err != nil {
		return nil, err
	}
	sc, err := p.Script()
	if err != nil {
		return nil, err
	}
	for _, n := range sc.Scenes() {
		c.Printf("%3d  %-8s %-6s %-40s %d words\n", n.SceneNumber, n.SceneID, msToTime(float64(n.Duration)), n.Slugline, n.WordCount)
	}
	return sc, nil
}

func cmdScriptInfo(c *Ctx) (any, error) {
	p, err := scriptArg(c)
	if err != nil {
		return nil, err
	}
	sc, err := p.Script()
	if err != nil {
		return nil, err
	}
	n := len(sc.Scenes())
	c.Printf("%s\n  %d scenes, %s estimated", sc.Title, n, msToTime(float64(sc.TotalTime())))
	return map[string]any{"title": sc.Title, "scenes": n, "totalTime": sc.TotalTime(), "format": sc.Format, "file": p.File}, nil
}

func cmdScriptLocations(c *Ctx) (any, error) {
	p, err := scriptArg(c)
	if err != nil {
		return nil, err
	}
	sc, err := p.Script()
	if err != nil {
		return nil, err
	}
	for _, l := range sc.Locations {
		c.Printf("%4d  %s\n", l.Count, l.Name)
	}
	return map[string]any{"locations": sc.Locations}, nil
}

func cmdScriptCharacters(c *Ctx) (any, error) {
	p, err := scriptArg(c)
	if err != nil {
		return nil, err
	}
	sc, err := p.Script()
	if err != nil {
		return nil, err
	}
	for _, l := range sc.Characters {
		c.Printf("%4d  %s\n", l.Count, l.Name)
	}
	return map[string]any{"characters": sc.Characters}, nil
}

func cmdScriptAddIDs(c *Ctx) (any, error) {
	p, err := scriptArg(c)
	if err != nil {
		return nil, err
	}
	added, err := p.EnsureSceneIDs()
	if err != nil {
		return nil, err
	}
	if added {
		c.Printf("Added scene ids to %s. Reload it in your editor.", p.File)
	} else {
		c.Printf("All scenes already have ids.")
	}
	return map[string]any{"changed": added, "file": p.File}, nil
}

func cmdScriptInit(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	p, err := story.FindProject(c.Arg(0))
	if err != nil {
		return nil, err
	}
	if !p.IsScript() {
		return nil, usagef("script init needs a .fountain or .fdx file")
	}
	aspect := 1.7777777777777777
	if v := c.Flag("aspect"); v != "" {
		if aspect, err = parseAspect(v); err != nil {
			return nil, err
		}
	}
	if story.Exists(p.SettingsPath()) {
		return nil, fmt.Errorf("%s already exists", p.SettingsPath())
	}
	if _, err := p.EnsureSceneIDs(); err != nil {
		return nil, err
	}
	sc, err := p.Script()
	if err != nil {
		return nil, err
	}
	if sc.Format == "fountain" {
		numbered := 0
		for _, n := range sc.Scenes() {
			if n.SceneNumber > 0 {
				numbered++
			}
		}
		if numbered == 0 {
			return nil, fmt.Errorf("could not find any numbered scenes in this Fountain script")
		}
	}
	if err := os.MkdirAll(p.StoryboardsDir(), 0o755); err != nil {
		return nil, err
	}
	if err := p.SaveSettings(ojson.Obj("lastScene", 0, "aspectRatio", aspect)); err != nil {
		return nil, err
	}
	story.AddRecent(p.File, scriptMeta(p, sc))
	c.Printf("Created %s (%d scenes)", p.SettingsPath(), len(sc.Scenes()))
	return map[string]any{"settings": p.SettingsPath(), "scenes": len(sc.Scenes()), "aspectRatio": aspect}, nil
}

func cmdScriptWatch(c *Ctx) (any, error) {
	p, err := scriptArg(c)
	if err != nil {
		return nil, err
	}
	interval := 1000.0
	if f, ok, err := c.Float("interval"); err != nil {
		return nil, err
	} else if ok {
		interval = f
	}
	st, err := os.Stat(p.File)
	if err != nil {
		return nil, err
	}
	last := st.ModTime()
	if !c.JSON {
		fmt.Printf("Watching %s (Ctrl-C to stop)\n", p.File)
	}
	for {
		time.Sleep(time.Duration(interval) * time.Millisecond)
		st, err := os.Stat(p.File)
		if err != nil || st.ModTime().Equal(last) {
			continue
		}
		p.EnsureSceneIDs()
		st, _ = os.Stat(p.File)
		last = st.ModTime()
		sc, err := p.Script()
		ev := map[string]any{"event": "change", "file": p.File, "time": time.Now().UnixMilli()}
		if err != nil {
			ev["error"] = err.Error()
		} else {
			refs, _, _ := p.SceneRefs()
			ev["scenes"] = refs
			ev["title"] = sc.Title
		}
		if c.Bool("once") {
			if err == nil {
				c.Printf("%s changed: %v scenes", filepath.Base(p.File), len(sc.Scenes()))
			}
			return ev, nil
		}
		if c.JSON {
			printJSON(ev)
		} else if err != nil {
			fmt.Printf("%s changed: %v\n", filepath.Base(p.File), err)
		} else {
			fmt.Printf("%s changed: %v scenes\n", filepath.Base(p.File), len(sc.Scenes()))
		}
	}
}
