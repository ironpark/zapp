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
#### 🍺 Using Homebrew
```bash
brew tap ironpark/zapp
brew install zapp
```

#### 🛠️ Build from source code

```bash
go install github.com/ironpark/zapp@latest
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
