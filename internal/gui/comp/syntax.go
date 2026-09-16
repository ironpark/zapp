package comp

import (
	"image/color"
	"strconv"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type syntaxToken struct{ text, kind string }

// A tolerant highlighter, independent of validation: incomplete drafts must draw.
func yamlTokens(line string) []syntaxToken {
	r := []rune(line)
	tokens := []syntaxToken{}
	for i := 0; i < len(r); {
		start, kind := i, "plain"
		switch {
		case unicode.IsSpace(r[i]):
			for i < len(r) && unicode.IsSpace(r[i]) {
				i++
			}
		case r[i] == '#' && (i == 0 || unicode.IsSpace(r[i-1])):
			i = len(r)
			kind = "comment"
		case r[i] == '\'' || r[i] == '"':
			quote := r[i]
			i++
			for i < len(r) {
				if quote == '"' && r[i] == '\\' {
					i = min(len(r), i+2)
					continue
				}
				if r[i] == quote {
					i++
					if quote == '\'' && i < len(r) && r[i] == quote {
						i++
						continue
					}
					break
				}
				i++
			}
			kind = "string"
		case strings.ContainsRune("{}[],:", r[i]):
			i++
			kind = "punctuation"
		default:
			for i < len(r) && !unicode.IsSpace(r[i]) && !strings.ContainsRune("{}[],", r[i]) {
				if r[i] == ':' && (i+1 == len(r) || unicode.IsSpace(r[i+1])) {
					break
				}
				i++
			}
			if i == start {
				i++
			}
			value := strings.ToLower(string(r[start:i]))
			if value == "true" || value == "false" || value == "null" || value == "~" {
				kind = "literal"
			} else if _, err := strconv.ParseFloat(value, 64); err == nil {
				kind = "number"
			} else if _, err := strconv.ParseInt(value, 0, 64); err == nil {
				kind = "number"
			}
		}
		if kind != "comment" && kind != "punctuation" && strings.TrimSpace(string(r[start:i])) != "" {
			j := i
			for j < len(r) && unicode.IsSpace(r[j]) {
				j++
			}
			if j < len(r) && r[j] == ':' {
				kind = "key"
			}
		}
		tokens = append(tokens, syntaxToken{string(r[start:i]), kind})
	}
	return tokens
}

func (p *Painter) monoFace(size int) font.Face {
	if p.mono == nil {
		p.mono, _ = opentype.Parse(gomono.TTF)
		p.monoFaces = make(map[int]font.Face)
	}
	return cachedFace(p.mono, p.monoFaces, size)
}

// codeRunes walks s in the mono face, falling back to the UI face for glyphs the
// mono face lacks, and returns the advance reached after the last rune. Callers
// chain segments by passing the previous return value as start.
func (p *Painter) codeRunes(s string, size int, start fixed.Int26_6, draw func(rune, font.Face, fixed.Int26_6)) fixed.Int26_6 {
	face := p.monoFace(size)
	advance := start
	for _, r := range s {
		f := face
		a, ok := f.GlyphAdvance(r)
		if !ok {
			f = p.face(size)
			a, _ = f.GlyphAdvance(r)
		}
		if draw != nil {
			draw(r, f, advance)
		}
		advance += a
	}
	return advance
}
func (p *Painter) codeMeasure(s string, size int) int { return p.codeRunes(s, size, 0, nil).Ceil() }

// codeText draws s starting at the given advance and returns the advance after
// it. Runes sharing a face are batched into one draw call; only a fallback glyph
// breaks the run.
func (p *Painter) codeText(dst *ebiten.Image, s string, x, y, size int, start fixed.Int26_6, c color.Color) fixed.Int26_6 {
	var run []rune
	var runFace font.Face
	var runAt fixed.Int26_6
	flush := func() {
		if len(run) > 0 {
			text.Draw(dst, string(run), runFace, x+runAt.Round(), y+size, c)
			run = run[:0]
		}
	}
	end := p.codeRunes(s, size, start, func(r rune, f font.Face, a fixed.Int26_6) {
		if f != runFace {
			flush()
			runFace, runAt = f, a
		}
		run = append(run, r)
	})
	flush()
	return end
}
func (p *Painter) drawYAMLLine(dst *ebiten.Image, s string, x, y, size int) {
	var advance fixed.Int26_6
	for _, token := range yamlTokens(s) {
		c := p.Theme.Text
		switch token.kind {
		case "key":
			c = p.Theme.Accent
		case "string":
			c = color.RGBA{153, 207, 165, 255}
		case "number":
			c = color.RGBA{240, 193, 135, 255}
		case "literal":
			c = color.RGBA{198, 167, 238, 255}
		case "comment":
			c = p.Theme.Muted
		case "punctuation":
			c = color.RGBA{178, 186, 204, 255}
		}
		advance = p.codeText(dst, token.text, x, y, size, advance, c)
	}
}
