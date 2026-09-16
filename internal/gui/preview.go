package gui

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/icns"
)

type previewTransform struct {
	x, y, scale float64
	bounds      image.Rectangle
}

func (t previewTransform) content(x, y float64) (float64, float64) {
	return (x - t.x) / t.scale, (y - t.y) / t.scale
}
func (g *editor) previewArea() image.Rectangle { return comp.Box(420, 267, g.w-476, g.h-427) }
func (g *editor) clampPan() {
	l := layout(g.s.Project.DMG, g.s.Project.App)
	area := g.previewArea()
	limitX, limitY := max(0, (l.W-area.Dx()+1)/2), max(0, (l.H+28-area.Dy()+1)/2)
	g.panX = max(-limitX, min(limitX, g.panX))
	g.panY = max(-limitY, min(limitY, g.panY))
}
func (g *editor) transform() previewTransform {
	l := layout(g.s.Project.DMG, g.s.Project.App)
	available := g.previewArea()
	// Fit the whole Finder window, including its compact title bar, at one scale.
	scale := math.Min(1, math.Min(float64(available.Dx())/float64(max(1, l.W)), float64(available.Dy())/float64(max(1, l.H)+28)))
	if g.previewActual {
		scale = 1
		g.clampPan()
	}
	width, height := int(math.Round(float64(l.W)*scale)), int(math.Round(float64(l.H)*scale))
	headerHeight := max(1, int(math.Round(28*scale)))
	x := available.Min.X + (available.Dx()-width)/2
	y := available.Min.Y + (available.Dy()-height-headerHeight)/2 + headerHeight
	if g.previewActual {
		x += g.panX
		y += g.panY
	}
	return previewTransform{float64(x), float64(y), scale, comp.Box(x, y, width, height)}
}

// previewImageExts are the icon sources the preview can decode directly.
var previewImageExts = []string{".png", ".jpg", ".jpeg", ".icns"}

func (g *editor) assetPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(filepath.Dir(g.s.Path), path)
}

func (g *editor) loadAsset(key, path string) error {
	if path == "" {
		g.assets[key] = nil
		return nil
	}
	cacheKey := "file:" + path
	if cached, ok := g.assets[cacheKey]; ok {
		g.assets[key] = cached
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	var img image.Image
	if strings.EqualFold(filepath.Ext(path), ".icns") {
		family, e := icns.Decode(file)
		if e != nil {
			return e
		}
		img, err = family.ByResolution(256)
	} else {
		config, _, e := image.DecodeConfig(file)
		if e != nil {
			return e
		}
		if config.Width <= 0 || config.Height <= 0 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 32<<20 {
			return fmt.Errorf("preview image is too large (maximum 8192 per side / 32 million pixels)")
		}
		if _, err = file.Seek(0, 0); err != nil {
			return err
		}
		img, _, err = image.Decode(file)
	}
	if err != nil {
		return err
	}
	asset := ebiten.NewImageFromImage(img)
	g.assets[cacheKey] = asset
	g.assets[key] = asset
	return nil
}

func (g *editor) refreshPreview() {
	if g.tab != 1 || g.s.Project.DMG == nil {
		return
	}
	g.previewError = ""
	c := g.s.Project.DMG
	items := layout(c, g.s.Project.App).Items
	bg := g.assetPath(c.Background)
	paths := make([]string, len(items))
	for i, item := range items {
		paths[i] = g.assetPath(item.Path)
	}
	// Resolve a copy with only DMG enabled, so missing signing/PKG settings do
	// not prevent a layout preview. Keep the editable project's original paths.
	p := g.s.Project.Clone()
	p.PKG = nil
	p.Dep = nil
	p.Sign = nil
	p.Notarize = nil
	if plan, err := p.Resolve(); err == nil {
		bg = plan.DMG.Background
		// Without variables the lexical order of keys is unchanged by Resolve.
		hasVariables := false
		for _, item := range items {
			hasVariables = hasVariables || strings.Contains(item.Path, "${")
		}
		if !hasVariables && len(plan.DMG.Contents) == len(paths) {
			for i, item := range plan.DMG.Contents {
				paths[i] = item.Path
			}
		} else if hasVariables {
			for i, item := range items {
				one := p.Clone()
				x, y := item.X, item.Y
				one.DMG.Contents = map[string]zapp.Content{item.Path: {X: &x, Y: &y, Link: item.Link, Name: item.Name}}
				if resolved, err := one.Resolve(); err == nil {
					paths[i] = resolved.DMG.Contents[0].Path
				}
			}
		}
	} else {
		g.previewError = "Draft preview: " + err.Error()
	}
	g.assets["background"] = nil
	if err := g.loadAsset("background", bg); err != nil {
		g.previewError = "Background: " + err.Error()
	}
	for i, item := range items {
		key := "item:" + item.Path
		g.assets[key] = nil
		path := paths[i]
		if strings.HasSuffix(path, ".app") {
			if bundle, err := appbundle.Open(path); err == nil {
				if icon, err := bundle.IconFilePath(); err == nil {
					_ = g.loadAsset(key, icon)
				}
			}
		} else if item.Link && item.Path == "/Applications" {
			_ = g.loadAsset(key, "/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/ApplicationsFolderIcon.icns")
		} else if slices.Contains(previewImageExts, strings.ToLower(filepath.Ext(path))) {
			_ = g.loadAsset(key, path)
		}
	}
}

func (g *editor) drawPreview(dst *ebiten.Image) {
	comp.Panel{Bounds: comp.Box(404, 195, g.w-444, g.h-301), Title: "DMG preview", Description: "Drag icons to arrange · Space + drag to pan at 100%"}.Draw(dst, g.ui)
	c := g.s.Project.DMG
	l := layout(c, g.s.Project.App)
	size, label, items := l.IconSize, l.LabelSize, l.Items
	t := g.transform()
	full := dst
	dst = dst.SubImage(g.previewArea().Intersect(dst.Bounds())).(*ebiten.Image)
	headerHeight := max(1, int(math.Round(28*t.scale)))
	header := comp.Box(t.bounds.Min.X, t.bounds.Min.Y-headerHeight, t.bounds.Dx(), headerHeight)
	chrome := color.RGBA{232, 231, 229, 255}
	outline := color.RGBA{172, 172, 172, 255}
	radius := max(2, int(math.Round(6*t.scale)))
	comp.RoundedRect(dst, header, radius, chrome)
	comp.Rect(dst, comp.Box(header.Min.X, header.Max.Y-radius, header.Dx(), radius), chrome)
	for i, c := range []color.RGBA{{255, 95, 87, 255}, {254, 188, 46, 255}, {40, 200, 64, 255}} {
		cx, cy := float32(t.x+(14+float64(i)*20)*t.scale), float32(header.Min.Y)+float32(headerHeight)/2
		vector.FillCircle(dst, cx, cy, float32(6*t.scale), color.RGBA{150, 150, 150, 255}, true)
		vector.FillCircle(dst, cx, cy, float32(5.5*t.scale), c, true)
	}
	title := c.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(g.s.Project.App), ".app")
	}
	if title == "." || title == "" {
		title = "DMG preview"
	}
	fsTitle := max(8, int(math.Round(13*t.scale)))
	title = g.ui.Fit(title, header.Dx()-int(150*t.scale), fsTitle)
	g.ui.Text(dst, title, header.Min.X+(header.Dx()-g.ui.Measure(title, fsTitle))/2, header.Min.Y+(headerHeight-fsTitle)/2-2, fsTitle, color.RGBA{65, 65, 65, 255})
	comp.Rect(dst, t.bounds, color.RGBA{246, 247, 249, 255})
	if t.bounds.Empty() {
		return
	}
	canvas := dst.SubImage(t.bounds.Intersect(dst.Bounds())).(*ebiten.Image)
	if bg := g.assets["background"]; bg != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(t.scale, t.scale)
		op.GeoM.Translate(t.x, t.y)
		op.Filter = ebiten.FilterLinear
		canvas.DrawImage(bg, op)
	}
	for _, item := range items {
		x := t.x + float64(item.X)*t.scale
		y := t.y + float64(item.Y)*t.scale
		side := float64(size) * t.scale
		r := comp.Box(int(x-side/2), int(y-side/2), int(side), int(side))
		if item.Path == g.selected {
			comp.Rect(canvas, r.Inset(-5), color.RGBA{77, 153, 241, 55})
			comp.Border(canvas, r.Inset(-5), color.RGBA{53, 132, 226, 255})
		}
		if icon := g.assets["item:"+item.Path]; icon != nil {
			ratio := side / float64(max(icon.Bounds().Dx(), icon.Bounds().Dy()))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(ratio, ratio)
			op.GeoM.Translate(x-float64(icon.Bounds().Dx())*ratio/2, y-float64(icon.Bounds().Dy())*ratio/2)
			op.Filter = ebiten.FilterLinear
			canvas.DrawImage(icon, op)
		} else {
			// Generic artwork is intentional for files without an extractable
			// icon. It does not claim to reproduce Finder's per-file thumbnails.
			body := r.Inset(max(1, int(side/8)))
			if item.Link {
				comp.Rect(canvas, comp.Box(body.Min.X, body.Min.Y, body.Dx()/2, max(3, body.Dy()/6)), color.RGBA{144, 205, 240, 255})
				comp.Rect(canvas, comp.Box(body.Min.X, body.Min.Y+body.Dy()/8, body.Dx(), body.Dy()*7/8), color.RGBA{112, 178, 223, 255})
			} else {
				comp.Rect(canvas, body, color.RGBA{253, 254, 255, 255})
				comp.Border(canvas, body, color.RGBA{163, 184, 204, 255})
				fold := max(3, body.Dx()/4)
				comp.Rect(canvas, comp.Box(body.Max.X-fold, body.Min.Y, fold, fold), color.RGBA{203, 221, 237, 255})
				for row := range 3 {
					comp.Rect(canvas, comp.Box(body.Min.X+body.Dx()/5, body.Min.Y+body.Dy()/2+row*max(3, body.Dy()/9), body.Dx()*3/5, max(1, body.Dy()/35)), color.RGBA{157, 179, 200, 255})
				}
			}
		}
		fs := max(8, int(math.Round(float64(label)*t.scale)))
		name := g.ui.Fit(item.title(), max(int(side*1.7), 60), fs)
		width := g.ui.Measure(name, fs)
		lx, ly := int(x)-width/2, int(y+side/2)+3
		if item.Path == g.selected {
			comp.Rect(canvas, comp.Box(lx-3, ly, width+6, fs+5), color.RGBA{46, 117, 212, 255})
			g.ui.Text(canvas, name, lx, ly, fs, color.White)
		} else {
			g.ui.Text(canvas, name, lx+1, ly+1, fs, color.White)
			g.ui.Text(canvas, name, lx, ly, fs, color.RGBA{35, 38, 44, 255})
		}
	}
	// One shared outer edge prevents the title bar and image from differing by a pixel.
	comp.Rect(dst, comp.Box(header.Min.X, header.Max.Y-1, header.Dx(), 1), outline)
	comp.Rect(dst, comp.Box(header.Min.X, header.Min.Y+radius, 1, t.bounds.Max.Y-header.Min.Y-radius), outline)
	comp.Rect(dst, comp.Box(header.Max.X-1, header.Min.Y+radius, 1, t.bounds.Max.Y-header.Min.Y-radius), outline)
	comp.Rect(dst, comp.Box(t.bounds.Min.X, t.bounds.Max.Y-1, t.bounds.Dx(), 1), outline)
	dst = full
	g.ui.Text(dst, fmt.Sprintf("%d × %d  ·  %.0f%%  ·  Drag icons to arrange", l.W, l.H, t.scale*100), 420, g.h-148, 13, g.ui.Theme.Muted)
	if len(items) == 0 {
		g.ui.Wrapped(canvas, "Set the app path in Project, or add contents in DMG settings.", int(t.x)+20, int(t.y)+25, t.bounds.Dx()-40, 16, color.RGBA{80, 90, 106, 255}, 3)
	}
	message := "Layout approximation; Finder fonts and generic file icons may differ."
	if g.selected != "" {
		for _, item := range items {
			if item.Path == g.selected {
				message = fmt.Sprintf("%s — center (%d, %d)", item.title(), item.X, item.Y)
			}
		}
	}
	if g.previewError != "" {
		message = g.previewError
	}
	g.ui.Text(dst, g.ui.Fit(message, g.w-476, 12), 420, g.h-126, 12, g.ui.Theme.Muted)
}
