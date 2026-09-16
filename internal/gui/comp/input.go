package comp

import (
	"image"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
)

type InputSpec struct {
	Label, Value, Hint string
	// DisplayValue replaces Value only while unfocused (e.g. "640 (default)").
	DisplayValue string
	Multiline    bool
	Choices      []string
}

// Input owns a draft, not the application's committed value. Handle returns
// intents; the application may reject Submit and keep the draft and cursor.
type Input struct {
	Spec      InputSpec
	buffer    []rune
	cursor    int
	selectAll bool
}
type InputIntent uint8

const (
	InputIdle InputIntent = iota
	InputSubmit
	InputCancel
	InputNext
	InputPrevious
)

type InputResult struct {
	Intent InputIntent
	Err    error
}
type Clipboard interface {
	ReadText() (string, error)
	WriteText(string) error
}
type SystemClipboard struct{}

func (SystemClipboard) ReadText() (string, error) { return readClipboard() }
func (SystemClipboard) WriteText(s string) error  { return writeClipboard(s) }

func NewInput(spec InputSpec) Input {
	return Input{Spec: spec, buffer: []rune(spec.Value), cursor: len([]rune(spec.Value))}
}
func (i Input) Text() string { return string(i.buffer) }
func (i Input) Cursor() int  { return i.cursor }
func (i Input) Dirty() bool  { return i.Text() != i.Spec.Value }
func (i Input) Clone() Input { i.buffer = append([]rune(nil), i.buffer...); return i }
func (i *Input) SetText(s string) {
	i.buffer = []rune(i.clean(s))
	i.cursor = len(i.buffer)
	i.selectAll = false
}
func (i *Input) SetCursor(n int) { i.cursor = max(0, min(n, len(i.buffer))); i.selectAll = false }
func (i *Input) SelectAll()      { i.selectAll = true }
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
func (i *Input) Insert(s string) {
	if i.selectAll {
		i.buffer = nil
		i.cursor = 0
		i.selectAll = false
	}
	r := []rune(i.clean(s))
	next := append([]rune(nil), i.buffer[:i.cursor]...)
	next = append(next, r...)
	i.buffer = append(next, i.buffer[i.cursor:]...)
	i.cursor += len(r)
}
func (i *Input) NextChoice() {
	if len(i.Spec.Choices) == 0 {
		return
	}
	next := 0
	for n, v := range i.Spec.Choices {
		if v == i.Text() {
			next = (n + 1) % len(i.Spec.Choices)
			break
		}
	}
	i.SetText(i.Spec.Choices[next])
}
func (i *Input) verticalCursor(direction int) {
	start := i.cursor
	for start > 0 && i.buffer[start-1] != '\n' {
		start--
	}
	column := i.cursor - start
	if direction < 0 {
		if start == 0 {
			return
		}
		end := start - 1
		start = end
		for start > 0 && i.buffer[start-1] != '\n' {
			start--
		}
		i.cursor = min(start+column, end)
	} else {
		end := i.cursor
		for end < len(i.buffer) && i.buffer[end] != '\n' {
			end++
		}
		if end == len(i.buffer) {
			return
		}
		start = end + 1
		end = start
		for end < len(i.buffer) && i.buffer[end] != '\n' {
			end++
		}
		i.cursor = min(start+column, end)
	}
	i.selectAll = false
}
func (i *Input) Handle(k Keyboard, clipboard Clipboard) InputResult {
	if k.JustPressed(ebiten.KeyEscape) {
		return InputResult{Intent: InputCancel}
	}
	if k.JustPressed(ebiten.KeyTab) {
		if k.Shift {
			return InputResult{Intent: InputPrevious}
		}
		return InputResult{Intent: InputNext}
	}
	if len(i.Spec.Choices) > 0 && (k.JustPressed(ebiten.KeyEnter) || k.JustPressed(ebiten.KeySpace)) {
		i.NextChoice()
		return InputResult{Intent: InputSubmit}
	}
	if k.Command {
		if k.JustPressed(ebiten.KeyA) {
			i.SelectAll()
		}
		if (k.JustPressed(ebiten.KeyC) || k.JustPressed(ebiten.KeyX)) && i.selectAll && clipboard != nil {
			if err := clipboard.WriteText(i.Text()); err != nil {
				return InputResult{Err: err}
			}
			if k.JustPressed(ebiten.KeyX) {
				i.SetText("")
			}
		}
		if k.JustPressed(ebiten.KeyV) && clipboard != nil {
			s, err := clipboard.ReadText()
			if err != nil {
				return InputResult{Err: err}
			}
			i.Insert(s)
		}
		if k.JustPressed(ebiten.KeyEnter) {
			return InputResult{Intent: InputSubmit}
		}
		return InputResult{}
	}
	if k.Repeats(ebiten.KeyArrowLeft) {
		i.SetCursor(i.cursor - 1)
	}
	if k.Repeats(ebiten.KeyArrowRight) {
		i.SetCursor(i.cursor + 1)
	}
	if i.Spec.Multiline {
		if k.Repeats(ebiten.KeyArrowUp) {
			i.verticalCursor(-1)
		}
		if k.Repeats(ebiten.KeyArrowDown) {
			i.verticalCursor(1)
		}
	}
	if k.Repeats(ebiten.KeyHome) {
		i.SetCursor(0)
	}
	if k.Repeats(ebiten.KeyEnd) {
		i.SetCursor(len(i.buffer))
	}
	if k.Repeats(ebiten.KeyBackspace) {
		if i.selectAll {
			i.SetText("")
		} else if i.cursor > 0 {
			i.buffer = append(i.buffer[:i.cursor-1], i.buffer[i.cursor:]...)
			i.cursor--
		}
	}
	if k.Repeats(ebiten.KeyDelete) {
		if i.selectAll {
			i.SetText("")
		} else if i.cursor < len(i.buffer) {
			i.buffer = append(i.buffer[:i.cursor], i.buffer[i.cursor+1:]...)
		}
	}
	if k.JustPressed(ebiten.KeyEnter) {
		if i.Spec.Multiline {
			i.Insert("\n")
		} else {
			return InputResult{Intent: InputSubmit}
		}
	}
	if k.Text != "" {
		i.Insert(k.Text)
	}
	return InputResult{}
}

// Draw uses bounds for the editable box; its label sits 25px above and its hint
// 4px below. Form supplies consistent spacing and clips the complete row.
func (i Input) Draw(dst *ebiten.Image, p *Painter, bounds image.Rectangle, focused bool) {
	t := p.Theme
	p.Text(dst, i.Spec.Label, bounds.Min.X, bounds.Min.Y-25, 15, t.Text)
	RoundedRect(dst, bounds, Radius, t.Input)
	border := t.Border
	if focused {
		border = t.Accent
	}
	Surface(dst, bounds, Radius, t.Input, border)
	p.Text(dst, p.Fit(i.Spec.Hint, bounds.Dx(), 11), bounds.Min.X, bounds.Max.Y+4, 11, t.Muted)
	clipped := bounds.Inset(5).Intersect(dst.Bounds())
	if clipped.Empty() {
		return
	}
	clip := dst.SubImage(clipped).(*ebiten.Image)
	value := i.Spec.Value
	if i.Spec.DisplayValue != "" {
		value = i.Spec.DisplayValue
	}
	if len(i.Spec.Choices) > 0 {
		if value == "" {
			value = "Default"
		}
		p.Text(clip, "›", bounds.Max.X-20, bounds.Min.Y+5, 17, t.Accent)
	}
	if focused {
		value = i.Text()
	}
	lines := strings.Split(value, "\n")
	start := 0
	caretLine, caretCol := 0, 0
	if focused {
		before := string(i.buffer[:i.cursor])
		caretLine = strings.Count(before, "\n")
		caretCol = len([]rune(before[strings.LastIndex(before, "\n")+1:]))
	}
	visible := max(1, (bounds.Dy()-10)/20)
	if caretLine >= visible {
		start = caretLine - visible + 1
	}
	for n := start; n < len(lines) && n < start+visible; n++ {
		str := lines[n]
		col := caretCol
		if focused && n == caretLine {
			rr := []rune(str)
			for col > 0 && p.Measure(string(rr[:col]), 14) > bounds.Dx()-22 {
				rr = rr[1:]
				col--
			}
			str = string(rr)
		}
		y := bounds.Min.Y + 6 + (n-start)*20
		if focused && i.selectAll {
			Rect(clip, Box(bounds.Min.X+7, y, bounds.Dx()-14, 20), t.Selection)
		}
		p.Text(clip, str, bounds.Min.X+8, y, 14, t.Text)
		if focused && n == caretLine {
			rr := []rune(str)
			x := bounds.Min.X + 8 + p.Measure(string(rr[:min(col, len(rr))]), 14)
			Rect(clip, Box(x, y+1, 1, 18), t.Accent)
		}
	}
}
