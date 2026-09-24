// Package script parses Fountain and Final Draft scripts into the scene list
// the original app uses (vendor/fountain.js + fountain-data-parser.js,
// importers/final-draft.js), including its duration estimates.
package script

import (
	"regexp"
	"strings"
)

// Token is one element produced by the Fountain tokenizer.
type Token struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	HasText     bool   `json:"-"`
	SceneNumber string `json:"scene_number,omitempty"` // the #id# of a scene heading
	Depth       int    `json:"depth,omitempty"`
	Dual        string `json:"dual,omitempty"`
}

var (
	reTitlePage    = regexp.MustCompile(`(?m)^((?:title|credit|author[s]?|format|source|notes|draft date|date|contact|copyright|Title|Credit|Author[s]?|Format|Source|Notes|Draft date|Draft Date|Date|Contact|Copyright):)`)
	reSceneHeading = regexp.MustCompile(`(?i)^((?:\*{0,3}_?)?(?:(?:int|ext|est|i/e)[. ]).+)`)
	reSceneNumber  = regexp.MustCompile(`( *#(.+)# *)`)
	reTransition   = regexp.MustCompile(`^((?:FADE (?:TO BLACK|OUT)|CUT TO BLACK)\.|.+ TO:)|^(?:> *)(.+)`)
	reDialogue     = regexp.MustCompile(`^([A-Z*_]+[0-9A-Z (._\-'’,)]*)(\^?)?\n([\s\S]+)`)
	reParenSplit   = regexp.MustCompile(`(\(.+\))(?:\n+)`)
	reParenthetic  = regexp.MustCompile(`^(\(.+\))$`)
	reCentered     = regexp.MustCompile(`^(?:> *)(.+)(?: *<)(\n.+)*`)
	reSection      = regexp.MustCompile(`^(#+)(?: *)(.*)`)
	reBoneyard     = regexp.MustCompile(`(?m)/\*[\s\S]*?\*/|([^:]|^)//.*$`)
	rePageBreak    = regexp.MustCompile(`^={3,}$`)
	reLineBreak    = regexp.MustCompile(`^ {2}$`)
	reSplitter     = regexp.MustCompile(`\n{2,}`)
	reWhitespacer  = regexp.MustCompile(`(?m)^\t+|^ {3,}`)
	reColonSplit   = regexp.MustCompile(`:\n*`)

	// emphasis; the original's lookaheads only matter for mixed openers/closers
	reBoldItalicUnderline = regexp.MustCompile(`(_{1}\*{3}|\*{3}_{1})(.+?)(\*{3}_{1}|_{1}\*{3})`)
	reBoldUnderline       = regexp.MustCompile(`(_{1}\*{2}|\*{2}_{1})(.+?)(\*{2}_{1}|_{1}\*{2})`)
	reItalicUnderline     = regexp.MustCompile(`(?:_{1}\*{1}|\*{1}_{1})(.+?)(\*{1}_{1}|_{1}\*{1})`)
	reBoldItalic          = regexp.MustCompile(`(\*{3})(.+?)(\*{3})`)
	reBold                = regexp.MustCompile(`(\*{2})(.+?)(\*{2})`)
	reItalic              = regexp.MustCompile(`(\*{1})(.+?)(\*{1})`)
	reUnderline           = regexp.MustCompile(`(_{1})(.+?)(_{1})`)
)

// JS String#replace with a non-global regex replaces only the first match.
func replaceFirst(re *regexp.Regexp, s, repl string) string {
	loc := re.FindStringSubmatchIndex(s)
	if loc == nil {
		return s
	}
	var dst []byte
	dst = re.ExpandString(dst, repl, s, loc)
	return s[:loc[0]] + string(dst) + s[loc[1]:]
}

func lexer(script string) string {
	s := reBoneyard.ReplaceAllString(script, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// cleaner /^\n+|\n+$/ without g: only the first of the two
	if strings.HasPrefix(s, "\n") {
		s = strings.TrimLeft(s, "\n")
	} else {
		s = strings.TrimRight(s, "\n")
	}
	return reWhitespacer.ReplaceAllString(s, "")
}

func cleanFirst(s string) string {
	if strings.HasPrefix(s, "\n") {
		return strings.TrimLeft(s, "\n")
	}
	return strings.TrimRight(s, "\n")
}

// ^(?:\.(?!\.+))(.+) : a forced heading starts with one dot
func forcedHeading(line string) (string, bool) {
	if !strings.HasPrefix(line, ".") || strings.HasPrefix(line, "..") {
		return "", false
	}
	rest := line[1:]
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[:i]
	}
	return rest, rest != ""
}

// Tokenize ports fountain-js 0.1.10 tokenize + inline lexer.
func Tokenize(script string) []Token {
	src := reSplitter.Split(lexer(script), -1)
	var tokens []Token
	dual := false
	for i := len(src) - 1; i >= 0; i-- {
		line := src[i]

		if reTitlePage.MatchString(line) {
			match := reSplitter.Split(reTitlePage.ReplaceAllString(line, "\n$1"), -1)
			for x := len(match) - 1; x >= 0; x-- {
				parts := reColonSplit.Split(cleanFirst(match[x]), -1)
				typ := strings.Replace(strings.ToLower(jsTrim(parts[0])), " ", "_", 1)
				text := ""
				if len(parts) > 1 {
					text = jsTrim(parts[1])
				}
				tokens = append(tokens, Token{Type: typ, Text: text, HasText: true})
			}
			continue
		}

		var text string
		var ok bool
		if m := reSceneHeading.FindStringSubmatch(line); m != nil {
			text, ok = m[1], true
		} else {
			text, ok = forcedHeading(line)
		}
		if ok {
			if strings.Index(text, "  ") != len(text)-2 {
				meta := ""
				if m := reSceneNumber.FindStringSubmatch(text); m != nil {
					meta = m[2]
					text = replaceFirst(reSceneNumber, text, "")
				}
				tokens = append(tokens, Token{Type: "scene_heading", Text: text, HasText: true, SceneNumber: meta})
			}
			continue
		}

		if m := reCentered.FindString(line); m != "" {
			tokens = append(tokens, Token{Type: "centered", Text: strings.NewReplacer(">", "", "<", "").Replace(m), HasText: true})
			continue
		}

		if m := reTransition.FindStringSubmatch(line); m != nil {
			t := m[1]
			if t == "" {
				t = m[2]
			}
			tokens = append(tokens, Token{Type: "transition", Text: t, HasText: true})
			continue
		}

		if m := reDialogue.FindStringSubmatch(line); m != nil && strings.Index(m[1], "  ") != len(m[1])-2 {
			if m[2] != "" {
				tokens = append(tokens, Token{Type: "dual_dialogue_end"})
			}
			tokens = append(tokens, Token{Type: "dialogue_end"})
			parts := splitKeep(reParenSplit, m[3])
			for x := len(parts) - 1; x >= 0; x-- {
				t := parts[x]
				if len(t) > 0 {
					typ := "dialogue"
					if reParenthetic.MatchString(t) {
						typ = "parenthetical"
					}
					tokens = append(tokens, Token{Type: typ, Text: t, HasText: true})
				}
			}
			tokens = append(tokens, Token{Type: "character", Text: jsTrim(m[1]), HasText: true})
			d := ""
			if m[2] != "" {
				d = "right"
			} else if dual {
				d = "left"
			}
			tokens = append(tokens, Token{Type: "dialogue_begin", Dual: d})
			if dual {
				tokens = append(tokens, Token{Type: "dual_dialogue_begin"})
			}
			dual = m[2] != ""
			continue
		}

		if m := reSection.FindStringSubmatch(line); m != nil {
			tokens = append(tokens, Token{Type: "section", Text: m[2], HasText: true, Depth: len(m[1])})
			continue
		}

		// synopsis ^(?:\=(?!\=+) *)(.*)
		if strings.HasPrefix(line, "=") && !strings.HasPrefix(line, "==") {
			t := strings.TrimLeft(line[1:], " ")
			if i := strings.IndexByte(t, '\n'); i >= 0 {
				t = t[:i]
			}
			tokens = append(tokens, Token{Type: "synopsis", Text: t, HasText: true})
			continue
		}

		// note ^(?:\[{2}(?!\[+))(.+)(?:\]{2}(?!\[+))$
		if len(line) > 4 && strings.HasPrefix(line, "[[") && line[2] != '[' && strings.HasSuffix(line, "]]") && !strings.Contains(line, "\n") {
			tokens = append(tokens, Token{Type: "note", Text: line[2 : len(line)-2], HasText: true})
			continue
		}

		if reBoneyard.MatchString(line) {
			continue
		}
		if rePageBreak.MatchString(line) {
			tokens = append(tokens, Token{Type: "page_break"})
			continue
		}
		if reLineBreak.MatchString(line) {
			tokens = append(tokens, Token{Type: "line_break"})
			continue
		}
		tokens = append(tokens, Token{Type: "action", Text: line, HasText: true})
	}

	// parse(): inline lexing, then reverse into reading order
	for i := range tokens {
		if tokens[i].HasText {
			tokens[i].Text = inlineLex(tokens[i].Text)
			tokens[i].HasText = tokens[i].Text != ""
		}
	}
	for i, j := 0, len(tokens)-1; i < j; i, j = i+1, j-1 {
		tokens[i], tokens[j] = tokens[j], tokens[i]
	}
	return tokens
}

// splitKeep mimics JS String#split with a capturing regex.
func splitKeep(re *regexp.Regexp, s string) []string {
	var parts []string
	last := 0
	for _, m := range re.FindAllStringSubmatchIndex(s, -1) {
		parts = append(parts, s[last:m[0]], s[m[2]:m[3]])
		last = m[1]
	}
	return append(parts, s[last:])
}

// replaceNotes implements /(?:\[{2}(?!\[+))([\s\S]+?)(?:\]{2}(?!\[+))/g -> <!-- $1 -->
func replaceNotes(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		p := strings.Index(s[i:], "[[")
		if p < 0 {
			break
		}
		p += i
		if p+2 < len(s) && s[p+2] != '[' {
			// find the first "]]" after at least one char, not followed by "["
			q := -1
			for k := p + 3; k+1 < len(s); k++ {
				if s[k] == ']' && s[k+1] == ']' && (k+2 >= len(s) || s[k+2] != '[') {
					q = k
					break
				}
			}
			if q >= 0 {
				b.WriteString(s[i:p])
				b.WriteString("<!-- " + s[p+2:q] + " -->")
				i = q + 2
				continue
			}
		}
		b.WriteString(s[i : p+1])
		i = p + 1
	}
	b.WriteString(s[i:])
	return b.String()
}

func inlineLex(s string) string {
	if s == "" {
		return ""
	}
	s = replaceNotes(s)
	s = strings.ReplaceAll(s, `\*`, "[star]")
	s = strings.ReplaceAll(s, `\_`, "[underline]")
	s = strings.ReplaceAll(s, "\n", "<br />")
	for _, st := range []struct {
		re   *regexp.Regexp
		repl string
	}{
		{reBoldItalicUnderline, `<span class="bold italic underline">$2</span>`},
		{reBoldUnderline, `<span class="bold underline">$2</span>`},
		{reItalicUnderline, `<span class="italic underline">$2</span>`},
		{reBoldItalic, `<span class="bold italic">$2</span>`},
		{reBold, `<span class="bold">$2</span>`},
		{reItalic, `<span class="italic">$2</span>`},
		{reUnderline, `<span class="underline">$2</span>`},
	} {
		s = st.re.ReplaceAllString(s, st.repl)
	}
	s = strings.ReplaceAll(s, "[star]", "*")
	s = strings.ReplaceAll(s, "[underline]", "_")
	return jsTrim(s)
}

func jsTrim(s string) string { return strings.TrimSpace(strings.Trim(s, "\uFEFF")) }

// WordCount ports `text.trim().replace(/ +(?= )/g,”).split(' ').length`.
func WordCount(text string) int {
	if text == "" {
		return 0
	}
	t := jsTrim(text)
	for strings.Contains(t, "  ") {
		t = strings.ReplaceAll(t, "  ", " ")
	}
	return strings.Count(t, " ") + 1
}

func DurationOfWords(text string, perWord int) int { return WordCount(text) * perWord }
