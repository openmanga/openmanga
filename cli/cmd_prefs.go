package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"sb/internal/ojson"
	"sb/internal/story"
)

func init() {
	register(
		&Cmd{Path: "prefs get", Args: "[key]", Short: "Read pref.json, or one dotted key (e.g. toolbar.tools.pen.color)", Run: cmdPrefsGet},
		&Cmd{Path: "prefs set", Args: "<key> <value>", Short: "Write a pref.json key; values are parsed as JSON (true, 2000, \"text\") else kept as text",
			Long: "Known keys: enableTooltips, enableAutoSave, defaultBoardTiming (2000), lastUsedFps (24),\nstraightLineDelayInMsecs (650), enableHighQualityDrawingEngine, enableDrawingSoundEffects,\nenableDrawingMelodySoundEffects, enableUISoundEffects, enableHighQualityAudio, enableBoardAudition,\nenableNotifications, enableAspirationalMessages, allowNotificationsForLineMileage, enableDiagnostics,\npomodoroTimerMinutes (25). Deprecated keys kept for compatibility: enableCanvasPaintingOpacity,\nenableStabilizer, enableBrushCursor, importTargetLayer.",
			Run:  cmdPrefsSet},
		&Cmd{Path: "prefs set-tool", Args: "<tool>", Short: "Toolbar tool settings stored in prefs: color, 3 palette swatches, stroke opacity",
			Long:  "Tools: light-pencil, brush, tone, pencil, pen, note-pen, eraser.",
			Flags: []string{"color=#rrggbb", "palette=c1,c2,c3 as #rrggbb", "opacity=stroke opacity 0.05-1", "captions=true|false toolbar captions toggle"}, Run: cmdPrefsSetTool},
		&Cmd{Path: "prefs migrate", Short: "Merge new default prefs when pref.json comes from an older version", Run: cmdPrefsMigrate},
		&Cmd{Path: "prefs path", Short: "Location of pref.json (override the user-data folder with SB_USER_DATA)", Run: cmdPrefsPath},

		&Cmd{Path: "keymap list", Short: "All 89 default key bindings (87 from the original app plus P panel and B balloon tools) merged with keymap.json", Run: cmdKeymapList},
		&Cmd{Path: "keymap set", Args: "<command> <accelerator>", Short: `Set a binding, e.g. sb keymap set menu:boards:new-board "Shift+n"`, Run: cmdKeymapSet},
		&Cmd{Path: "keymap migrate", Short: "Migrate 1.5.x tool keys and the 1.7.1 Shift pan key, then write the merged keymap", Run: cmdKeymapMigrate},

		&Cmd{Path: "lang list", Short: "Languages (en-US, ru-RU, zh-CN plus custom) and the selected one", Run: cmdLangList},
		&Cmd{Path: "lang set", Args: "<code>", Short: "Select the UI language", Run: cmdLangSet},
		&Cmd{Path: "lang copy", Args: "<code>", Short: "Duplicate a language as an editable custom language",
			Flags: []string{"name=display name (default: \"<name> copy\")"}, Run: cmdLangCopy},
		&Cmd{Path: "lang edit", Args: "<code> <key> <value>", Short: "Change one string of a custom language (dotted key, e.g. menu.file.open)", Run: cmdLangEdit},
		&Cmd{Path: "lang import", Args: "<file.json>", Short: "Import a locale JSON as a custom language and select it", Run: cmdLangImport},
		&Cmd{Path: "lang export", Args: "<code> <dir>", Short: "Write a language as <dir>/<name>.json", Run: cmdLangExport},
		&Cmd{Path: "lang remove", Args: "<code>", Short: "Remove a custom language", Run: cmdLangRemove},

		&Cmd{Path: "doctor", Short: "Check ffmpeg (-version), ffprobe and the print command", Run: cmdDoctor},
		&Cmd{Path: "app log-path", Short: "Where the original app writes its log file", Run: cmdLogPath},
		&Cmd{Path: "help urls", Short: "Website, getting started, FAQ and issue tracker links", Run: cmdHelpURLs},
		&Cmd{Path: "timelapse list", Short: "Sprint timelapse recordings (userData recordings.json)", Run: cmdTimelapse},
		&Cmd{Path: "tip", Short: "A random story tip", Flags: []string{"all list every tip"}, Run: cmdTip},
	)
}

func cmdPrefsGet(c *Ctx) (any, error) {
	p, err := story.LoadPrefs()
	if err != nil {
		return nil, err
	}
	if c.Arg(0) == "" {
		c.Printf("%s", ojson.Stringify(p, "  "))
		return p, nil
	}
	v, ok := story.PrefsGetPath(p, c.Arg(0))
	if !ok {
		return nil, fmt.Errorf("pref %q is not set", c.Arg(0))
	}
	c.Printf("%s", ojson.Stringify(v, "  "))
	return map[string]any{"key": c.Arg(0), "value": v}, nil
}

func cmdPrefsSet(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	p, err := story.LoadPrefs()
	if err != nil {
		return nil, err
	}
	v := story.ParseValue(strings.Join(c.Pos[1:], " "))
	story.PrefsSetPath(p, c.Arg(0), v)
	if err := story.SavePrefs(p); err != nil {
		return nil, err
	}
	c.Printf("%s = %s", c.Arg(0), ojson.Stringify(v, ""))
	return map[string]any{"key": c.Arg(0), "value": v}, nil
}

func cmdPrefsSetTool(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	tool := c.Arg(0)
	if _, ok := story.ToolDefaults[tool]; !ok {
		return nil, usagef("unknown tool %q (%s)", tool, strings.Join(story.Tools, ", "))
	}
	p, err := story.LoadPrefs()
	if err != nil {
		return nil, err
	}
	base := "toolbar.tools." + tool + "."
	if v := c.Flag("color"); v != "" {
		n, err := story.ColorNumber(v)
		if err != nil {
			return nil, usagef("%v", err)
		}
		story.PrefsSetPath(p, base+"color", n)
	}
	if v := c.Flag("palette"); v != "" {
		parts := strings.Split(v, ",")
		if len(parts) != 3 {
			return nil, usagef("--palette needs 3 colors")
		}
		var arr []any
		for _, s := range parts {
			n, err := story.ColorNumber(s)
			if err != nil {
				return nil, usagef("%v", err)
			}
			arr = append(arr, n)
		}
		story.PrefsSetPath(p, base+"palette", arr)
	}
	if f, ok, err := c.Float("opacity"); err != nil {
		return nil, err
	} else if ok {
		if f < 0.05 || f > 1 {
			return nil, usagef("--opacity must be between 0.05 and 1")
		}
		story.PrefsSetPath(p, base+"strokeOpacity", f)
	}
	if v := c.Flag("captions"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, usagef("--captions expects true or false")
		}
		story.PrefsSetPath(p, "toolbar.captions", b)
	}
	if err := story.SavePrefs(p); err != nil {
		return nil, err
	}
	v, _ := story.PrefsGetPath(p, "toolbar.tools."+tool)
	c.Printf("%s: %s", tool, ojson.Stringify(v, ""))
	return map[string]any{"tool": tool, "settings": v}, nil
}

func cmdPrefsMigrate(c *Ctx) (any, error) {
	p, err := story.LoadPrefs()
	if err != nil {
		return nil, err
	}
	changed, added := story.MigratePrefs(p)
	if changed {
		if err := story.SavePrefs(p); err != nil {
			return nil, err
		}
		c.Printf("Migrated prefs to %s, added %d keys", story.AppVersion, len(added))
	} else {
		c.Printf("Prefs are up to date")
	}
	if added == nil {
		added = []string{}
	}
	return map[string]any{"migrated": changed, "added": added}, nil
}

func cmdPrefsPath(c *Ctx) (any, error) {
	c.Printf("%s", story.PrefsPath())
	return map[string]any{"path": story.PrefsPath(), "userData": story.UserDataDir()}, nil
}

func cmdKeymapList(c *Ctx) (any, error) {
	user, err := story.UserKeymap()
	if err != nil {
		return nil, err
	}
	m := story.MergedKeymap(user)
	for _, k := range m.Keys() {
		c.Printf("%-48s %s\n", k, m.Str(k))
	}
	return map[string]any{"keymap": m, "count": m.Len()}, nil
}

func cmdKeymapSet(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	user, err := story.UserKeymap()
	if err != nil {
		return nil, err
	}
	m := story.MergedKeymap(user)
	if !m.Has(c.Arg(0)) {
		return nil, fmt.Errorf("unknown command %q (see `sb keymap list`)", c.Arg(0))
	}
	m.Set(c.Arg(0), c.Arg(1))
	if err := story.SaveKeymap(m); err != nil {
		return nil, err
	}
	c.Printf("%s = %s", c.Arg(0), c.Arg(1))
	return map[string]any{"command": c.Arg(0), "accelerator": c.Arg(1)}, nil
}

func cmdKeymapMigrate(c *Ctx) (any, error) {
	user, err := story.UserKeymap()
	if err != nil {
		return nil, err
	}
	notes := story.MigrateKeymap(user)
	if err := story.SaveKeymap(story.MergedKeymap(user)); err != nil {
		return nil, err
	}
	if len(notes) == 0 {
		c.Printf("Nothing to migrate; wrote the merged keymap")
		notes = []string{}
	}
	for _, n := range notes {
		c.Printf("%s\n", n)
	}
	return map[string]any{"changes": notes, "path": story.KeymapPath()}, nil
}

func cmdLangList(c *Ctx) (any, error) {
	langs, err := story.Languages()
	if err != nil {
		return nil, err
	}
	for _, l := range langs {
		mark := " "
		if l.Selected {
			mark = "*"
		}
		kind := "custom"
		if l.BuiltIn {
			kind = "built-in"
		}
		c.Printf("%s %-50s %-12s %s\n", mark, l.FileName, l.DisplayName, kind)
	}
	return map[string]any{"languages": langs}, nil
}

func cmdLangSet(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	l, err := story.FindLanguage(c.Arg(0))
	if err != nil {
		return nil, err
	}
	s, err := story.LangSettings()
	if err != nil {
		return nil, err
	}
	s.Set("selectedLanguage", l.FileName)
	if err := story.SaveLangSettings(s); err != nil {
		return nil, err
	}
	c.Printf("Language: %s", l.DisplayName)
	return map[string]any{"selected": l.FileName}, nil
}

func cmdLangCopy(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	l, err := story.FindLanguage(c.Arg(0))
	if err != nil {
		return nil, err
	}
	data, err := story.LocaleJSON(l)
	if err != nil {
		return nil, err
	}
	name := c.Flag("name")
	if name == "" {
		langs, _ := story.Languages()
		name = l.DisplayName + " copy"
		for i := 1; ; i++ {
			taken := false
			for _, x := range langs {
				if x.DisplayName == name {
					taken = true
				}
			}
			if !taken {
				break
			}
			name = fmt.Sprintf("%s copy%d", l.DisplayName, i)
		}
	}
	data.Set("Name", name)
	nl, err := story.AddCustomLanguage(name, data)
	if err != nil {
		return nil, err
	}
	c.Printf("Created %s (%s) and selected it", nl.DisplayName, nl.FileName)
	return nl, nil
}

func cmdLangEdit(c *Ctx) (any, error) {
	if err := c.Need(3); err != nil {
		return nil, err
	}
	l, err := story.FindLanguage(c.Arg(0))
	if err != nil {
		return nil, err
	}
	if l.BuiltIn {
		return nil, fmt.Errorf("built-in languages are read-only; run `sb lang copy %s` first", l.FileName)
	}
	data, err := story.LocaleJSON(l)
	if err != nil {
		return nil, err
	}
	val := strings.Join(c.Pos[2:], " ")
	story.PrefsSetPath(data, c.Arg(1), val)
	if err := story.SaveCustomLocale(l.FileName, data); err != nil {
		return nil, err
	}
	if c.Arg(1) == "Name" {
		story.RenameCustomLanguage(l.FileName, val)
	}
	c.Printf("%s: %s = %s", l.DisplayName, c.Arg(1), val)
	return map[string]any{"language": l.FileName, "key": c.Arg(1), "value": val}, nil
}

func cmdLangImport(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(c.Arg(0))
	if err != nil {
		return nil, err
	}
	data, err := ojson.ParseObject(raw)
	if err != nil {
		return nil, err
	}
	if data.Str("Name") == "" {
		data.Set("Name", "NewLanguage")
	}
	base := strings.TrimSuffix(filepath.Base(c.Arg(0)), filepath.Ext(c.Arg(0)))
	nl, err := story.AddCustomLanguage(base, data)
	if err != nil {
		return nil, err
	}
	c.Printf("Imported %s (%s) and selected it", nl.DisplayName, nl.FileName)
	return nl, nil
}

func cmdLangExport(c *Ctx) (any, error) {
	if err := c.Need(2); err != nil {
		return nil, err
	}
	l, err := story.FindLanguage(c.Arg(0))
	if err != nil {
		return nil, err
	}
	data, err := story.LocaleJSON(l)
	if err != nil {
		return nil, err
	}
	out := filepath.Join(c.Arg(1), story.StripUUID(l.FileName)+".json")
	if story.Exists(out) {
		out = filepath.Join(c.Arg(1), story.StripUUID(l.FileName)+" new.json")
	}
	if err := story.WriteJSON(out, data, ""); err != nil {
		return nil, err
	}
	c.Printf("Wrote %s", out)
	return map[string]any{"path": out}, nil
}

func cmdLangRemove(c *Ctx) (any, error) {
	if err := c.Need(1); err != nil {
		return nil, err
	}
	l, err := story.FindLanguage(c.Arg(0))
	if err != nil {
		return nil, err
	}
	if err := story.RemoveCustomLanguage(l); err != nil {
		return nil, err
	}
	c.Printf("Removed %s", l.DisplayName)
	return map[string]any{"removed": l.FileName}, nil
}

var reFfmpegVersion = regexp.MustCompile(`^ffmpeg version (\S+)`)

func cmdDoctor(c *Ctx) (any, error) {
	res := map[string]any{"os": runtime.GOOS, "userData": story.UserDataDir()}
	ok := true
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		out, err := exec.Command(p, "-version").Output()
		m := reFfmpegVersion.FindStringSubmatch(string(out))
		if err != nil || m == nil {
			res["ffmpeg"] = map[string]any{"path": p, "ok": false}
			ok = false
		} else {
			res["ffmpeg"] = map[string]any{"path": p, "version": m[1], "ok": true}
		}
	} else {
		res["ffmpeg"] = map[string]any{"ok": false, "error": "ffmpeg not found on PATH (needed for export video)"}
		ok = false
	}
	if p, err := exec.LookPath("ffprobe"); err == nil {
		res["ffprobe"] = map[string]any{"path": p, "ok": true}
	} else {
		res["ffprobe"] = map[string]any{"ok": false, "error": "ffprobe not found (audio durations only for WAV)"}
	}
	printer := "lpr"
	if runtime.GOOS == "linux" {
		printer = "lp"
	}
	if p, err := exec.LookPath(printer); err == nil {
		res["print"] = map[string]any{"path": p, "ok": true}
	} else {
		res["print"] = map[string]any{"ok": false, "error": printer + " not found"}
	}
	res["ok"] = ok
	for _, k := range []string{"ffmpeg", "ffprobe", "print"} {
		c.Printf("%-8s %v\n", k, res[k])
	}
	return res, nil
}

func cmdLogPath(c *Ctx) (any, error) {
	home, _ := os.UserHomeDir()
	var p string
	switch runtime.GOOS {
	case "darwin":
		p = filepath.Join(home, "Library", "Logs", "Storyboarder", "log.log")
	case "windows":
		p = filepath.Join(os.Getenv("APPDATA"), "Storyboarder", "logs", "log.log")
	default:
		p = filepath.Join(home, ".config", "Storyboarder", "logs", "log.log")
	}
	c.Printf("%s", p)
	return map[string]any{"path": p, "exists": story.Exists(p)}, nil
}

func cmdHelpURLs(c *Ctx) (any, error) {
	urls := []map[string]string{
		{"name": "Learn more", "url": "https://wonderunit.com/storyboarder"},
		{"name": "Getting started", "url": "https://wonderunit.com/storyboarder/faq/#How-do-I-get-started"},
		{"name": "FAQ", "url": "https://wonderunit.com/storyboarder/faq"},
		{"name": "Submit a bug", "url": "https://github.com/wonderunit/storyboarder/issues/new"},
	}
	for _, u := range urls {
		c.Printf("%-16s %s\n", u["name"], u["url"])
	}
	return map[string]any{"urls": urls}, nil
}

func cmdTimelapse(c *Ctx) (any, error) {
	list, err := story.Timelapses()
	if err != nil {
		return nil, err
	}
	for _, t := range list {
		c.Printf("%v\n", t["path"])
	}
	if len(list) == 0 {
		c.Printf("No timelapses recorded.")
	}
	return map[string]any{"timelapses": list}, nil
}

func cmdTip(c *Ctx) (any, error) {
	if c.Bool("all") {
		tips := story.Tips()
		for i, t := range tips {
			c.Printf("%d. %s\n\n", i+1, t)
		}
		return map[string]any{"tips": tips}, nil
	}
	t := story.RandomTip()
	c.Printf("%s", t)
	return map[string]any{"tip": t}, nil
}
