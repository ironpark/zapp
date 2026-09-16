// Package comp provides reusable Ebitengine UI components. It has no project,
// filesystem-layout or packaging knowledge; applications own value validation
// and decide when to apply a component's draft or requested action.
package comp

import (
	"image"
	"image/color"
	"sort"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

type Theme struct {
	Background, Panel, Text, Muted, Accent, AccentText color.RGBA
	Border, Hover, Disabled, DisabledText              color.RGBA
	Input, Selection, Error, Overlay                   color.RGBA
}

func DarkTheme() Theme {
	return Theme{
		Background: color.RGBA{19, 21, 26, 255}, Panel: color.RGBA{28, 31, 38, 255},
		Text: color.RGBA{237, 239, 244, 255}, Muted: color.RGBA{161, 168, 183, 255},
		Accent: color.RGBA{139, 165, 255, 255}, AccentText: color.RGBA{20, 27, 52, 255},
		Border: color.RGBA{53, 58, 70, 255}, Hover: color.RGBA{40, 45, 57, 255},
		Disabled: color.RGBA{25, 28, 34, 255}, DisabledText: color.RGBA{119, 128, 147, 255},
		Input: color.RGBA{23, 26, 33, 255}, Selection: color.RGBA{57, 73, 121, 255},
		Error: color.RGBA{255, 159, 151, 255}, Overlay: color.RGBA{0, 0, 0, 170},
	}
}

// Painter owns shared fonts and styling. Create one per UI and Close it after
// RunGame returns. Drawing and measuring use the same face cache.
type Painter struct {
	mono      *opentype.Font
	monoFaces map[int]font.Face
	Theme     Theme
	font      *opentype.Font
	faces     map[int]font.Face
	icons     map[Icon]*ebiten.Image
}

func NewPainter(ttf []byte, theme Theme) (*Painter, error) {
	if len(ttf) == 0 {
		ttf = goregular.TTF
	}
	f, err := opentype.Parse(ttf)
	if err != nil {
		return nil, err
	}
	icons, err := loadIcons()
	if err != nil {
		return nil, err
	}
	return &Painter{Theme: theme, font: f, faces: make(map[int]font.Face), icons: icons}, nil
}
func (p *Painter) Close() {
	for _, face := range p.monoFaces {
		_ = face.Close()
	}
	p.monoFaces = nil
	p.mono = nil
	for _, icon := range p.icons {
		icon.Deallocate()
	}
	p.icons = nil
	for _, f := range p.faces {
		_ = f.Close()
	}
	p.faces = nil
}
func (p *Painter) face(size int) font.Face { return cachedFace(p.font, p.faces, size) }

// cachedFace memoizes one face per size so the UI and mono fonts share a single
// construction path.
func cachedFace(f *opentype.Font, cache map[int]font.Face, size int) font.Face {
	size = max(1, size)
	if face := cache[size]; face != nil {
		return face
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	} // The font and positive size were validated above.
	cache[size] = face
	return face
}
func (p *Painter) Measure(s string, size int) int { return font.MeasureString(p.face(size), s).Ceil() }
func (p *Painter) Text(dst *ebiten.Image, s string, x, y, size int, c color.Color) {
	text.Draw(dst, s, p.face(size), x, y+size, c)
}

// TextY centers the font's cap height in a control, independent of its height.
// Using a stable reference avoids labels moving when their text has descenders.
func (p *Painter) TextY(bounds image.Rectangle, size int) int {
	ink, _ := font.BoundString(p.face(size), "H")
	return bounds.Min.Y + (bounds.Dy()-ink.Max.Y.Ceil()-ink.Min.Y.Floor())/2 - size
}

// fitRunes returns the longest prefix length of r that still fits in width with
// suffix appended. Measured width grows monotonically with the prefix, so a
// binary search replaces a scan that re-measured the whole prefix per dropped rune.
func (p *Painter) fitRunes(r []rune, suffix string, width, size int) int {
	return sort.Search(len(r), func(n int) bool {
		return p.Measure(string(r[:n+1])+suffix, size) > width
	})
}
func (p *Painter) Fit(s string, width, size int) string {
	if width <= 0 {
		return ""
	}
	if p.Measure(s, size) <= width {
		return s
	}
	if p.Measure("…", size) > width {
		return ""
	}
	r := []rune(s)
	return string(r[:p.fitRunes(r, "…", width, size)]) + "…"
}
func (p *Painter) Wrapped(dst *ebiten.Image, s string, x, y, width, size int, c color.Color, maxLines int) {
	for i := 0; i < maxLines && s != ""; i++ {
		r := []rune(s)
		n := p.fitRunes(r, "", width, size)
		if n == 0 {
			return
		}
		// Prefer word boundaries while retaining rune wrapping for long paths
		// and languages that do not separate words with spaces.
		if n < len(r) {
			for j := n; j > 0; j-- {
				if unicode.IsSpace(r[j-1]) {
					n = j
					break
				}
			}
		}
		line := strings.TrimRightFunc(string(r[:n]), unicode.IsSpace)
		if i == maxLines-1 && n < len(r) {
			line = p.Fit(line+"…", width, size)
		}
		p.Text(dst, line, x, y+i*(size+5), size, c)
		s = strings.TrimLeftFunc(string(r[n:]), unicode.IsSpace)
	}
}
func Rect(dst *ebiten.Image, r image.Rectangle, c color.Color) {
	if !r.Empty() {
		vector.FillRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), c, false)
	}
}
func Border(dst *ebiten.Image, r image.Rectangle, c color.Color) {
	if !r.Empty() {
		vector.StrokeRect(dst, float32(r.Min.X)+.5, float32(r.Min.Y)+.5, float32(r.Dx()-1), float32(r.Dy()-1), 1, c, false)
	}
}
func Box(x, y, w, h int) image.Rectangle { return image.Rect(x, y, x+w, y+h) }
func Center(bounds image.Rectangle, w, h int) image.Rectangle {
	return Box(bounds.Min.X+(bounds.Dx()-w)/2, bounds.Min.Y+(bounds.Dy()-h)/2, w, h)
}

// Shared spacing keeps control interiors and grouped content consistent.
const (
	Padding = 16
	Radius  = 8
)

// RoundedRect draws a subtly rounded surface with antialiased corners.
func RoundedRect(dst *ebiten.Image, r image.Rectangle, radius int, c color.Color) {
	if r.Empty() {
		return
	}
	radius = max(0, min(radius, min(r.Dx(), r.Dy())/2))
	if radius == 0 {
		Rect(dst, r, c)
		return
	}
	Rect(dst, Box(r.Min.X+radius, r.Min.Y, r.Dx()-2*radius, r.Dy()), c)
	Rect(dst, Box(r.Min.X, r.Min.Y+radius, r.Dx(), r.Dy()-2*radius), c)
	for _, x := range []int{r.Min.X + radius, r.Max.X - radius} {
		for _, y := range []int{r.Min.Y + radius, r.Max.Y - radius} {
			vector.FillCircle(dst, float32(x), float32(y), float32(radius), c, true)
		}
	}
}
func Surface(dst *ebiten.Image, r image.Rectangle, radius int, fill, border color.Color) {
	RoundedRect(dst, r, radius, border)
	RoundedRect(dst, r.Inset(1), max(0, radius-1), fill)
}
