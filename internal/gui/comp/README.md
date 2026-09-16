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
