package script

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

type fdxText struct {
	Attrs []xml.Attr `xml:",any,attr"`
	Data  string     `xml:",chardata"`
}

type fdxParagraph struct {
	Type         string    `xml:"Type,attr"`
	Number       *string   `xml:"Number,attr"`
	Text         []fdxText `xml:"Text"`
	DualDialogue *struct{} `xml:"DualDialogue"`
}

type fdxDoc struct {
	Paragraphs []fdxParagraph `xml:"Content>Paragraph"`
}

// xml2js gives a plain string for <Text> without attributes, an object otherwise.
func (t fdxText) isString() bool { return len(t.Attrs) == 0 }

// text is xml2js's element.Text[i] string, or its `_` (whitespace-only text is dropped).
func (t fdxText) text() string {
	if strings.TrimSpace(t.Data) == "" {
		return ""
	}
	return t.Data
}

func elementText(p fdxParagraph) string {
	if len(p.Text) == 0 {
		return ""
	}
	return p.Text[0].text()
}

// ParseFDX ports importers/final-draft.js importFdxData.
func ParseFDX(data []byte) ([]Node, error) {
	var doc fdxDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("could not parse Final Draft XML: %w", err)
	}
	var script []Node
	scene := Node{Type: "scene"}
	curTime, curScene, sceneWords, startSceneTime := 0, 0, 0, 0
	character := ""
	flush := func() {
		if len(scene.Script) == 0 {
			return
		}
		scene.Duration = curTime - startSceneTime
		scene.WordCount = sceneWords
		if scene.SceneNumber == 0 {
			scene.SceneNumber = curScene
		}
		if scene.SceneID == "" {
			scene.SceneID = fmt.Sprintf("G%d", curScene)
		}
		if scene.Slugline == "" {
			scene.Slugline = "BLACK"
		}
		script = append(script, scene)
	}
	for _, p := range doc.Paragraphs {
		switch p.Type {
		case "Scene Heading":
			flush()
			startSceneTime = curTime
			sceneWords = 0
			scene = Node{Type: "scene"}
			curScene++
			scene.Script = append(scene.Script, Item{Type: "scene_padding", Time: curTime, Duration: 2000, Scene: curScene})
			curTime += 2000
			scene.SceneNumber = curScene
			scene.Slugline = strings.ToUpper(elementText(p))
			scene.Time = curTime
		case "Action", "General":
			if p.DualDialogue != nil || len(p.Text) == 0 || !p.Text[0].isString() {
				continue
			}
			text := p.Text[0].text()
			if len(p.Text) > 1 && !p.Text[1].isString() {
				text += p.Text[1].text()
			}
			it := Item{Type: "action", Text: text, Time: curTime, Duration: DurationOfWords(text, 200) + 500, Scene: curScene}
			curTime += it.Duration
			scene.Script = append(scene.Script, it)
			sceneWords += WordCount(text)
		case "Character":
			if t := elementText(p); t != "" {
				character = strings.ToUpper(t)
				sceneWords += WordCount(character)
			}
		case "Parenthetical", "Dialogue":
			if len(p.Text) == 0 {
				continue
			}
			if p.Type == "Dialogue" && !p.Text[0].isString() {
				continue
			}
			text := p.Text[0].text()
			words := text
			if !p.Text[0].isString() {
				words = "" // the original counts an object as zero words
			}
			it := Item{Type: strings.ToLower(p.Type), Text: text, Time: curTime, Duration: DurationOfWords(words, 300) + 1000, Scene: curScene, Character: character}
			curTime += it.Duration
			scene.Script = append(scene.Script, it)
			sceneWords += WordCount(words)
		}
	}
	flush()
	return script, nil
}

// FDXCharacters counts dialogue/parenthetical lines per character, in first-seen order.
func FDXCharacters(nodes []Node) []Count {
	var out []Count
	idx := map[string]int{}
	for _, n := range nodes {
		if n.Type != "scene" {
			continue
		}
		for _, it := range n.Script {
			if it.Type != "dialogue" && it.Type != "parenthetical" {
				continue
			}
			name := strings.TrimSpace(strings.SplitN(strings.SplitN(it.Character, "(", 2)[0], " AND ", 2)[0])
			if i, ok := idx[name]; ok {
				out[i].Count++
			} else {
				idx[name] = len(out)
				out = append(out, Count{name, 1})
			}
		}
	}
	return out
}

// NodeLocations counts scene locations in first-seen order.
func NodeLocations(nodes []Node) []Count {
	var out []Count
	idx := map[string]int{}
	for _, n := range nodes {
		if n.Type != "scene" {
			continue
		}
		loc := LocationOf(n.Slugline)
		if i, ok := idx[loc]; ok {
			out[i].Count++
		} else {
			idx[loc] = len(out)
			out = append(out, Count{loc, 1})
		}
	}
	return out
}

var reParagraphTag = regexp.MustCompile(`<Paragraph\b[^>]*>`)
var reTypeSceneHeading = regexp.MustCompile(`\bType="Scene Heading"`)
var reNumberAttr = regexp.MustCompile(`\bNumber=`)

// InsertFDXSceneIDs adds a Number attribute to scene headings that lack one.
// It edits the tag text in place instead of re-serializing the whole document.
func InsertFDXSceneIDs(data []byte, gen func() string) ([]byte, []string) {
	var added []string
	out := reParagraphTag.ReplaceAllFunc(data, func(tag []byte) []byte {
		if !reTypeSceneHeading.Match(tag) || reNumberAttr.Match(tag) {
			return tag
		}
		id := gen()
		added = append(added, id)
		end := len(tag) - 1
		if bytes.HasSuffix(tag, []byte("/>")) {
			end--
		}
		return []byte(string(tag[:end]) + ` Number="` + id + `"` + string(tag[end:]))
	})
	return out, added
}
