package gui

import (
	"fmt"
	"image"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
)

func (g *editor) previewPanel() comp.Panel {
	x := g.settingsPanel().Bounds.Max.X + 16
	return comp.Panel{Bounds: comp.Box(x, workspaceTop, g.itemsPanel().Bounds.Min.X-16-x, g.contentBottom()-workspaceTop), Title: "DMG preview", TitleInset: 156, Description: "Drop files or folders · Drag to arrange"}
}
func (g *editor) itemsPanel() comp.Panel {
	return comp.Panel{Bounds: comp.Box(g.w-268, workspaceTop, 244, g.inspectorPanel().Bounds.Min.Y-16-workspaceTop), Title: "Contents", TitleInset: 48}
}
func (g *editor) inspectorPanel() comp.Panel {
	return comp.Panel{Bounds: comp.Box(g.w-268, g.contentBottom()-376, 244, 376), Title: "Item details", TitleInset: 96}
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
	item := g.editSelectedContent(func(c *zapp.Content) { c.Link = !c.Link })
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
	if g.desktop != nil {
		if index >= 0 && index < len(g.fields) {
			id := g.fieldIdentity(index)
			g.desktop.focus = &id
		}
		return
	}
	form, local := g.fieldForm(index)
	form.Reveal(local)
}

const itemRowHeight = 44

func (g *editor) itemListLimit() int {
	return max(0, len(g.s.layout().Items)*itemRowHeight-g.itemsPanel().Content().Dy())
}
func (g *editor) revealItem() {
	if g.desktop != nil {
		for i, item := range g.s.layout().Items {
			if item.Path == g.selected {
				g.desktop.offset("items").Set(float64(i * 48))
				break
			}
		}
		return
	}
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

// editSelectedContent materializes the contents map, applies f to the selected
// entry and writes it back, so every mutation of a selected item follows one
// path. Callers own checkpointing and rebuilding.
func (g *editor) editSelectedContent(f func(*zapp.Content)) zapp.Content {
	g.s.materialize()
	item := g.s.Project.DMG.Contents[g.selected]
	f(&item)
	g.s.Project.DMG.Contents[g.selected] = item
	return item
}

func (g *editor) refreshItemKinds() {
	g.itemKinds = make(map[string]string)
	if g.tab != tabDMG || !g.enabled() {
		return
	}
	for _, item := range g.s.layout().Items {
		kind := "Link"
		if !item.Link {
			kind = itemKindNames[fileIconForPath(g.assetPath(item.Path))]
		}
		g.itemKinds[item.Path] = kind
	}
}
