// Command sb is the Storyboarder Next backend: every feature is a
// `sb <noun> <verb> [args] [flags]` command with human or --json output.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"sb/internal/ojson"
	"sb/internal/story"
)

// Cmd is one CLI command.
type Cmd struct {
	Path  string   // "board add"
	Args  string   // positional usage, e.g. "<i> <ms>"
	Short string   // one-line description
	Long  string   // extra help (optional)
	Flags []string // "name=desc" takes a value, "name desc" is boolean; first word is the flag
	Run   func(c *Ctx) (any, error)
}

// Ctx carries parsed arguments for a command run.
type Ctx struct {
	Cmd   *Cmd
	Pos   []string
	Flags map[string]string
	Bools map[string]bool
	JSON  bool
	Text  strings.Builder // human output; when empty the result is printed as JSON
}

// UsageError marks bad invocations (exit 2).
type UsageError struct{ msg string }

func (e UsageError) Error() string { return e.msg }

func usagef(format string, a ...any) error { return UsageError{fmt.Sprintf(format, a...)} }

var commands []*Cmd

func register(cs ...*Cmd) { commands = append(commands, cs...) }

// globalFlags apply to every command.
var globalFlags = []string{
	"project=path to a .storyboarder, .fountain/.fdx or a folder (default: current folder)",
	"scene=scene number inside a script project (default: storyboard.settings lastScene + 1)",
	"json print one JSON object (result or {\"error\":{...}}) instead of text",
	"help show help for the command",
}

func (c *Ctx) Printf(format string, a ...any) { fmt.Fprintf(&c.Text, format, a...) }

func (c *Ctx) Flag(name string) string { return c.Flags[name] }
func (c *Ctx) Has(name string) bool    { _, ok := c.Flags[name]; return ok || c.Bools[name] }
func (c *Ctx) Bool(name string) bool   { return c.Bools[name] }
func (c *Ctx) Arg(i int) string {
	if i < len(c.Pos) {
		return c.Pos[i]
	}
	return ""
}

// Need checks the number of positional arguments.
func (c *Ctx) Need(n int) error {
	if len(c.Pos) < n {
		return usagef("missing arguments: sb %s %s", c.Cmd.Path, c.Cmd.Args)
	}
	return nil
}

// Float reads a numeric flag.
func (c *Ctx) Float(name string) (float64, bool, error) {
	v, ok := c.Flags[name]
	if !ok {
		return 0, false, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, true, usagef("--%s expects a number, got %q", name, v)
	}
	return f, true, nil
}

func parseFloatArg(s, what string) (float64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, usagef("%s must be a number, got %q", what, s)
	}
	return f, nil
}

func flagName(spec string) (string, bool) {
	name := strings.Fields(strings.SplitN(spec, "=", 2)[0])[0]
	return name, strings.Contains(strings.Fields(spec)[0], "=")
}

func findCmd(args []string) (*Cmd, int) {
	var best *Cmd
	bestN := 0
	for _, c := range commands {
		parts := strings.Fields(c.Path)
		if len(parts) > len(args) || len(parts) <= bestN {
			continue
		}
		match := true
		for i, p := range parts {
			if args[i] != p {
				match = false
				break
			}
		}
		if match {
			best, bestN = c, len(parts)
		}
	}
	return best, bestN
}

func parseArgs(c *Cmd, args []string) (*Ctx, error) {
	ctx := &Ctx{Cmd: c, Flags: map[string]string{}, Bools: map[string]bool{}}
	takes := map[string]bool{}
	for _, spec := range append(append([]string{}, c.Flags...), globalFlags...) {
		n, v := flagName(spec)
		takes[n] = v
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			ctx.Pos = append(ctx.Pos, args[i+1:]...)
			break
		}
		if a == "-h" {
			ctx.Bools["help"] = true
			continue
		}
		if !strings.HasPrefix(a, "--") || len(a) == 2 {
			ctx.Pos = append(ctx.Pos, a)
			continue
		}
		name, val, hasVal := strings.Cut(a[2:], "=")
		wants, known := takes[name]
		if !known {
			return ctx, usagef("unknown flag --%s for `sb %s` (see `sb %s --help`)", name, c.Path, c.Path)
		}
		if wants {
			if !hasVal {
				if i+1 >= len(args) {
					return ctx, usagef("--%s needs a value", name)
				}
				i++
				val = args[i]
			}
			ctx.Flags[name] = val
		} else {
			if hasVal {
				b, err := strconv.ParseBool(val)
				if err != nil {
					return ctx, usagef("--%s is a switch", name)
				}
				ctx.Bools[name] = b
			} else {
				ctx.Bools[name] = true
			}
		}
	}
	ctx.JSON = ctx.Bools["json"]
	return ctx, nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}
	if len(args) == 0 || args[0] == "help" && !(len(args) > 1 && args[1] == "urls") || args[0] == "--help" || args[0] == "-h" {
		rest := []string{}
		if len(args) > 0 && args[0] == "help" {
			for _, a := range args[1:] {
				if !strings.HasPrefix(a, "-") {
					rest = append(rest, a)
				}
			}
		}
		return printHelp(rest, jsonOut)
	}
	c, n := findCmd(args)
	if c == nil {
		// `sb board` or `sb board --help`
		if nounHasCommands(args[0]) {
			return printHelp(args[:1], jsonOut)
		}
		var words []string
		for _, a := range args {
			if !strings.HasPrefix(a, "-") {
				words = append(words, a)
			}
		}
		return fail(usagef("unknown command %q (run `sb help`)", strings.Join(words, " ")), jsonOut)
	}
	ctx, err := parseArgs(c, args[n:])
	if err != nil {
		return fail(err, jsonOut)
	}
	if ctx.Bools["help"] {
		return printHelp(strings.Fields(c.Path), jsonOut)
	}
	if v, ok := ctx.Flags["project"]; ok {
		projectArg = v
	}
	if v, ok := ctx.Flags["scene"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return fail(usagef("--scene expects a scene number"), ctx.JSON)
		}
		sceneArg = n
	}
	res, err := c.Run(ctx)
	if err != nil {
		return fail(err, ctx.JSON)
	}
	if ctx.JSON {
		printJSON(asObject(res))
	} else if ctx.Text.Len() > 0 {
		fmt.Print(ctx.Text.String())
		if !strings.HasSuffix(ctx.Text.String(), "\n") {
			fmt.Println()
		}
	} else if res != nil {
		printJSONIndent(res)
	}
	return 0
}

func nounHasCommands(noun string) bool {
	for _, c := range commands {
		if strings.Fields(c.Path)[0] == noun {
			return true
		}
	}
	return false
}

// asObject guarantees the JSON output is an object.
func asObject(v any) any {
	if v == nil {
		return map[string]any{"ok": true}
	}
	b, _ := json.Marshal(v)
	if len(b) > 0 && b[0] == '{' {
		return v
	}
	return map[string]any{"result": v}
}

func marshal(v any, indent string) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil {
		return []byte(fmt.Sprintf(`{"error":{"code":"internal","message":%q}}`+"\n", err.Error()))
	}
	return buf.Bytes()
}

func printJSON(v any)       { os.Stdout.Write(marshal(v, "")) }
func printJSONIndent(v any) { os.Stdout.Write(marshal(v, "  ")) }

func fail(err error, jsonOut bool) int {
	code, exit := "failed", 1
	var ue UsageError
	if errors.As(err, &ue) {
		code, exit = "usage", 2
	} else if errors.Is(err, os.ErrNotExist) || errors.Is(err, story.ErrNoProject) {
		code = "not_found"
	}
	if jsonOut {
		printJSON(map[string]any{"error": map[string]any{"code": code, "message": err.Error()}})
	} else {
		fmt.Fprintln(os.Stderr, "sb: "+err.Error())
	}
	return exit
}

// ---- help ----

func printHelp(path []string, jsonOut bool) int {
	var matched []*Cmd
	prefix := strings.Join(path, " ")
	for _, c := range commands {
		if prefix == "" || c.Path == prefix || strings.HasPrefix(c.Path, prefix+" ") {
			matched = append(matched, c)
		}
	}
	if len(matched) == 0 {
		return fail(usagef("no help for %q", prefix), jsonOut)
	}
	if jsonOut {
		var list []map[string]any
		for _, c := range matched {
			list = append(list, map[string]any{"command": "sb " + c.Path, "args": c.Args, "summary": c.Short, "details": c.Long, "flags": c.Flags})
		}
		printJSON(map[string]any{"commands": list, "globalFlags": globalFlags})
		return 0
	}
	var b strings.Builder
	if prefix == "" {
		b.WriteString(helpIntro)
	}
	if len(matched) == 1 && matched[0].Path == prefix {
		c := matched[0]
		fmt.Fprintf(&b, "sb %s %s\n\n  %s\n", c.Path, c.Args, c.Short)
		if c.Long != "" {
			b.WriteString("\n" + indent(c.Long, "  ") + "\n")
		}
		writeFlags(&b, c.Flags)
		b.WriteString("\nGlobal flags:\n")
		writeFlags(&b, globalFlags)
		fmt.Print(b.String())
		return 0
	}
	groups := map[string][]*Cmd{}
	var nouns []string
	for _, c := range matched {
		noun := strings.Fields(c.Path)[0]
		if _, ok := groups[noun]; !ok {
			nouns = append(nouns, noun)
		}
		groups[noun] = append(groups[noun], c)
	}
	rank := func(n string) int {
		for i, x := range nounOrder {
			if x == n {
				return i
			}
		}
		return len(nounOrder)
	}
	sort.SliceStable(nouns, func(i, j int) bool { return rank(nouns[i]) < rank(nouns[j]) })
	for _, noun := range nouns {
		fmt.Fprintf(&b, "\n%s\n", strings.ToUpper(noun))
		for _, c := range groups[noun] {
			line := strings.TrimSpace("sb " + c.Path + " " + c.Args)
			fmt.Fprintf(&b, "  %-58s %s\n", line, c.Short)
			if prefix != "" {
				if c.Long != "" {
					b.WriteString(indent(c.Long, "      ") + "\n")
				}
				for _, f := range c.Flags {
					n, v := flagName(f)
					desc := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(f, n), "="))
					arg := ""
					if v {
						arg = " <v>"
					}
					fmt.Fprintf(&b, "      --%-18s %s\n", n+arg, desc)
				}
			}
		}
	}
	if prefix == "" {
		b.WriteString("\nGlobal flags:\n")
		writeFlags(&b, globalFlags)
		b.WriteString("\nRun `sb <noun> --help` for flags and details of every command in a group.\n")
	}
	fmt.Print(b.String())
	return 0
}

func writeFlags(b *strings.Builder, flags []string) {
	if len(flags) == 0 {
		return
	}
	if !strings.HasSuffix(b.String(), "Global flags:\n") {
		b.WriteString("\nFlags:\n")
	}
	for _, f := range flags {
		n, v := flagName(f)
		desc := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(f, n), "="))
		arg := ""
		if v {
			arg = " <v>"
		}
		fmt.Fprintf(b, "  --%-20s %s\n", n+arg, desc)
	}
}

func indent(s, pre string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = pre + l
	}
	return strings.Join(lines, "\n")
}

var nounOrder = []string{"project", "recent", "page", "pages", "panel", "balloon", "spread", "board", "draw", "layer", "import", "export", "print", "scene", "script", "audio", "shotlist", "sg", "prefs", "keymap", "lang", "doctor", "app", "help", "timelapse", "tip"}

const helpIntro = `sb — Storyboarder Next command line.

Usage: sb <noun> <verb> [args] [flags]

Projects are .storyboarder files (one scene) or script projects (.fountain/.fdx
next to storyboards/storyboard.settings with one Scene-<n>-<slug>-<id>/ folder
per scene). Commands run against --project, or the .storyboarder (or script
project) found in the current folder. In a script project, board and scene
commands act on --scene <n> (default: storyboard.settings lastScene + 1).

Manga projects (project new --manga) add pages[] next to the boards: pages
hold panels (koma), balloons and optional page drawing layers; boards are free
drawings placed into panels. Film projects work exactly like the original app.
Drawing for an agent: draw svg|strokes|text|erase, then board/page render
--grid and look at the PNG (see docs/claude-drawing-loop.md).

Boards are addressed by number (1-based, as shown in the app) or by uid;
lists accept "1,3,5", ranges "2-4", "all" and "last".
Every write keeps unknown JSON fields and saves atomically
(<file>.backup-<ms> renamed over the original), like the original app.
`

// ---- shared helpers for commands ----

var (
	projectArg string
	sceneArg   int
)

func openProject() (*story.Project, error) { return story.FindProject(projectArg) }

func prefsFps() float64 {
	p, err := story.LoadPrefs()
	if err != nil {
		return 24
	}
	if f, ok := p.Num("lastUsedFps"); ok && f > 0 {
		return f
	}
	return 24
}

// openScene resolves the scene the board commands act on.
func openScene() (*story.Scene, error) {
	p, err := openProject()
	if err != nil {
		return nil, err
	}
	if !p.IsScript() {
		return story.LoadScene(p.File)
	}
	s, _, err := p.OpenScene(sceneArg, prefsFps(), false)
	return s, err
}

// boardIndexes parses "3", "1-3", "1,4", "last", "all" or uids into 0-based indexes.
func boardIndexes(s *story.Scene, spec string) ([]int, error) {
	boards := s.Boards()
	var out []int
	seen := map[int]bool{}
	add := func(i int) error {
		if i < 0 || i >= len(boards) {
			return fmt.Errorf("board %d does not exist (the scene has %d boards)", i+1, len(boards))
		}
		if !seen[i] {
			seen[i] = true
			out = append(out, i)
		}
		return nil
	}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		switch {
		case part == "":
			continue
		case part == "all":
			for i := range boards {
				add(i)
			}
		case part == "last":
			add(len(boards) - 1)
		default:
			if i := uidIndex(boards, part); i >= 0 {
				add(i)
				continue
			}
			if a, b, ok := strings.Cut(part, "-"); ok && a != "" {
				x, err1 := strconv.Atoi(a)
				y, err2 := strconv.Atoi(b)
				if err1 != nil || err2 != nil || y < x {
					return nil, usagef("bad board range %q", part)
				}
				for i := x; i <= y; i++ {
					if err := add(i - 1); err != nil {
						return nil, err
					}
				}
				continue
			}
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("no board %q (use a number, a range like 2-4, or a uid)", part)
			}
			if err := add(n - 1); err != nil {
				return nil, err
			}
		}
	}
	if len(out) == 0 {
		return nil, usagef("no boards selected")
	}
	return out, nil
}

func uidIndex(boards []*ojson.Object, uid string) int {
	if len(uid) != 5 {
		return -1
	}
	for i, b := range boards {
		if strings.EqualFold(story.UID(b), uid) {
			return i
		}
	}
	return -1
}

// oneBoard parses a single board reference.
func oneBoard(s *story.Scene, spec string) (int, *ojson.Object, error) {
	idx, err := boardIndexes(s, spec)
	if err != nil {
		return 0, nil, err
	}
	if len(idx) != 1 {
		return 0, nil, usagef("expected one board, got %q", spec)
	}
	return idx[0], s.Boards()[idx[0]], nil
}

func sortedInts(a []int) []int {
	b := append([]int(nil), a...)
	sort.Ints(b)
	return b
}

// msToTime is util.msToTime: m:ss or h:mm:ss.
func msToTime(ms float64) string {
	if ms < 0 {
		ms = 0
	}
	s := int64(ms/1000 + 0.5)
	secs := s % 60
	s = (s - secs) / 60
	mins := s % 60
	hrs := (s - mins) / 60
	if hrs > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hrs, mins, secs)
	}
	return fmt.Sprintf("%d:%02d", mins, secs)
}
