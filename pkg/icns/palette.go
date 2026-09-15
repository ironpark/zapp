package icns

import "image/color"

// The fixed Macintosh icon palettes. The 256-color table consists of a
// descending 6x6x6 RGB cube (except black), then red/green/blue/gray ramps,
// and finally black. See the format references in README.md.
var palette16 = color.Palette{
	color.NRGBA{255, 255, 255, 255}, color.NRGBA{252, 243, 5, 255},
	color.NRGBA{255, 100, 2, 255}, color.NRGBA{221, 8, 6, 255},
	color.NRGBA{242, 8, 132, 255}, color.NRGBA{70, 0, 165, 255},
	color.NRGBA{0, 0, 212, 255}, color.NRGBA{2, 171, 234, 255},
	color.NRGBA{31, 183, 20, 255}, color.NRGBA{0, 100, 17, 255},
	color.NRGBA{86, 44, 5, 255}, color.NRGBA{144, 113, 58, 255},
	color.NRGBA{192, 192, 192, 255}, color.NRGBA{128, 128, 128, 255},
	color.NRGBA{64, 64, 64, 255}, color.NRGBA{0, 0, 0, 255},
}
var palette256 = func() color.Palette {
	p := make(color.Palette, 0, 256)
	for r := 255; r >= 0; r -= 51 {
		for g := 255; g >= 0; g -= 51 {
			for b := 255; b >= 0; b -= 51 {
				if r+g+b != 0 {
					p = append(p, color.NRGBA{byte(r), byte(g), byte(b), 255})
				}
			}
		}
	}
	for ch := 0; ch < 4; ch++ {
		for _, v := range []byte{238, 221, 187, 170, 136, 119, 85, 68, 34, 17} {
			c := color.NRGBA{A: 255}
			switch ch {
			case 0:
				c.R = v
			case 1:
				c.G = v
			case 2:
				c.B = v
			case 3:
				c.R = v
				c.G = v
				c.B = v
			}
			p = append(p, c)
		}
	}
	return append(p, color.NRGBA{A: 255})
}()
