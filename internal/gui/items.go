package gui

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp/internal/gui/comp"
)

func (g *editor) previewPanel() comp.Panel {
	x := g.settingsPanel().Bounds.Max.X + 16
	return comp.Panel{Bounds: comp.Box(x, workspaceTop, g.itemsPanel().Bounds.Min.X-16-x, g.contentBottom()-workspaceTop), Title: "DMG preview", TitleInset: 156, Description: "Drop files or folders · Drag to arrange"}
}
func (g *editor) itemsPanel() comp.Panel {
	return comp.Panel{Bounds: comp.Box(g.w-268, workspaceTop, 244, g.inspectorPanel().Bounds.Min.Y-12-workspaceTop), Title: "Contents", TitleInset: 48}
}
func (g *editor) inspectorPanel() comp.Panel {
	return comp.Panel{Bounds: comp.Box(g.w-268, g.contentBottom()-376, 244, 376), Title: "Item details", TitleInset: 96}
}
func (g *editor) linkToggle() comp.Toggle {
	panel := g.inspectorPanel().Bounds
	item, _ := g.s.layout().find(g.selected)
	return comp.Toggle{Bounds: comp.Box(panel.Max.X-108, panel.Min.Y+8, 100, 32), Label: "Link", Checked: item.Link, OnChange: g.toggleItemLink}
}

func (g *editor) toggleItemLink() {
	if g.tab != tabDMG || !g.enabled() || g.selected == "" || !g.commit() {
		return
	}
	if _, ok := g.s.layout().find(g.selected); !ok {
		return
	}
	current, _ := g.selectedContent()
	if !current.Link && current.Icon != "" {
		g.report(fmt.Errorf("Reset the item icon before enabling Link; symbolic links use their target icon."), "")
		return
	}
	g.s.checkpoint()
	g.s.materialize()
	item := g.s.Project.DMG.Contents[g.selected]
	item.Link = !item.Link
	g.s.Project.DMG.Contents[g.selected] = item
	g.rebuild()
	if item.Link {
		g.report(nil, "Link enabled. The DMG will contain a symbolic link to this path.")
	} else {
		g.report(nil, "Link disabled. The file or folder will be copied into the DMG.")
	}
}

func (g *editor) inspectorArea() image.Rectangle {
	area := g.inspectorPanel().Content()
	area.Max.Y -= 40
	return area
}

// Removing contents only changes the DMG layout; source files stay on disk.
func (g *editor) removeSelected() {
	if g.tab != tabDMG || !g.enabled() || g.selected == "" || !g.commit() {
		return
	}
	if _, ok := g.s.layout().find(g.selected); !ok {
		return
	}
	g.s.checkpoint()
	g.s.materialize()
	delete(g.s.Project.DMG.Contents, g.selected)
	g.selected = ""
	g.rebuild()
	g.report(nil, "Removed from DMG. Source file unchanged. Undo restores the item.")
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
	return max(0, len(g.s.layout().Items)*itemRowHeight-g.itemsPanel().Content().Dy())
}
func (g *editor) revealItem() {
	area := g.itemsPanel().Content()
	for i, item := range g.s.layout().Items {
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
	items := g.s.layout().Items
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
	items := g.s.layout().Items
	panel.Title = fmt.Sprintf("Contents · %d", len(items))
	panel.Draw(dst, p)
	area := panel.Content()
	canvas := dst.SubImage(area.Intersect(dst.Bounds())).(*ebiten.Image)
	limit := g.itemListLimit()
	g.itemScroll = max(0, min(g.itemScroll, limit))
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
		iconBounds := comp.Box(row.Min.X+4, row.Min.Y+5, 30, 30)
		drawFileIcon(canvas, g.assets["item:"+item.Path], iconBounds)
		if item.Link {
			drawFileIcon(canvas, g.assets["badge:alias"], iconBounds)
		}
		p.Text(canvas, p.Fit(name, row.Dx()-46, 13), row.Min.X+40, row.Min.Y+3, 13, t.Text)
		p.Text(canvas, p.Fit(kind+" · "+item.Path, row.Dx()-46, 11), row.Min.X+40, row.Min.Y+23, 11, t.Muted)
	}
	comp.Scrollbar(dst, comp.Box(area.Max.X+6, area.Min.Y, 3, area.Dy()), g.itemScroll, limit, t.Muted)
	g.inspectorPanel().Draw(dst, p)
	if g.selected != "" {
		g.linkToggle().Draw(dst, p, pointer)
	}
	if g.selected == "" || len(g.inspector.Inputs) == 0 {
		a := g.inspectorPanel().Content()
		p.Wrapped(dst, "Select an item in the list or preview to edit its name and position.", a.Min.X, a.Min.Y, a.Dx(), 13, t.Muted, 4)
	} else {
		active := -1
		if g.active >= g.inspectorStart {
			active = g.active - g.inspectorStart
		}
		g.inspector.Draw(dst, p, active, &g.input, pointer)
		if item, ok := g.selectedContent(); ok && item.Link {
			area := g.inspectorArea()
			p.Wrapped(dst, "Link icons follow the target. Turn off Link to use a custom icon.", area.Min.X, area.Max.Y-66, area.Dx(), 12, t.Muted, 3)
		}
	}
}

func (g *editor) refreshItemKinds() {
	g.itemKinds = make(map[string]string)
	if g.tab != tabDMG || !g.enabled() {
		return
	}
	for _, item := range g.s.layout().Items {
		kind := map[string]string{
			"app": "App", "folder": "Folder", "applications": "Folder", "file": "File",
			"text": "Text", "pdf": "PDF", "image": "Image", "audio": "Audio", "video": "Video",
			"archive": "Archive", "disk": "Disk image", "package": "Package", "font": "Font",
			"script": "Script", "source": "Source", "executable": "Executable",
		}[fileIconForPath(g.assetPath(item.Path))]
		if item.Link {
			kind = "Link"
		}
		g.itemKinds[item.Path] = kind
	}
}
