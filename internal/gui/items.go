package gui

import (
	"fmt"
	"image"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp/internal/gui/comp"
)

func (g *editor) previewPanel() comp.Panel {
	return comp.Panel{Bounds: comp.Box(404, 195, g.w-688, g.contentBottom()-195), Title: "DMG preview", TitleInset: 156, Description: "Drop files or folders · Drag to arrange"}
}
func (g *editor) itemsPanel() comp.Panel {
	return comp.Panel{Bounds: comp.Box(g.w-268, 195, 244, g.inspectorPanel().Bounds.Min.Y-12-195), Title: "Contents"}
}
func (g *editor) inspectorPanel() comp.Panel {
	return comp.Panel{Bounds: comp.Box(g.w-268, g.contentBottom()-244, 244, 244), Title: "Item details"}
}
func (g *editor) fieldForm(index int) (*comp.Form, int) {
	if index >= g.inspectorStart {
		return &g.inspector, index - g.inspectorStart
	}
	return &g.form, index
}
func (g *editor) revealField(index int) {
	form, local := g.fieldForm(index)
	form.Reveal(local)
}

const itemRowHeight = 44

func (g *editor) itemListLimit() int {
	return max(0, len(layout(g.s.Project.DMG, g.s.Project.App).Items)*itemRowHeight-g.itemsPanel().Content().Dy())
}
func (g *editor) revealItem() {
	area := g.itemsPanel().Content()
	for i, item := range layout(g.s.Project.DMG, g.s.Project.App).Items {
		if item.Path != g.selected {
			continue
		}
		top := i * itemRowHeight
		if top < g.itemScroll {
			g.itemScroll = top
		}
		if top+itemRowHeight > g.itemScroll+area.Dy() {
			g.itemScroll = top + itemRowHeight - area.Dy()
		}
	}
	g.itemScroll = max(0, min(g.itemScroll, g.itemListLimit()))
}
func (g *editor) selectListItem(point image.Point) bool {
	area := g.itemsPanel().Content()
	if !point.In(area) {
		return false
	}
	if !g.commit() {
		return true
	}
	items := layout(g.s.Project.DMG, g.s.Project.App).Items
	index := (point.Y - area.Min.Y + g.itemScroll) / itemRowHeight
	if index < len(items) {
		g.selected = items[index].Path
		g.rebuild()
		g.inspector.ScrollTo(0)
	}
	return true
}
func (g *editor) drawItems(dst *ebiten.Image, pointer image.Point) {
	p, t := g.ui, g.ui.Theme
	panel := g.itemsPanel()
	panel.Draw(dst, p)
	area := panel.Content()
	canvas := dst.SubImage(area.Intersect(dst.Bounds())).(*ebiten.Image)
	items := layout(g.s.Project.DMG, g.s.Project.App).Items
	g.itemScroll = max(0, min(g.itemScroll, g.itemListLimit()))
	if len(items) == 0 {
		p.Wrapped(canvas, "Drop files onto the preview to add contents.", area.Min.X, area.Min.Y, area.Dx(), 13, t.Muted, 3)
	}
	for i, item := range items {
		row := comp.Box(area.Min.X, area.Min.Y+i*itemRowHeight-g.itemScroll, area.Dx(), itemRowHeight-2)
		if !row.Overlaps(area) {
			continue
		}
		if item.Path == g.selected {
			comp.RoundedRect(canvas, row, 6, t.Selection)
		} else if pointer.In(row) {
			comp.RoundedRect(canvas, row, 6, t.Hover)
		}
		name := item.Name
		if name == "" {
			name = item.title()
		}
		kind := g.itemKinds[item.Path]
		p.Text(canvas, p.Fit(name, row.Dx()-16, 13), row.Min.X+8, row.Min.Y+3, 13, t.Text)
		p.Text(canvas, fmt.Sprintf("%s · %d, %d", kind, item.X, item.Y), row.Min.X+8, row.Min.Y+23, 11, t.Muted)
	}
	if limit := g.itemListLimit(); limit > 0 {
		height := max(16, area.Dy()*area.Dy()/(limit+area.Dy()))
		y := area.Min.Y + (area.Dy()-height)*g.itemScroll/limit
		comp.RoundedRect(dst, comp.Box(area.Max.X+6, y, 3, height), 1, t.Muted)
	}
	g.inspectorPanel().Draw(dst, p)
	if g.selected == "" || len(g.inspector.Inputs) == 0 {
		a := g.inspectorPanel().Content()
		p.Wrapped(dst, "Select an item in the list or preview to edit its name and position.", a.Min.X, a.Min.Y, a.Dx(), 13, t.Muted, 4)
	} else {
		active := -1
		if g.active >= g.inspectorStart {
			active = g.active - g.inspectorStart
		}
		g.inspector.Draw(dst, p, active, &g.input, pointer)
	}
}

func (g *editor) refreshItemKinds() {
	g.itemKinds = make(map[string]string)
	if g.tab != tabDMG || !g.enabled() {
		return
	}
	for _, item := range layout(g.s.Project.DMG, g.s.Project.App).Items {
		kind := "File"
		if item.Link {
			kind = "Link"
		} else if strings.HasSuffix(strings.ToLower(item.Path), ".app") {
			kind = "App"
		} else if info, err := os.Stat(g.assetPath(item.Path)); err == nil && info.IsDir() {
			kind = "Folder"
		}
		g.itemKinds[item.Path] = kind
	}
}
