# ZAPP

**Simplify your macOS App deployment**

Zapp is a powerful CLI tool designed to simplify and streamline the deployment of macOS applications. With Zapp, you can effortlessly create dmg and pkg files, perform code signing, notarize your apps, and modify plist files.


## Examples
Create a DMG file from the app bundle.
```bash
zapp dmg "<path of app-bundle>"
```

Create a PKG file from the app bundle.
```bash
zapp pkg "<path of app-bundle>"
```

## Features
- [x] Create DMG files
- [ ] Create PKG files
- [ ] Code signing
- [ ] Notarization / Stapling With Retries
- [ ] Modify plist (version)
- [ ] Auto binary dependencies bundling
- [ ] Support GitHub Actions
