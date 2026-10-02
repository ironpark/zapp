package gui

import (
	"strconv"
	"strings"
	"unicode"
)

// InputSpec is how a field presents: its label, hint, placeholder, error,
// choices, grouping, syntax and number stepping.
type InputSpec struct {
	// Group starts a titled section above this row.
	Group              string
	Boolean            bool
	Secret             bool
	Syntax             string
	Label, Value, Hint string
	Placeholder        string
	Number             *NumberSpec
	Error              string
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

// Input is the draft text of the focused field, kept apart from its
// committed Spec.Value. SetText, StepNumber and Clone change or copy it
// without touching the value; commit applies it as a validated project
// transaction and, when that fails, keeps the draft for repair.
type Input struct {
	Spec   InputSpec
	buffer []rune
}

func NewInput(spec InputSpec) Input {
	return Input{Spec: spec, buffer: []rune(spec.Value)}
}
func (i Input) Text() string      { return string(i.buffer) }
func (i Input) Dirty() bool       { return i.Text() != i.Spec.Value }
func (i Input) Clone() Input      { i.buffer = append([]rune(nil), i.buffer...); return i }
func (i *Input) SetText(s string) { i.buffer = []rune(i.clean(s)) }

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
