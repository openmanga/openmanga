package story

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"sb/internal/ojson"
	"sb/internal/render"
)

var reParseFloat = regexp.MustCompile(`^\s*[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?`)

// ParseFloatJS mimics parseFloat: the longest numeric prefix.
func ParseFloatJS(s string) (float64, bool) {
	m := reParseFloat.FindString(s)
	if m == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(m), 64)
	return f, err == nil
}

// MigrateReport describes what `project migrate` did.
type MigrateReport struct {
	StringDurations []string `json:"stringDurations"`
	LayersMigrated  []string `json:"layersMigrated"`
	Backup          string   `json:"backup,omitempty"`
	Skipped         string   `json:"skipped,omitempty"`
	Changed         bool     `json:"changed"`
}

// Migrate runs migrateScene: string durations to numbers, and pre-1.6
// single-PNG boards into the fill layer after a `<name>-backup` copy.
func Migrate(s *Scene) (MigrateReport, error) {
	rep := MigrateReport{StringDurations: []string{}, LayersMigrated: []string{}}
	for _, b := range s.Boards() {
		if str, ok := b.Get("duration").(string); ok {
			if f, ok := ParseFloatJS(str); ok {
				b.Set("duration", f)
				rep.StringDurations = append(rep.StringDurations, UID(b))
				rep.Changed = true
			}
		}
	}
	needs := false
	for _, b := range s.Boards() {
		if exists(s.ImagePath(URL(b))) && Layer(b, "fill") == nil {
			needs = true
			break
		}
	}
	if needs {
		folder := s.Dir()
		isBackup := regexp.MustCompile(`(?i)-backup$`).MatchString(folder)
		original := regexp.MustCompile(`(?i)-backup$`).ReplaceAllString(folder, "")
		if isBackup && exists(original) {
			rep.Skipped = "this appears to be a backup of a scene created with an older version; it will not be migrated"
		} else {
			dst := filepath.Join(filepath.Dir(folder), s.Name()+"-backup")
			if exists(dst) {
				return rep, fmt.Errorf("tried to migrate scene but a backup already exists: %s. Move or rename it and retry", dst)
			}
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return rep, err
			}
			if _, err := CopyProject(s.Path, dst, true, true); err != nil {
				return rep, err
			}
			rep.Backup = dst
			for _, b := range s.Boards() {
				if !exists(s.ImagePath(URL(b))) || Layer(b, "fill") != nil {
					continue
				}
				fill := LayerFile(b, "fill")
				if err := os.Rename(s.ImagePath(URL(b)), s.ImagePath(fill)); err != nil {
					return rep, err
				}
				Layers(b, true).Set("fill", ojson.Obj("url", fill))
				rep.LayersMigrated = append(rep.LayersMigrated, UID(b))
				rep.Changed = true
			}
		}
	}
	if rep.Changed {
		return rep, s.Save()
	}
	return rep, nil
}

// VerifyReport lists problems found by verifyScene.
type VerifyReport struct {
	MissingFiles        []string `json:"missingFiles"`
	MissingLinks        []string `json:"missingLinks"`
	MissingPosterframes []string `json:"missingPosterframes"`
	Fixed               bool     `json:"fixed"`
}

// Verify checks layer files, thumbnails, linked PSDs and posterframes.
// With fix it writes transparent placeholders, unlinks missing PSDs and
// regenerates posterframes (main-window verifyScene).
func Verify(s *Scene, fix bool) (VerifyReport, error) {
	rep := VerifyReport{MissingFiles: []string{}, MissingLinks: []string{}, MissingPosterframes: []string{}}
	for _, b := range s.Boards() {
		files := []string{}
		for _, n := range OrderedLayers(b) {
			files = append(files, Layer(b, n).Str("url"))
		}
		files = append(files, ThumbnailFile(b))
		for _, f := range files {
			if !exists(s.ImagePath(f)) {
				rep.MissingFiles = append(rep.MissingFiles, f)
			}
		}
		if link, ok := b.Get("link").(string); ok && !exists(s.ImagePath(link)) {
			rep.MissingLinks = append(rep.MissingLinks, link)
		}
		if !exists(s.ImagePath(PosterframeFile(b))) {
			rep.MissingPosterframes = append(rep.MissingPosterframes, PosterframeFile(b))
		}
	}
	if !fix {
		return rep, nil
	}
	os.MkdirAll(s.ImagesDir(), 0o755)
	w, h := s.ImageSize()
	for _, f := range rep.MissingFiles {
		// the original writes a blank full-size canvas for every missing PNG
		if err := render.SavePNG(s.ImagePath(f), render.New(w, h)); err != nil {
			return rep, err
		}
	}
	dirty := false
	for _, b := range s.Boards() {
		if link, ok := b.Get("link").(string); ok && !exists(s.ImagePath(link)) {
			b.Delete("link")
			dirty = true
		}
	}
	for _, b := range s.Boards() {
		if !exists(s.ImagePath(PosterframeFile(b))) {
			if _, err := SavePosterframe(s, b); err != nil {
				return rep, err
			}
		}
	}
	rep.Fixed = true
	if dirty {
		return rep, s.Save()
	}
	return rep, nil
}

// Rename is one file rename in a cleanup plan.
type Rename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// CleanupPlan is prepareCleanup plus the files that would be trashed.
type CleanupPlan struct {
	Renames      []Rename `json:"renames"`
	DroppedLinks []string `json:"droppedLinks"`
	DroppedAudio []string `json:"droppedAudio"`
	Trash        []string `json:"trash"`
	boards       []*ojson.Object
}

// PrepareCleanup renames files to index order (exporters/cleanup prepareCleanup).
func PrepareCleanup(s *Scene) CleanupPlan {
	orig := s.Boards()
	cleaned := make([]*ojson.Object, len(orig))
	for i, b := range orig {
		c := b.Clone()
		UpdateURLsFromIndex(c, i)
		if c.Get("link") != nil {
			c.Set("link", LinkFile(c))
		}
		cleaned[i] = c
	}
	var pairs [][2]string
	for i := range orig {
		o, c := orig[i], cleaned[i]
		for _, n := range OrderedLayers(o) {
			pairs = append(pairs, [2]string{Layer(o, n).Str("url"), Layer(c, n).Str("url")})
		}
	}
	for i := range orig {
		pairs = append(pairs, [2]string{ThumbnailFile(orig[i]), ThumbnailFile(cleaned[i])})
	}
	for i := range orig {
		if l, ok := orig[i].Get("link").(string); ok {
			pairs = append(pairs, [2]string{l, cleaned[i].Str("link")})
		}
	}
	for i := range orig {
		pairs = append(pairs, [2]string{PosterframeFile(orig[i]), PosterframeFile(cleaned[i])})
	}
	plan := CleanupPlan{Renames: []Rename{}, DroppedLinks: []string{}, DroppedAudio: []string{}, Trash: []string{}, boards: cleaned}
	for _, p := range pairs {
		if p[0] != p[1] {
			plan.Renames = append(plan.Renames, Rename{p[0], p[1]})
		}
	}
	return plan
}

// Cleanup plans (dryRun) or applies the cleanup: renames, dead link/audio removal,
// trashing of unused files in images/, then saves the scene.
func Cleanup(s *Scene, dryRun bool) (CleanupPlan, error) {
	plan := PrepareCleanup(s)
	renamed := map[string]string{}
	for _, r := range plan.Renames {
		if exists(s.ImagePath(r.From)) {
			renamed[r.To] = r.From
			if !dryRun {
				if err := os.Rename(s.ImagePath(r.From), s.ImagePath(r.To)); err != nil {
					return plan, err
				}
			}
		}
	}
	// in a dry run the files are still at their old names
	has := func(name string) bool {
		if dryRun {
			if from, ok := renamed[name]; ok {
				return exists(s.ImagePath(from))
			}
		}
		return exists(s.ImagePath(name))
	}
	for _, b := range plan.boards {
		if l, ok := b.Get("link").(string); ok && !has(l) {
			plan.DroppedLinks = append(plan.DroppedLinks, l)
			b.Delete("link")
		}
		if a := b.Obj("audio"); a != nil && !has(a.Str("filename")) {
			plan.DroppedAudio = append(plan.DroppedAudio, a.Str("filename"))
			b.Delete("audio")
		}
	}
	used := map[string]bool{}
	for _, b := range plan.boards {
		for _, f := range MediaFiles(b) {
			used[f] = true
		}
	}
	for _, pg := range s.Pages() {
		for _, f := range MediaFiles(pg) {
			used[f] = true
		}
	}
	entries, _ := os.ReadDir(s.ImagesDir())
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") { // images/.history and other hidden files
			continue
		}
		if dryRun {
			// map the current name to its post-rename name
			for to, from := range renamed {
				if from == name {
					name = to
				}
			}
		}
		if !used[name] {
			plan.Trash = append(plan.Trash, e.Name())
		}
	}
	if dryRun {
		return plan, nil
	}
	for _, f := range plan.Trash {
		if err := TrashFunc(s.ImagePath(f)); err != nil {
			return plan, err
		}
	}
	s.SetBoards(plan.boards)
	return plan, s.Save()
}

// SceneFiles lists absolute paths used by one scene (images/ media + custom 3D files).
func SceneFiles(sceneFile string, withMainImages bool) ([]string, error) {
	s, err := LoadScene(sceneFile)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, b := range s.Boards() {
		for _, f := range MediaFiles(b) {
			out = append(out, s.ImagePath(f))
		}
	}
	for _, pg := range s.Pages() {
		for _, f := range MediaFiles(pg) {
			out = append(out, s.ImagePath(f))
		}
	}
	for _, b := range s.Boards() {
		for _, f := range SGExportableFiles(b) {
			out = append(out, filepath.Join(s.Dir(), filepath.FromSlash(f)))
		}
	}
	if withMainImages {
		for _, b := range s.Boards() {
			if p := s.ImagePath(URL(b)); exists(p) {
				out = append(out, p)
			}
		}
	}
	return out, nil
}

// SGExportableFiles lists project-relative custom model files of a board's 3D scene
// (models/shot-generator-data getExportableMediaFilenames), plus image plane and
// volume textures stored under models/.
func SGExportableFiles(b *ojson.Object) []string {
	sg := b.Obj("sg")
	if sg == nil {
		return nil
	}
	data := sg.Obj("data")
	if data == nil {
		return nil
	}
	var out []string
	custom := func(s string) bool { return strings.Contains(s, "/") && filepath.Ext(s) != "" && !filepath.IsAbs(s) }
	if objs := data.Obj("sceneObjects"); objs != nil {
		for _, k := range objs.Keys() {
			o := objs.Obj(k)
			if o == nil {
				continue
			}
			if m := o.Str("model"); custom(m) {
				out = append(out, m)
			}
			for _, key := range []string{"imageAttachmentIds", "volumeImageAttachmentIds"} {
				for _, v := range o.Arr(key) {
					if p, ok := v.(string); ok && custom(p) {
						out = append(out, p)
					}
				}
			}
		}
	}
	if w := data.Obj("world"); w != nil {
		if env := w.Obj("environment"); env != nil {
			if f := env.Str("file"); custom(f) {
				out = append(out, f)
			}
		}
	}
	return out
}

// ProjectFiles is getFilesUsedByProject (absolute paths, project file excluded for scenes).
func ProjectFiles(p *Project, withMainImages bool) ([]string, error) {
	if !p.IsScript() {
		return SceneFiles(p.File, withMainImages)
	}
	if !exists(p.SettingsPath()) {
		return nil, fmt.Errorf("this script is not part of a Storyboarder project")
	}
	files := []string{p.SettingsPath()}
	entries, _ := os.ReadDir(p.StoryboardsDir())
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(p.StoryboardsDir(), e.Name())
		inner, _ := os.ReadDir(dir)
		for _, f := range inner {
			if filepath.Ext(f.Name()) == ".storyboarder" {
				sf := filepath.Join(dir, f.Name())
				used, err := SceneFiles(sf, withMainImages)
				if err != nil {
					return nil, err
				}
				files = append(files, sf)
				files = append(files, used...)
				break
			}
		}
	}
	return files, nil
}

// CopyProject copies the project file (renamed after the destination folder)
// and every used file (exporters/copy-project copyProject). dst must exist.
func CopyProject(src, dst string, withMainImages, ignoreMissing bool) ([]string, error) {
	p := &Project{File: src}
	files, err := ProjectFiles(p, withMainImages)
	if err != nil {
		return nil, err
	}
	if !exists(dst) {
		return nil, fmt.Errorf("ENOENT: could not find destination folder %s", dst)
	}
	srcDir := filepath.Dir(src)
	pairs := [][2]string{{src, filepath.Join(dst, filepath.Base(dst)+filepath.Ext(src))}}
	for _, f := range files {
		pairs = append(pairs, [2]string{f, strings.Replace(f, srcDir, dst, 1)})
	}
	missing := []string{}
	for _, pr := range pairs {
		if !exists(pr[0]) {
			missing = append(missing, pr[0])
			if !ignoreMissing {
				return missing, fmt.Errorf("ENOENT: could not find source file %s", pr[0])
			}
			continue
		}
		if err := CopyTree(pr[0], pr[1]); err != nil {
			return missing, err
		}
	}
	return missing, nil
}

// ZipProject copies the project into a temp folder and zips its contents
// at the archive root (exporters/archive exportAsZIP).
func ZipProject(src, zipPath string) ([]string, error) {
	tmp, err := os.MkdirTemp("", "sb-zip-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	dst := filepath.Join(tmp, trimExt(filepath.Base(src)))
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return nil, err
	}
	missing, err := CopyProject(src, dst, false, true)
	if err != nil {
		return missing, err
	}
	if err := os.MkdirAll(filepath.Dir(zipPath), 0o755); err != nil {
		return missing, err
	}
	f, err := os.Create(zipPath)
	if err != nil {
		return missing, err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	err = filepath.Walk(dst, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dst, p)
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
	if err != nil {
		return missing, err
	}
	return missing, zw.Close()
}
