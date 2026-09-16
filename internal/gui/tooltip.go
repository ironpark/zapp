package gui

import (
	"fmt"
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp/internal/gui/comp"
)

// Tooltips reveal paths and shortcuts without adding permanent screen chrome.
func (g *editor) tooltipAt(point image.Point) string {
	for _, segment := range g.segmentedControls() {
		for _, button := range segment.Buttons() {
			if point.In(button.Bounds) {
				switch button.Label {
				case "Fit":
					return "Fit the whole DMG window in the preview"
				case "100%":
					return "Actual size · Space + drag to pan"
				case "Form":
					return "Edit with fields. Valid source edits are applied when switching."
				case "JSON":
					return "Edit JSON · Ctrl/Cmd+Enter: apply · Esc: cancel · Tab: indent"
				case "Text":
					return "One search directory per line · Ctrl/Cmd+Enter: apply"
				case "YAML":
					return "Edit DMG YAML · Ctrl/Cmd+Enter: apply · Esc: cancel · Tab: indent"
				}
			}
		}
	}

	for i, tab := range g.tabs().Buttons() {
		if point.In(tab.Bounds) {
			state := "Project settings"
			if sections[i].Optional() {
				state = "Disabled · Click to configure or enable"
				if sections[i].Enabled(g.s.Project) {
					state = "Enabled in this project"
				}
			}
			if g.issueOn(i) {
				state = "Needs attention: " + g.issue.message
			}
			return fmt.Sprintf("%s · Ctrl/Cmd+%d", state, i+1)
		}
	}
	for _, button := range g.controls() {
		if point.In(button.Bounds) {
			switch button.Label {
			case "Undo":
				return "Undo · Ctrl/Cmd+Z"
			case "Redo":
				return "Redo · Ctrl/Cmd+Shift+Z"
			case "Save":
				return "Save · Ctrl/Cmd+S"
			case "Build":
				return "Build current settings, including enabled signing and notarization steps. Project file is saved separately."
			case "Validate":
				return "Check build inputs without building, signing or submitting files"
			case "Add file":
				return "Add file to DMG · You can also drop files or folders onto the preview"
			case "Default layout":
				return "Replace custom contents with the app and Applications link. Undo restores the previous layout."
			case "Remove from DMG":
				return "Remove the selected item from this DMG · Delete / Backspace. The source file stays on disk."
			}
		}
	}
	if !g.enabled() {
		return ""
	}
	for i, field := range g.fields {
		form, local := g.fieldForm(i)
		if point.In(form.FieldBounds(local).Intersect(form.Bounds)) {
			if field.Browse {
				return field.Value
			}
			if field.Number != nil {
				return "Up / Down: step by 1 · Shift: step by 10 · Enter: apply · Esc: cancel"
			}
			return field.Hint
		}
	}
	if g.tab == tabDMG {
		if g.selected != "" && point.In(g.linkToggle().Bounds) {
			return "Link: create a symbolic link to the source path instead of copying its contents into the DMG."
		}
		area := g.itemsPanel().Content()
		if point.In(area) {
			index := (point.Y - area.Min.Y + g.itemScroll) / itemRowHeight
			items := g.s.layout().Items
			if index < len(items) {
				return items[index].Path
			}
		}
	}
	return ""
}

func (g *editor) drawTooltip(dst *ebiten.Image, pointer image.Point) {
	if g.hoverTicks < 30 || g.drag != "" || g.panning {
		return
	}
	message := g.tooltipAt(pointer)
	if message == "" {
		return
	}
	p := g.ui
	measured := p.Measure(message, 12)
	width := min(g.w-48, 600, measured+24)
	x := min(pointer.X+12, g.w-24-width)
	height := 76
	if measured <= width-24 {
		height = 36
	}
	y := min(pointer.Y+24, g.h-footerHeight-height-12)
	box := comp.Box(max(24, x), max(12, y), width, height)
	comp.Surface(dst, box, comp.Radius, p.Theme.Hover, p.Theme.Border)
	p.Wrapped(dst, message, box.Min.X+12, box.Min.Y+10, box.Dx()-24, 12, p.Theme.Text, 3)
}
