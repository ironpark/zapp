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

	"github.com/ironpark/ggfx"
	"github.com/ironpark/ggui"
	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/pkg/icns"
)

type previewTransform struct {
	x, y, scale float64
	bounds      image.Rectangle
}

func (t previewTransform) content(x, y float64) (float64, float64) {
	return (x - t.x) / t.scale, (y - t.y) / t.scale
}

func (g *editor) previewArea() image.Rectangle { return g.previewBounds }
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
	return previewTransform{float64(x), float64(y), scale, image.Rect(x, y, x+width, y+height)}
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
	asset := ggfx.NewImageFromImage(img)
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
	// Drop the preview's previous keys so items removed from the layout stop
	// pinning their textures; the "file:" cache and other views' keys survive.
	maps.DeleteFunc(g.assets, func(k string, _ *ggfx.Image) bool { return previewAsset(k) })
	maps.DeleteFunc(g.appIconPaths, func(k, _ string) bool { return previewAsset(k) })
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
				one.DMG.Contents = map[string]zapp.Content{item.Path: item.content()}
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

// previewAsset reports whether the DMG preview owns an asset key.
func previewAsset(key string) bool {
	return key == "background" || key == "badge:alias" || strings.HasPrefix(key, "item:")
}

// pruneAssets releases cached textures that no live preview key references.
// Every path typed during a session would otherwise hold a GPU texture forever.
func (g *editor) pruneAssets() {
	live := map[*ggfx.Image]bool{}
	for key, img := range g.assets {
		if img != nil && !strings.HasPrefix(key, assetCachePrefix) {
			live[img] = true
		}
	}
	maps.DeleteFunc(g.assets, func(key string, img *ggfx.Image) bool {
		if !strings.HasPrefix(key, assetCachePrefix) || live[img] {
			return false
		}
		if img != nil {
			img.Deallocate()
		}
		return true
	})
}

// drawPreview draws the Finder window the DMG opens as, with the preview
// area's top-left corner at origin. It draws in logical pixels on ggui's
// Canvas, so text and icons stay sharp at any display scale.
func (g *editor) drawPreview(dst *ggui.Canvas, origin ggui.Point) {
	c := g.s.Project.DMG
	l := g.s.layout()
	size, label, items := l.IconSize, l.LabelSize, l.Items
	t := g.transform()
	at := func(x, y float64) ggui.Point { return origin.Add(ggui.Pt(x, y)) }
	rect := func(r image.Rectangle) ggui.Rect {
		return ggui.Rct(at(float64(r.Min.X), float64(r.Min.Y)), ggui.Sz(r.Dx(), r.Dy()))
	}
	dst = dst.Clip(rect(g.previewArea()))
	headerHeight := max(1, int(math.Round(28*t.scale)))
	header := image.Rect(t.bounds.Min.X, t.bounds.Min.Y-headerHeight, t.bounds.Max.X, t.bounds.Min.Y)
	chrome := color.RGBA{232, 231, 229, 255}
	outline := color.RGBA{172, 172, 172, 255}
	radius := max(2, int(math.Round(6*t.scale)))
	dst.FillRoundRect(rect(header), float64(radius), chrome)
	dst.FillRect(rect(image.Rect(header.Min.X, header.Max.Y-radius, header.Max.X, header.Max.Y)), chrome)
	for i, c := range []color.RGBA{{255, 95, 87, 255}, {254, 188, 46, 255}, {40, 200, 64, 255}} {
		center := at(t.x+(14+float64(i)*20)*t.scale, float64(header.Min.Y)+float64(headerHeight)/2)
		dst.FillCircle(center, 6*t.scale, color.RGBA{150, 150, 150, 255})
		dst.FillCircle(center, 5.5*t.scale, c)
	}
	title := c.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(g.s.Project.App), ".app")
	}
	if title == "." || title == "" {
		title = "DMG preview"
	}
	fsTitle := max(8, math.Round(13*t.scale))
	title = dst.FitText(title, nil, fsTitle, float64(header.Dx())-150*t.scale)
	titleX := float64(header.Min.X) + (float64(header.Dx())-dst.TextWidth(title, nil, fsTitle))/2
	dst.DrawText(title, nil, fsTitle, at(titleX, float64(header.Min.Y)+(float64(headerHeight)-fsTitle)/2-2), color.RGBA{65, 65, 65, 255})
	dst.FillRect(rect(t.bounds), color.RGBA{246, 247, 249, 255})
	if t.bounds.Empty() {
		return
	}
	canvas := dst.Clip(rect(t.bounds))
	if g.previewActual {
		canvas.HitCursor(rect(t.bounds), ggui.CursorShapeMove) // drag empty space to pan
	}
	if bg := g.assets["background"]; bg != nil {
		b := bg.Bounds()
		canvas.DrawImage(bg, ggui.Rct(at(t.x, t.y), ggui.Sz(float64(b.Dx())*t.scale, float64(b.Dy())*t.scale)), ggui.ImageOptions{Fit: ggui.FitFill})
	}
	for _, item := range items {
		x := t.x + float64(item.X)*t.scale
		y := t.y + float64(item.Y)*t.scale
		side := float64(size) * t.scale
		r := ggui.Rct(at(x-side/2, y-side/2), ggui.Sz(side, side))
		canvas.HitCursor(r, ggui.CursorShapeMove)
		if item.Path == g.selected {
			halo := ggui.Rct(r.Origin.Sub(ggui.Pt(5, 5)), ggui.Sz(side+10, side+10))
			canvas.FillRect(halo, color.NRGBA{77, 153, 241, 55})
			canvas.StrokeRoundRect(halo, 0, 1, color.RGBA{53, 132, 226, 255})
		}
		canvas.DrawImage(g.assets["item:"+item.Path], r, ggui.ImageOptions{})
		if item.Link {
			canvas.DrawImage(g.assets["badge:alias"], r, ggui.ImageOptions{})
		}
		fs := max(8, math.Round(float64(label)*t.scale))
		name := canvas.FitText(item.title(), nil, fs, max(side*1.7, 60))
		width := canvas.TextWidth(name, nil, fs)
		lx, ly := x-width/2, y+side/2+3
		if item.Path == g.selected {
			canvas.FillRect(ggui.Rct(at(lx-3, ly), ggui.Sz(width+6, fs+5)), color.RGBA{46, 117, 212, 255})
			canvas.DrawText(name, nil, fs, at(lx, ly), color.White)
		} else {
			canvas.DrawText(name, nil, fs, at(lx+1, ly+1), color.White)
			canvas.DrawText(name, nil, fs, at(lx, ly), color.RGBA{35, 38, 44, 255})
		}
	}
	// One shared outer edge prevents the title bar and image from differing by a pixel.
	dst.FillRect(rect(image.Rect(header.Min.X, header.Max.Y-1, header.Max.X, header.Max.Y)), outline)
	dst.FillRect(rect(image.Rect(header.Min.X, header.Min.Y+radius, header.Min.X+1, t.bounds.Max.Y)), outline)
	dst.FillRect(rect(image.Rect(header.Max.X-1, header.Min.Y+radius, header.Max.X, t.bounds.Max.Y)), outline)
	dst.FillRect(rect(image.Rect(t.bounds.Min.X, t.bounds.Max.Y-1, t.bounds.Max.X, t.bounds.Max.Y)), outline)
}
