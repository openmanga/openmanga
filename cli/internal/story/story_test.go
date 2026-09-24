package story

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sb/internal/ojson"
	"sb/internal/render"
)

// fixture copies testdata/<name> into a temp dir and returns the new folder.
func fixture(t *testing.T, name string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), name)
	if err := CopyTree(filepath.Join("..", "..", "testdata", name), dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestSceneRoundTripKeepsEverything(t *testing.T) {
	for _, f := range []string{"example/example.storyboarder", "ducks/ducks.storyboarder", "audio/audio.storyboarder", "old-scene/old-scene.storyboarder", "shot-generator/shot-generator.storyboarder"} {
		dir := fixture(t, strings.Split(f, "/")[0])
		path := filepath.Join(dir, filepath.Base(f))
		before, _ := os.ReadFile(path)
		s, err := LoadScene(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Save(); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(path)
		a, _ := ojson.ParseObject(before)
		b, _ := ojson.ParseObject(after)
		if b.Str("version") != AppVersion {
			t.Errorf("%s: version not stamped", f)
		}
		b.Set("version", a.Get("version"))
		// shot/number/time are recomputed on save (ducks has hand-edited stale values)
		for _, o := range []*ojson.Object{a, b} {
			for _, v := range o.Arr("boards") {
				for _, k := range []string{"shot", "number", "time"} {
					v.(*ojson.Object).Delete(k)
				}
			}
		}
		if string(ojson.Stringify(a, "  ")) != string(ojson.Stringify(b, "  ")) {
			t.Errorf("%s: content changed on a no-op save\n%s", f, after)
		}
		leftovers, _ := filepath.Glob(path + ".backup-*")
		if len(leftovers) != 0 {
			t.Errorf("backup file left behind: %v", leftovers)
		}
	}
}

func TestShotNumbering(t *testing.T) {
	s := &Scene{Data: ojson.Obj("defaultBoardTiming", "2001", "boards", []any{})}
	var boards []*ojson.Object
	for i := 0; i < 30; i++ {
		boards = append(boards, ojson.Obj("uid", "B", "newShot", i == 0 || i == 2))
	}
	boards[1].Set("duration", "1500") // old projects store strings
	s.SetBoards(boards)
	UpdateTiming(s)
	got := []string{}
	for _, b := range s.Boards() {
		got = append(got, b.Str("shot"))
	}
	if got[0] != "1A" || got[1] != "1B" || got[2] != "2A" || got[3] != "2B" || got[27] != "2Z" || got[28] != "2A2" {
		t.Errorf("shots: %v", got)
	}
	if tm, _ := s.Boards()[2].Num("time"); tm != 2001+1500 {
		t.Errorf("time of board 3 = %v", tm)
	}
	if n, _ := s.Boards()[29].Num("number"); n != 30 {
		t.Errorf("number = %v", n)
	}
	// without any newShot every board is its own shot
	for _, b := range boards {
		b.Set("newShot", false)
	}
	UpdateTiming(s)
	if s.Boards()[4].Str("shot") != "5A" {
		t.Errorf("no-shot numbering: %s", s.Boards()[4].Str("shot"))
	}
}

func TestDurations(t *testing.T) {
	s := &Scene{Data: ojson.Obj("defaultBoardTiming", "2001", "boards", []any{
		ojson.Obj("uid", "A"),
		ojson.Obj("uid", "B", "duration", 500, "audio", ojson.Obj("filename", "x.wav", "duration", 3000)),
	})}
	UpdateTiming(s)
	if d := Duration(s, s.Boards()[0]); d != 2001 {
		t.Errorf("default duration %v", d)
	}
	if d := SceneDuration(s); d != 2001+3000 {
		t.Errorf("scene duration incl. audio %v", d)
	}
	if FramesToMs(36, 24) != 1500 || MsToFrames(1500, 24) != 36 {
		t.Error("frame conversion")
	}
	if SuggestedDuration(ojson.Obj("dialogue", "one two  three")) != 1200 {
		t.Error("suggested duration")
	}
}

func TestFlattenPixels(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "images"), 0o755)
	path := filepath.Join(dir, "p.storyboarder")
	WriteJSON(path, ojson.Obj("aspectRatio", 1, "fps", 24, "defaultBoardTiming", 2000, "boards", []any{
		ojson.Obj("uid", "AAAAA", "url", "board-1-AAAAA.png", "layers", ojson.Obj(
			"reference", ojson.Obj("url", "board-1-AAAAA-reference.png", "opacity", 0.5),
			"fill", ojson.Obj("url", "board-1-AAAAA-fill.png"),
		)),
	}), "  ")
	s, _ := LoadScene(path)
	red := render.NewFilled(900, 900, color.RGBA{255, 0, 0, 255})
	render.SavePNG(s.ImagePath("board-1-AAAAA-reference.png"), red)
	fill := render.New(900, 900)
	for y := 0; y < 900; y++ {
		for x := 450; x < 900; x++ {
			fill.Set(x, y, color.RGBA{0, 0, 255, 255})
		}
	}
	render.SavePNG(s.ImagePath("board-1-AAAAA-fill.png"), fill)
	img, missing := Flatten(s, s.Boards()[0], 900, 900)
	if len(missing) != 0 {
		t.Fatal(missing)
	}
	// left: red at 50% over white; right: opaque blue on top
	if c := img.RGBAAt(100, 100); c.R != 255 || c.G < 126 || c.G > 128 || c.B < 126 || c.B > 128 {
		t.Errorf("left pixel %v", c)
	}
	if c := img.RGBAAt(800, 100); c != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("right pixel %v", c)
	}
	// thumbnail is 120 high, floor(60*aspect)*2 wide
	SaveThumbnail(s, s.Boards()[0])
	th, _ := render.Load(s.ImagePath("board-1-AAAAA-thumbnail.png"))
	if th.Bounds() != image.Rect(0, 0, 120, 120) {
		t.Errorf("thumbnail size %v", th.Bounds())
	}
}

func TestCleanupDucks(t *testing.T) {
	dir := fixture(t, "ducks")
	images := filepath.Join(dir, "images")
	os.MkdirAll(images, 0o755)
	for _, f := range []string{"board-2-42VR9.png", "board-2-42VR9-reference.png", "board-2-42VR9-notes.png", "board-2-42VR9-thumbnail.png", "board-2-42VR9.psd", "audio.wav", "board-2-42VR9-posterframe.jpg",
		"board-2-J74F5.png", "board-2-J74F5-reference.png", "board-2-J74F5-thumbnail.png",
		"board-0-P2FLS.png", "board-0-P2FLS-reference.png", "board-0-P2FLS-notes.png", "board-0-P2FLS-thumbnail.png",
		"board-1-WEBM4.png", "board-1-WEBM4-reference.png", "board-1-WEBM4-notes.png", "board-1-WEBM4-thumbnail.png",
		"board-98-PQKJM.png", "board-98-PQKJM-reference.png", "board-98-PQKJM-notes.png", "board-98-PQKJM-thumbnail.png", "board-98-PQKJM.psd",
		"unused.wav", "unused.png", "unused.psd"} {
		os.WriteFile(filepath.Join(images, f), []byte{8, 6, 7, 5, 3, 0, 9}, 0o644)
	}
	s, _ := LoadScene(filepath.Join(dir, "ducks.storyboarder"))
	plan := PrepareCleanup(s)
	if plan.Renames[0] != (Rename{"board-2-42VR9-reference.png", "board-1-42VR9-reference.png"}) {
		t.Errorf("first rename %v", plan.Renames[0])
	}
	var trashed []string
	TrashFunc = func(p string) error { trashed = append(trashed, filepath.Base(p)); return os.Remove(p) }
	defer func() { TrashFunc = Trash }()
	if _, err := Cleanup(s, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"unused.png", "unused.psd", "board-98-PQKJM.psd", "unused.wav", "board-0-P2FLS.png"} {
		if !contains(trashed, want) {
			t.Errorf("expected %s to be trashed, got %v", want, trashed)
		}
	}
	if len(trashed) != 9 {
		t.Errorf("trashed %d files: %v", len(trashed), trashed)
	}
	s, _ = LoadScene(filepath.Join(dir, "ducks.storyboarder"))
	b := s.Boards()
	if URL(b[len(b)-1]) != "board-5-PQKJM.png" || b[2].Get("link") != nil || b[2].Get("audio") != nil || b[0].Obj("audio").Str("filename") != "audio.wav" {
		t.Errorf("cleaned data wrong: %s", ojson.Stringify(s.Data, "  "))
	}
	for _, f := range []string{"board-1-42VR9.psd", "board-1-42VR9-thumbnail.png", "board-1-42VR9-posterframe.jpg"} {
		if !Exists(filepath.Join(images, f)) {
			t.Errorf("%s missing after cleanup", f)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestMigrateOldScene(t *testing.T) {
	dir := fixture(t, "old-scene")
	s, _ := LoadScene(filepath.Join(dir, "old-scene.storyboarder"))
	rep, err := Migrate(s)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Backup == "" || !Exists(filepath.Join(rep.Backup, "old-scene-backup.storyboarder")) || !Exists(filepath.Join(rep.Backup, "images", "board-1-UDRF3.png")) {
		t.Errorf("backup missing: %+v", rep)
	}
	if Layer(s.Boards()[0], "fill").Str("url") != "board-1-UDRF3-fill.png" || !Exists(s.ImagePath("board-1-UDRF3-fill.png")) || Exists(s.ImagePath("board-1-UDRF3.png")) {
		t.Errorf("fill not migrated: %+v", rep)
	}
	if _, err := Migrate(s); err != nil {
		t.Errorf("second migrate should be a no-op: %v", err)
	}
}

func TestClipRoundTrip(t *testing.T) {
	dir := fixture(t, "example")
	s, _ := LoadScene(filepath.Join(dir, "example.storyboarder"))
	n := len(s.Boards())
	clip, err := ExportClip(s, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	clipFile := filepath.Join(t.TempDir(), "clip.json")
	WriteJSON(clipFile, clip, "  ")
	read, err := ReadClip(clipFile)
	if err != nil {
		t.Fatal(err)
	}
	added, err := PasteClip(s, read, n)
	if err != nil {
		t.Fatal(err)
	}
	b := added[0]
	if len(s.Boards()) != n+1 || UID(b) == UID(s.Boards()[1]) || URL(b) != "board-"+itoa(n+1)+"-"+UID(b)+".png" {
		t.Errorf("pasted board %s", ojson.Stringify(b, ""))
	}
	orig, _ := render.Load(s.ImagePath(Layer(s.Boards()[1], "fill").Str("url")))
	copyImg, _ := render.Load(s.ImagePath(Layer(b, "fill").Str("url")))
	if render.ToRGBA(orig).RGBAAt(500, 400) != render.ToRGBA(copyImg).RGBAAt(500, 400) {
		t.Error("pasted fill pixels differ")
	}
	if !Exists(s.ImagePath(PosterframeFile(b))) || !Exists(s.ImagePath(ThumbnailFile(b))) {
		t.Error("paste did not render posterframe/thumbnail")
	}
}

func itoa(i int) string { return string(ojson.Stringify(i, "")) }

func TestDuplicateAndDelete(t *testing.T) {
	dir := fixture(t, "example")
	s, _ := LoadScene(filepath.Join(dir, "example.storyboarder"))
	n := len(s.Boards())
	s.Boards()[1].Set("dialogue", "hi")
	d, err := DuplicateBoard(s, 1)
	if err != nil {
		t.Fatal(err)
	}
	if d.Str("dialogue") != "" || d.Get("audio") != nil || !Exists(s.ImagePath(Layer(d, "fill").Str("url"))) {
		t.Errorf("duplicate: %s", ojson.Stringify(d, ""))
	}
	all := []int{}
	for i := range s.Boards() {
		all = append(all, i)
	}
	if got := DeleteBoards(s, all); got != n {
		t.Errorf("deleted %d of %d (one must stay)", got, n+1)
	}
	if len(s.Boards()) != 1 {
		t.Error("expected one board left")
	}
}

func TestExportGIFAndImages(t *testing.T) {
	dir := fixture(t, "example")
	s, _ := LoadScene(filepath.Join(dir, "example.storyboarder"))
	s.Boards()[0].Set("dialogue", "A long line of dialogue that certainly needs to wrap across more than one line of the caption")
	s.Boards()[1].Set("duration", 500)
	out, err := ExportGIF(s, s.Boards(), 888, "")
	if err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(out)
	defer f.Close()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Image) != len(s.Boards()) || g.Delay[0] != 200 || g.Delay[1] != 50 || g.Image[0].Bounds().Dx() != 888 || g.Image[0].Bounds().Dy() != 499 {
		t.Errorf("gif: %d frames, delays %v, size %v", len(g.Image), g.Delay, g.Image[0].Bounds())
	}
	files, err := ExportImages(s, filepath.Join(dir, "out"))
	if err != nil || len(files) != len(s.Boards()) || filepath.Base(files[0]) != "example-board-00001.png" {
		t.Errorf("images: %v %v", files, err)
	}
}

func TestVideoArgs(t *testing.T) {
	dir := fixture(t, "audio")
	s, _ := LoadScene(filepath.Join(dir, "audio.storyboarder"))
	job, err := PrepareVideo(s, filepath.Join(dir, "out.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(job.Dir)
	args := strings.Join(job.Args, " ")
	for _, want := range []string{"-safe 0 -i ", "[0]scale=-2:900[frame];[frame]null[vid];[1]areverse, afade=d=0.25:curve=exp, areverse[s1];[2]areverse, afade=d=0.25:curve=exp, areverse,adelay=2000|2000[s2];[s1][s2]amix=2[mix]", "-r 30", "-map [mix]:a", "-preset veryslow"} {
		if !strings.Contains(args, want) {
			t.Errorf("args missing %q:\n%s", want, args)
		}
	}
	if strings.Count(job.Concat, "\nfile ") != len(s.Boards())+1 || !strings.HasPrefix(job.Concat, "ffconcat version 1.0\n\nfile ") {
		t.Errorf("concat:\n%s", job.Concat)
	}
	if _, err := FfmpegPath(); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if err := job.Run(nil); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(dir, "out.mp4")); err != nil || st.Size() == 0 {
		t.Error("no video written")
	}
}

func TestLassoRegions(t *testing.T) {
	dir := fixture(t, "example")
	s, _ := LoadScene(filepath.Join(dir, "example.storyboarder"))
	b := s.Boards()[1]
	sq := [][2]float64{{0, 0}, {400, 0}, {400, 400}, {0, 400}}
	if err := FillRegion(s, b, sq, color.RGBA{255, 0, 0, 255}, 1); err != nil {
		t.Fatal(err)
	}
	fill := LoadLayer(s, b, "fill")
	if c := fill.RGBAAt(200, 200); c != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("fill pixel %v", c)
	}
	if err := MoveRegion(s, b, sq, 500, 0); err != nil {
		t.Fatal(err)
	}
	fill = LoadLayer(s, b, "fill")
	if fill.RGBAAt(200, 200).A != 0 || fill.RGBAAt(700, 200) != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("move: %v %v", fill.RGBAAt(200, 200), fill.RGBAAt(700, 200))
	}
	if err := EraseRegion(s, b, [][2]float64{{450, 0}, {1000, 0}, {1000, 450}, {450, 450}}); err != nil {
		t.Fatal(err)
	}
	if LoadLayer(s, b, "fill").RGBAAt(700, 200).A != 0 {
		t.Error("erase left pixels")
	}
}

func TestLayerUndoRestoresBytes(t *testing.T) {
	dir := fixture(t, "example")
	s, _ := LoadScene(filepath.Join(dir, "example.storyboarder"))
	b := s.Boards()[1]
	file := s.ImagePath(Layer(b, "fill").Str("url"))
	before, _ := os.ReadFile(file)
	if err := FillRegion(s, b, [][2]float64{{0, 0}, {300, 0}, {300, 300}}, color.RGBA{255, 0, 0, 255}, 1); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(file); string(after) == string(before) {
		t.Fatal("fill did not change the layer")
	}
	if h := LayerHistory(s, b, "fill"); len(h) != 1 {
		t.Fatalf("history %d", len(h))
	}
	if _, err := UndoLayer(s, b, "fill"); err != nil {
		t.Fatal(err)
	}
	if restored, _ := os.ReadFile(file); string(restored) != string(before) {
		t.Error("undo did not restore the previous bytes")
	}
	// undoing the first write of a new layer removes the layer again
	if Layer(b, "tone") != nil {
		t.Fatal("fixture board unexpectedly has a tone layer")
	}
	SaveLayer(s, b, "tone", render.New(10, 10))
	if _, err := UndoLayer(s, b, "tone"); err != nil || Layer(b, "tone") != nil || Exists(s.ImagePath(LayerFile(b, "tone"))) {
		t.Errorf("undo of a new layer should remove it: %v", err)
	}
	for i := 0; i < HistoryDepth+5; i++ {
		SaveLayer(s, b, "tone", render.New(10, 10))
	}
	if n := len(LayerHistory(s, b, "tone")); n != HistoryDepth {
		t.Errorf("history depth %d, want %d", n, HistoryDepth)
	}
}

func TestContactSheetSize(t *testing.T) {
	var cells []render.Cell
	for i := 0; i < 5; i++ {
		cells = append(cells, render.Cell{Image: render.White(160, 90), Title: "x", Caption: "some dialogue here"})
	}
	img := render.ContactSheet(cells, 3, 200)
	// 24 px padding, 3 columns of 200, 2 rows of (113 image + 70 text)
	if img.Bounds().Dx() != 24+3*(200+24) || img.Bounds().Dy() != 24+2*(113+70+24) {
		t.Errorf("sheet %v", img.Bounds())
	}
}

func TestLayerRedoAndOpacity(t *testing.T) {
	dir := fixture(t, "example")
	s, _ := LoadScene(filepath.Join(dir, "example.storyboarder"))
	b := s.Boards()[1]
	file := s.ImagePath(Layer(b, "fill").Str("url"))
	v1, _ := os.ReadFile(file)
	FillRegion(s, b, [][2]float64{{0, 0}, {300, 0}, {300, 300}}, color.RGBA{255, 0, 0, 255}, 1)
	v2, _ := os.ReadFile(file)
	UndoLayer(s, b, "fill")
	if got, _ := os.ReadFile(file); string(got) != string(v1) || RedoCount(s, b, "fill") != 1 {
		t.Fatal("undo")
	}
	if _, err := RedoLayer(s, b, "fill"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(file); string(got) != string(v2) || RedoCount(s, b, "fill") != 0 {
		t.Error("redo did not restore the undone bytes")
	}
	UndoLayer(s, b, "fill")
	FillRegion(s, b, [][2]float64{{0, 0}, {10, 0}, {10, 10}}, color.Black, 1)
	if RedoCount(s, b, "fill") != 0 {
		t.Error("a new edit must clear redo")
	}
	// opacity on any layer, including one without a file yet
	if err := SetLayerOpacity(s, b, "tone", 0.3); err != nil {
		t.Fatal(err)
	}
	if LayerOpacity(b, "tone") != 0.3 || !Exists(s.ImagePath(Layer(b, "tone").Str("url"))) {
		t.Error("tone opacity / blank file")
	}
	w, h := SizeOf(s, b)
	SaveLayer(s, b, "pencil", render.NewFilled(w, h, color.RGBA{0, 0, 255, 255}))
	SetLayerOpacity(s, b, "pencil", 0.5)
	img, _ := Flatten(s, b, w, h)
	if c := img.RGBAAt(w-5, h-5); c.B < 250 || c.R > 135 || c.R < 120 {
		t.Errorf("pencil at 50%% over white should be light blue, got %v", c)
	}
}
