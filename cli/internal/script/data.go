package script

import (
	"sort"
	"strconv"
	"strings"
)

// Item is a timed element inside a scene (action, dialogue, ...).
type Item struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	SceneNumber string `json:"scene_number,omitempty"`
	Depth       int    `json:"depth,omitempty"`
	Dual        string `json:"dual,omitempty"`
	Time        int    `json:"time"`
	Duration    int    `json:"duration"`
	Scene       int    `json:"scene"`
	Page        int    `json:"page,omitempty"`
	Character   string `json:"character,omitempty"`
}

// Node is a top-level script node: title, section or scene.
type Node struct {
	Type        string            `json:"type"`
	Text        string            `json:"text,omitempty"`
	Depth       int               `json:"depth,omitempty"`
	Time        int               `json:"time"`
	Duration    int               `json:"duration"`
	Scene       int               `json:"scene,omitempty"`
	TitlePage   map[string]string `json:"title_page,omitempty"`
	Script      []Item            `json:"script,omitempty"`
	SceneNumber int               `json:"scene_number,omitempty"`
	SceneID     string            `json:"scene_id,omitempty"`
	Slugline    string            `json:"slugline,omitempty"`
	Synopsis    string            `json:"synopsis,omitempty"`
	WordCount   int               `json:"word_count,omitempty"`
	Page        int               `json:"page,omitempty"`
}

// Count is a [name, count] pair, as the original returns them.
type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func linesForText(text string, charWidth int) int {
	if text == "" {
		return 0
	}
	line, curs := 0, 0
	for _, word := range strings.Split(text, " ") {
		if strings.Contains(word, "/>") {
			line++
			curs = len([]rune(word)) - 1
		} else if strings.Contains(word, "<br") {
			curs = 0
		} else if curs+len([]rune(word)) < charWidth {
			curs += len([]rune(word)) + 1
		} else {
			line++
			curs = len([]rune(word)) + 1
		}
	}
	return line + 1
}

// paginate assigns a page number to each token (fountain-data-parser.js).
func paginate(tokens []Token) []int {
	pages := make([]int, len(tokens))
	page, curLine, req := 0, 0, 0
	inDialogue := false
	for i, t := range tokens {
		if !inDialogue {
			req = 0
		}
		switch t.Type {
		case "scene_heading":
			req += 3
		case "action":
			req += linesForText(t.Text, 63) + 1
		case "dialogue_begin", "dual_dialogue_begin":
			inDialogue = true
		case "character", "parenthetical":
			req++
		case "dialogue":
			req += linesForText(t.Text, 35)
		case "dialogue_end", "dual_dialogue_end":
			req++
			inDialogue = false
		case "centered", "transition":
			req += 2
		}
		if !inDialogue {
			if curLine+req < 55 {
				curLine += req
			} else {
				page++
				curLine = req
				switch t.Type {
				case "scene_heading", "action", "centered", "transition", "dialogue_end", "dual_dialogue_end":
					curLine--
				}
			}
		}
		pages[i] = page + 1
	}
	return pages
}

// ParseFountainTokens ports fountain-data-parser parse(): scenes with estimated timing.
func ParseFountainTokens(tokens []Token) []Node {
	pages := paginate(tokens)
	var script []Node
	scene := Node{Type: "scene"}
	curTime, curScene, sceneWords, startSceneTime := 0, 0, 0, 0
	character := ""
	item := func(t Token, i int) Item {
		return Item{Type: t.Type, Text: t.Text, SceneNumber: t.SceneNumber, Depth: t.Depth, Dual: t.Dual, Page: pages[i]}
	}
	for i, t := range tokens {
		switch t.Type {
		case "title":
			script = append(script, Node{Type: "title", Text: t.Text, Time: curTime, Duration: 2000, Scene: curScene, Page: pages[i]})
			curTime += 2000
		case "credit", "author", "authors", "format", "source", "notes", "draft_date", "date", "contact", "copyright":
			if len(script) > 0 {
				if script[0].TitlePage == nil {
					script[0].TitlePage = map[string]string{}
				}
				script[0].TitlePage[t.Type] = t.Text
			}
		case "scene_heading":
			if len(scene.Script) > 0 {
				scene.Duration = curTime - startSceneTime
				scene.WordCount = sceneWords
				if scene.SceneNumber == 0 {
					scene.SceneNumber = curScene
				}
				if scene.SceneID == "" {
					scene.SceneID = "G" + strconv.Itoa(curScene)
				}
				if scene.Slugline == "" {
					scene.Slugline = "BLACK"
				}
				script = append(script, scene)
			}
			startSceneTime = curTime
			sceneWords = 0
			curScene++
			scene = Node{Type: "scene"}
			scene.Script = append(scene.Script, Item{Type: "scene_padding", Time: curTime, Duration: 2000, Scene: curScene, Page: pages[i]})
			curTime += 2000
			scene.SceneNumber = curScene
			scene.SceneID = t.SceneNumber
			scene.Slugline = t.Text
			scene.Time = curTime
			scene.Page = pages[i]
			it := item(t, i)
			it.Time, it.Duration, it.Scene = curTime, 0, curScene
			scene.Script = append(scene.Script, it)
			sceneWords += WordCount(t.Text)
		case "action", "parenthetical", "dialogue", "transition":
			it := item(t, i)
			it.Time, it.Scene = curTime, curScene
			switch t.Type {
			case "action":
				it.Duration = DurationOfWords(t.Text, 200) + 500
			case "transition":
				it.Duration = 1000
			default:
				it.Duration = DurationOfWords(t.Text, 300) + 1000
				it.Character = character
			}
			curTime += it.Duration
			scene.Script = append(scene.Script, it)
			sceneWords += WordCount(t.Text)
		case "character":
			character = t.Text
			sceneWords += WordCount(t.Text)
		case "section":
			if t.Depth == 1 {
				if len(scene.Script) > 0 {
					scene.Duration = curTime - startSceneTime
					scene.WordCount = sceneWords
					script = append(script, scene)
					scene = Node{Type: "scene"}
				}
				script = append(script, Node{Type: "section", Text: t.Text, Depth: t.Depth, Time: curTime, Scene: curScene, Page: pages[i]})
			} else {
				it := item(t, i)
				it.Time, it.Scene = curTime, curScene
				scene.Script = append(scene.Script, it)
			}
		case "synopsis":
			scene.Synopsis = t.Text
		}
	}
	if len(scene.Script) > 0 {
		scene.Duration = curTime - startSceneTime
		scene.WordCount = sceneWords
		script = append(script, scene)
	}
	return script
}

// Characters counts character cues, most frequent first (fountain).
func Characters(tokens []Token) []Count {
	var out []Count
	idx := map[string]int{}
	for _, t := range tokens {
		if t.Type != "character" {
			continue
		}
		name := strings.TrimSpace(strings.SplitN(strings.SplitN(t.Text, "(", 2)[0], " AND ", 2)[0])
		if i, ok := idx[name]; ok {
			out[i].Count++
		} else {
			idx[name] = len(out)
			out = append(out, Count{name, 1})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

// LocationOf strips the time of day: "INT. HOUSE - DAY" -> "INT. HOUSE".
func LocationOf(slug string) string {
	parts := strings.Split(slug, " - ")
	if len(parts) > 1 {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, " - ")
}

// Locations counts slugline locations in order of first appearance (fountain).
func Locations(tokens []Token) []Count {
	var out []Count
	idx := map[string]int{}
	for _, t := range tokens {
		if t.Type != "scene_heading" {
			continue
		}
		loc := LocationOf(t.Text)
		if i, ok := idx[loc]; ok {
			out[i].Count++
		} else {
			idx[loc] = len(out)
			out = append(out, Count{loc, 1})
		}
	}
	return out
}
