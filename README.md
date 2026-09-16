# ZAPP
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![GitHub Repo stars](https://img.shields.io/github/stars/ironpark/zapp)](https://github.com/ironpark/zapp/stargazers)


🌐 [**English**](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) | [简体中文](README.zh-cn.md) | [繁體中文](README.zh-tw.md)

![asd](/docs/demo.gif)

**Simplify your macOS App deployment**

`zapp` is a powerful CLI tool designed to streamline and automate the deployment process for macOS applications. It handles all stages of deployment in one tool, from dependency bundling to DMG/PKG creation, code signing, and notarization.

## ✨ Features

- [x] Create DMG files
- [x] Create PKG files
- [x] Code signing
- [x] Notarization / Stapling
- [x] Modify plist (version)
- [x] Auto binary dependencies bundling
- [ ] Support GitHub Actions

## ⚡️ Quick start
#### Install with a script

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

#### 🍺 Using Homebrew
```bash
brew tap ironpark/zapp
brew install --cask zapp
```

#### 🛠️ Build from source code

```bash
go install github.com/ironpark/zapp/cmd/zapp@latest
```

## 📖 Usage
### 🔏 Code Signing

> [!TIP]
>
> If the `--identity` flag is not used to select a certificate, Zapp will automatically select an available certificate from the current keychain.

```bash
zapp sign --target="path/to/target.(app,dmg,pkg)"
```
```bash
zapp sign --identity="Developer ID Application" --target="path/to/target.(app,dmg,pkg)"
```

### 🏷️ Notarization & Stapling
> [!NOTE]
>
> When executing the notarize command, if Zapp receives an app bundle path, it automatically compresses the app bundle and attempts to notarize it.

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/target.(app,dmg,pkg)" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/target.(app,dmg,pkg)" --staple
```

### 🔗 Dependency Bundling
> [!NOTE]
> 
> This process inspects the dependencies of the application executable, includes the necessary libraries within `/Contents/Frameworks` and modifies the link paths to enable standalone execution.

```bash
zapp dep --app="path/to/target.app"
```
#### additional paths to search for libraries
```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```
#### with sign & notarize & staple
> [!TIP]
>
> `dep`, `dmg`, `pkg` commands can be used with the `--sign`, `--notarize`, and `--staple` flags.
> - The `--sign` flag will automatically sign the app bundle after bundling the dependencies.
> - The `--notarize` flag will automatically notarize the app bundle after signing.

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

Use `--config dmg.yaml` (or JSON with the same fields) to include files,
directories, and symbolic links at explicit icon positions. For example:

```yaml
version: 1
title: MyApp
window: {width: 720, height: 460}
iconSize: 96
labelSize: 14
contents:
  dist/MyApp.app:
    x: 180
    y: 200
  /Applications:
    link: true
    x: 540
    y: 200
  docs/README.pdf:
    name: Guide.pdf
    x: 360
    y: 350
```

```sh
zapp dmg --config dmg.yaml --out dist/MyApp.dmg
# Adjust the two default icons without a config file:
zapp dmg --app MyApp.app --app-position 180,200 --applications-position 540,200
```

- Set `version: 1`. Unknown fields and duplicate keys are rejected.
- Explicit CLI options override config values; omitted fields use CLI defaults.
- Input paths (`app`, `icon`, `background`, and file/directory keys in `contents`) are relative
  to the config file. CLI paths and `out` are relative to the working directory.
  Link targets are preserved literally, including relative targets.
- `contents` replaces the default app and Applications link. Each key is a source path, with
  `x` and `y` coordinates in its value. Set `link: true` for a symbolic link;
  otherwise the source is detected as a file or directory. Optional `name` changes
  its name inside the image. Names must be unique ignoring case and Unicode
  normalization, and must not use reserved DMG metadata names.
- Coordinates are nonnegative icon centers measured from the top-left of the
  Finder content area. `--app-position` and `--applications-position` apply only
  to the default two-item layout and cannot be combined with explicit `contents`.
- Without `contents`, provide `app` or `--app`. With explicit `contents`, `app`
  is optional and is used only to derive the default title and disk icon. Without
  `app`, provide `title`; omit `icon` for no custom disk icon.
- Other config fields: `out`, `app`, `icon`, `background`, `fs`, and `format`.
  PNG icon conversion and signing/notarization flags also work with `--config`.
- Layouts exceeding the current single-node `.DS_Store` capacity fail with an
  error; reduce the number of items or shorten names.

See [the layout example](examples/dmg/layout.yaml).

#### with sign & notarize & staple
> [!TIP]
>
> `dep`, `dmg`, `pkg` commands can be used with the `--sign`, `--notarize`, and `--staple` flags.
> - The `--sign` flag will automatically sign the app bundle after bundling the dependencies.
> - The `--notarize` flag will automatically notarize the app bundle after signing.

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
#### with sign & notarize & staple
> [!TIP]
>
> `dep`, `dmg`, `pkg` commands can be used with the `--sign`, `--notarize`, and `--staple` flags.
> - The `--sign` flag will automatically sign the app bundle after bundling the dependencies.
> - The `--notarize` flag will automatically notarize the app bundle after signing.

```bash
zapp pkg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 📝 Editing Info.plist

`zapp plist` reads and edits a `.plist` file, or the `Info.plist` inside a `.app`
bundle. The file is rewritten in the format it was read in, so a binary
`Info.plist` stays binary and the enclosing bundle's signature is not disturbed.

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
zapp sign --target="MyApp.app"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
zapp notarize --profile="key-chain-profile" --target="MyApp.dmg" --staple
```
or just use the shorthand command
```bash
zapp dep --app="MyApp.app" --sign --notarize --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \ 
  --sign --notarize --profile="key-chain-profile" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple
```

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zapp is released under the [MIT License](LICENSE).

## Support

If you encounter any issues or have questions, please file an issue on the [GitHub issue tracker](https://github.com/ironpark/zapp/issues).

## Project files and library API

Use one `.zapp.yaml` for dependency bundling, DMG/PKG packaging, signing, and notarization.

```sh
go install github.com/ironpark/zapp/cmd/zapp@latest
zapp init --app dist/MyApp.app
zapp config show
zapp build                  # dep → sign(app) → dmg/pkg → sign → notarize → staple
zapp build dmg pkg          # select packaging steps
zapp dmg --title "MyApp"     # overrides project title
zapp pkg --no-sign --no-notarize
```

```yaml
version: 1
app: dist/MyApp.app
out: dist
# dep: {libs: [/opt/homebrew/lib]}
dmg: {}
pkg: {}
```

`dmg`, `pkg`, and `dep` discover `.zapp.yaml` by walking up from the working directory. Use `--config path` to select a file or `--no-config` to ignore files. Precedence is **CLI flags > `ZAPP_*` environment > file > defaults**. Flag names become uppercase environment names with underscores, e.g. `ZAPP_APP`, `ZAPP_TITLE`, `ZAPP_OUT`, `ZAPP_WINDOW_WIDTH`, `ZAPP_LIBS` (comma-separated). File paths are relative to the configuration directory; CLI and environment paths are relative to the working directory. Shared `out` is a directory; `dmg.out` and `pkg.out` are filenames.

`${env:NAME}`, `${app}`, `${app.name}`, and `${app.version}` work in string values and content keys. Unset environment references fail. `sign:` and `notarize:` enable their steps automatically; `--no-sign` and `--no-notarize` skip them. Existing `--sign --notarize --profile ... --staple` scripts continue to work. Password keys (`sign.p12Password`, `notarize.password`) are forbidden in files: supply `ZAPP_P12_PASSWORD` / `ZAPP_PASSWORD` or their CLI flags. `config show` omits passwords. `init` refuses to overwrite a file without `--force`.

PKG short form supports `identifier`, `version`, `installLocation`, `scripts`, `minOS`, and `license` (a path or `{default: path, en: path, ...}`). Identifier/version default from Info.plist. Full form supports `components` and `distribution` with selectable `choices`; short and full fields cannot be mixed. `type: component` builds a single component without product UI. See the [annotated project example](examples/zapp.yaml) and [PKG engine documentation](pkg/macpkg/README.md).

Legacy flat DMG files remain accepted by `zapp dmg --config examples/dmg/layout.yaml --out release.dmg`, with a deprecation warning. Their output remains relative to the working directory. Migrate by moving layout fields under `dmg:` and keeping shared `app` at the root.

The module root is now importable; the executable moved to `cmd/zapp`:

```go
project, err := zapp.Load(".zapp.yaml")
if err != nil { return err }
project.DMG.Title = "MyApp"
plan, err := project.Resolve(zapp.WithClock(time.Unix(1700000000, 0)))
if err != nil { return err }
artifacts, err := plan.Build(ctx, zapp.StepDMG, zapp.StepPKG)
```

Import `github.com/ironpark/zapp`. You can also construct `zapp.Project` directly. `Plan.BundleDeps`, `BuildDMG`, `BuildPKG`, `Sign`, and `Notarize` run individual operations. Logging is silent unless `WithLogger` is supplied. Failures wrap `*zapp.StepError` and support `errors.As` / `errors.Is`. `WithClock` fixes generated DMG metadata and PKG timestamps; reproducible images also require stable source files and source metadata.
