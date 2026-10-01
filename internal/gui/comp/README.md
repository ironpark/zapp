# GUI components

The running GUI uses ggui for controls, focus, input, accessibility and dialogs.
`comp` keeps the small pieces ggui does not own. It does not import Zapp's
project model or packaging code. See `../ggui_runtime.go`, `../ggui_form.go`
and `../ggui_designer.go` for the presentation layer.

| Component | Owns |
| --- | --- |
| `InputSpec` / `NumberSpec` | A field's presentation: label, hint, placeholder, error, choices, grouping, syntax, number stepping |
| `Input` | The draft text of the focused field, separate from its committed `Spec.Value` |
| `Painter` | The DMG preview's font: text drawing, measurement and `Fit` truncation |
| `Box` / `Rect` / `Border` / `RoundedRect` | Shape painting for the DMG preview |

`NewInput(spec)` starts a draft; `SetText`, `StepNumber` and `Clone` change or
copy it without touching `spec.Value`, and `Dirty` reports whether it differs.
The editor applies a draft as a validated project transaction in `editor.go`;
on failure it keeps the draft so the user can repair it.

Create one `Painter` per window and close it when the window ends.
