package gui

import (
	"fmt"

	"github.com/ironpark/zapp"
)

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

// revealField moves ggui focus, and with it the scroll position, to a field.
func (g *editor) revealField(index int) {
	if g.desktop != nil && index >= 0 && index < len(g.fields) {
		g.desktop.fieldFocus(g.fieldIdentity(index)).Focus()
	}
}
func (g *editor) revealItem() {
	if g.desktop == nil {
		return
	}
	// The canvas keeps focus: its arrow keys move the item.
	if g.selected != "" {
		g.desktop.items.Reveal(g.selected)
	}
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
