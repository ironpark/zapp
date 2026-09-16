# Project settings GUI

```sh
zapp gui
zapp gui --config release/.zapp.yaml
```

`gui` opens an Ebitengine desktop window. It discovers `.zapp.yaml` in the current
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
| Notarization | Keychain profile, Apple ID, team ID, API key file and stapling |

Each step can be enabled or disabled. Disabled sections are omitted from the
saved project. Their values are retained while toggling within the same editor
session. Lists, license mappings, explicit DMG contents and full-form PKG data
use JSON text fields. For PKG, **Switch package form** changes between short and
full forms after incompatible fields have been cleared.

Relative paths remain relative to the configuration file. Expressions such as
`${app.name}` and `${env:ZAPP_IDENTITY}` are preserved. The GUI edits the file's
values; build-time `ZAPP_*` option overrides are not copied into the project.
Passwords are not entered or saved in the GUI; supply them through the existing
CLI environment variables or supported credential files when building.

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
   outside the preview. **Remove selected** removes the selected entry. **Default
   layout** restores the automatic app + Applications arrangement.
6. Click **Save**, then build normally with `zapp dmg` or `zapp build`.

Moving a default icon creates explicit `dmg.contents` entries for both the app
and the Applications link. The preview displays app-bundle icons when they can
be decoded, with generic artwork for other items. Background images are drawn
at their native size and clipped to the content area. The preview approximates
Finder: fonts, file icons and thumbnails can differ on the target Mac. Preview
images are limited to 8192 pixels per side and 32 million pixels in total.

## Editing and saving

- **Enter** applies a single-line field. **Ctrl/Cmd+Enter** applies a JSON field.
- **Tab / Shift+Tab** applies the current field and focuses the next/previous one.
- **Ctrl/Cmd+A**, **C**, **X**, **V** select all, copy, cut and paste within a field.
  Arrow keys move the cursor; multiline fields also support Up/Down.
- **Esc** cancels the current field edit.
- **Ctrl/Cmd+S** saves all tabs together.
- **Ctrl/Cmd+1–6** switches tabs. **Tab** also focuses the first field when none
  is being edited. Choice fields open a dropdown below the input with a click, Enter, Space or
  Down. Click an option or use Up/Down and Enter to select; Esc or an outside
  click dismisses the list without changing the value.
- With an icon selected and no text field active, **arrow keys** move it one
  pixel; **Shift+arrow** moves it ten pixels.
- **Undo / Redo** revert or restore settings and icon moves. **Ctrl/Cmd+Z** undoes
  a committed change, or cancels an active field edit; **Ctrl/Cmd+Shift+Z** redoes
  a committed change when no field is active.
- Scroll the settings panel to reach additional fields.

**Save** checks schema and layout values but permits drafts whose build inputs
are not present yet. **Validate** resolves the project and checks build inputs
without building, signing or submitting anything. Invalid edits show an error
below the input and keep the draft for correction. Validation moves to the
relevant field for missing or invalid path inputs; other errors appear in the
status bar. Saving rewrites formatting and
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

A progress dialog shows build messages and supports **Cancel build**. Editing
and duplicate builds are blocked while the job runs. Completion shows output
paths or the error; closing the window during a build cancels it and waits for
cleanup before continuing the normal unsaved-changes flow.

## Desktop requirements

The GUI uses Ebitengine 2.10 and builds with the regular `cmd/zapp` binary, including
`CGO_ENABLED=0` builds. Linux GUI use requires an X11/XWayland session and OpenGL
runtime libraries. CLI commands and help do not open a window. Clipboard support
on Linux requires `xclip` or `xsel`. File pickers use native macOS panels attached to the editor window,
Windows PowerShell/Windows Forms, or `zenity` on Linux. If a picker is unavailable,
paths can still be entered directly. A bundled font is used when a supported system
Unicode font is unavailable; glyph coverage then depends on that font.
On macOS, local window-event coordinates are used so remote and assistive input
can select and drag items independently of the physical system pointer.

The DMG preview offers **Fit** and **100%** view modes. At 100%, drag empty
preview space or hold **Space** while dragging to pan. Dragging an icon normally
still edits its position. View panning does not change the saved DMG layout;
clicking either view button recenters the preview.

Empty text inputs show example values or automatic-value placeholders; placeholders
are never saved as input. Numeric inputs provide up/down arrow controls and Up/Down keyboard
stepping (1 per step, or 10 with Shift). Stepping starts from the effective default
for automatic numeric values and stays within the field's supported range.
As with typed edits, Enter or leaving the field applies the draft; Escape cancels it.

The tab bar groups step actions on its right: **Enabled** and DMG
**Default layout**, or PKG **Switch package form**. **Add file** stays in Contents
and **Remove from DMG** in Item details. The settings
column adjusts to the window width to leave more room for the preview. Contents
shows the item count, type and source path; hover a row or path input to read its
path. Hover tabs and toolbar buttons for enabled-state and shortcut hints.

With no input focused, **Delete / Backspace** removes the selected DMG item and
**Escape** clears its selection. Removal changes only the layout, and Undo restores
the item. **Shift+Tab** starts at the last field when no input is focused.

The **Link** toggle in Item details switches the selected item between copying its
contents and creating a symbolic link to its source path. Name and position are
preserved, and Undo restores the previous mode.

Undo, Redo and Add file use embedded SVG icon buttons with hover tooltips.
Save, Validate and Remove from DMG show icons alongside their labels.

**Validate** is in the top header beside **Save**. The compact bottom status bar
shows feedback; hover a truncated status message to read its expanded text.

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
The editor uses a fixed-width font and highlights keys, strings, numbers, booleans,
and comments. Click to place the cursor, use the wheel to scroll, and press Tab
to insert two spaces; Enter retains indentation.

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
The preview and Contents list update immediately. The reset icon beside the
field restores the original icon; both selection and reset support Undo/Redo.
The project stores the path in `dmg.contents.<source>.icon` (relative to the
project file when selected with Browse). Only the copy inside the DMG is changed.

Custom icons are supported for files, folders and unsigned apps on HFS+ and APFS.
Symbolic links use the target icon: their Item icon field is hidden, and a custom
icon must be reset before enabling Link. Finder ignores custom symlink icons.
Signed bundles and signed Mach-O executables reject custom icons at validation
and build time because Finder resource forks invalidate strict signature checks.
For a signed app, change its bundled app icon before signing instead.
