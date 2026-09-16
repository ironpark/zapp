package comp

import "testing"

func TestEmbeddedIconsRenderVisibleMasks(t *testing.T) {
	for _, name := range []Icon{IconPlay, IconUndo, IconRedo, IconSave, IconPlus, IconTrash, IconCheck, IconChevronUp, IconChevronDown} {
		t.Run(string(name), func(t *testing.T) {
			data, err := iconFiles.ReadFile("icons/" + string(name) + ".svg")
			if err != nil {
				t.Fatal(err)
			}
			img, err := rasterizeIcon(data)
			if err != nil {
				t.Fatal(err)
			}
			visible, transparent := 0, 0
			for y := range img.Bounds().Dy() {
				for x := range img.Bounds().Dx() {
					pixel := img.RGBAAt(x, y)
					if pixel.A == 0 {
						transparent++
						continue
					}
					visible++
					if pixel.R != pixel.A || pixel.G != pixel.A || pixel.B != pixel.A {
						t.Fatal("icon must be a white mask so button colors can tint it")
					}
				}
			}
			if visible == 0 || transparent == 0 {
				t.Fatal("icon must have visible strokes on a transparent background")
			}
		})
	}
}
