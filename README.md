# ZAPP

**Simplify your macOS App deployment**

Zapp is a powerful CLI tool designed to simplify and streamline the deployment of macOS applications. With Zapp, you can effortlessly create dmg and pkg files, perform code signing, notarize your apps, and modify plist files.


## Basic Examples
Create a DMG file from the app bundle.
```bash
zapp dmg "<path of app-bundle>"
```
### Create PKG
#### Default Usage
```bash
zapp pkg "<path of app-bundle>"
```
#### With EULA Files
```bash
zapp pkg "<path of app-bundle>" --eula en:eula.txt,ko:ko_eula.txt
```

## Features
- [x] Create DMG files
- [x] Create PKG files
- [ ] Code signing
- [ ] Notarization / Stapling With Retries
- [ ] Modify plist (version)
- [ ] Auto binary dependencies bundling
- [ ] Support GitHub Actions
