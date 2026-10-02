package gui

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/ironpark/ggui"
)

// InputSpec is how a field presents: its label, hint, placeholder,
// choices, grouping, syntax and number stepping. The error a field shows is
// its state's; see fieldState.
type InputSpec struct {
	// Group starts a titled section above this row.
	Group              string
	Boolean            bool
	Secret             bool
	Syntax             string
	Label, Value, Hint string
	Placeholder        string
	Number             *NumberSpec
	Browse             bool
	// DisplayValue replaces Value only while unfocused (e.g. "640 (default)").
	DisplayValue string
	Multiline    bool
	Choices      []string
	// SameRow places this input beside the previous input.
	SameRow bool
}

// NumberSpec describes integer stepping. Zero may represent an automatic default.
type NumberSpec struct {
	Min, Max, Step int
	Default        int
}

func (i *Input) StepNumber(direction int, large bool) {
	n := i.Spec.Number
	if n == nil {
		return
	}
	value, err := strconv.Atoi(strings.TrimSpace(i.Text()))
	if err != nil && strings.TrimSpace(i.Text()) != "" {
		return
	}
	if value == 0 && n.Default != 0 {
		value = n.Default
	}
	step := max(1, n.Step)
	if large {
		step *= 10
	}
	value = max(n.Min, min(n.Max, value))
	i.SetText(strconv.Itoa(max(n.Min, min(n.Max, value+direction*step))))
}

// Input is the draft of the field being edited: the text its input shows,
// against the committed Spec.Value. It holds no text of its own; SetText
// writes the field's text, which the input is bound to, so what the user
// typed and what the editor commits are one value. Commit applies it as a
// validated project transaction and, when that fails, keeps the draft for
// repair.
type Input struct {
	Spec InputSpec // the field as it was when editing began
	text *ggui.StateValue[string]
}

// NewInput is a draft of spec with a text of its own, starting at the
// committed value.
func NewInput(spec InputSpec) Input {
	return Input{Spec: spec, text: ggui.State(spec.Value)}
}

func (i Input) Text() string {
	if i.text == nil {
		return ""
	}
	return i.text.Get()
}

func (i Input) Dirty() bool       { return i.Text() != i.Spec.Value }
func (i *Input) SetText(s string) { i.text.Set(i.clean(s)) }

// clean keeps newlines only in multiline inputs, turns tabs into spaces and
// drops other control characters.
func (i *Input) clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' && i.Spec.Multiline {
			return r
		}
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
