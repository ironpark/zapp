# Project settings GUI

```sh
zapp gui
zapp gui --config release/.zapp.yaml
```

`gui` opens a [ggui](https://github.com/ironpark/ggui) desktop window rendered by ggfx. It discovers `.zapp.yaml` in the current
directory or its parents, just like the other project commands. `--config` (or
`ZAPP_CONFIG`) selects a YAML or JSON project explicitly. If there is no project,
the GUI starts an unsaved draft; the file is created when you click **Save**.
The parent directory must already exist. Legacy flat DMG configurations must be
migrated to the version 1 project format first.

## Settings tabs

| Tab | Settings |
| --- | --- |
| Project | App bundle and output directory |
| DMG | Preview, icon positions, background, disk icon, window and icon sizes, filesystem, compression, output and contents |
| PKG | Product/component type, output, identifier, version, install location, scripts, minimum OS and licenses; full-form components and distribution |
| Dependencies | Library search paths |
| Signing | Identity, PKCS#12/PEM certificate and password-file path |
| Notarization | Keychain profile, Apple ID, team ID, API key file, stapling and timeout |
| Distribution | ZIP archive and checksums with their outputs, verification before publishing; uploads, the Sparkle appcast and the Homebrew cask as YAML |

The header names the app being packaged, read from its Info.plist, with the
configuration path below it. The project is checked as you edit, without moving
you: the header badge shows **Ready** or the number of issues, and a tab whose
step has a problem carries a red dot whose tooltip names it. Disabled steps are
dimmed. Clicking the badge (or **Ctrl/Cmd+Shift+V**) runs Validate, which opens
the first problem.

**Project** shows the app's icon, name, version and bundle identifier, then the
build steps in the order Build runs them — Dependencies, Signing, DMG, PKG,
Notarization, Distribution — each with its state and output path. Click a step to open its tab.

Each step can be enabled or disabled. Disabled sections are omitted from the
saved project. Their values are retained while toggling within the same editor
session. The step toolbar shows whether the selected step is enabled. Validation errors
are shown inline and can be revisited with **Go to issue**. Non-DMG settings fill the available width until **Help**
is opened, making room for the step guide beside the form.

**PKG** has a **Single app / Components** selector. Basic settings appear first;
**Advanced settings** reveals scripts, minimum OS, licenses and distribution.
Components have an add/remove list and a detail form. Shared package settings, the selected payload and installer presentation have separate group headings. **Form / JSON** switches
to a multiline source editor; valid edits apply before switching. The component
list and view switch stay in place, and each view retains its scroll position.
Click a component to return to its Form view; removal is available in Form view.
Entry accepts one direct child name (for example `MyApp.app`), not a nested path;
leave it blank to include the entire root. In Components mode, a blank install
location uses `/` and a blank version uses `1.0`. Single app mode defaults to
`/Applications` and reads package identity and version from the app. Removing a
component referenced by distribution choices is blocked until those references
are removed. Switching back to a single app requires custom components and
distribution settings to be cleared; the generated default component can be
switched back directly.

**Dependencies** provides a path list with Browse and remove controls. Use the
empty row to type another path, or its **Browse** button to choose one. **Text** mode
edits one directory per line; spaces within paths are preserved. Both modes edit
the same list and support Undo. Leave the list empty for automatic discovery;
disable Dependencies when the app has no external libraries to bundle.

Relative paths remain relative to the configuration file. Expressions such as
`${app.name}` and `${env:ZAPP_IDENTITY}` are preserved. The GUI edits the file's
values; build-time `ZAPP_*` option overrides are not copied into the project.
**Signing** selects Keychain, PKCS#12 or PEM credentials. **Notarization** selects
Profile, Apple ID or API key credentials. Only the selected method contributes
credentials to the project; switching methods retains previous input within the
current session. The Keychain method lists the keychain's signing identities;
**Refresh** looks them up again after you import a certificate.
Keychain/Profile/Apple ID use macOS tools; PKCS#12 and PEM
certificates and API key JSON work on every host (macOS imports certificates
into a temporary keychain).

On macOS the Keychain method lists the valid signing identities found in your
keychains; click one to use it, or leave the identity blank to pick the first
matching Developer ID. **Check** asks the signing backend which certificate the
current settings would sign the app with, and the installer too when PKG is
enabled, without building. Apple uses separate Developer ID Application and
Installer certificates, so each is reported on its own. A result is hidden once
the settings change.

Drop a `.app` bundle anywhere outside the DMG preview to make it the project's
app, or a `.p12`/`.pem` certificate to sign with it; both are undoable.

Apple ID authentication includes a masked app-specific password input. Its value
is session-only and never serialized into the project; reopening requires entering
it again. PKCS#12 can use a password file. **Staple** is a one-click toggle.

Use **Browse** beside app, image, icon, certificate and output path fields to
open the system picker. Selected paths are stored relative to the configuration
when possible. Cancel keeps the current value or draft. Picking an output file
only selects its destination; it does not create or overwrite that file. Background
image pickers accept PNG/JPEG, and disk icon pickers accept ICNS/PNG. The generic
Add file picker remains unrestricted.

## Arrange a DMG

1. Set the app bundle path in **Project**, then open **DMG**.
2. Set an optional background and adjust the window/icon/label sizes. **Advanced**
   reveals the disk icon, filesystem, compression, output and contents settings.
3. Drag the app, Applications link or other content icons in the preview.
   Positions are icon centers relative to the Finder content area's top left.
   Preview scaling and the decorative title bar do not affect saved coordinates.
4. Drop files or folders from your file manager onto the preview to add them at
   the drop location. Multiple items are added as one undoable change; existing
   paths are skipped. Original files stay in place and paths are stored relative
   to the configuration when possible. Folders and app bundles remain single items.
   Alternatively, click **Add file** and choose a file in the system picker.
   It is added at the preview center without changing or scrolling the layout
   settings. Cancel leaves the layout unchanged. Use **YAML** mode in **Layout settings** to
   configure links or rename source paths.
5. Select an icon or a row in **Contents**. The fixed **Item details** panel edits
   its name and X/Y coordinates without moving the layout settings. The list
   shows item types and coordinates and scrolls independently, including items
   outside the preview. **Remove selected** removes the selected entry. **Reset
   layout** in the preview header restores the automatic app + Applications arrangement.
6. Click **Save**, then build normally with `zapp dmg` or `zapp build`.

Moving a default icon creates explicit `dmg.contents` entries for both the app
and the Applications link. The preview displays app-bundle icons when they can
be decoded, with generic artwork for other items. Background images are drawn
at their native size and clipped to the content area. The preview approximates
Finder: fonts, file icons and thumbnails can differ on the target Mac. Preview
images are limited to 8192 pixels per side and 32 million pixels in total.

## Editing and saving

- **Enter** applies a single-line field. **Ctrl/Cmd+Enter** applies a multiline field.
- **Tab / Shift+Tab** applies the current field and focuses the next/previous control, scrolling offscreen fields into view.
- **Ctrl/Cmd+A**, **C**, **X**, **V** select all, copy, cut and paste within a field.
  Arrow keys move the cursor; multiline fields also support Up/Down.
- **Esc** cancels the current field edit.
- **Ctrl/Cmd+S** saves all tabs together. **Ctrl/Cmd+B** builds and
  **Ctrl/Cmd+Shift+V** validates.
- **Ctrl/Cmd+1–7** switches tabs. **Tab** moves through tabs, fields and actions. Choice fields open a dropdown with a click, Enter or Space. Click an option or use Up/Down and Enter to select; Esc or an outside
  click dismisses the list without changing the value.
- With an icon selected and no text field active, **arrow keys** move it one
  pixel; **Shift+arrow** moves it ten pixels.
- **Undo / Redo** revert or restore settings and icon moves. **Ctrl/Cmd+Z** undoes
  typing within a text field, or a committed project change outside an active edit.
  **Ctrl/Cmd+Shift+Z** redoes it.
- Scroll the settings panel to reach additional fields.

**Save** checks schema and layout values but permits drafts whose build inputs
are not present yet. **Validate** resolves the project and checks build inputs
without building, signing or submitting anything. Invalid edits show an error
below the input and keep the draft for correction. Validation moves to the
relevant field for missing or invalid path inputs; other errors appear in the
status bar. **Go to issue** returns to the relevant tab/field, including hidden
advanced settings. Saving rewrites formatting and
comments; YAML stays YAML and `.json` stays JSON. File permissions are retained.
If the file changed externally after opening, saving reports a conflict instead
of overwriting those edits; reopen the editor to load them.

Closing a modified project offers **Save & close**, **Discard changes** and
**Keep editing**. Opening or previewing a project does not modify its app bundle.

## Build from the GUI

**Build** in the top header runs the enabled project steps using the current
settings, including configured signing and notarization. It uses the same
`Plan.Build` engine as the CLI. Pending input is applied and validated first;
building does not save or rewrite the project file.

A progress dialog shows the build log as it runs and supports **Cancel build**;
**Copy log** copies it. Building with unsaved settings says so at the top of the
log. Editing and duplicate builds are blocked while the job runs. Completion
shows output paths or the error, and **Show in Finder** (Explorer on Windows,
the folder elsewhere) reveals the built artifact. A failed build offers **Go to issue** to review the relevant
settings. Closing the window during a build cancels it and waits for
cleanup before continuing the normal unsaved-changes flow.

## Desktop requirements

The GUI uses ggui on ggfx and builds with the regular `cmd/zapp` binary, including
`CGO_ENABLED=0` builds. Linux GUI use requires an X11/XWayland session and OpenGL
runtime libraries. CLI commands and help do not open a window. Clipboard support
on Linux requires `xclip` or `xsel`. File pickers use native macOS panels attached to the editor window,
Windows PowerShell/Windows Forms, or `zenity` on Linux. If a picker is unavailable,
paths can still be entered directly. A bundled font is used when a supported system
Unicode font is unavailable; glyph coverage then depends on that font.
On macOS, local window-event coordinates are used so remote and assistive input
can select and drag items independently of the physical system pointer.

The DMG preview offers **Fit** and **100%** view modes. At 100%, drag empty
preview space to pan. Dragging an icon still edits its position. View panning does not change the saved DMG layout;
clicking either view button recenters the preview.

Empty text inputs show example values or automatic-value placeholders; placeholders
are never saved as input. Numeric inputs provide up/down arrow controls and Up/Down keyboard
stepping (1 per step, or 10 with Shift). Stepping starts from the effective default
for automatic numeric values and stays within the field's supported range.
As with typed edits, Enter or leaving the field applies the draft; Escape cancels it.

The tab bar groups step actions on its right: **Enabled**, and PKG
**Single app / Components**. **Reset layout** sits in the DMG preview header next
to **Fit / 100%**. **Add file** stays in Contents
and **Remove from DMG** in Item details. The settings
column adjusts to the window width to leave more room for the preview. Contents
shows the item count, type and source path; hover a row or path input to read its
path. Hover tabs and toolbar buttons for enabled-state and shortcut hints.

With no input focused, **Delete / Backspace** removes the selected DMG item and
**Escape** clears its selection. Removal changes only the layout, and Undo restores
the item. **Shift+Tab** moves backward through controls.

The **Link** toggle in Item details switches the selected item between copying its
contents and creating a symbolic link to its source path. Name and position are
preserved, and Undo restores the previous mode.

Undo, Redo and Add file have named controls with hover tooltips.
Save, Validate and Remove from DMG show icons alongside their labels.

The Validate badge is in the top header beside **Save**. The compact bottom
status bar shows feedback; hover a truncated status message to read its expanded
text. Single-column forms are held to a readable width in wide windows.

The preview and Contents list use embedded macOS default icons for apps, folders,
documents, text, PDF, images, audio, video, archives, disk images, installer packages,
fonts, source code, scripts and executables. An app's own icon or a supported image
thumbnail takes priority. Link items also display the macOS alias badge.

App icons are read from bundle resources before using a default. ICNS files do
not need a 256px representation; another available size is used when necessary.
PNG resources and `CFBundleIconName` are supported. On macOS, AppKit also supplies
icons stored in Asset Catalogs asynchronously, refreshing both preview and list.

The DMG preview uses a **Fit / 100%** segmented toggle. **Layout settings** has a
**Form / YAML** view switch in its upper-right corner. YAML edits the DMG section
itself (without an outer `dmg:` key), including advanced settings and contents.
The editor uses ggui’s multiline text field with a fixed-width font, selection,
clipboard, typing undo and IME support. Tab moves focus; Enter inserts a newline.
Syntax colors and automatic indentation are not supplied by this text field.

**Ctrl/Cmd+Enter** applies the YAML draft; **Escape** discards it. Switching back to
Form applies valid changes too. Invalid YAML, duplicate/unknown keys, and invalid
layout values keep the draft and show an error. Applying normalizes YAML formatting;
comments are not stored in the project model. Each apply can be undone once. View
switching by itself does not change settings. Advanced field titles omit the
“(JSON)” suffix; their structured fields still accept JSON in Form mode.

DMG content coordinates use `pos: [x, y]`, an array of exactly two integers.
Replace existing `x` and `y` properties with `pos`; the Form view still provides
separate X and Y controls, and both update the same position array.

Input changes update the preview immediately while preserving focus and cursor.
Incomplete or invalid values retain the last valid preview until corrected.
Enter or leaving the field commits one undoable edit; Escape restores the value
from before editing. This also applies to YAML and number steppers.

**Advanced settings** is a collapsible control inside the bottom of Layout
settings in Form mode. Opening it reveals the advanced fields; closing it returns
to the basic fields. The control remains available while the form scrolls.

Select a copied item and use **Item icon → Browse** to choose PNG, JPG/JPEG or ICNS.
The preview and Contents list update immediately. The **Reset item icon** action below the
field restores the original icon; both selection and reset support Undo/Redo.
The project stores the path in `dmg.contents.<source>.icon` (relative to the
project file when selected with Browse). Only the copy inside the DMG is changed.

Custom icons are supported for files, folders and unsigned apps on HFS+ and APFS.
Symbolic links use the target icon: their Item icon field is hidden, and a custom
icon must be reset before enabling Link. Finder ignores custom symlink icons.
Signed bundles and signed Mach-O executables reject custom icons at validation
and build time because Finder resource forks invalidate strict signature checks.
For a signed app, change its bundled app icon before signing instead.

## GUI implementation

The desktop is composed like ggui's `examples/sqlite`: a reactive model,
`Row`/`Column`/`Expanded` layouts, native `Scroll`, labelled fields and dialogs.
There is no absolute-positioned widget scene or full-screen revision counter.

- The editor (`editor.go`) is the one source of truth, held in a
  `ggui.Store`. `model.go` selects what the window shows from it, recomputed
  when the editor publishes a change, so nothing is copied across and kept in
  step by hand; a `ggui_debug` build reports a change left unpublished. A field's text and error
  are the editor's own reactive state (`formstate.go`), which the inputs bind
  to: the draft being edited is that text, not a second copy. Project/session
  transactions own save, undo, validation and builds.
- `view.go` composes the toolbar, underline tabs, status line and dialogs;
  `workspace.go` the page under the tabs: settings, component sidebar and Help.
  Settings expand until Help is opened. The toolbar also offers light/dark
  themes. The window is built once: colors are theme tokens such as
  `uitheme.MutedFg`, so a theme switch rebuilds nothing.
- `form.go` builds labelled controls and grouped rows. ggui handles clipping,
  keyboard focus and automatic reveal. Each form/source view retains its scroll
  position. Path fields accept a dropped file or folder.
- `designer.go` composes resizable DMG panels around a custom Finder canvas.
  Canvas input/drop coordinates are local to the space assigned by the layout.
- `app.go` connects lifecycle, shortcuts and native ggui file dialogs.
  The ggui version is pinned to a commit because its API is under development.

`go test ./internal/gui/...` runs headless widget tests, including save/undo,
invalid JSON, mode changes, focus reveal, native picker stubs, file drops,
canvas dragging and modal actions. Native IME composition still needs testing
in a running desktop app.

Like the SQLite example, an opt-in GPU renderer writes screenshots using a
fresh temporary project; it does not modify the working project:

```sh
ZAPP_GUI_RENDER_DIR=/tmp/zapp-previews go test ./internal/gui -run '^$' -count=1
```

The output covers all steps, component/JSON/Help views, dialogs and light/dark
appearance at 1080 and 1200 pixels. Rendering requires a desktop GPU session.
