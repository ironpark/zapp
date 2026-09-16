package gui

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp/internal/gui/comp"
)

func (g *editor) Draw(dst *ebiten.Image) {
	p := g.ui
	t := p.Theme
	mx, my := comp.PointerPosition(g.w, g.h)
	pointer := image.Pt(mx, my)
	dst.Fill(t.Background)
	g.drawHeader(dst)
	g.tabs().Draw(dst, p, pointer)
	if !g.enabled() {
		comp.Panel{Bounds: comp.Box(24, workspaceTop, g.w-48, 180), Title: g.section().Name + " is disabled", Description: "Enable this step above to include it in your project."}.Draw(dst, p)
		p.Wrapped(dst, g.section().Description, 40, workspaceTop+90, g.w-80, 15, t.Text, 2)
		p.Wrapped(dst, "Settings are retained during this session. Enable the step when you are ready to configure it.", 40, workspaceTop+125, g.w-80, 13, t.Muted, 2)
	} else {
		g.settingsPanel().Draw(dst, p)
		mainActive := g.active
		if mainActive >= g.inspectorStart {
			mainActive = -1
		}
		g.form.Draw(dst, p, mainActive, &g.input, pointer)
		if g.tab == tabDMG {
			g.drawPreview(dst)
			g.drawItems(dst, pointer)
		} else {
			g.drawHelp(dst)
		}
	}
	footerTop := g.h - footerHeight
	comp.Rect(dst, comp.Box(0, footerTop, g.w, footerHeight), t.Panel)
	comp.Rect(dst, comp.Box(0, footerTop, g.w, 1), t.Border)
	statusColor := t.Muted
	if g.failed {
		statusColor = t.Error
	}
	comp.RoundedRect(dst, comp.Box(24, footerTop+(footerHeight-5)/2, 5, 5), 2, statusColor)
	statusWidth := g.w - 64
	p.Text(dst, p.Fit(g.status, statusWidth, 12), 40, footerTop+(footerHeight-16)/2, 12, statusColor)
	// Keep long validation messages readable without a permanent tall footer.
	if pointer.In(comp.Box(24, footerTop+1, g.w-48, footerHeight-1)) && p.Measure(g.status, 12) > statusWidth {
		box := comp.Box(24, footerTop-136, g.w-48, 124)
		comp.Surface(dst, box, comp.Radius, t.Panel, t.Border)
		p.Wrapped(dst, g.status, box.Min.X+16, box.Min.Y+14, box.Dx()-32, 13, statusColor, 5)
	}
	// Every control is drawn exactly once, after its panel background.
	for _, segmented := range g.segmentedControls() {
		segmented.Draw(dst, p, pointer)
	}
	for _, button := range g.controls() {
		button.Draw(dst, p, pointer)
	}
	if g.section().Optional() {
		g.stepToggle().Draw(dst, p, pointer)
	}
	if !g.choiceOpen && g.picking == nil && !g.confirmClose && g.build == nil {
		g.drawTooltip(dst, pointer)
	}
	if g.choiceOpen {
		g.drawChoice(dst, pointer)
	}
	if g.picking != nil {
		comp.Dialog{Visible: true, Bounds: comp.Center(dst.Bounds(), 460, 170), Title: "Choose a path", Message: "Use the system file picker to select a path, or cancel to keep the current value."}.Draw(dst, p, pointer)
	}
	if g.build != nil {
		g.buildDialog().Draw(dst, p, pointer)
		return
	}
	if g.confirmClose {
		g.closeDialog().Draw(dst, p, pointer)
	}
}

// drawHeader keeps project identity together and reserves the right side for
// save state and actions. Long paths cannot intrude into the toolbar.
func (g *editor) drawHeader(dst *ebiten.Image) {
	p, t := g.ui, g.ui.Theme
	comp.Rect(dst, comp.Box(0, 63, g.w, 1), t.Border)
	comp.RoundedRect(dst, comp.Box(24, 12, 32, 32), 9, t.Accent)
	p.Text(dst, "Z", 34, 15, 21, t.AccentText)
	p.Text(dst, "Zapp", 68, 13, 22, t.Text)
	comp.Rect(dst, comp.Box(144, 12, 1, 32), t.Border)

	state := "Saved"
	if !g.s.exists {
		state = "New project"
	}
	if g.dirty() {
		state = "Unsaved changes"
	}
	stateWidth := p.Measure(state, 12) + 30
	stateX := g.w - 480 - stateWidth
	projectWidth := max(0, stateX-192)
	p.Text(dst, p.Fit(g.s.Name, projectWidth, 16), 168, 7, 16, t.Text)
	p.Text(dst, p.Fit(g.s.Dir, projectWidth, 12), 168, 31, 12, t.Muted)
	stateColor := t.Muted
	if g.dirty() {
		stateColor = t.Accent
	}
	comp.RoundedRect(dst, comp.Box(stateX, 17, stateWidth, 24), 12, t.Panel)
	comp.RoundedRect(dst, comp.Box(stateX+10, 27, 5, 5), 2, stateColor)
	p.Text(dst, state, stateX+21, 20, 12, stateColor)
}

// The help section tables are fixed copy read every frame, so they live here
// rather than being rebuilt on each draw.
var (
	sharedHelpSections = [][2]string{
		{"SAVE & VALIDATE", "Save writes all enabled steps. Validate checks build inputs. Build runs enabled steps using current settings."},
		{"EDITING", "Tab / Shift+Tab moves between fields. Enter applies; Ctrl/Cmd+Enter applies JSON. Esc cancels."},
		{"SHORTCUTS", "Ctrl/Cmd+S saves. Ctrl/Cmd+Z undoes; add Shift to redo. Ctrl/Cmd+1–6 switches tabs."},
	}
	helpSections = append([][2]string{
		{"PATHS & VALUES", "Paths are relative to this configuration. ${env:NAME} expressions are preserved."},
	}, sharedHelpSections...)
	credentialHelpSections = append([][2]string{
		{"CREDENTIALS", "Use environment variables for passwords and other sensitive values."},
	}, sharedHelpSections...)
)

func (g *editor) drawHelp(dst *ebiten.Image) {
	x := g.settingsPanel().Bounds.Max.X + 16
	panel := comp.Panel{Bounds: comp.Box(x, workspaceTop, g.w-x-24, g.contentBottom()-workspaceTop), Title: "About this step"}
	panel.Draw(dst, g.ui)
	area := panel.Content()
	g.ui.Wrapped(dst, g.section().Description, area.Min.X, area.Min.Y, area.Dx(), 15, g.ui.Theme.Text, 4)
	y := area.Min.Y + 110
	help := helpSections
	if g.section().Credentials {
		help = credentialHelpSections
	}
	for _, entry := range help {
		if y+90 > area.Max.Y {
			break
		}
		g.ui.Text(dst, entry[0], area.Min.X, y, 11, g.ui.Theme.Accent)
		g.ui.Wrapped(dst, entry[1], area.Min.X, y+25, area.Dx(), 13, g.ui.Theme.Muted, 3)
		y += 110
	}
}
