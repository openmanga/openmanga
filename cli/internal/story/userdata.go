package story

import (
	"crypto/rand"
	_ "embed"
	"encoding/json"
	"fmt"
	mrand "math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"sb/internal/ojson"
)

// UserDataDir is Electron's app.getPath('userData') for "Storyboarder",
// overridable with SB_USER_DATA (tests, portable setups).
func UserDataDir() string {
	if d := os.Getenv("SB_USER_DATA"); d != "" {
		return d
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "Storyboarder")
}

func userPath(parts ...string) string {
	return filepath.Join(append([]string{UserDataDir()}, parts...)...)
}

// DefaultPrefs mirrors prefs.js defaultPrefs (undefined values are omitted, as JSON.stringify does).
func DefaultPrefs() *ojson.Object {
	o := ojson.Obj(
		"version", AppVersion,
		"enableDrawingSoundEffects", false,
		"enableDrawingMelodySoundEffects", false,
		"enableUISoundEffects", false,
		"enableHighQualityAudio", false,
		"enableTooltips", true,
		"enableAspirationalMessages", true,
		"defaultBoardTiming", 2000,
		"pomodoroTimerMinutes", 25,
		"importTargetLayer", "reference",
		"enableCanvasPaintingOpacity", true,
		"enableBrushCursor", true,
		"enableStabilizer", true,
		"enableAnalytics", true,
		"enableAutoSave", true,
		"enableForcePsdReloadOnFocus", true,
		"enableDiagnostics", false,
		"lastUsedFps", 24,
		"allowNotificationsForLineMileage", true,
		"enableNotifications", true,
		"import", ojson.Obj("offset", []any{0, 0}, "skipBlankBoards", true),
		"enableBoardAudition", true,
		"enableHighQualityDrawingEngine", true,
		"straightLineDelayInMsecs", 650,
		"enableWatermark", true,
	)
	// slow computers (<= 2 cores) get sounds off; the original also checks clock speed
	if runtime.NumCPU() <= 2 {
		for _, k := range []string{"enableDrawingSoundEffects", "enableDrawingMelodySoundEffects", "enableUISoundEffects", "enableCanvasPaintingOpacity", "enableStabilizer"} {
			o.Set(k, false)
		}
	}
	return o
}

func PrefsPath() string { return userPath("pref.json") }

// LoadPrefs reads pref.json, falling back to defaults (and writing them) like prefs.js load().
func LoadPrefs() (*ojson.Object, error) {
	data, err := os.ReadFile(PrefsPath())
	if err == nil {
		if o, perr := ojson.ParseObject(data); perr == nil {
			return o, nil
		}
	}
	o := DefaultPrefs()
	return o, SavePrefs(o)
}

func SavePrefs(o *ojson.Object) error { return WriteJSON(PrefsPath(), o, "  ") }

// PrefsGetPath reads a dotted key path.
func PrefsGetPath(o *ojson.Object, key string) (any, bool) {
	var cur any = o
	for _, k := range strings.Split(key, ".") {
		obj, ok := cur.(*ojson.Object)
		if !ok || !obj.Has(k) {
			return nil, false
		}
		cur = obj.Get(k)
	}
	return cur, true
}

// PrefsSetPath is prefs.js set(): creates intermediate objects.
func PrefsSetPath(o *ojson.Object, key string, v any) {
	parts := strings.Split(key, ".")
	cur := o
	for _, k := range parts[:len(parts)-1] {
		next := cur.Obj(k)
		if next == nil {
			next = ojson.New()
			cur.Set(k, next)
		}
		cur = next
	}
	cur.Set(parts[len(parts)-1], v)
}

// ParseValue turns a CLI string into JSON: numbers, booleans, null, JSON
// literals; anything else stays a string.
func ParseValue(s string) any {
	if v, err := ojson.Parse([]byte(s)); err == nil {
		return v
	}
	return s
}

func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x < y
		}
	}
	return false
}

// MigratePrefs merges missing defaults when the stored version is older (prefs.js migrate).
func MigratePrefs(o *ojson.Object) (bool, []string) {
	if !versionLess(o.Str("version"), AppVersion) {
		return false, nil
	}
	def := DefaultPrefs()
	var added []string
	merged := def.Clone()
	for _, k := range o.Keys() {
		merged.Set(k, o.Get(k))
	}
	for _, k := range def.Keys() {
		if !o.Has(k) {
			added = append(added, k)
		}
	}
	merged.Set("version", AppVersion)
	*o = *merged
	return true, added
}

// Tools are the toolbar tools whose color/palette/strokeOpacity persist in prefs.toolbar.
var Tools = []string{"light-pencil", "brush", "tone", "pencil", "pen", "note-pen", "eraser"}

// ToolDefaults mirrors shared/reducers/toolbar.js initialState (color, palette, strokeOpacity).
var ToolDefaults = map[string]struct {
	Color   int
	Palette [3]int
	Size    float64
}{
	"light-pencil": {0x90CBF9, [3]int{0xCFCFCF, 0x9FA8DA, 0x90CBF9}, 20},
	"brush":        {0x90CBF9, [3]int{0x4DABF5, 0x607D8B, 0x9E9E9E}, 26},
	"tone":         {0x162A3F, [3]int{0x162A3F, 0x162A3F, 0x162A3F}, 50},
	"pencil":       {0x121212, [3]int{0x373737, 0x223131, 0x121212}, 4},
	"pen":          {0x000000, [3]int{0x373737, 0x223131, 0x000000}, 2},
	"note-pen":     {0xF44336, [3]int{0x4CAF50, 0xFF9800, 0xF44336}, 8},
	"eraser":       {0xffffff, [3]int{0xffffff, 0xffffff, 0xffffff}, 26},
}

// ColorNumber parses #rrggbb into the integer the toolbar stores.
func ColorNumber(s string) (int, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	n, err := strconv.ParseInt(s, 16, 32)
	if err != nil || len(s) != 6 {
		return 0, fmt.Errorf("invalid color %q (use #rrggbb)", s)
	}
	return int(n), nil
}

// ---- keymap ----

//go:embed data/keymap.json
var defaultKeymapJSON []byte

func DefaultKeymap() *ojson.Object {
	o, err := ojson.ParseObject(defaultKeymapJSON)
	if err != nil {
		panic(err)
	}
	return o
}

func KeymapPath() string { return userPath("keymap.json") }

// UserKeymap reads keymap.json (nil when absent).
func UserKeymap() (*ojson.Object, error) {
	data, err := os.ReadFile(KeymapPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ojson.ParseObject(data)
}

// MergedKeymap is Object.assign({}, defaults, user).
func MergedKeymap(user *ojson.Object) *ojson.Object {
	m := DefaultKeymap()
	if user != nil {
		for _, k := range user.Keys() {
			m.Set(k, user.Get(k))
		}
	}
	return m
}

// MigrateKeymap applies the 1.5.x tool-key and 1.7.1 pan-mode migrations from main.js.
func MigrateKeymap(user *ojson.Object) []string {
	var notes []string
	if user == nil {
		return notes
	}
	if user.Str("menu:tools:pencil") == "2" && user.Str("menu:tools:pen") == "3" && user.Str("menu:tools:brush") == "4" &&
		user.Str("menu:tools:note-pen") == "5" && user.Str("menu:tools:eraser") == "6" {
		for _, k := range []string{"menu:tools:pencil", "menu:tools:pen", "menu:tools:brush", "menu:tools:note-pen", "menu:tools:eraser"} {
			user.Delete(k)
		}
		notes = append(notes, "reset 1.5.x tool keys to defaults")
	}
	if user.Str("drawing:pan-mode") == "Shift" {
		user.Set("drawing:pan-mode", "Space")
		notes = append(notes, "re-mapped drawing:pan-mode from Shift to Space")
	}
	return notes
}

// SaveKeymap writes the full merged keymap with a trailing newline, as main.js does.
func SaveKeymap(m *ojson.Object) error {
	if err := os.MkdirAll(UserDataDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(KeymapPath(), append(ojson.Stringify(m, "  "), '\n'), 0o644)
}

// ---- recent documents (prefs.recentDocuments) ----

// AddRecent puts a project at the top of prefs.recentDocuments (main.js addToRecentDocs).
func AddRecent(filename string, meta *ojson.Object) error {
	prefs, err := LoadPrefs()
	if err != nil {
		return err
	}
	var list []any
	for _, v := range prefs.Arr("recentDocuments") {
		if d, ok := v.(*ojson.Object); ok && d.Str("filename") == filename {
			continue
		}
		list = append(list, v)
	}
	if meta == nil {
		meta = ojson.New()
	}
	if meta.Str("title") == "" {
		base := filepath.Base(filename)
		parts := strings.Split(base, ".")
		if len(parts) > 1 {
			parts = parts[:len(parts)-1]
		}
		meta.Set("title", strings.Join(parts, "."))
	}
	meta.Set("filename", filename)
	meta.Set("time", NowMs())
	prefs.Set("recentDocuments", append([]any{meta}, list...))
	return SavePrefs(prefs)
}

// PruneRecent removes entries whose files are gone.
func PruneRecent() ([]string, error) {
	prefs, err := LoadPrefs()
	if err != nil {
		return nil, err
	}
	var keep []any
	var removed []string
	for _, v := range prefs.Arr("recentDocuments") {
		if d, ok := v.(*ojson.Object); ok && !exists(d.Str("filename")) {
			removed = append(removed, d.Str("filename"))
			continue
		}
		keep = append(keep, v)
	}
	if keep == nil {
		keep = []any{}
	}
	prefs.Set("recentDocuments", keep)
	return removed, SavePrefs(prefs)
}

// ---- languages (locales/language-settings.json) ----

//go:embed data/en-US.json
var localeEnUS []byte

//go:embed data/ru-RU.json
var localeRuRU []byte

//go:embed data/zh-CN.json
var localeZhCN []byte

var builtinLocales = map[string][]byte{"en-US": localeEnUS, "ru-RU": localeRuRU, "zh-CN": localeZhCN}

// Language is one entry of builtInLanguages / customLanguages.
type Language struct {
	FileName    string `json:"fileName"`
	DisplayName string `json:"displayName"`
	BuiltIn     bool   `json:"builtIn"`
	Selected    bool   `json:"selected"`
}

func localesDir() string          { return userPath("locales") }
func langSettingsPath() string    { return filepath.Join(localesDir(), "language-settings.json") }
func IsBuiltinLang(c string) bool { _, ok := builtinLocales[c]; return ok }

// LangSettings reads language-settings.json, initializing it like main.js on first run.
func LangSettings() (*ojson.Object, error) {
	o := ojson.New()
	if data, err := os.ReadFile(langSettingsPath()); err == nil && len(data) > 0 {
		if p, err := ojson.ParseObject(data); err == nil {
			o = p
		}
	}
	var built []any
	for _, c := range []string{"en-US", "ru-RU", "zh-CN"} {
		var j struct{ Name string }
		json.Unmarshal(builtinLocales[c], &j)
		built = append(built, ojson.Obj("fileName", c, "displayName", j.Name))
	}
	o.Set("builtInLanguages", built)
	if o.Arr("customLanguages") == nil {
		// main.js rebuilds this from the locales folder
		var custom []any
		entries, _ := os.ReadDir(localesDir())
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".json")
			if filepath.Ext(e.Name()) != ".json" || name == "language-settings" {
				continue
			}
			data, _ := os.ReadFile(filepath.Join(localesDir(), e.Name()))
			var j struct{ Name string }
			json.Unmarshal(data, &j)
			custom = append(custom, ojson.Obj("fileName", name, "displayName", j.Name))
		}
		if custom == nil {
			custom = []any{}
		}
		o.Set("customLanguages", custom)
	}
	if o.Str("selectedLanguage") == "" {
		o.Set("selectedLanguage", "en-US")
	}
	if o.Str("defaultLanguage") == "" {
		o.Set("defaultLanguage", "en-US")
	}
	return o, nil
}

func SaveLangSettings(o *ojson.Object) error { return WriteJSON(langSettingsPath(), o, "  ") }

// Languages lists built-in and custom languages.
func Languages() ([]Language, error) {
	s, err := LangSettings()
	if err != nil {
		return nil, err
	}
	var out []Language
	for _, key := range []string{"builtInLanguages", "customLanguages"} {
		for _, v := range s.Arr(key) {
			if l, ok := v.(*ojson.Object); ok {
				out = append(out, Language{FileName: l.Str("fileName"), DisplayName: l.Str("displayName"), BuiltIn: key == "builtInLanguages", Selected: l.Str("fileName") == s.Str("selectedLanguage")})
			}
		}
	}
	return out, nil
}

// FindLanguage resolves a code or display name to its fileName.
func FindLanguage(code string) (Language, error) {
	langs, err := Languages()
	if err != nil {
		return Language{}, err
	}
	for _, l := range langs {
		if l.FileName == code {
			return l, nil
		}
	}
	for _, l := range langs {
		if strings.EqualFold(l.DisplayName, code) || strings.HasSuffix(l.FileName, "_"+code) {
			return l, nil
		}
	}
	return Language{}, fmt.Errorf("unknown language %q (see `sb lang list`)", code)
}

// LocaleJSON returns a language file's JSON with Name set to its display name.
func LocaleJSON(l Language) (*ojson.Object, error) {
	var data []byte
	if l.BuiltIn {
		data = builtinLocales[l.FileName]
	} else {
		var err error
		if data, err = os.ReadFile(filepath.Join(localesDir(), l.FileName+".json")); err != nil {
			return nil, err
		}
	}
	o, err := ojson.ParseObject(data)
	if err != nil {
		return nil, err
	}
	o.Set("Name", l.DisplayName)
	return o, nil
}

// SaveCustomLocale writes a custom language file compactly (JSON.stringify(value)).
func SaveCustomLocale(fileName string, o *ojson.Object) error {
	return WriteJSON(filepath.Join(localesDir(), fileName+".json"), o, "")
}

// AddCustomLanguage registers a new custom language and selects it.
func AddCustomLanguage(label string, data *ojson.Object) (Language, error) {
	s, err := LangSettings()
	if err != nil {
		return Language{}, err
	}
	fileName := uuid() + "_" + label
	if err := SaveCustomLocale(fileName, data); err != nil {
		return Language{}, err
	}
	s.Set("customLanguages", append(s.Arr("customLanguages"), ojson.Obj("fileName", fileName, "displayName", data.Str("Name"))))
	s.Set("selectedLanguage", fileName)
	return Language{FileName: fileName, DisplayName: data.Str("Name"), Selected: true}, SaveLangSettings(s)
}

// RemoveCustomLanguage deletes a custom language and selects the first built-in.
func RemoveCustomLanguage(l Language) error {
	if l.BuiltIn {
		return fmt.Errorf("you cannot remove built-in language %s", l.FileName)
	}
	s, err := LangSettings()
	if err != nil {
		return err
	}
	os.Remove(filepath.Join(localesDir(), l.FileName+".json"))
	var keep []any
	for _, v := range s.Arr("customLanguages") {
		if o, ok := v.(*ojson.Object); ok && o.Str("fileName") == l.FileName {
			continue
		}
		keep = append(keep, v)
	}
	if keep == nil {
		keep = []any{}
	}
	s.Set("customLanguages", keep)
	s.Set("selectedLanguage", "en-US")
	return SaveLangSettings(s)
}

// RenameCustomLanguage updates the display name stored in the settings.
func RenameCustomLanguage(fileName, display string) error {
	s, err := LangSettings()
	if err != nil {
		return err
	}
	for _, v := range s.Arr("customLanguages") {
		if o, ok := v.(*ojson.Object); ok && o.Str("fileName") == fileName {
			o.Set("displayName", display)
		}
	}
	return SaveLangSettings(s)
}

// StripUUID is removeUUIDFromName: "<uuid>_name" -> "name".
func StripUUID(name string) string {
	if i := strings.Index(name, "_"); i >= 0 && len(name[:i]) == 36 {
		return name[i+1:]
	}
	return name
}

func uuid() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]))
}

// UUID returns an upper-case v4 UUID (THREE.Math.generateUUID format).
func UUID() string { return uuid() }

// ---- timelapses and tips ----

// Timelapses reads userData/recordings.json (newest first).
func Timelapses() ([]map[string]any, error) {
	data, err := os.ReadFile(userPath("recordings.json"))
	if os.IsNotExist(err) {
		return []map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []any
	if err := json.Unmarshal(data, &paths); err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, p := range paths {
		s := fmt.Sprint(p)
		out = append(out, map[string]any{"path": s, "exists": exists(s)})
	}
	return out, nil
}

//go:embed data/tips.json
var tipsJSON []byte

// Tips returns the built-in story tips.
func Tips() []string {
	var t []string
	json.Unmarshal(tipsJSON, &t)
	return t
}

// RandomTip picks one tip.
func RandomTip() string {
	t := Tips()
	return t[mrand.IntN(len(t))]
}

// SortedKeys returns object keys sorted (for stable listings).
func SortedKeys(o *ojson.Object) []string {
	k := o.Keys()
	sort.Strings(k)
	return k
}
