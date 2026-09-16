package gui

// restoreLive ends the temporary preview transaction. Field setters are rebuilt
// because they capture pointers into the project being edited.
func (g *editor) restoreLive() {
	if g.liveBase == nil {
		return
	}
	g.s.Project = g.liveBase
	g.liveBase = nil
}

func (g *editor) previewInput() {
	if g.active < 0 {
		return
	}
	index, draft, selected := g.active, g.input.Clone(), g.selected
	previous := g.s.Project.Clone()
	base := g.liveBase
	if base == nil {
		base = g.s.Project.Clone()
	}
	g.restoreLive()
	g.s.Project = base.Clone()
	g.rebuildFields()
	g.selected = selected
	if index >= len(g.fields) {
		return
	}
	err := g.fields[index].set(draft.Text())
	if err == nil {
		err = g.s.validateLayout()
	}
	if err != nil {
		// Incomplete numbers or YAML keep the last valid preview visible.
		g.s.Project = previous
	}
	g.rebuild()
	g.liveBase = base
	g.active, g.input = index, draft
	g.selected = selected
	g.projectDirty = g.s.Dirty()
}
