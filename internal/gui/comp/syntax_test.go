package comp

import (
	"image"
	"strings"
	"testing"

	"github.com/ironpark/ggfx"
)

func TestYAMLHighlightKeepsSourceAndQuotedComments(t *testing.T) {
	for _, line := range []string{"  width: 640 # pixels", `title: "hello # world"`, "link: true", "'한글 키': '값'", "url: https://example.test/a#b", "title: \"unfinished"} {
		var source strings.Builder
		for _, token := range yamlTokens(line) {
			source.WriteString(token.text)
			if token.kind == "comment" && strings.Contains(token.text, "world") {
				t.Fatal("quoted # became a comment")
			}
		}
		if source.String() != line {
			t.Fatalf("highlight altered text: %q", line)
		}
	}
	kinds := map[string]bool{}
	for _, token := range yamlTokens(`width: 640 # pixels`) {
		kinds[token.kind] = true
	}
	if !kinds["key"] || !kinds["number"] || !kinds["comment"] {
		t.Fatal("missing highlighting")
	}
}

func TestCodeIndentAndScroll(t *testing.T) {
	i := NewInput(InputSpec{Multiline: true, Syntax: "yaml", Value: "window:"})
	i.Handle(Keyboard{Pressed: []ggfx.Key{ggfx.KeyEnter}}, nil)
	if i.Text() != "window:\n  " {
		t.Fatal("newline did not indent")
	}
	i.Handle(Keyboard{Pressed: []ggfx.Key{ggfx.KeyTab}}, nil)
	if i.Text() != "window:\n    " {
		t.Fatal("Tab did not indent")
	}
	i.SetText(strings.Repeat("key: value\n", 40))
	bounds := image.Rect(0, 0, 300, 210)
	i.ScrollLines(-100, bounds)
	if i.visibleStart(bounds, true) != 0 {
		t.Fatal("cannot scroll to first line")
	}
	i.ScrollLines(100, bounds)
	if i.visibleStart(bounds, true) != 31 {
		t.Fatal("scroll not clamped")
	}
}

func TestJSONHighlightAndIndent(t *testing.T) {
	line := `{"id":"app","selected":true,"size":42}`
	var source strings.Builder
	kinds := map[string]bool{}
	for _, token := range yamlTokens(line) {
		source.WriteString(token.text)
		kinds[token.kind] = true
	}
	if source.String() != line || !kinds["key"] || !kinds["string"] || !kinds["literal"] || !kinds["number"] {
		t.Fatal("JSON tokens lost source or colors")
	}
	i := NewInput(InputSpec{Multiline: true, Syntax: "json", Value: "["})
	i.Handle(Keyboard{Pressed: []ggfx.Key{ggfx.KeyEnter}}, nil)
	if i.Text() != "[\n  " {
		t.Fatalf("JSON indentation: %q", i.Text())
	}
}
