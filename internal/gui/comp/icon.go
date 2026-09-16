package comp

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

type Icon string

const IconPlay Icon = "play"

const (
	IconUndo        Icon = "undo"
	IconRedo        Icon = "redo"
	IconSave        Icon = "save"
	IconPlus        Icon = "plus"
	IconTrash       Icon = "trash"
	IconCheck       Icon = "check"
	IconChevronUp   Icon = "chevron-up"
	IconChevronDown Icon = "chevron-down"
)

//go:embed icons/*.svg
var iconFiles embed.FS

// Rasterize once at 4x the button size; the painter owns the resulting textures.
// White SVG strokes form a mask tinted with the button's current text color.
func rasterizeIcon(data []byte) (*image.RGBA, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.StrictErrorMode)
	if err != nil {
		return nil, err
	}
	const size = 72
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	icon.SetTarget(0, 0, size, size)
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, img, img.Bounds())), 1)
	return img, nil
}

func loadIcons() (map[Icon]*ebiten.Image, error) {
	entries, err := iconFiles.ReadDir("icons")
	if err != nil {
		return nil, err
	}
	icons := make(map[Icon]*ebiten.Image, len(entries))
	for _, entry := range entries {
		data, err := iconFiles.ReadFile("icons/" + entry.Name())
		var img *image.RGBA
		if err == nil {
			img, err = rasterizeIcon(data)
		}
		if err != nil {
			for _, texture := range icons {
				texture.Deallocate()
			}
			return nil, fmt.Errorf("icon %s: %w", entry.Name(), err)
		}
		icons[Icon(entry.Name()[:len(entry.Name())-4])] = ebiten.NewImageFromImage(img)
	}
	return icons, nil
}

func (p *Painter) drawIcon(dst *ebiten.Image, icon Icon, bounds image.Rectangle, tint color.RGBA) {
	img := p.icons[icon]
	if img == nil || bounds.Empty() {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(bounds.Dx())/float64(img.Bounds().Dx()), float64(bounds.Dy())/float64(img.Bounds().Dy()))
	op.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
	op.ColorScale.ScaleWithColor(tint)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(img, op)
}
