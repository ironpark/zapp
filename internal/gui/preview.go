package gui

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
	"github.com/ironpark/zapp/pkg/icns"
)

type previewTransform struct {
	x, y, scale float64
	bounds      image.Rectangle
}

func (t previewTransform) content(x, y float64) (float64, float64) {
	return (x - t.x) / t.scale, (y - t.y) / t.scale
}
func (g *editor) previewArea() image.Rectangle {
	panel := g.previewPanel().Bounds
	return image.Rect(panel.Min.X+16, panel.Min.Y+76, panel.Max.X-16, panel.Max.Y-58)
}
func (g *editor) clampPan() {
	l := g.s.layout()
	area := g.previewArea()
	limit := image.Pt(max(0, (l.W-area.Dx()+1)/2), max(0, (l.H+28-area.Dy()+1)/2))
	g.pan.X = max(-limit.X, min(limit.X, g.pan.X))
	g.pan.Y = max(-limit.Y, min(limit.Y, g.pan.Y))
}
func (g *editor) transform() previewTransform {
	l := g.s.layout()
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
		x += g.pan.X
		y += g.pan.Y
	}
	return previewTransform{float64(x), float64(y), scale, comp.Box(x, y, width, height)}
}

// assetCachePrefix marks decoded-by-path entries in the asset map, keeping them
// distinct from the logical "background" / "item:<path>" keys the preview draws.
const assetCachePrefix = "file:"

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
	cacheKey := assetCachePrefix + path
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
		if err != nil {
			img, err = family.HighestResolution()
		}
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

// derivedSignature captures every input refreshPreview and refreshItemKinds
// consume except item coordinates. Dragging and arrow-key nudging only change
// X/Y, so a matching signature means both caches are still valid.
func (g *editor) derivedSignature() string {
	if g.tab != tabDMG || g.s.Project.DMG == nil || !g.enabled() {
		return ""
	}
	return previewSignature(g.s.Project, g.s.layout().Items)
}

func previewSignature(p *zapp.Project, items []layoutItem) string {
	c := p.DMG
	var b strings.Builder
	for _, s := range []string{p.App, p.Out, c.Title, c.Icon, c.Background, c.FS, c.Out} {
		b.WriteString(s)
		b.WriteByte(0)
	}
	for _, i := range items {
		b.WriteString(i.Path)
		b.WriteByte(1)
		b.WriteString(i.Icon)
		b.WriteByte(1)
		b.WriteString(i.Name)
		b.WriteByte(1)
		if i.Link {
			b.WriteByte('L')
		}
		b.WriteByte(2)
	}
	return b.String()
}

func (g *editor) refreshPreview() {
	if g.tab != tabDMG || g.s.Project.DMG == nil {
		return
	}
	c := g.s.Project.DMG
	items := g.s.layout().Items
	g.previewError = ""
	g.appIconPaths = make(map[string]string)
	// Drop the previous logical keys so items removed from the layout stop
	// pinning their textures; the "file:" entries below survive as the cache.
	maps.DeleteFunc(g.assets, func(k string, _ *ebiten.Image) bool {
		return !strings.HasPrefix(k, assetCachePrefix)
	})
	bg := g.assetPath(c.Background)
	paths := make([]string, len(items))
	icons := make([]string, len(items))
	for i, item := range items {
		paths[i] = g.assetPath(item.Path)
		icons[i] = g.assetPath(item.Icon)
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
			hasVariables = hasVariables || (strings.Contains(item.Path, "${") || strings.Contains(item.Icon, "${"))
		}
		if !hasVariables && len(plan.DMG.Contents) == len(paths) {
			for i, item := range plan.DMG.Contents {
				paths[i] = item.Path
				icons[i] = item.Icon
			}
		} else if hasVariables {
			// Resolve clones internally, so one scratch project serves every item.
			one := p.Clone()
			for i, item := range items {
				x, y := item.X, item.Y
				one.DMG.Contents = map[string]zapp.Content{item.Path: {Pos: &zapp.Position{x, y}, Link: item.Link, Name: item.Name, Icon: item.Icon}}
				if resolved, err := one.Resolve(); err == nil {
					paths[i] = resolved.DMG.Contents[0].Path
					icons[i] = resolved.DMG.Contents[0].Icon
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
		if item.Icon != "" && !item.Link {
			if err := g.loadAsset(key, icons[i]); err != nil {
				g.previewError = "Item icon: " + err.Error()
			}
		} else if strings.EqualFold(filepath.Ext(path), ".app") {
			g.loadAppIcon(key, path)
		} else if slices.Contains(previewImageExts, strings.ToLower(filepath.Ext(path))) {
			_ = g.loadAsset(key, path)
		}
		if g.assets[key] == nil {
			g.assets[key] = g.defaultFileIcon(fileIconForPath(path))
		}
	}
	if slices.ContainsFunc(items, func(item layoutItem) bool { return item.Link }) {
		g.assets["badge:alias"] = g.defaultFileIcon("alias")
	}
	g.pruneAssets()
}

// pruneAssets releases cached textures that no live preview key references.
// Every path typed during a session would otherwise hold a GPU texture forever.
func (g *editor) pruneAssets() {
	live := map[*ebiten.Image]bool{}
	for key, img := range g.assets {
		if img != nil && !strings.HasPrefix(key, assetCachePrefix) {
			live[img] = true
		}
	}
	maps.DeleteFunc(g.assets, func(key string, img *ebiten.Image) bool {
		if !strings.HasPrefix(key, assetCachePrefix) || live[img] {
			return false
		}
		if img != nil {
			img.Deallocate()
		}
		return true
	})
}

func (g *editor) drawPreview(dst *ebiten.Image) {
	g.previewPanel().Draw(dst, g.ui)
	comp.Surface(dst, g.previewArea().Inset(-1), comp.Radius, g.ui.Theme.Background, g.ui.Theme.Border)
	c := g.s.Project.DMG
	l := g.s.layout()
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
		drawFileIcon(canvas, g.assets["item:"+item.Path], r)
		if item.Link {
			drawFileIcon(canvas, g.assets["badge:alias"], r)
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
	g.ui.Text(dst, fmt.Sprintf("%d × %d  ·  %.0f%%", l.W, l.H, t.scale*100), g.previewArea().Min.X, g.contentBottom()-42, 13, g.ui.Theme.Muted)
	if len(items) == 0 {
		g.ui.Wrapped(canvas, "Drop files or folders here, or set the app path in Project.", int(t.x)+20, int(t.y)+25, t.bounds.Dx()-40, 16, color.RGBA{80, 90, 106, 255}, 3)
	}
	message := "Approximate preview · Drop files or folders to add"
	if g.previewActual {
		message = "Space + drag to pan · Fit shows the whole window"
	}
	if slices.ContainsFunc(items, func(item layoutItem) bool { return item.Path == g.selected }) {
		message = "Arrows: move · Shift: 10 px · Delete: remove"
	}
	if g.previewError != "" {
		message = g.previewError
	}
	g.ui.Text(dst, g.ui.Fit(message, g.previewArea().Dx(), 12), g.previewArea().Min.X, g.contentBottom()-20, 12, g.ui.Theme.Muted)
}
