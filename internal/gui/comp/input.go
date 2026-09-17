package comp

import (
	"image"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
)

// lineWindow returns up to count lines of s starting at line start. A JSON
// field can hold hundreds of lines while only a few are drawn, so the whole
// value is never split.
func lineWindow(s string, start, count int) []string {
	for range start {
		_, rest, found := strings.Cut(s, "\n")
		if !found {
			return nil
		}
		s = rest
	}
	lines := make([]string, 0, count)
	for range count {
		line, rest, found := strings.Cut(s, "\n")
		lines = append(lines, line)
		if !found {
			break
		}
		s = rest
	}
	return lines
}

type InputSpec struct {
	// Group starts a titled section above this row.
	Group              string
	Boolean            bool
	Secret             bool
	Syntax             string
	Height             int
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
func StepBounds(bounds image.Rectangle, direction int) image.Rectangle {
	bounds.Min.X = bounds.Max.X - 26
	middle := (bounds.Min.Y + bounds.Max.Y) / 2
	if direction > 0 {
		bounds.Max.Y = middle
	} else {
		bounds.Min.Y = middle
	}
	return bounds
}

// Input owns a draft, not the application's committed value. Handle returns
// intents; the application may reject Submit and keep the draft and cursor.
type Input struct {
	Spec         InputSpec
	buffer       []rune
	cursor       int
	selectAll    bool
	viewLine     int
	manualScroll bool
}
type InputIntent uint8

const (
	InputIdle InputIntent = iota
	InputSubmit
	InputCancel
	InputNext
	InputPrevious
	InputOpenChoice
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
	i.manualScroll = false
	i.viewLine = 0
	i.buffer = []rune(i.clean(s))
	i.cursor = len(i.buffer)
	i.selectAll = false
}
func (i *Input) SetCursor(n int) { i.cursor = max(0, min(n, len(i.buffer))); i.selectAll = false }
func (i *Input) SelectAll()      { i.selectAll = true }

func (i Input) visibleStart(bounds image.Rectangle, focused bool) int {
	if i.manualScroll {
		return i.viewLine
	}
	if !focused {
		return 0
	}
	line := strings.Count(string(i.buffer[:i.cursor]), "\n")
	return max(0, line-max(1, (bounds.Dy()-10)/20)+1)
}

func (i *Input) ScrollLines(delta int, bounds image.Rectangle) {
	visible := max(1, (bounds.Dy()-10)/20)
	limit := max(0, strings.Count(i.Text(), "\n")+1-visible)
	i.viewLine = max(0, min(limit, i.visibleStart(bounds, true)+delta))
	i.manualScroll = true
}

// PlaceCursor uses the same font and visible lines as Draw.
func (i *Input) PlaceCursor(p *Painter, bounds image.Rectangle, point image.Point, wasFocused bool) {
	start := i.visibleStart(bounds, wasFocused)
	line := start + max(0, (point.Y-bounds.Min.Y-6)/20)
	lines := strings.Split(i.Text(), "\n")
	line = min(line, len(lines)-1)
	runes := []rune(lines[line])
	offset := 0
	for n := 0; n < line; n++ {
		offset += len([]rune(lines[n])) + 1
	}
	measure, size := p.Measure, 14
	if i.Spec.Syntax != "" {
		measure = p.codeMeasure
		size = 13
	}
	drop := 0
	if wasFocused && i.cursor >= offset && i.cursor <= offset+len(runes) {
		col := i.cursor - offset
		for drop < col && measure(string(runes[drop:col]), size) > bounds.Dx()-38 {
			drop++
		}
	}
	x := point.X - bounds.Min.X - 8
	col := drop
	for col < len(runes) {
		left := measure(string(runes[drop:col]), size)
		right := measure(string(runes[drop:col+1]), size)
		if x < (left+right)/2 {
			break
		}
		col++
	}
	i.SetCursor(offset + col)
	i.viewLine = start
	i.manualScroll = true
}
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
	i.buffer = slices.Insert(i.buffer, i.cursor, r...)
	i.cursor += len(r)
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
	if len(k.Pressed) > 0 || len(k.Repeated) > 0 || k.Text != "" {
		i.manualScroll = false
	}
	if k.JustPressed(ebiten.KeyEscape) {
		return InputResult{Intent: InputCancel}
	}
	if k.JustPressed(ebiten.KeyTab) {
		if i.Spec.Syntax != "" && !k.Shift && !k.Command {
			i.Insert("  ")
			return InputResult{}
		}
		if k.Shift {
			return InputResult{Intent: InputPrevious}
		}
		return InputResult{Intent: InputNext}
	}
	if len(i.Spec.Choices) > 0 {
		if k.JustPressed(ebiten.KeyEnter) || k.JustPressed(ebiten.KeySpace) || k.JustPressed(ebiten.KeyArrowDown) {
			return InputResult{Intent: InputOpenChoice}
		}
		return InputResult{}
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
	if i.Spec.Number != nil {
		if k.Repeats(ebiten.KeyArrowUp) {
			i.StepNumber(1, k.Shift)
		}
		if k.Repeats(ebiten.KeyArrowDown) {
			i.StepNumber(-1, k.Shift)
		}
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
			i.buffer = slices.Delete(i.buffer, i.cursor-1, i.cursor)
			i.cursor--
		}
	}
	if k.Repeats(ebiten.KeyDelete) {
		if i.selectAll {
			i.SetText("")
		} else if i.cursor < len(i.buffer) {
			i.buffer = slices.Delete(i.buffer, i.cursor, i.cursor+1)
		}
	}
	if k.JustPressed(ebiten.KeyEnter) {
		if i.Spec.Multiline {
			indent := ""
			if i.Spec.Syntax != "" {
				start := i.cursor
				for start > 0 && i.buffer[start-1] != '\n' {
					start--
				}
				end := start
				for end < i.cursor && i.buffer[end] == ' ' {
					end++
				}
				indent = string(i.buffer[start:end])
				line := strings.TrimSpace(string(i.buffer[start:i.cursor]))
				if strings.HasSuffix(line, ":") || (i.Spec.Syntax == "json" && (strings.HasSuffix(line, "{") || strings.HasSuffix(line, "["))) {
					indent += "  "
				}
			}
			i.Insert("\n" + indent)
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
// maskRunes backs the secret-field mask so redrawing a password does not
// allocate a new string on every frame.
const maskRunes = "••••••••••••••••••••••••••••••••"

func mask(n int) string {
	if width := n * len("•"); width <= len(maskRunes) {
		return maskRunes[:width]
	}
	return strings.Repeat("•", n)
}

func (i Input) Draw(dst *ebiten.Image, p *Painter, bounds image.Rectangle, focused bool) {
	t := p.Theme
	p.Text(dst, p.Fit(i.Spec.Label, bounds.Dx(), 14), bounds.Min.X, bounds.Min.Y-25, 14, t.Text)
	border := t.Border
	if focused {
		border = t.Accent
		Surface(dst, bounds.Inset(-2), Radius+2, t.Background, t.Selection)
	}
	if i.Spec.Error != "" {
		border = t.Error
	}
	Surface(dst, bounds, Radius, t.Input, border)
	if i.Spec.Error != "" {
		p.Wrapped(dst, i.Spec.Error, bounds.Min.X, bounds.Max.Y+4, bounds.Dx(), 11, t.Error, 2)
	} else {
		p.Text(dst, p.Fit(i.Spec.Hint, bounds.Dx(), 12), bounds.Min.X, bounds.Max.Y+4, 12, t.Muted)
	}
	if i.Spec.Boolean {
		label := "Off"
		if i.Spec.Value == "true" {
			label = "On"
		}
		(Toggle{Bounds: bounds, Label: label, Checked: i.Spec.Value == "true"}).Draw(dst, p, image.Pt(-1, -1))
		return
	}
	textWidth := bounds.Dx()
	if i.Spec.Number != nil {
		for _, direction := range []int{1, -1} {
			r := StepBounds(bounds, direction)
			icon := IconChevronUp
			if direction < 0 {
				icon = IconChevronDown
			}
			p.drawIcon(dst, icon, Center(r, 16, 16), t.Accent)
		}
		textWidth -= 28
	}
	clipped := bounds.Inset(5).Intersect(dst.Bounds())
	if i.Spec.Number != nil {
		clipped.Max.X = min(clipped.Max.X, bounds.Max.X-28)
	}
	if clipped.Empty() {
		return
	}
	clip := dst.SubImage(clipped).(*ebiten.Image)
	value := i.Spec.Value
	if i.Spec.DisplayValue != "" {
		value = i.Spec.DisplayValue
	}
	if len(i.Spec.Choices) > 0 {
		p.drawIcon(clip, IconChevronDown, Box(bounds.Max.X-23, bounds.Min.Y+(bounds.Dy()-16)/2, 16, 16), t.Accent)
		textBounds := clipped
		textBounds.Max.X = min(textBounds.Max.X, bounds.Max.X-26)
		if textBounds.Empty() {
			return
		}
		clip = dst.SubImage(textBounds).(*ebiten.Image)
		// Choices have focus styling but no text-editing cursor.
		focused = false
	}
	if focused {
		value = i.Text()
	}
	if i.Spec.Secret {
		value = mask(utf8.RuneCountInString(value))
	}
	if value == "" && i.Spec.Placeholder != "" {
		placeholderY := bounds.Min.Y + 6
		if !i.Spec.Multiline {
			placeholderY = p.TextY(bounds, 14)
		}
		p.Text(clip, p.Fit(i.Spec.Placeholder, textWidth-16, 14), bounds.Min.X+8, placeholderY, 14, t.Muted)
	}
	start := 0
	measure := p.Measure
	textSize := 14
	if i.Spec.Syntax != "" {
		measure = p.codeMeasure
		textSize = 13
	}
	caretLine, caretCol := 0, 0
	if focused {
		for _, r := range i.buffer[:i.cursor] {
			if r == '\n' {
				caretLine, caretCol = caretLine+1, 0
			} else {
				caretCol++
			}
		}
	}
	visible := max(1, (bounds.Dy()-10)/20)
	if caretLine >= visible {
		start = caretLine - visible + 1
	}
	if i.manualScroll {
		start = i.viewLine
	}
	for offset, str := range lineWindow(value, start, visible) {
		n := start + offset
		col := caretCol
		if focused && n == caretLine {
			rr := []rune(str)
			// Scroll the line so the caret stays visible. Measured width falls
			// monotonically as leading runes are dropped, so a binary search
			// replaces a scan that re-measured the prefix per dropped rune.
			drop := min(col, sort.Search(col+1, func(d int) bool {
				return measure(string(rr[d:col]), textSize) <= textWidth-22
			}))
			rr, col = rr[drop:], col-drop
			str = string(rr)
		}
		y := bounds.Min.Y + 6 + (n-start)*20
		if !i.Spec.Multiline {
			y = p.TextY(bounds, textSize)
		}
		if focused && i.selectAll {
			Rect(clip, Box(bounds.Min.X+7, y, bounds.Dx()-14, 20), t.Selection)
		}
		if i.Spec.Syntax != "" {
			p.drawYAMLLine(clip, str, bounds.Min.X+8, y, textSize)
		} else {
			p.Text(clip, str, bounds.Min.X+8, y, textSize, t.Text)
		}
		if focused && n == caretLine {
			rr := []rune(str)
			x := bounds.Min.X + 8 + measure(string(rr[:min(col, len(rr))]), textSize)
			Rect(clip, Box(x, y+1, 1, 18), t.Accent)
		}
	}
	if i.Spec.Syntax != "" {
		limit := max(0, strings.Count(value, "\n")+1-visible)
		Scrollbar(dst, Box(bounds.Max.X-5, bounds.Min.Y+5, 2, bounds.Dy()-10), start, limit, t.Muted)
	}

}
