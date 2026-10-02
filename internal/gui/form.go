package gui

import (
	"strconv"

	"github.com/ironpark/ggui"
	"github.com/ironpark/ggui/ui"
	uitheme "github.com/ironpark/ggui/ui/theme"
)

func formView(m *desktopModel, source ggui.Readable[[]*desktopField]) ggui.Widget {
	return ggui.View(source, func(fields []*desktopField) ggui.Widget {
		children := []ggui.Widget{}
		for i := 0; i < len(fields); {
			first := fields[i]
			if first.Spec.Group != "" {
				children = append(children, ggui.Column(ggui.Text(first.Spec.Group).Size(12).Color(uitheme.Use().Primary), ui.Divider()).Gap(8))
			}
			end := i + 1
			for end < len(fields) && fields[end].Spec.SameRow {
				end++
			}
			if end-i == 1 {
				children = append(children, fieldView(m, first))
			} else {
				row := []ggui.Widget{}
				for _, f := range fields[i:end] {
					row = append(row, ggui.Expanded(fieldView(m, f)))
				}
				children = append(children, ggui.Row(row...).Gap(12).Align(ggui.AlignStart))
			}
			i = end
		}
		if len(children) == 0 {
			return ui.Empty("No fields", "Add a component to configure its payload.")
		}
		return ggui.Column(children...).Gap(20).Align(ggui.AlignStretch)
	})
}

func fieldView(m *desktopModel, f *desktopField) ggui.Widget {
	g, spec := m.editor, f.Spec
	binding := m.fieldBinding(f)
	var control ggui.Widget
	switch {
	case spec.Boolean:
		on := ggui.Bind(func() bool { return f.Value.Get() == "true" }, func(v bool) {
			binding.Set(strconv.FormatBool(v))
			m.commitField(f)
		})
		control = ui.Switch(on, spec.Label).Name(spec.Label)
	case len(spec.Choices) > 0:
		control = ui.Select(binding).Options(spec.Choices).Name(spec.Label).Format(func(value string) string {
			if value == "" {
				if spec.DisplayValue != "" {
					return spec.DisplayValue
				}
				return "Default"
			}
			return value
		}).OnChange(func(string) { m.commitField(f) })
	default:
		input := ui.TextField(binding).Key(f.ID).Name(spec.Label).Placeholder(spec.Placeholder).OnCommit(func(string) { m.commitField(f) })
		if spec.Secret {
			input.Password()
		}
		if spec.Multiline {
			input.Multiline().Lines(6)
			if spec.Syntax != "" {
				input.Lines(12).Style(ggui.TextStyle{Font: ggui.DefaultMonoFont(), Size: 13})
			}
		}
		input.OnKey(func(e ggui.KeyEvent) bool {
			if e.Key == ggui.KeyEscape {
				g.rebuild()
				m.sync()
				return true
			}
			if spec.Number != nil && (e.Key == ggui.KeyArrowUp || e.Key == ggui.KeyArrowDown) {
				dir := 1
				if e.Key == ggui.KeyArrowDown {
					dir = -1
				}
				m.stepField(f, dir, e.Mods.Shift, false)
				return true
			}
			return false
		})
		control = f.focus.Attach(input)
	}
	if spec.Number != nil {
		up := iconButton("chevron-up", spec.Label+" +", func() { m.stepField(f, 1, false, true) })
		down := iconButton("chevron-down", spec.Label+" −", func() { m.stepField(f, -1, false, true) })
		control = ggui.Row(ggui.Expanded(control), ggui.Column(up, down)).Gap(6)
	}
	if spec.Browse {
		browse := ui.Button("Browse", g.action(func() { g.browse(f.ID.index) })).Name("Browse " + spec.Label).Outline()
		control = ggui.Row(ggui.Expanded(control), browse).Gap(8)
	}
	if f.ID.tab == tabDep && !g.depRaw && f.ID.index < len(g.s.Project.Dep.Libs) {
		control = ggui.Row(ggui.Expanded(control), iconButton("x", "Remove "+spec.Label, g.action(func() { removeLibrary(g, f.ID.index) }))).Gap(8)
	}
	if f.ID.tab == tabDMG && spec.Label == "Item icon" {
		// ggui.If follows the value itself; reading it here would rebuild the
		// whole form on every keystroke.
		reset := ggui.If(f.Value.Map(func(v string) bool { return v != "" }), func() ggui.Widget {
			return ui.Button("Reset item icon", g.action(func() { binding.Set(""); m.commitField(f) })).Ghost()
		})
		control = ggui.Column(control, reset).Gap(6)
	}
	if spec.Boolean {
		// The switch carries the label itself; a caption above would repeat it.
		return ggui.Column(control, ui.Caption(spec.Hint)).Gap(uitheme.Use().Space / 2).Align(ggui.AlignStretch)
	}
	field := ui.Field(spec.Label, control).Help(spec.Hint).BindError(f.Error)
	if spec.Browse {
		return ggui.Pointer(field).OnDrop(func(e ggui.DropEvent) {
			paths := e.Paths()
			if len(paths) != 1 {
				return
			}
			binding.Set(pickedPath(g.s.Path, paths[0]))
			m.commitField(f)
		})
	}
	return field
}

func (m *desktopModel) stepField(f *desktopField, direction int, large, commit bool) {
	g := m.editor
	if !g.matchesField(f.ID) {
		return
	}
	if g.active != f.ID.index {
		if !g.commit() {
			m.sync()
			return
		}
		g.focus(f.ID.index)
	}
	g.input.StepNumber(direction, large)
	g.clearFieldError()
	g.previewInput()
	if commit {
		g.commit()
	}
	m.sync()
}
