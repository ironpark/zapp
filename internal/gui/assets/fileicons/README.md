# macOS file icons

These PNGs were extracted from macOS 15.7.9 at 256×256 pixels. The artwork is
Apple system artwork, not original Zapp artwork.

- `app`, `folder`, `file`, `applications`, `executable`, `font`, `alias`:
  `/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/*.icns`.
  Exact source filenames are recorded in `export.swift`.
- Other file-type icons: AppKit `NSWorkspace.icon(for:)` for the Uniform Type
  Identifiers recorded in `export.swift` (including the system's registered PKG type).

To refresh on macOS, run `swift internal/gui/assets/fileicons/export.swift` from
  the project root. The script writes PNGs beside itself. Record the source macOS
version here when refreshing. Go embeds only the PNG files; exporting is not a
build step and installed macOS assets are not required at runtime.
