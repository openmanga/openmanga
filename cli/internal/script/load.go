package script

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Parsed is a loaded script with the metadata main.js derives from it.
type Parsed struct {
	Path       string  `json:"path"`
	Format     string  `json:"format"` // fountain | fdx
	Title      string  `json:"title"`
	Nodes      []Node  `json:"nodes"`
	Tokens     []Token `json:"-"`
	Locations  []Count `json:"locations"`
	Characters []Count `json:"characters"`
}

var reTags = regexp.MustCompile(`<(?:.|\n)*?>`)

// Load parses a .fountain or .fdx file without modifying it.
func Load(path string) (*Parsed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := &Parsed{Path: path}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".fountain":
		p.Format = "fountain"
		p.Tokens = Tokenize(string(data))
		p.Locations = Locations(p.Tokens)
		p.Characters = Characters(p.Tokens)
		p.Nodes = ParseFountainTokens(p.Tokens)
	case ".fdx":
		p.Format = "fdx"
		if p.Nodes, err = ParseFDX(data); err != nil {
			return nil, err
		}
		p.Locations = NodeLocations(p.Nodes)
		p.Characters = FDXCharacters(p.Nodes)
	default:
		return nil, fmt.Errorf("not a script file: %s", path)
	}
	p.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, n := range p.Nodes {
		if n.Type == "title" && n.Text != "" {
			p.Title = reTags.ReplaceAllString(n.Text, "")
		}
	}
	return p, nil
}

// Scenes returns only the scene nodes.
func (p *Parsed) Scenes() []Node {
	var out []Node
	for _, n := range p.Nodes {
		if n.Type == "scene" {
			out = append(out, n)
		}
	}
	return out
}

// Scene finds a scene by its scene_number.
func (p *Parsed) Scene(number int) (Node, bool) {
	for _, n := range p.Scenes() {
		if n.SceneNumber == number {
			return n, true
		}
	}
	return Node{}, false
}

// TotalTime is metadata.totalMovieTime from main.js processFountainData.
func (p *Parsed) TotalTime() int {
	if len(p.Nodes) == 0 {
		return 0
	}
	last := p.Nodes[len(p.Nodes)-1]
	switch last.Type {
	case "section":
		return last.Time + last.Duration
	case "scene":
		if len(last.Script) > 0 {
			it := last.Script[len(last.Script)-1]
			return it.Time + it.Duration
		}
	}
	return 0
}

// UID mirrors util.uidGen(5): 5 upper-case base36 characters.
func UID() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(36*36*36*36*36))
	s := strings.ToUpper(n.Text(36))
	return strings.Repeat("0", 5-len(s)) + s
}

// InsertFountainSceneIDs ports fountain-scene-id-util: appends " #<n>-<uid>#"
// to scene headings that have no scene number. Returns the new text and whether it changed.
func InsertFountainSceneIDs(text string, gen func() string) (string, bool) {
	lines := strings.Split(text, "\n")
	count := 0
	added := false
	for i, line := range lines {
		var t string
		var ok bool
		if m := reSceneHeading.FindStringSubmatch(line); m != nil {
			t, ok = m[1], true
		} else {
			t, ok = forcedHeading(line)
		}
		if !ok {
			continue
		}
		count++
		if strings.Index(t, "  ") != len(t)-2 && !reSceneNumber.MatchString(t) {
			eol := ""
			if strings.HasSuffix(line, "\r") {
				eol = "\r"
			}
			// trim() in the original removes only the first line break
			lines[i] = strings.TrimSuffix(line, "\r") + fmt.Sprintf(" #%d-%s#", count, gen()) + eol
			added = true
		}
	}
	return strings.Join(lines, "\n"), added
}

// SceneFolderID is the id part used in folder names: "1-ABCDE" -> "ABCDE".
func SceneFolderID(sceneID string) string {
	parts := strings.Split(sceneID, "-")
	if len(parts) > 1 {
		return parts[1]
	}
	return parts[0]
}

var reFilenameA = regexp.MustCompile(`\|&;\$%@"<>\(\)\+,`)
var reFilenameB = regexp.MustCompile(`[|&;/:$%@"{}?|<>()+,]`)

// SceneFolderName is `Scene-<n>-<slug>-<scene_id>` (models/shot-list getSceneFolderName).
func SceneFolderName(n Node) string {
	desc := n.Synopsis
	if desc == "" {
		desc = n.Slugline
	}
	r := []rune(desc)
	if len(r) > 50 {
		r = r[:50]
	}
	s := reFilenameA.ReplaceAllString(string(r), "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, " - ", " ")
	s = strings.ReplaceAll(s, " ", "-")
	s = reFilenameB.ReplaceAllString(s, "-")
	return fmt.Sprintf("Scene-%d-%s-%s", n.SceneNumber, s, n.SceneID)
}
