# ZAPP

**Simplify your macOS App deployment**

Zapp is a powerful CLI tool designed to streamline the deployment process for macOS applications. With Zapp, you can effortlessly create DMG and PKG files, perform code signing, notarize your apps, and modify plist files.

## Features

- [x] Create DMG files
- [x] Create PKG files
- [x] Code signing
- [ ] Notarization / Stapling with Retries
- [ ] Modify plist (version)
- [ ] Auto binary dependencies bundling
- [ ] Support GitHub Actions

## Installation

(TODO: Add installation instructions)

## Usage

### Creating DMG Files

> Zapp can be used to create DMG files, a common format used for distributing macOS apps.
It greatly simplifies the DMG creation process by automatically extracting icons from the app bundle, compositing disk icons, and providing an interface for drag-and-drop installation of the app.


```bash
zapp dmg --app "path/to/target.app"
```

```bash
zapp dmg --title "My App" --out "MyApp.dmg" --icon "path/to/icon.icns" --app "path/to/target.app"
```

### Creating PKG Files

#### Create a PKG file from the app bundle
```bash
zapp pkg "path/to/target.app"
```

```bash
zapp pkg --out "MyApp.pkg" --version "1.2.3" --identifier "com.example.myapp" "path/to/target.app"
```

#### With EULA Files

Include End User License Agreement (EULA) files in multiple languages:

```bash
zapp pkg "path/to/target.app" --eula en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt
```
### Code Signing

If the `--identity` flag is not used to select a certificate, Zapp will automatically select an available certificate from the current keychain.

```bash
zapp sign "path/to/target.(app,dmg,pkg)"
```
```bash
zapp sign --identity "Developer ID Application" "path/to/target.(app,dmg,pkg)"
```
## Advanced Usage

(TODO: Add advanced usage examples)

## License

Zapp is released under the [MIT License](LICENSE).

## Support

If you encounter any issues or have questions, please file an issue on the [GitHub issue tracker](https://github.com/your-repo/zapp/issues).
