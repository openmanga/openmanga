package story

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sb/internal/ojson"
	"sb/internal/script"
)

// Project is either a single scene (.storyboarder) or a script project
// (.fountain/.fdx with storyboards/storyboard.settings and one folder per scene).
type Project struct {
	File string // absolute path of the .storyboarder or the script
}

var ErrNoProject = errors.New("no project: pass --project <file> or run inside a folder with a .storyboarder file")

// FindProject resolves a --project value (file or directory, "" = cwd).
func FindProject(arg string) (*Project, error) {
	if arg == "" {
		arg = "."
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		switch strings.ToLower(filepath.Ext(abs)) {
		case ".storyboarder", ".fountain", ".fdx":
			return &Project{File: abs}, nil
		}
		return nil, fmt.Errorf("unsupported project file %s (expected .storyboarder, .fountain or .fdx)", abs)
	}
	entries, _ := os.ReadDir(abs)
	var sb, scripts []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".storyboarder":
			sb = append(sb, filepath.Join(abs, e.Name()))
		case ".fountain", ".fdx":
			scripts = append(scripts, filepath.Join(abs, e.Name()))
		}
	}
	if len(sb) > 0 {
		sort.Strings(sb)
		return &Project{File: sb[0]}, nil
	}
	if _, err := os.Stat(filepath.Join(abs, "storyboards", "storyboard.settings")); err == nil && len(scripts) > 0 {
		// prefer .fountain, and skip empty files
		sort.Slice(scripts, func(i, j int) bool {
			fi, fj := filepath.Ext(scripts[i]) == ".fountain", filepath.Ext(scripts[j]) == ".fountain"
			if fi != fj {
				return fi
			}
			return scripts[i] < scripts[j]
		})
		for _, s := range scripts {
			if st, err := os.Stat(s); err == nil && st.Size() > 0 {
				return &Project{File: s}, nil
			}
		}
	}
	return nil, ErrNoProject
}

func (p *Project) IsScript() bool {
	ext := strings.ToLower(filepath.Ext(p.File))
	return ext == ".fountain" || ext == ".fdx"
}

func (p *Project) Root() string           { return filepath.Dir(p.File) }
func (p *Project) StoryboardsDir() string { return filepath.Join(p.Root(), "storyboards") }
func (p *Project) SettingsPath() string {
	return filepath.Join(p.StoryboardsDir(), "storyboard.settings")
}

// Settings reads storyboard.settings.
func (p *Project) Settings() (*ojson.Object, error) {
	data, err := os.ReadFile(p.SettingsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("this script is not part of a Storyboarder project yet: run `sb script init %s --aspect <ratio>`", p.File)
		}
		return nil, err
	}
	o, err := ojson.ParseObject(data)
	if err != nil {
		return nil, err
	}
	if !o.Has("lastScene") {
		o.Set("lastScene", 0)
	}
	return o, nil
}

// SaveSettings writes storyboard.settings (compact, as main.js does).
func (p *Project) SaveSettings(o *ojson.Object) error {
	return WriteJSON(p.SettingsPath(), o, "")
}

// EnsureSceneIDs adds scene ids to the script like the app does on open.
// It rewrites the script file only when ids were missing.
func (p *Project) EnsureSceneIDs() (bool, error) {
	data, err := os.ReadFile(p.File)
	if err != nil {
		return false, err
	}
	if strings.ToLower(filepath.Ext(p.File)) == ".fdx" {
		out, added := script.InsertFDXSceneIDs(data, script.UID)
		if len(added) == 0 {
			return false, nil
		}
		return true, os.WriteFile(p.File, out, 0o644)
	}
	out, changed := script.InsertFountainSceneIDs(string(data), script.UID)
	if !changed {
		return false, nil
	}
	return true, os.WriteFile(p.File, []byte(out), 0o644)
}

// Script parses the project's script.
func (p *Project) Script() (*script.Parsed, error) {
	if !p.IsScript() {
		return nil, fmt.Errorf("%s is not a script project", filepath.Base(p.File))
	}
	return script.Load(p.File)
}

// SceneRef ties a script scene to its folder.
type SceneRef struct {
	Number   int    `json:"number"`
	ID       string `json:"id"`
	Slugline string `json:"slugline"`
	Synopsis string `json:"synopsis,omitempty"`
	Duration int    `json:"duration"`
	Folder   string `json:"folder,omitempty"`
	File     string `json:"file,omitempty"`
	Exists   bool   `json:"exists"`
}

// SceneRefs maps every script scene to its folder (existing or not).
func (p *Project) SceneRefs() ([]SceneRef, *script.Parsed, error) {
	sc, err := p.Script()
	if err != nil {
		return nil, nil, err
	}
	dirs := map[string]string{}
	entries, _ := os.ReadDir(p.StoryboardsDir())
	for _, e := range entries {
		if !e.IsDir() || strings.HasSuffix(strings.ToLower(e.Name()), "-backup") {
			continue
		}
		parts := strings.Split(e.Name(), "-")
		dirs[parts[len(parts)-1]] = e.Name()
	}
	var refs []SceneRef
	for _, n := range sc.Scenes() {
		id := script.SceneFolderID(n.SceneID)
		if n.SceneID == "" {
			id = "G1" // main-window loadScene fallback
		}
		r := SceneRef{Number: n.SceneNumber, ID: n.SceneID, Slugline: n.Slugline, Synopsis: n.Synopsis, Duration: n.Duration}
		if dir, ok := dirs[id]; ok {
			r.Folder = dir
			r.File = filepath.Join(p.StoryboardsDir(), dir, dir+".storyboarder")
			_, err := os.Stat(r.File)
			r.Exists = err == nil
		} else {
			r.Folder = script.SceneFolderName(n)
			r.File = filepath.Join(p.StoryboardsDir(), r.Folder, r.Folder+".storyboarder")
		}
		refs = append(refs, r)
	}
	return refs, sc, nil
}

// OpenScene returns the scene for a script scene number, creating its folder,
// .storyboarder and first board when missing (main-window loadScene + ensureBoardExists).
// number <= 0 means the settings' lastScene.
func (p *Project) OpenScene(number int, fps float64, create bool) (*Scene, SceneRef, error) {
	settings, err := p.Settings()
	if err != nil {
		return nil, SceneRef{}, err
	}
	if number <= 0 {
		last, _ := settings.Num("lastScene")
		number = int(last) + 1
	}
	refs, _, err := p.SceneRefs()
	if err != nil {
		return nil, SceneRef{}, err
	}
	for _, r := range refs {
		if r.Number != number {
			continue
		}
		if r.Exists {
			os.MkdirAll(filepath.Join(filepath.Dir(r.File), "images"), 0o755)
			s, err := LoadScene(r.File)
			return s, r, err
		}
		if !create {
			return nil, r, fmt.Errorf("scene %d has no storyboard folder yet (run `sb scene open %d`)", number, number)
		}
		if err := os.MkdirAll(filepath.Join(filepath.Dir(r.File), "images"), 0o755); err != nil {
			return nil, r, err
		}
		data := ojson.Obj(
			"version", AppVersion,
			"aspectRatio", settings.Get("aspectRatio"),
			"fps", fps,
			"defaultBoardTiming", 2000,
			"boards", []any{},
		)
		if err := WriteJSON(r.File, data, "  "); err != nil {
			return nil, r, err
		}
		s, err := LoadScene(r.File)
		if err != nil {
			return nil, r, err
		}
		if err := EnsureBoardExists(s); err != nil {
			return nil, r, err
		}
		r.Exists = true
		return s, r, nil
	}
	return nil, SceneRef{}, fmt.Errorf("scene %d not found in %s", number, filepath.Base(p.File))
}

// EnsureBoardExists adds a blank board to an empty scene and saves it.
func EnsureBoardExists(s *Scene) error {
	if len(s.Boards()) > 0 {
		return nil
	}
	if _, err := InsertBlankBoard(s, 0); err != nil {
		return err
	}
	return s.Save()
}

// Scenes returns every scene file of the project, in script order for script projects.
func (p *Project) Scenes() ([]SceneRef, error) {
	if !p.IsScript() {
		return []SceneRef{{Number: 1, File: p.File, Exists: true, Slugline: trimExt(filepath.Base(p.File))}}, nil
	}
	refs, _, err := p.SceneRefs()
	return refs, err
}
