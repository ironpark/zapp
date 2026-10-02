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
	draft, selected := g.input.Text(), g.selected
	previous := g.s.Project.Clone()
	base := g.liveBase
	if base == nil {
		base = g.s.Project.Clone()
	}
	g.keepingDraft(func() {
		g.restoreLive()
		g.s.Project = base.Clone()
		g.rebuildFields()
		g.selected = selected
		if g.active < 0 {
			return
		}
		err := g.fields[g.active].set(draft)
		if err == nil {
			err = g.s.validateLayout()
		}
		if err != nil {
			// Incomplete numbers or YAML keep the last valid preview visible.
			g.s.Project = previous
		}
		// Only the fields and the preview follow a draft. The health check
		// stats paths and reads the app bundle, so it waits for the value to
		// be committed.
		g.rebuildFields()
		g.refreshDerived()
	})
	g.liveBase = base
	g.selected = selected
	g.projectDirty = g.s.Dirty()
}
