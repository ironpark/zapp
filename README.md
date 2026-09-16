# ZAPP

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![GitHub Repo stars](https://img.shields.io/github/stars/ironpark/zapp)](https://github.com/ironpark/zapp/stargazers)

🌐 [**English**](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) | [简体中文](README.zh-cn.md) | [繁體中文](README.zh-tw.md)

![Zapp packaging a macOS application](docs/demo.gif)

**Simplify your macOS App deployment**

`zapp` is a CLI and Go library for packaging and distributing macOS applications. Bundle dependencies, create DMG/PKG installers, sign, and notarize with one `.zapp.yaml`, or run individual commands as needed.

[Installation](#installation) · [Quick start](#quick-start) · [Configuration](#project-configuration) · [Usage](#-usage) · [Go library](#go-library)

## ✨ Features

- [x] Create DMG files
- [x] Create PKG files
- [x] Code signing
- [x] Notarization / Stapling
- [x] Modify plist (version)
- [x] Auto binary dependencies bundling
- [x] Declarative project configuration and Go library API

## Installation

### Install with a script

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/scripts/install.ps1 | iex"
```

Installs the latest published release after SHA-256 verification. macOS/Linux: `~/.local/bin` (add it to your PATH if prompted). Windows: `%LOCALAPPDATA%\zapp\bin`, added to your user PATH. No administrator access is required.

Set `ZAPP_VERSION` to a release tag to pin a version, or `ZAPP_INSTALL_DIR` to an absolute installation directory.

### 🍺 Using Homebrew

```bash
brew tap ironpark/zapp
brew install --cask zapp
```

### 🛠️ Build from source code

```bash
go install github.com/ironpark/zapp/cmd/zapp@latest
```

Source builds require Go 1.27.0 or later. Linux and Windows (amd64/arm64) include prebuilt signing libraries, so no separate download or Rust build is needed. Enable cgo and install a C compiler: GCC on Linux, LLVM MinGW on Windows. See the [platform build instructions](docs/signing.md#building).

## Quick start

Start in a directory containing an already-built `dist/MyApp.app`. Zapp does not compile your application. `init` creates DMG and PKG sections; `build` writes both installers to `dist` by default.

Use one `.zapp.yaml` for dependency bundling, DMG/PKG packaging, signing, and notarization.

```sh
zapp init --app dist/MyApp.app
zapp config show
zapp build                  # Run configured steps
zapp build dmg pkg          # Skip dep; configured signing/notarization still run
zapp dmg --title "MyApp"     # overrides project title
zapp pkg --no-sign --no-notarize
```

The generated configuration has this shape. Add `dep`, `sign`, or `notarize` to enable those steps.

```yaml
version: 1
app: dist/MyApp.app
out: dist
# dep: {libs: [/opt/homebrew/lib]}

dmg: {}
pkg: {}
```

## Project configuration

Define shared paths and build steps in `.zapp.yaml`. See the [annotated example](examples/zapp.yaml) for the full set of options.

### Select a configuration file

You can also edit the project in the Ebitengine desktop GUI:

```sh
zapp gui
zapp gui --config release/.zapp.yaml
```

Tabs cover project, DMG, PKG, dependencies, signing and notarization settings.
The DMG tab previews backgrounds and app icons and lets you drag icons into
position. Save the configuration, then build with the existing CLI commands.
See the [GUI guide](docs/gui.md) for shortcuts, JSON fields and desktop requirements.
Saving preserves relative paths and variable expressions but rewrites formatting
and comments.

`dmg`, `pkg`, and `dep` search for `.zapp.yaml` from the current directory upward.

```sh
zapp dmg                              # Discover .zapp.yaml
zapp dmg --config release/.zapp.yaml   # Choose a configuration file
zapp dmg --no-config --app MyApp.app   # Run without a configuration file
zapp config show                      # Inspect the effective configuration
```

`zapp init` creates a starter configuration. Overwriting an existing file requires `--force`.

### Override values

When an option is set in more than one place, the leftmost source wins:

**CLI flags → `ZAPP_*` environment variables → configuration file → defaults**

To derive an environment variable name, uppercase the flag, replace hyphens with underscores, and add `ZAPP_`.

| CLI flag | Environment variable | Example value |
| --- | --- | --- |
| `--app` | `ZAPP_APP` | `dist/MyApp.app` |
| `--title` | `ZAPP_TITLE` | `MyApp` |
| `--out` | `ZAPP_OUT` | `dist/MyApp.dmg` (for `dmg`) |
| `--window-width` | `ZAPP_WINDOW_WIDTH` | `720` |
| `--libs` | `ZAPP_LIBS` | `/usr/local/lib,/opt/homebrew/lib` |

### Resolve paths

| Where a path is set | Relative to |
| --- | --- |
| Configuration file | The configuration file's directory |
| CLI flag or environment variable | The current working directory |

Top-level `out` is an **output directory**; `dmg.out` and `pkg.out` are **output file paths**.

```yaml
version: 1
app: dist/MyApp.app
out: dist

dmg:
  out: dist/MyApp-installer.dmg
pkg:
  out: dist/MyApp-installer.pkg
```

### Use variables

The following variables work in string values and `contents` keys:

| Variable | Value |
| --- | --- |
| `${app}` | App path |
| `${app.name}` | App name |
| `${app.version}` | App version |
| `${env:NAME}` | Environment variable `NAME`; fails if unset |

```yaml
dmg:
  title: ${app.name}
  out: dist/${app.name}-${app.version}.dmg
```

### Enable signing and notarization

Adding a `sign:` or `notarize:` section enables that step. This example uses a macOS keychain profile:

```yaml
sign:
  identity: ${env:ZAPP_IDENTITY}
notarize:
  profile: my-profile
  staple: true
```

- Skip a step for an individual run with `--no-sign` or `--no-notarize`.
- Existing `--sign --notarize --profile ... --staple` flags also work.
- Supply passwords through `ZAPP_P12_PASSWORD`, `ZAPP_PASSWORD`, or their CLI flags. Configuration files cannot contain `sign.p12Password` or `notarize.password`.
- `zapp config show` omits passwords.

See [signing and notarization](docs/signing.md) for platform-specific certificates and authentication.

### Advanced configuration

<details>
<summary>PKG configuration forms</summary>

| Form | Purpose and fields |
| --- | --- |
| Short | Package one app with `identifier`, `version`, `installLocation`, `scripts`, `minOS`, and `license` |
| Full | Define multiple `components` and a `distribution`, with selectable `choices` |

Short and full fields cannot be mixed.

- Identifier and version default to the app's Info.plist values.
- `license` accepts a file path or a language map such as `{default: license.txt, en: license-en.txt}`.
- `type: component` creates a single component without product installer UI.

See the [PKG engine documentation](pkg/macpkg/README.md) for details.

</details>

<details>
<summary>Migrate a legacy DMG configuration</summary>

Flat DMG configurations still work, with a deprecation warning:

```sh
zapp dmg --config examples/dmg/layout.yaml --out release.dmg
```

Their output paths remain relative to the working directory. To migrate to a project configuration, move layout fields under `dmg:` and keep shared `app` at the root.

</details>

## 📖 Usage

`dep`, `dmg`, and `pkg` accept `--sign --notarize --profile "profile" --staple`. With signing enabled, Zapp signs the app before packaging and signs the generated installers. Notarization and stapling apply to the resulting artifacts. The keychain examples below are for macOS; see [signing and notarization](docs/signing.md) for Linux/Windows certificates and API keys.

### 🔏 Code Signing

> [!TIP]
>
> If the `--identity` flag is not used to select a certificate, Zapp will automatically select an available certificate from the current keychain.

```bash
zapp sign --target="path/to/MyApp.app"
```
```bash
zapp sign --identity="Developer ID Application" --target="path/to/MyApp.app"
```

### 🏷️ Notarization & Stapling

> [!NOTE]
>
> When executing the notarize command, if Zapp receives an app bundle path, it automatically compresses the app bundle and attempts to notarize it.

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/MyApp.app" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/MyApp.app" --staple
```

### 🔗 Dependency Bundling

> [!NOTE]
>
> This process inspects the dependencies of the application executable, includes the necessary libraries within `/Contents/Frameworks` and modifies the link paths to enable standalone execution.

```bash
zapp dep --app="path/to/target.app"
```
#### Additional library search paths

```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```
#### Sign, notarize, and staple

```bash
zapp dep --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 💽 Creating DMG Files

> Zapp can be used to create DMG files, a common format used for distributing macOS apps.
It greatly simplifies the DMG creation process by automatically extracting icons from the app bundle, compositing disk icons, and providing an interface for drag-and-drop installation of the app.

DMG generation is pure Go, built on the [macfs](pkg/macfs), [udif](pkg/udif)
and [lzfse](pkg/lzfse) packages, with no `hdiutil` dependency. The image is
assembled whole rather than created and then mounted to be decorated, so no
volume is ever mounted during a build. Signing and notarization use the existing
backends.

HFS+ remains the default filesystem. Use `--fs apfs` for APFS, or
`--fs apfs-case-sensitive` to distinguish names such as `App` and `app`.
Both APFS modes require macOS 10.13 or later to open and can be generated on
Linux or macOS. They create a single unencrypted volume; snapshots and existing
image editing are not supported.

FinderInfo and resource forks are preserved from input files. On Linux these
attributes use the `user.com.apple.*` namespace. The mounted volume always
contains its icon; attaching an icon to the host `.dmg` file is skipped when
Linux cannot store it because of extended-attribute size or support limits.

```bash
zapp dmg --app="MyApp.app" --fs apfs
zapp dmg --app="MyApp.app" --fs apfs-case-sensitive --format ulfo
```

Pass `--format ulfo` to compress with LZFSE instead of zlib, which produces a
smaller image that only macOS 10.11 and later can read. The default, `udzo`, is
read by every version of macOS when used with HFS+. Selecting APFS still requires
macOS 10.13 regardless of compression.

```bash
zapp dmg --app="path/to/target.app"
```

```bash
zapp dmg --title="My App" \
  --app="path/to/target.app" \
  --icon="path/to/icon.icns" \
  --bg="path/to/background.png" \
  --out="MyApp.dmg"
```

`--icon` accepts ICNS or PNG. PNG artwork is converted to ICNS with its aspect
ratio and transparency preserved. Omitting the extension in `--out MyApp`
creates `MyApp.dmg`; signing and notarization use that same path.

#### Custom layouts

Define files and icon positions in `dmg.contents` in `.zapp.yaml`. This example places the app, an Applications link, and a guide in the image.

```yaml
version: 1
app: dist/MyApp.app

dmg:
  title: MyApp
  window:
    width: 720
    height: 460
  iconSize: 96
  labelSize: 14
  contents:
    dist/MyApp.app:
      pos: [180, 200]
    /Applications:
      link: true
      pos: [540, 200]
    docs/README.pdf:
      name: Guide.pdf
      pos: [360, 350]
```

```sh
zapp dmg --config .zapp.yaml --out dist/MyApp.dmg
```

**Item fields**

Each `contents` key is a source path. Explicit `contents` replaces the default app and Applications layout, so include every item you need.

| Field | Description |
| --- | --- |
| `pos: [x, y]` | Nonnegative icon-center coordinates from the top-left of the Finder content area |
| `icon` | Optional PNG/JPEG/ICNS custom icon for copied items; unsupported for links or signed code |
| `link` | `true` creates a symbolic link; omit to copy a file or directory |
| `name` | Name inside the image; defaults to the source name |

File and directory paths are relative to the configuration file. Link targets are preserved literally. The CLI `--out` path is relative to the working directory.

**Move only the default icons**

To reposition the app and Applications link, run without `contents`:

```sh
zapp dmg --app MyApp.app \
  --app-position 180,200 \
  --applications-position 540,200
```

<details>
<summary>Additional options and constraints</summary>

- `--app-position` and `--applications-position` cannot be combined with explicit `contents`.
- Without `contents`, `app` is required. With `contents`, `app` supplies the default title and disk icon. Set `dmg.title` when omitting `app`.
- Set `dmg.icon` to ICNS or PNG artwork and `dmg.background` to a background image. Filesystem, compression, signing, and notarization options also work with custom layouts.
- Set `version: 1`. Unknown fields and duplicate keys are rejected.
- Image names must be unique ignoring case and Unicode normalization, and cannot use reserved DMG metadata names.
- If the layout exceeds `.DS_Store` capacity, reduce the item count or shorten names.

See the [project example](examples/zapp.yaml) for a complete configuration, or the [migration notes](#advanced-configuration) for legacy flat layouts.

</details>

#### Sign, notarize, and staple

```bash
zapp dmg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```
### 📦 Creating PKG Files

PKG generation uses the pure Go [macpkg package](pkg/macpkg/README.md), with no
`pkgbuild` or `productbuild` dependency. Signing and notarization use the existing backends.

> [!TIP]
>
> If the `--version` and `--identifier` flags are not set, these values will be automatically retrieved from the Info.plist file of the provided app bundle

#### Create a PKG file from the app bundle

```bash
zapp pkg --app="path/to/target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="path/to/target.app"
```

#### With EULA Files

Include End User License Agreement (EULA) files in multiple languages:

```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="path/to/target.app"
```
#### Sign, notarize, and staple

```bash
zapp pkg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 📝 Editing Info.plist

`zapp plist` reads and edits a `.plist` file, or the `Info.plist` inside a `.app`
bundle. The file is rewritten in the format it was read in, so a binary
`Info.plist` stays binary. Editing signed bundle contents requires signing the
app again; make plist changes before signing.

```bash
zapp plist get "path/to/target.app" CFBundleVersion
zapp plist set "path/to/target.app" CFBundleVersion 1.2.3
zapp plist delete "path/to/target.app" LSUIElement
```

A key keeps the type it already has, so a boolean stays a boolean:

```bash
# writes <false/>, not the string "false", which macOS would read as true

zapp plist set "path/to/target.app" LSUIElement false
```

A key that does not exist yet becomes a boolean for `true` or `false`, an
integer for a whole number, and a string otherwise, so a version like `1.0`
stays a string.

Nested values are addressed with a dotted path. A key that itself contains dots,
as entitlements keys do, is matched before the name is read as a path:

```bash
zapp plist get "path/to/target.app" NSAppTransportSecurity.NSAllowsArbitraryLoads
zapp plist get "entitlements.plist" com.apple.security.app-sandbox
```

#### Raising a version

```bash
zapp plist bump "path/to/target.app"                 # 1.4.2 -> 1.4.3
zapp plist bump "path/to/target.app" --minor         # 1.4.2 -> 1.5.0
zapp plist bump "path/to/target.app" --major         # 1.4.2 -> 2.0.0
zapp plist bump "path/to/target.app" --key CFBundleShortVersionString
```

Without a flag the last component is raised, which is what a build number
wants. `--major`, `--minor` and `--patch` raise that component and reset the
ones after it.

### Full Example

The following is a complete example showing how to use `zapp` to dependency bundling, codesign, packaging, notarize, and staple `MyApp.app`:

```bash
# Dependency bundling

zapp dep --app="MyApp.app"

# Codesign / notarize / staple

zapp sign --target="MyApp.app"
zapp notarize --profile="key-chain-profile" --target="MyApp.app" --staple

# Create pkg/dmg file

zapp pkg --app="MyApp.app" --out="MyApp.pkg"
zapp dmg --app="MyApp.app" --out="MyApp.dmg"

# Codesign / notarize / staple for pkg/dmg

zapp sign --target="MyApp.dmg"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
zapp notarize --profile="key-chain-profile" --target="MyApp.dmg" --staple
```
or just use the shorthand command
```bash
zapp dep --app="MyApp.app" --sign --notarize --profile="key-chain-profile" --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple
```

## Go library

Import `github.com/ironpark/zapp` to load a project and build DMG/PKG installers.

```go
project, err := zapp.Load(".zapp.yaml")
if err != nil {
	return err
}

plan, err := project.Resolve()
if err != nil {
	return err
}

artifacts, err := plan.Build(ctx, zapp.StepDMG, zapp.StepPKG)
if err != nil {
	return err
}
```

You can also construct `zapp.Project` directly or run individual operations with `BuildDMG`, `BuildPKG`, and other methods. Output paths are available in `artifacts.DMG` and `artifacts.PKG`.

## License

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zapp is released under the [MIT License](LICENSE).

## Support

If you encounter any issues or have questions, please file an issue on the [GitHub issue tracker](https://github.com/ironpark/zapp/issues).
