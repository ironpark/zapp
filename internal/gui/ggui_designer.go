package gui

import (
	"fmt"
	"image"
	"math"

	"github.com/ironpark/ggui"
	"github.com/ironpark/ggui/ui"
	uitheme "github.com/ironpark/ggui/ui/theme"
)

func designerView(m *desktopModel, v workspaceState) ggui.Widget {
	g := m.editor
	canvas := &designerCanvas{model: m}
	preview := ui.Card(ggui.Column(
		ggui.Row(ggui.Text("DMG preview"), ggui.Spacer(),
			ui.Button("Reset layout", g.action(g.guard(func() { g.s.checkpoint(); g.s.Project.DMG.Contents = nil; g.selected = ""; g.rebuild() }))).Ghost().Pad(5, 10).Disabled(v.DefaultLayout),
			modeButtons("Preview zoom", []string{"Fit", "100%"}, boolIndex(v.Actual), func(i int) { g.previewActual = i == 1; g.pan = image.Point{}; m.sync() })).Gap(8),
		ui.Caption("Drop files or folders · Drag to arrange"),
		ggui.Expanded(canvas),
		ggui.View(m.PreviewError, designerHint),
	).Gap(12).Align(ggui.AlignStretch))
	// The canvas, which the arrow keys move items on, keeps the focus when it
	// selects an item; the list scrolls to the item by itself.
	selected := ggui.Bind(func() string { return m.Workspace.Get().Selected }, func(path string) {
		g.action(g.guard(func() { g.selected = path; g.rebuild() }))()
	})
	contents := ui.ListBox(m.Items, func(i layoutItem) string { return i.Path }, func(item ggui.Readable[layoutItem]) ggui.Widget {
		return ggui.Reactive(func() ggui.Widget {
			m.Assets.Get()
			row := item.Get()
			kind := g.itemKinds[row.Path]
			if row.Link {
				kind = "Link"
			}
			icon := ggui.Image(g.assets["item:"+row.Path]).Size(28, 28)
			return ggui.Row(icon, ggui.Expanded(ggui.Column(ggui.Text(row.title()).NoWrap().Ellipsis(), ui.Caption(kind)).Gap(3))).Gap(8).Align(ggui.AlignCenter)
		})
	}).RowName(func(i layoutItem) string { return "Item " + i.title() }).Name("DMG contents").BindSelected(selected).
		Else(func() ggui.Widget { return ui.Caption("Drop files onto the preview to add contents.") })
	list := ui.Card(ggui.Column(ggui.Row(ggui.TextOf(m.Items.Map(func(items []layoutItem) string { return fmt.Sprintf("Contents · %d", len(items)) })), ggui.Spacer(), iconButton("plus", "Add file", g.action(g.guard(g.addFile)))), ggui.Expanded(ggui.Scroll(contents).BindOffset(m.offset("items")))).Gap(10).Align(ggui.AlignStretch))
	details := ggui.ViewOf(m.Workspace, func(state workspaceState) string { return state.Selected }, func(selected string) ggui.Widget {
		if selected == "" {
			return ui.Card(ggui.Column(ggui.Text("Item details"), ui.Caption("Select an item to edit its name and position.")).Gap(12))
		}
		// Read Link from m.Items when the switch paints: toggling it does not
		// change m.Workspace, so this View would not rebuild with a new value.
		link := func() bool {
			for _, item := range m.Items.Get() {
				if item.Path == selected {
					return item.Link
				}
			}
			return false
		}
		return ui.Card(ggui.Column(ggui.Row(ggui.Text("Item details"), ggui.Spacer(), ui.Switch(ggui.Bind(link, func(bool) { g.toggleItemLink(); m.sync() }), "Link")), ggui.Expanded(ggui.Scroll(formView(m, m.Inspector)).Key("inspector:"+selected)), ui.Button("Remove from DMG", g.action(g.removeSelected)).Outline()).Gap(12).Align(ggui.AlignStretch))
	})
	side := ggui.Box(ui.Resizable(m.InspectorSplit, list, details).Vertical().MinSizes(130, 220)).Width(248)
	right := ggui.Row(ggui.Expanded(preview), side).Gap(12).Align(ggui.AlignStretch)
	return ui.Resizable(m.Split, settingsView(m, v), right).MinSizes(290, 570).WithHandle()
}

// The only custom widget is the Finder-like canvas. Layout supplies its size;
// pointer and file-drop positions are translated into its local coordinates.
// No toolbar, form or sidebar uses absolute positioning.
type designerCanvas struct {
	model *desktopModel
	rect  ggui.Rect
	theme uitheme.Theme // from Layout, for Paint
}

func (c *designerCanvas) Layout(l ggui.Constraints, env ggui.Env) ggui.Size {
	c.theme = uitheme.From(env)
	return l.Constrain(ggui.Sz(l.MaxW, l.MaxH))
}
func (c *designerCanvas) HitID() any                     { return "dmg-canvas" }
func (c *designerCanvas) Semantics() (ggui.Role, string) { return ggui.RoleGroup, "DMG preview canvas" }
func (c *designerCanvas) Paint(dst *ggui.Canvas, r ggui.Rect) {
	c.rect = r
	g := c.model.editor
	g.previewBounds = image.Rect(0, 0, max(1, int(r.Size.W)), max(1, int(r.Size.H)))
	dst.HitPointer(r, c)
	dst.HitKey(r, c)
	dst.Describe(r, c)
	dst.FillRect(r, c.theme.Bg)
	g.drawPreview(dst, r.Origin)
	// Outlined while it has the focus, so the arrow keys are seen to move
	// the selected item, and while files are dragged over it.
	switch {
	case c.model.DropHover.Get():
		dst.StrokeRoundRect(r, 0, 2, c.theme.Primary)
	case dst.FocusWithin(r):
		dst.StrokeRoundRect(r, 0, 2, c.theme.Ring)
	}
}

// designerHint explains the canvas controls, unless part of the preview
// could not be drawn; then it says why.
func designerHint(previewError string) ggui.Widget {
	if previewError != "" {
		return ui.Caption(previewError).Color(uitheme.Use().Destructive)
	}
	return ui.Caption("Arrow keys move · Shift: 10 px · At 100%, drag empty space to pan")
}
func (c *designerCanvas) local(p ggui.Point) ggui.Point {
	return ggui.Pt(p.X-c.rect.Origin.X, p.Y-c.rect.Origin.Y)
}
func (c *designerCanvas) HandlePointer(e ggui.PointerEvent) bool {
	g := c.model.editor
	p := c.local(e.Pos)
	point := image.Pt(int(p.X), int(p.Y))
	switch e.Kind {
	case ggui.PointerDown:
		if e.Button != ggui.MouseButtonLeft || !g.commit() {
			return true
		}
		g.selectPreviewItem(point)
		g.startPan(point)
		c.model.sync()
	case ggui.PointerDrag:
		g.moveDesigner(p)
	case ggui.PointerUp:
		g.moveDesigner(p)
		moved := g.dragMoved
		g.drag, g.panning, g.dragMoved = "", false, false
		if moved {
			g.rebuild()
			g.report(nil, "Position updated.")
		}
		c.model.sync()
	case ggui.PointerScroll:
		return false
	}
	return true
}
func (c *designerCanvas) HandleKey(e ggui.KeyEvent) {
	if e.Kind != ggui.KeyPress {
		return
	}
	g := c.model.editor
	switch e.Key {
	case ggui.KeyEscape:
		g.selected = ""
		g.rebuild()
	case ggui.KeyDelete, ggui.KeyBackspace:
		g.removeSelected()
	default:
		dx, dy := 0, 0
		switch e.Key {
		case ggui.KeyArrowLeft:
			dx = -1
		case ggui.KeyArrowRight:
			dx = 1
		case ggui.KeyArrowUp:
			dy = -1
		case ggui.KeyArrowDown:
			dy = 1
		}
		if e.Mods.Shift {
			dx *= 10
			dy *= 10
		}
		if dx != 0 || dy != 0 {
			if item, ok := g.s.layout().find(g.selected); ok {
				g.s.checkpoint()
				g.s.move(item.Path, item.X+dx, item.Y+dy)
				g.rebuild()
			}
		}
	}
	c.model.sync()
}
func (c *designerCanvas) HandleDrop(e ggui.DropEvent) bool {
	g := c.model.editor
	if !g.commit() {
		return true
	}
	p := c.local(e.Pos)
	count, err := g.addDroppedPaths(e.Paths(), image.Pt(int(p.X), int(p.Y)))
	g.report(err, fmt.Sprintf("Added %d item(s). Drag to arrange; Undo removes this batch.", count))
	c.model.DropHover.Set(false)
	c.model.sync()
	return true
}
func (c *designerCanvas) HandleDrag(e ggui.DragEvent) bool {
	c.model.DropHover.Set(e.Kind == ggui.DragOver)
	return true
}
func (g *editor) moveDesigner(pos ggui.Point) {
	p := image.Pt(int(pos.X), int(pos.Y))
	if g.panning {
		g.pan = g.panOrigin.Add(p.Sub(g.panStart))
		g.clampPan()
	}
	if g.drag == "" {
		return
	}
	x, y := g.transform().content(pos.X, pos.Y)
	x -= g.dragX
	y -= g.dragY
	nx, ny := int(math.Round(x)), int(math.Round(y))
	if item, ok := g.s.layout().find(g.drag); ok && (nx != item.X || ny != item.Y) {
		if !g.dragMoved {
			g.s.checkpoint()
			g.dragMoved = true
		}
		g.s.move(g.drag, nx, ny)
	}
}
