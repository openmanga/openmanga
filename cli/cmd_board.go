package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"sb/internal/manga"
	"sb/internal/ojson"
	"sb/internal/render"
	"sb/internal/story"
)

var reTags = regexp.MustCompile(`<[^>]+>`)

func init() {
	register(
		&Cmd{Path: "board list", Short: "Boards: number, uid, shot, duration, dialogue, audio, has 3D shot", Run: cmdBoardList},
		&Cmd{Path: "board get", Args: "<i>", Short: "The full board object as stored in the .storyboarder", Run: cmdBoardGet},
		&Cmd{Path: "board info", Args: "<i>", Short: `"Shot X", "Board N of M", line miles, duration and text of a board`, Run: cmdBoardInfo},
		&Cmd{Path: "board add", Short: "Add a blank board (default: at the end)",
			Flags: []string{"after=insert after board <i>", "before=insert before board <i>", "end append at the end", "size=own canvas size WxH px, e.g. 900x2400 for a tall drawing (default: 900 px high x project aspect)", "like-panel=<page>:<panel>: canvas with that panel's box aspect, long side 1800 px (manga)", "place with --like-panel: also show the new board in that panel (fit fill)"}, Run: cmdBoardAdd},
		&Cmd{Path: "board delete", Args: "<i..>", Short: "Delete boards (at least one board always stays); files stay on disk", Run: cmdBoardDelete},
		&Cmd{Path: "board duplicate", Args: "<i>", Short: "Copy a board after itself under a new uid with its layers; text, audio and new-shot are cleared", Run: cmdBoardDuplicate},
		&Cmd{Path: "board move", Args: "<i..>", Short: "Reorder boards: --by -1/+1 shifts, --to <j> puts the first selected board at number j",
			Flags: []string{"by=shift by n positions", "to=target board number"}, Run: cmdBoardMove},
		&Cmd{Path: "board set-new-shot", Args: "<i..> <on|off>", Short: "Set newShot, which starts a new shot in the numbering", Run: cmdBoardNewShot},
		&Cmd{Path: "board set-duration", Args: "<i..> [ms]", Short: "Set duration in ms, or --frames converted with the scene fps; --clear uses the scene default",
			Flags: []string{"frames=duration in frames", "clear remove the board duration (use the scene default)"}, Run: cmdBoardDuration},
		&Cmd{Path: "board suggest-duration", Args: "<i>", Short: "Suggested duration from dialogue: words x 300 ms + 300",
			Flags: []string{"apply also set it as the board duration"}, Run: cmdBoardSuggest},
		&Cmd{Path: "board set-name", Args: "<i> <text>", Short: "Set the board's name (shown in the boards list; the original app ignores it)", Run: textSetter("name")},
		&Cmd{Path: "board set-description", Args: "<i> <text>", Short: "Set the board's description (separate from notes)", Run: textSetter("description")},
		&Cmd{Path: "board set-dialogue", Args: "<i> <text>", Short: "Set the board dialogue", Run: textSetter("dialogue")},
		&Cmd{Path: "board set-action", Args: "<i> <text>", Short: "Set the board action", Run: textSetter("action")},
		&Cmd{Path: "board set-notes", Args: "<i> <text>", Short: "Set the board notes", Run: textSetter("notes")},
		&Cmd{Path: "board set-from-script", Args: "<i> <line>", Short: `Copy a script line (see "sb scene script") to a board: dialogue as "CHAR: text", action, notes, duration`, Run: cmdBoardFromScript},
		&Cmd{Path: "board path", Args: "<i>", Short: "Absolute path of a board file (default: images/ folder) for Finder or external tools",
			Flags: []string{"layer=layer name (shot-generator, reference, fill, tone, pencil, ink, notes)", "file=one of: thumbnail, posterframe, camera-plot, audio"}, Run: cmdBoardPath},
		&Cmd{Path: "board export-clip", Args: "<i..>", Short: "Copy boards: write {boards, layerDataByBoardIndex} JSON with base64 layers",
			Flags: []string{"out=output file (default: stdout)"}, Run: cmdBoardExportClip},
		&Cmd{Path: "board cut", Args: "<i..>", Short: "Cut boards: export-clip then delete",
			Flags: []string{"out=output file (required)"}, Run: cmdBoardCut},
		&Cmd{Path: "board paste", Short: "Insert boards from a clip JSON, or one image file as a new board on the reference layer",
			Flags: []string{"from=clip .json or image file (required)", "after=insert after board <i> (default: at the end)", "before=insert before board <i>"}, Run: cmdBoardPaste},
		&Cmd{Path: "board replace", Args: "<i>", Short: "Paste and replace: the board's layers become the clip's (one board) or the image (reference)",
			Flags: []string{"from=clip .json or image file (required)"}, Run: cmdBoardReplace},
		&Cmd{Path: "board flip", Args: "<i..>", Short: "Mirror all layers horizontally (default) or --vertical",
			Flags: []string{"horizontal mirror left/right (default)", "vertical mirror top/bottom"}, Run: cmdBoardFlip},
		&Cmd{Path: "board transform", Args: "<i>", Short: "Move and/or scale all layers; scale is around --anchor x,y (default: image center)",
			Flags: []string{"dx=move right by px", "dy=move down by px", "scale=scale factor", "anchor=x,y in image pixels"}, Run: cmdBoardTransform},
		&Cmd{Path: "board erase-region", Args: "<i>", Short: "Lasso erase: clear a polygon on all layers",
			Flags: []string{`polygon=points "x,y x,y x,y" or JSON [[x,y],...] in image pixels`}, Run: cmdBoardErase},
		&Cmd{Path: "board fill-region", Args: "<i>", Short: "Lasso fill: paint a polygon on the fill layer",
			Flags: []string{"polygon=points in image pixels", "color=#rrggbb (default: brush tool color from prefs)", "opacity=0-1 (default 1)"}, Run: cmdBoardFill},
		&Cmd{Path: "board move-region", Args: "<i>", Short: "Lasso move: cut a polygon from all layers and paste it offset",
			Flags: []string{"polygon=points in image pixels", "dx=px", "dy=px"}, Run: cmdBoardMoveRegion},
		&Cmd{Path: "board render", Args: "<i..>", Short: "Rebuild posterframe (JPG) and thumbnail (PNG), or write a flattened PNG with --out",
			Flags: []string{"posterframe only the posterframe", "thumbnail only the thumbnail", "out=write the flattened full-size PNG here (one board)", "width=with --out: output width (height keeps aspect)", "layer=with --out: render only this layer (on white)", "grid with --out: overlay a labelled coordinate grid", "grid-step=grid spacing px (default 100)"}, Run: cmdBoardRender},

		&Cmd{Path: "layer list", Args: "[board]", Short: "The six drawing layers (plus 3D/derived ones present) with file, opacity, exists", Flags: []string{"page=a manga page instead of a board"}, Run: cmdLayerList},
		&Cmd{Path: "layer replace", Args: "<i> <layer> <image>", Short: "Fit an image into a layer (reference gets opacity 1)", Run: cmdLayerReplace},
		&Cmd{Path: "layer clear", Args: "<i> [layer]", Short: "Clear one layer, or --all layers (a board that is already empty is deleted)",
			Flags: []string{"all clear every layer"}, Run: cmdLayerClear},
		&Cmd{Path: "layer merge", Args: "<i>", Short: "Merge reference and fill: --into reference (merge down) or --into fill (merge up)",
			Flags: []string{"into=reference or fill"}, Run: cmdLayerMerge},
		&Cmd{Path: "layer set-opacity", Args: "[board..] <layer> <0-1>", Short: "Set a drawing layer's opacity (defaults: reference 0.75, others 1; reference also sets the 3D layer)", Flags: []string{"page=a manga page instead of boards"}, Run: cmdLayerOpacity},

		&Cmd{Path: "audio set", Args: "<i> <file>", Short: "Copy wav/mp3/m4a/mp4 as images/<uid>-<name> and set board.audio (duration via ffprobe)",
			Flags: []string{"force overwrite an existing file of the same name"}, Run: cmdAudioSet},
		&Cmd{Path: "audio clear", Args: "<i..>", Short: "Remove board audio (the file stays on disk)", Run: cmdAudioClear},
		&Cmd{Path: "audio refresh", Short: "Recompute audio.duration for every board", Run: cmdAudioRefresh},

		&Cmd{Path: "import images", Args: "<files or folders..>", Short: "One new board per PNG/JPG (folders recursive) on the reference layer; or one new page per image (--pages); or one image into a panel (--page --panel)",
			Flags: []string{"after=insert after board <i> (with --pages: after page <p>; default: at the end)", "pages manga: one new page per image, on the page reference layer (opacity 1)", "fit=with --pages: fit (inside the page, default) or fill (cover it)", "page=manga page for --panel", "panel=put one image on the page reference layer, fitted inside this panel and clipped to it (layer opacity 0.75)"}, Run: cmdImportImages},
	)
}

func boardRow(s *story.Scene, i int, b *ojson.Object) map[string]any {
	row := map[string]any{
		"number":      i + 1,
		"uid":         story.UID(b),
		"shot":        b.Str("shot"),
		"newShot":     b.Bool("newShot"),
		"duration":    story.Duration(s, b),
		"time":        b.NumOr("time", 0),
		"dialogue":    b.Str("dialogue"),
		"action":      b.Str("action"),
		"notes":       b.Str("notes"),
		"hasAudio":    b.Obj("audio") != nil,
		"has3D":       b.Get("sg") != nil,
		"layers":      story.OrderedLayers(b),
		"name":        b.Str("name"),
		"description": b.Str("description"),
		"thumbnail":   s.ImagePath(story.ThumbnailFile(b)),
		"posterframe": s.ImagePath(story.PosterframeFile(b)),
	}
	w, h := story.SizeOf(s, b)
	row["size"] = []int{w, h}
	if !b.Has("duration") {
		row["durationIsDefault"] = true
	}
	return row
}

func boardList(c *Ctx, s *story.Scene) map[string]any {
	story.UpdateTiming(s)
	var rows []map[string]any
	for i, b := range s.Boards() {
		rows = append(rows, boardRow(s, i, b))
		flags := ""
		if b.Obj("audio") != nil {
			flags += " [audio]"
		}
		if b.Get("sg") != nil {
			flags += " [3D]"
		}
		c.Printf("%3d  %-5s %-5s %6s%s  %s\n", i+1, story.UID(b), b.Str("shot"), msToTime(story.Duration(s, b)), flags, b.Str("dialogue"))
	}
	return map[string]any{"scene": s.Path, "boards": rows}
}

func cmdBoardList(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	return boardList(c, s), nil
}

func cmdBoardGet(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	_, b, err := oneBoard(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	c.Printf("%s", ojson.Stringify(b, "  "))
	return b, nil
}

func cmdBoardInfo(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	story.UpdateTiming(s)
	i, b, err := oneBoard(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	row := boardRow(s, i, b)
	miles := b.NumOr("lineMileage", 0) / 5280
	row["lineMiles"] = miles
	row["frames"] = story.MsToFrames(story.Duration(s, b), s.Fps())
	row["of"] = len(s.Boards())
	if b.Str("dialogue") != "" {
		row["suggestedDuration"] = story.SuggestedDuration(b)
	}
	used := []map[string]any{}
	for pi, pg := range s.Pages() {
		for _, p := range manga.Panels(pg) {
			if ct := p.Obj("content"); ct != nil && ct.Str("board") == story.UID(b) {
				used = append(used, map[string]any{"page": pi + 1, "pageId": pg.Str("id"), "panel": p.Str("id")})
			}
		}
	}
	row["usedIn"] = used
	c.Printf("Shot: %s\nBoard: %d of %d\n%.1f line miles\nDuration: %v ms (%v frames)\n", b.Str("shot"), i+1, len(s.Boards()), miles, story.Duration(s, b), row["frames"])
	for _, k := range []string{"dialogue", "action", "notes"} {
		if v := b.Str(k); v != "" {
			c.Printf("%s: %s\n", strings.Title(k), v)
		}
	}
	return row, nil
}

func insertPosition(c *Ctx, s *story.Scene) (int, error) {
	n := len(s.Boards())
	if v := c.Flag("after"); v != "" {
		i, _, err := oneBoard(s, v)
		return i + 1, err
	}
	if v := c.Flag("before"); v != "" {
		i, _, err := oneBoard(s, v)
		return i, err
	}
	return n, nil
}

func cmdBoardAdd(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	pos, err := insertPosition(c, s)
	if err != nil {
		return nil, err
	}
	var bw, bh int
	if v := c.Flag("size"); v != "" {
		if bw, bh, err = parseSize(v); err != nil {
			return nil, err
		}
	}
	var likePage, likePanel *ojson.Object
	if v := c.Flag("like-panel"); v != "" {
		pref, panelRef, ok := strings.Cut(v, ":")
		if !ok {
			return nil, usagef("--like-panel expects <page>:<panel>, e.g. 2:K3")
		}
		if err := manga.RequireManga(s); err != nil {
			return nil, err
		}
		if _, likePage, err = manga.FindPage(s, pref); err != nil {
			return nil, err
		}
		if likePanel, err = manga.FindPanel(likePage, panelRef); err != nil {
			return nil, err
		}
		box, _ := manga.PanelArea(s, likePanel)
		bw, bh = manga.LikeSize(box, 1800)
	} else if c.Bool("place") {
		return nil, usagef("--place needs --like-panel")
	}
	b, err := story.InsertSizedBoard(s, pos, bw, bh)
	if err != nil {
		return nil, err
	}
	res := boardRow(s, pos, b)
	w, h := story.SizeOf(s, b)
	res["size"] = []int{w, h}
	if likePanel != nil && c.Bool("place") {
		likePanel.Set("content", ojson.Obj("board", story.UID(b), "x", 0, "y", 0, "scale", 1, "rotation", 0, "fit", "fill"))
		if err := manga.RefreshPage(s, likePage); err != nil {
			return nil, err
		}
		res["placedIn"] = likePage.Str("id") + ":" + likePanel.Str("id")
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("Added board %d (%s), %dx%d", pos+1, story.UID(b), w, h)
	if p, ok := res["placedIn"]; ok {
		c.Printf(", shown in panel %s", p)
	}
	return res, nil
}

func cmdBoardDelete(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	idx, err := boardIndexes(s, strings.Join(c.Pos, ","))
	if err != nil {
		return nil, err
	}
	n := story.DeleteBoards(s, idx)
	if n == 0 {
		return nil, fmt.Errorf("cannot delete: a scene has to keep at least one board")
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("Deleted %d board(s); %d left", n, len(s.Boards()))
	return map[string]any{"deleted": n, "boards": len(s.Boards())}, nil
}

func cmdBoardDuplicate(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	i, _, err := oneBoard(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	b, err := story.DuplicateBoard(s, i)
	if err != nil {
		return nil, err
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("Duplicated board %d as board %d (%s)", i+1, i+2, story.UID(b))
	return boardRow(s, i+1, b), nil
}

func cmdBoardMove(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	idx, err := boardIndexes(s, strings.Join(c.Pos, ","))
	if err != nil {
		return nil, err
	}
	first := sortedInts(idx)[0]
	var to int
	switch {
	case c.Flag("by") != "":
		by, err := strconv.Atoi(c.Flag("by"))
		if err != nil {
			return nil, usagef("--by expects an integer")
		}
		to = first + by
	case c.Flag("to") != "":
		j, err := strconv.Atoi(c.Flag("to"))
		if err != nil {
			return nil, usagef("--to expects a board number")
		}
		to = j - 1
	default:
		return nil, usagef("give --by <n> or --to <j>")
	}
	story.MoveBoards(s, sortedInts(idx), to)
	if err := s.Save(); err != nil {
		return nil, err
	}
	return boardList(c, s), nil
}

func cmdBoardNewShot(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	val := strings.ToLower(c.Pos[len(c.Pos)-1])
	on := val == "on" || val == "true" || val == "1"
	if !on && val != "off" && val != "false" && val != "0" {
		return nil, usagef("last argument must be on or off")
	}
	idx, err := boardIndexes(s, strings.Join(c.Pos[:len(c.Pos)-1], ","))
	if err != nil {
		return nil, err
	}
	for _, i := range idx {
		s.Boards()[i].Set("newShot", on)
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	return boardList(c, s), nil
}

func cmdBoardDuration(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	refs := c.Pos
	var ms float64
	clear := c.Bool("clear")
	if f, ok, err := c.Float("frames"); err != nil {
		return nil, err
	} else if ok {
		ms = story.FramesToMs(f, s.Fps())
	} else if !clear {
		if len(c.Pos) < 2 {
			return nil, usagef("give <i..> <ms>, --frames <n> or --clear")
		}
		if ms, err = parseFloatArg(c.Pos[len(c.Pos)-1], "duration"); err != nil {
			return nil, err
		}
		refs = c.Pos[:len(c.Pos)-1]
	}
	idx, err := boardIndexes(s, strings.Join(refs, ","))
	if err != nil {
		return nil, err
	}
	for _, i := range idx {
		if clear {
			s.Boards()[i].Delete("duration")
		} else {
			s.Boards()[i].Set("duration", ms)
		}
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	return boardList(c, s), nil
}

func cmdBoardSuggest(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	_, b, err := oneBoard(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	if b.Str("dialogue") == "" {
		return nil, fmt.Errorf("board has no dialogue")
	}
	d := story.SuggestedDuration(b)
	if c.Bool("apply") {
		b.Set("duration", d)
		if err := s.Save(); err != nil {
			return nil, err
		}
	}
	c.Printf("about %v seconds (%d ms)", float64(d)/1000, d)
	return map[string]any{"suggestedDuration": d, "applied": c.Bool("apply")}, nil
}

func textSetter(key string) func(*Ctx) (any, error) {
	return func(c *Ctx) (any, error) {
		if err := c.Need(2); err != nil {
			return nil, err
		}
		s, err := openScene()
		if err != nil {
			return nil, err
		}
		i, b, err := oneBoard(s, c.Arg(0))
		if err != nil {
			return nil, err
		}
		b.Set(key, strings.Join(c.Pos[1:], " "))
		if err := s.Save(); err != nil {
			return nil, err
		}
		return boardRow(s, i, b), nil
	}
}

func cmdBoardFromScript(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	node, err := currentScriptScene(p, "")
	if err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	i, b, err := oneBoard(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(c.Arg(1))
	lines := scriptLines(node)
	if err != nil || n < 1 || n > len(lines) {
		return nil, usagef("line must be 1..%d (see `sb scene script`)", len(lines))
	}
	changed := story.ApplyScriptLine(b, lines[n-1])
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("Board %d: set %s", i+1, strings.Join(changed, ", "))
	return map[string]any{"board": boardRow(s, i, b), "changed": changed}, nil
}

func cmdBoardPath(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	_, b, err := oneBoard(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	path := s.ImagesDir()
	if l := c.Flag("layer"); l != "" {
		if !story.ValidLayer(l) {
			return nil, usagef("unknown layer %q", l)
		}
		name := story.LayerFile(b, l)
		if lo := story.Layer(b, l); lo != nil {
			name = lo.Str("url")
		}
		path = s.ImagePath(name)
	}
	switch c.Flag("file") {
	case "":
	case "thumbnail":
		path = s.ImagePath(story.ThumbnailFile(b))
	case "posterframe":
		path = s.ImagePath(story.PosterframeFile(b))
	case "camera-plot":
		path = s.ImagePath(story.CameraPlotFile(b))
	case "audio":
		if b.Obj("audio") == nil {
			return nil, fmt.Errorf("board has no audio")
		}
		path = s.ImagePath(b.Obj("audio").Str("filename"))
	default:
		return nil, usagef("--file must be thumbnail, posterframe, camera-plot or audio")
	}
	c.Printf("%s", path)
	return map[string]any{"path": path, "exists": story.Exists(path)}, nil
}

func cmdBoardExportClip(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	idx, err := boardIndexes(s, strings.Join(c.Pos, ","))
	if err != nil {
		return nil, err
	}
	clip, err := story.ExportClip(s, sortedInts(idx))
	if err != nil {
		return nil, err
	}
	data := ojson.Stringify(clip, "  ")
	if out := c.Flag("out"); out != "" {
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return nil, err
		}
		c.Printf("Wrote %d board(s) to %s", len(idx), out)
		return map[string]any{"out": out, "boards": len(idx)}, nil
	}
	if c.JSON {
		return clip, nil
	}
	c.Printf("%s", data)
	return nil, nil
}

func cmdBoardCut(c *Ctx) (any, error) {
	if c.Flag("out") == "" {
		return nil, usagef("--out <file> is required for cut")
	}
	res, err := cmdBoardExportClip(c)
	if err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	idx, _ := boardIndexes(s, strings.Join(c.Pos, ","))
	n := story.DeleteBoards(s, idx)
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Text.Reset()
	c.Printf("Cut %d board(s) to %s", n, c.Flag("out"))
	m := res.(map[string]any)
	m["deleted"] = n
	return m, nil
}

func cmdBoardPaste(c *Ctx) (any, error) {
	if c.Flag("from") == "" {
		return nil, usagef("--from <clip.json|image> is required")
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	clip, err := story.ReadClip(c.Flag("from"))
	if err != nil {
		return nil, err
	}
	pos, err := insertPosition(c, s)
	if err != nil {
		return nil, err
	}
	added, err := story.PasteClip(s, clip, pos)
	if err != nil {
		return nil, err
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("Pasted %d board(s) at %d\n", len(added), pos+1)
	return boardList(c, s), nil
}

func cmdBoardReplace(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	if c.Flag("from") == "" {
		return nil, usagef("--from <clip.json|image> is required")
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	i, b, err := oneBoard(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	clip, err := story.ReadClip(c.Flag("from"))
	if err != nil {
		return nil, err
	}
	if err := story.ReplaceFromClip(s, b, clip); err != nil {
		return nil, err
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("Replaced the layers of board %d", i+1)
	return boardRow(s, i, b), nil
}

func eachBoard(c *Ctx, fn func(s *story.Scene, b *ojson.Object) error) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	idx, err := boardIndexes(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	for _, i := range idx {
		if err := fn(s, s.Boards()[i]); err != nil {
			return nil, err
		}
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	var rows []map[string]any
	for _, i := range idx {
		rows = append(rows, boardRow(s, i, s.Boards()[i]))
	}
	c.Printf("Updated %d board(s)", len(idx))
	return map[string]any{"boards": rows}, nil
}

func cmdBoardFlip(c *Ctx) (any, error) {
	vertical := c.Bool("vertical")
	return eachBoard(c, func(s *story.Scene, b *ojson.Object) error {
		return story.EditLayers(s, b, story.DrawingLayers, func(_ string, img *image.RGBA) *image.RGBA {
			return render.Flip(img, vertical)
		})
	})
}

func cmdBoardTransform(c *Ctx) (any, error) {
	dx, _, err := c.Float("dx")
	if err != nil {
		return nil, err
	}
	dy, _, err := c.Float("dy")
	if err != nil {
		return nil, err
	}
	scale, ok, err := c.Float("scale")
	if err != nil {
		return nil, err
	}
	if !ok {
		scale = 1
	}
	return eachBoard(c, func(s *story.Scene, b *ojson.Object) error {
		w, h := story.SizeOf(s, b)
		ax, ay := float64(w)/2, float64(h)/2
		if a := c.Flag("anchor"); a != "" {
			if _, err := fmt.Sscanf(a, "%g,%g", &ax, &ay); err != nil {
				return usagef("--anchor expects x,y")
			}
		}
		return story.EditLayers(s, b, story.DrawingLayers, func(_ string, img *image.RGBA) *image.RGBA {
			return render.Transform(img, dx, dy, scale, ax, ay)
		})
	})
}

func polygonFlag(c *Ctx) ([][2]float64, error) {
	if c.Flag("polygon") == "" {
		return nil, usagef("--polygon is required")
	}
	pts, err := story.ParsePolygon(c.Flag("polygon"))
	if err != nil {
		return nil, usagef("--polygon: %v", err)
	}
	return pts, nil
}

func cmdBoardErase(c *Ctx) (any, error) {
	pts, err := polygonFlag(c)
	if err != nil {
		return nil, err
	}
	return eachBoard(c, func(s *story.Scene, b *ojson.Object) error { return story.EraseRegion(s, b, pts) })
}

func cmdBoardFill(c *Ctx) (any, error) {
	pts, err := polygonFlag(c)
	if err != nil {
		return nil, err
	}
	hex := c.Flag("color")
	if hex == "" {
		hex = toolColor("brush")
	}
	col, err := render.ParseColor(hex)
	if err != nil {
		return nil, usagef("%v", err)
	}
	op, ok, err := c.Float("opacity")
	if err != nil {
		return nil, err
	}
	if !ok {
		op = 1
	}
	return eachBoard(c, func(s *story.Scene, b *ojson.Object) error { return story.FillRegion(s, b, pts, col, op) })
}

// toolColor reads a tool color from prefs.toolbar or the toolbar default.
func toolColor(tool string) string {
	n := story.ToolDefaults[tool].Color
	if p, err := story.LoadPrefs(); err == nil {
		if v, ok := story.PrefsGetPath(p, "toolbar.tools."+tool+".color"); ok {
			if f, ok := ojson.Num(v); ok {
				n = int(f)
			}
		}
	}
	return fmt.Sprintf("#%06x", n)
}

func cmdBoardMoveRegion(c *Ctx) (any, error) {
	pts, err := polygonFlag(c)
	if err != nil {
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
	return eachBoard(c, func(s *story.Scene, b *ojson.Object) error { return story.MoveRegion(s, b, pts, dx, dy) })
}

func cmdBoardRender(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	idx, err := boardIndexes(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	if out := c.Flag("out"); out != "" {
		if len(idx) != 1 {
			return nil, usagef("--out renders one board")
		}
		b := s.Boards()[idx[0]]
		w, h := story.SizeOf(s, b)
		if c.Bool("grid") || c.Flag("layer") != "" {
			step := 100.0
			if f, ok, err := c.Float("grid-step"); err != nil {
				return nil, err
			} else if ok && f > 0 {
				step = f
			}
			if l := c.Flag("layer"); l != "" && !story.ValidLayer(l) {
				return nil, usagef("unknown layer %q", l)
			}
			t := &drawTarget{s: s, obj: b, index: idx[0], w: w, h: h}
			p, err := renderTarget(t, out, c.Bool("grid"), step, c.Flag("layer"))
			if err != nil {
				return nil, err
			}
			c.Printf("Wrote %s (%dx%d)", p, w, h)
			return map[string]any{"out": p, "width": w, "height": h}, nil
		}
		if f, ok, err := c.Float("width"); err != nil {
			return nil, err
		} else if ok {
			h = int(float64(h) * f / float64(w))
			w = int(f)
		}
		img, missing := story.Flatten(s, b, w, h)
		if err := render.SavePNG(out, img); err != nil {
			return nil, err
		}
		abs, _ := filepath.Abs(out)
		c.Printf("Wrote %s (%dx%d)", abs, w, h)
		return map[string]any{"out": abs, "width": w, "height": h, "missing": missing}, nil
	}
	var written []string
	var missing []string
	for _, i := range idx {
		b := s.Boards()[i]
		if !c.Bool("thumbnail") {
			m, err := story.SavePosterframe(s, b)
			if err != nil {
				return nil, err
			}
			missing = append(missing, m...)
			written = append(written, s.ImagePath(story.PosterframeFile(b)))
		}
		if !c.Bool("posterframe") {
			if _, err := story.SaveThumbnail(s, b); err != nil {
				return nil, err
			}
			written = append(written, s.ImagePath(story.ThumbnailFile(b)))
		}
	}
	for _, w := range written {
		c.Printf("%s\n", w)
	}
	return map[string]any{"written": written, "missingLayers": missing}, nil
}

func cmdLayerList(c *Ctx) (any, error) {
	t, err := resolveTarget(c)
	if err != nil {
		return nil, err
	}
	return layerRows(c, t)
}

// layerRows lists the six drawing layers (existing or not) plus any other
// present layer (shot-generator, frames, balloons), in composite order.
func layerRows(c *Ctx, t *drawTarget) (any, error) {
	rows := []map[string]any{}
	for _, n := range story.LayerOrder {
		l := story.Layer(t.obj, n)
		drawing := n != "shot-generator" && n != "frames" && n != "balloons"
		if l == nil && !drawing {
			continue
		}
		file := story.LayerFile(t.obj, n)
		if l != nil {
			file = l.Str("url")
		}
		exists := l != nil && story.Exists(t.s.ImagePath(file))
		row := map[string]any{"name": n, "file": file, "path": t.s.ImagePath(file), "opacity": story.LayerOpacity(t.obj, n), "exists": exists, "drawing": drawing,
			"undo": len(story.LayerHistory(t.s, t.obj, n)), "redo": story.RedoCount(t.s, t.obj, n)}
		if l != nil {
			if th, ok := l.Get("thumbnail").(string); ok {
				row["thumbnail"] = t.s.ImagePath(th)
			}
		}
		rows = append(rows, row)
		mark := " "
		if !exists {
			mark = "-"
		}
		c.Printf("%s %-15s %-5v %s\n", mark, n, story.LayerOpacity(t.obj, n), file)
	}
	return map[string]any{"target": t.label(), "layers": rows}, nil
}

func cmdLayerReplace(c *Ctx) (any, error) {
	if err := c.Need(3); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	i, b, err := oneBoard(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	if !story.ValidLayer(c.Arg(1)) {
		return nil, usagef("unknown layer %q", c.Arg(1))
	}
	if err := story.ReplaceLayerImage(s, b, c.Arg(1), c.Arg(2)); err != nil {
		return nil, err
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("Replaced %s layer of board %d", c.Arg(1), i+1)
	return boardRow(s, i, b), nil
}

func cmdLayerClear(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	i, b, err := oneBoard(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	if c.Bool("all") {
		if story.IsBoardEmpty(s, b) {
			if story.DeleteBoards(s, []int{i}) == 0 {
				return nil, fmt.Errorf("board is empty and is the last board")
			}
			if err := s.Save(); err != nil {
				return nil, err
			}
			c.Printf("Board %d was empty: deleted it", i+1)
			return map[string]any{"deleted": 1}, nil
		}
		if err := story.ClearLayers(s, b, story.DrawingLayers); err != nil {
			return nil, err
		}
		c.Printf("Cleared all layers of board %d", i+1)
	} else {
		name := c.Arg(1)
		if !story.ValidLayer(name) {
			return nil, usagef("give a layer name or --all")
		}
		if err := story.ClearLayers(s, b, []string{name}); err != nil {
			return nil, err
		}
		c.Printf("Cleared %s on board %d", name, i+1)
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	return boardRow(s, i, b), nil
}

func cmdLayerMerge(c *Ctx) (any, error) {
	into := c.Flag("into")
	if into != "reference" && into != "fill" {
		return nil, usagef("--into reference|fill is required")
	}
	return eachBoard(c, func(s *story.Scene, b *ojson.Object) error { return story.MergeLayers(s, b, into) })
}

func cmdLayerOpacity(c *Ctx) (any, error) {
	if len(c.Pos) < 2 {
		return nil, usagef("usage: sb layer set-opacity [board..] <layer> <0-1>")
	}
	layer, val := c.Pos[len(c.Pos)-2], c.Pos[len(c.Pos)-1]
	v, err := parseFloatArg(val, "opacity")
	if err != nil || v < 0 || v > 1 {
		return nil, usagef("opacity must be between 0 and 1")
	}
	c.Pos = c.Pos[:len(c.Pos)-2]
	if c.Flag("page") != "" {
		t, err := resolveTarget(c)
		if err != nil {
			return nil, err
		}
		if err := t.checkLayer(layer); err != nil {
			return nil, err
		}
		if err := story.SetLayerOpacity(t.s, t.obj, layer, v); err != nil {
			return nil, err
		}
		if err := t.s.Save(); err != nil {
			return nil, err
		}
		return layerRows(c, t)
	}
	if !story.ValidLayer(layer) {
		return nil, usagef("layer must be one of %s", strings.Join(story.DrawingLayers, ", "))
	}
	if len(c.Pos) == 0 {
		return nil, usagef("which board(s)?")
	}
	c.Pos = []string{strings.Join(c.Pos, ",")}
	return eachBoard(c, func(s *story.Scene, b *ojson.Object) error { return story.SetLayerOpacity(s, b, layer, v) })
}

func cmdAudioSet(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	i, b, err := oneBoard(s, c.Arg(0))
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(c.Arg(1))) {
	case ".wav", ".mp3", ".m4a", ".mp4":
	default:
		return nil, usagef("audio must be wav, mp3, m4a or mp4")
	}
	name, err := story.SetAudio(s, b, c.Arg(1), c.Bool("force"))
	if err != nil {
		return nil, err
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	c.Printf("Board %d audio: %s (%v ms)", i+1, name, story.AudioDuration(b))
	return map[string]any{"board": i + 1, "audio": b.Obj("audio")}, nil
}

func cmdAudioClear(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	c.Pos = []string{strings.Join(c.Pos, ",")}
	return eachBoard(c, func(s *story.Scene, b *ojson.Object) error { b.Delete("audio"); return nil })
}

func cmdAudioRefresh(c *Ctx) (any, error) {
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	d, failed := story.RefreshAudioDurations(s)
	if err := s.Save(); err != nil {
		return nil, err
	}
	for uid, ms := range d {
		c.Printf("%s %v ms\n", uid, ms)
	}
	for _, f := range failed {
		c.Printf("could not load audio file %s\n", f)
	}
	if failed == nil {
		failed = []string{}
	}
	return map[string]any{"durations": d, "failed": failed}, nil
}

func cmdImportImages(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	s, err := openScene()
	if err != nil {
		return nil, err
	}
	files, err := story.CollectImages(c.Pos)
	if err != nil {
		return nil, err
	}
	if c.Bool("pages") || c.Flag("page") != "" {
		return importToPages(c, s, files)
	}
	pos, err := insertPosition(c, s)
	if err != nil {
		return nil, err
	}
	added, failed, err := story.ImportImages(s, files, pos)
	if err != nil {
		return nil, err
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	var uids []string
	for _, b := range added {
		uids = append(uids, story.UID(b))
	}
	c.Printf("Imported %d image(s) on the reference layer", len(added))
	for _, f := range failed {
		c.Printf("\n  failed: %s", f)
	}
	if failed == nil {
		failed = []string{}
	}
	return map[string]any{"imported": len(added), "uids": uids, "failed": failed, "insertedAt": pos + 1}, nil
}

func importToPages(c *Ctx, s *story.Scene, files []string) (any, error) {
	if err := manga.RequireManga(s); err != nil {
		return nil, err
	}
	if id := c.Flag("panel"); id != "" {
		if len(files) != 1 {
			return nil, usagef("--panel takes exactly one image")
		}
		i, pg, err := manga.FindPage(s, c.Flag("page"))
		if err != nil {
			return nil, err
		}
		p, err := manga.FindPanel(pg, id)
		if err != nil {
			return nil, err
		}
		if err := manga.ImportIntoPanel(s, pg, p, files[0]); err != nil {
			return nil, err
		}
		if err := s.Save(); err != nil {
			return nil, err
		}
		file := s.ImagePath(story.Layer(pg, "reference").Str("url"))
		c.Printf("Put %s into panel %s of page %d (reference layer)", filepath.Base(files[0]), p.Str("id"), i+1)
		return map[string]any{"page": i + 1, "panel": p.Str("id"), "file": file, "posterframe": s.ImagePath(story.PosterframeFile(pg))}, nil
	}
	if !c.Bool("pages") {
		return nil, usagef("give --pages, or --page with --panel")
	}
	fit := c.Flag("fit")
	if fit != "" && fit != "fit" && fit != "fill" {
		return nil, usagef("--fit must be fit or fill")
	}
	pos := len(s.Pages())
	if v := c.Flag("after"); v != "" {
		i, _, err := manga.FindPage(s, v)
		if err != nil {
			return nil, err
		}
		pos = i + 1
	}
	added, failed, err := manga.ImportPages(s, files, pos, fit)
	if err != nil {
		return nil, err
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	var rows []map[string]any
	for k, pg := range added {
		rows = append(rows, pageRow(s, pos+k, pg))
	}
	if failed == nil {
		failed = []string{}
	}
	c.Printf("Imported %d image(s) as pages", len(added))
	return map[string]any{"imported": len(added), "pages": rows, "failed": failed, "insertedAt": pos + 1}, nil
}
