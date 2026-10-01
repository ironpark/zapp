// Package comp holds the editor's input drafts and the text and shape painting
// for the DMG preview. It has no project, filesystem-layout or
// packaging knowledge; applications own value validation and decide when to
// apply a draft.
package comp

import (
	"image"
	"image/color"
	"sort"

	"github.com/ironpark/ggfx"
	"github.com/ironpark/ggfx/text/v2"
	"github.com/ironpark/ggfx/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

// Painter owns a shared font. Create one per UI and Close it when the window
// ends. Drawing and measuring use the same face cache.
type Painter struct {
	font     *opentype.Font
	faces    map[int]font.Face
	goxFaces map[font.Face]*text.GoXFace
}

func NewPainter(ttf []byte) (*Painter, error) {
	if len(ttf) == 0 {
		ttf = goregular.TTF
	}
	f, err := opentype.Parse(ttf)
	if err != nil {
		return nil, err
	}
	return &Painter{font: f, faces: make(map[int]font.Face)}, nil
}
func (p *Painter) Close() {
	for _, f := range p.faces {
		_ = f.Close()
	}
	p.faces = nil
	p.goxFaces = nil
}

// face memoizes one face per size.
func (p *Painter) face(size int) font.Face {
	size = max(1, size)
	if face := p.faces[size]; face != nil {
		return face
	}
	face, err := opentype.NewFace(p.font, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	} // The font and positive size were validated above.
	p.faces[size] = face
	return face
}
func (p *Painter) Measure(s string, size int) int { return font.MeasureString(p.face(size), s).Ceil() }
func (p *Painter) Text(dst *ggfx.Image, s string, x, y, size int, c color.Color) {
	p.drawText(dst, s, p.face(size), x, y+size, c)
}

// goxFace adapts a cached x/image face for text/v2. A GoXFace owns its glyph
// image cache, so one is kept per face rather than rebuilt per draw.
func (p *Painter) goxFace(f font.Face) *text.GoXFace {
	if p.goxFaces == nil {
		p.goxFaces = map[font.Face]*text.GoXFace{}
	}
	if g := p.goxFaces[f]; g != nil {
		return g
	}
	g := text.NewGoXFace(f)
	p.goxFaces[f] = g
	return g
}

// drawText puts the baseline at y, which is what text v1's Draw did, so every
// caller's layout maths is unchanged. text/v2 positions the top of the line box
// instead, hence the ascent offset.
func (p *Painter) drawText(dst *ggfx.Image, s string, f font.Face, x, y int, c color.Color) {
	face := p.goxFace(f)
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x), float64(y)-face.Metrics().HAscent)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(dst, s, face, op)
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
func Rect(dst *ggfx.Image, r image.Rectangle, c color.Color) {
	if !r.Empty() {
		vector.FillRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), c, false)
	}
}
func Border(dst *ggfx.Image, r image.Rectangle, c color.Color) {
	if !r.Empty() {
		vector.StrokeRect(dst, float32(r.Min.X)+.5, float32(r.Min.Y)+.5, float32(r.Dx()-1), float32(r.Dy()-1), 1, c, false)
	}
}
func Box(x, y, w, h int) image.Rectangle { return image.Rect(x, y, x+w, y+h) }

// RoundedRect draws a subtly rounded surface with antialiased corners.
func RoundedRect(dst *ggfx.Image, r image.Rectangle, radius int, c color.Color) {
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
