# ZAPP

[**English**](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) 

**Simplify your macOS App deployment**

Zapp is a powerful CLI tool designed to streamline the deployment process for macOS applications. With Zapp, you can effortlessly create DMG and PKG files, perform code signing, notarize your apps, and modify plist files.

## Features

- [x] Create DMG files
- [x] Create PKG files
- [x] Code signing
- [x] Notarization / Stapling
  - [ ] Retrieve notarization
- [ ] Modify plist (version)
- [x] Auto binary dependencies bundling
- [ ] Support GitHub Actions

## Installation
Using Homebrew
```bash
brew tap ironpark/zapp
brew install zapp
```
Using Go
```bash
go install github.com/ironpark/zapp@latest
```

## Usage

### Full Example
The following is a complete example showing how to use `zapp` to dependency bundling, codesign, packaging, notarize, and staple `MyApp.app`:
```bash
zapp dep --app="MyApp.app"
zapp sign --target="MyApp.app"
zapp pkg --out="MyApp.pkg" --app="MyApp.app"
zapp sign --target="MyApp.pkg"
zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
```


### Dependency Bundling
This process inspects the dependencies of the application executable, includes the necessary libraries within /Contents/Frameworks, and modifies the link paths to enable standalone execution.

```bash
zapp dep --app="path/to/target.app"
```
You can add additional paths to search for libraries
```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```


### Creating DMG Files

> Zapp can be used to create DMG files, a common format used for distributing macOS apps.
It greatly simplifies the DMG creation process by automatically extracting icons from the app bundle, compositing disk icons, and providing an interface for drag-and-drop installation of the app.


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

### Creating PKG Files
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
### Code Signing

If the `--identity` flag is not used to select a certificate, Zapp will automatically select an available certificate from the current keychain.

```bash
zapp sign --target="path/to/target.(app,dmg,pkg)"
```
```bash
zapp sign --identity="Developer ID Application" --target="path/to/target.(app,dmg,pkg)"
```

### Notarization & Stapling
> When executing the notarize command, if Zapp receives an app bundle path, it automatically compresses the app bundle and attempts to notarize it.

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/target.(app,dmg,pkg)" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/target.(app,dmg,pkg)" --staple
```

## Advanced Usage

(TODO: Add advanced usage examples)

## License

Zapp is released under the [MIT License](LICENSE).

## Support

If you encounter any issues or have questions, please file an issue on the [GitHub issue tracker](https://github.com/ironpark/zapp/issues).
