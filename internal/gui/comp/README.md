# GUI components

`comp` is the reusable presentation layer for the Ebitengine editor. It does not
import Zapp's project model or packaging code.

| Component | Owns |
| --- | --- |
| `Button` | Bounds, label, selected/disabled styling, hit testing and activation |
| `Tabs` | Tab layout and selection requests, using the same bounds for drawing and input |
| `Input` / `InputSpec` | Draft text, Unicode cursor editing, clipboard, choice-opening intents, inline errors and submit/focus intents |
| `Form` | Shared input rows, browse-button bounds, clipping, scrolling and focus reveal |
| `Dialog` | Modal layout, action buttons, cancellation and blocking background input |
| `Painter` / `Theme` | Shared font cache, text measurement, clipping helpers and colors |

Components receive explicit state; labels are never used as action identifiers.
The editor declares buttons and tab callbacks in `controls.go`, binds input
specifications to project setters in `fields.go`, and applies input drafts as
validated project transactions in `editor.go`.

For a text field, create a draft with `NewInput(spec)`, feed it a `Keyboard`
snapshot with `Handle`, then inspect its `InputResult`. `InputSubmit` requests
application validation; it does **not** update `spec.Value`. On failure, keep the
draft so the user can repair it. `InputNext` and `InputPrevious` also leave the
decision to apply the value and change focus to the owner. A `Clipboard`
interface allows tests to exercise cut/paste without touching the OS clipboard.

Call `Dialog.Handle` before background controls and stop dispatching when it
returns true. Disabled buttons consume hits without invoking callbacks. Draw the
dialog last, over the rest of the screen. Button/tab callbacks may be guarded by
an application's pending-edit validation without putting that policy into `comp`.

Create one `Painter` per window, share it among components, and close it after
`ebiten.RunGame` returns. `CaptureKeyboard`, `ObservePointer`, `PointerPosition`
and `PointerPressPosition` adapt platform input; the editing and hit-testing
methods can also be driven directly in unit tests without a window.

`Panel` groups content with shared title/inset geometry; `Badge` displays a
non-interactive state. `Toggle` requests a boolean change through `OnChange`,
letting the owner validate before applying it. Tabs use label-sized widths and an underline to identify the active tab. Supply the painter’s Measure method for exact sizing.

`Button.Icon` adds an embedded SVG icon beside the label; `IconOnly: true` centers
just the icon while retaining `Label` for application tooltips. For example:

```go
comp.Button{Label: "Undo", Icon: comp.IconUndo, IconOnly: true, Bounds: bounds, OnClick: undo}
comp.Button{Label: "Save", Icon: comp.IconSave, Primary: true, Bounds: bounds, OnClick: save}
```

Icons in `icons/*.svg` are original 24×24 white-stroke artwork embedded with
`go:embed`. The painter rasterizes them once at 72×72 with oksvg/rasterx, caches
the textures, and releases them in `Close`. Button drawing scales to 18×18 and
tints icons with the current foreground color, including disabled and primary
states. Add an SVG and an `Icon` constant to extend the set; no runtime asset files
are required. This supports the SVG subset understood by oksvg.

`Segmented` draws a shared track for mutually exclusive labeled toggles. Supply
`Labels`, `Selected`, and `OnSelect`; clicking the selected segment is a no-op.
`InputSpec.Syntax = "yaml"` enables tolerant syntax highlighting and a monospaced
font with Unicode fallback. `Height` expands multiline editors. Code inputs support
cursor hit testing, wheel scrolling via `ScrollLines`, and automatic indentation.

`Button.Ghost` renders without a resting background or border. Hover and selected
states receive a subtle fill; disabled buttons remain transparent. Combine with
`Primary` for an accent-colored label/icon. Header actions use this style.
