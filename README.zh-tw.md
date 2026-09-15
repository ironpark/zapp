# ZAPP
[![FOSSA 狀態](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![GitHub 儲存庫星標數](https://img.shields.io/github/stars/ironpark/zapp)](https://github.com/ironpark/zapp/stargazers)


🌐 [English](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) | [简体中文](README.zh-cn.md) | [**繁體中文**](README.zh-tw.md)

![asd](/docs/demo.gif)

**簡化你的 macOS 應用程式部署**

`zapp` 是一個強大的 CLI 工具，旨在簡化和自動化 macOS 應用程式的部署流程。它在一個工具中處理所有部署階段，從依賴項打包到 DMG/PKG 建立、程式碼簽名和驗證。

## ✨ 特性

- [x] 建立 DMG 檔案
- [x] 建立 PKG 檔案
- [x] 程式碼簽名
- [x] 驗證 / 加蓋
- [x] 修改 plist（版本）
- [x] 自動二進位制依賴項打包
- [ ] 支援 GitHub Actions

## ⚡️ 快速開始
#### 🍺 使用 Homebrew
```bash
brew tap ironpark/zapp
brew install zapp
```

#### 🛠️ 從源碼建立

```bash
go install github.com/ironpark/zapp@latest
```

## 📖 使用方式
### 🔏 程式碼簽名

> [!TIP]
>
> 如果未使用 `--identity` 參數選擇憑證，Zapp 會自動從當前金鑰串列中選擇可用憑證。

```bash
zapp sign --target="path/to/target.(app,dmg,pkg)"
```
```bash
zapp sign --identity="Developer ID Application" --target="path/to/target.(app,dmg,pkg)"
```

### 🏷️ 驗證與加蓋
> [!NOTE]
>
> 當執行驗證指令時，如果 Zapp 收到應用程式封裝路徑，它會自動壓縮應用程式封裝並嘗試驗證它。

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/target.(app,dmg,pkg)" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/target.(app,dmg,pkg)" --staple
```

### 🔗 依賴項打包
> [!NOTE]
> 
> 這個過程會檢查應用程式可執行檔的依賴項，將必要的函式庫包含在 `/Contents/Frameworks` 中，並修改連結路徑以實現獨立執行。

```bash
zapp dep --app="path/to/target.app"
```
#### 增加搜尋函式庫的路徑
```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```
#### 帶有簽名與驗證與加蓋
> [!TIP]
>
> `dep`、`dmg`、`pkg` 指令可以與 `--sign`、`--notarize` 和 `--staple` 參數一起使用。
> - `--sign` 參數會在打包依賴項後自動對應用程式封裝進行簽名。
> - `--notarize` 參數會在簽名後自動對應用程式封裝進行驗證。

```bash
zapp dep --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 💽 建立 DMG 檔案

> Zapp 可以用來建立 DMG 檔案，這是用於分發 macOS 應用程式的常見格式。
它通過自動從應用程式封裝中提取圖示、合成磁碟圖示並提供應用程式拖放安裝的介面，大大簡化了 DMG 建立流程。


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
#### 帶有簽名與驗證與加蓋
> [!TIP]
>
> `dep`、`dmg`、`pkg` 指令可以與 `--sign`、`--notarize` 和 `--staple` 參數一起使用。
> - `--sign` 參數會在打包依賴項後自動對應用程式封裝進行簽名。
> - `--notarize` 參數會在簽名後自動對應用程式封裝進行驗證。

```bash
zapp dmg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```
### 📦 建立 PKG 檔案

> [!TIP]
> 
> 如果未設定 `--version` 和 `--identifier` 參數，這些值會自動從提供的應用程式封裝的 Info.plist 檔案中取得。

#### 從應用程式封裝建立 PKG 檔案
```bash
zapp pkg --app="path/to/target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="path/to/target.app"
```

#### 帶有 EULA 檔案

包含多語言的最終用戶許可協議（EULA）檔案：

```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="path/to/target.app" 
```
#### 帶有簽名與驗證與加蓋
> [!TIP]
>
> `dep`、`dmg`、`pkg` 指令可以與 `--sign`、`--notarize` 和 `--staple` 參數一起使用。
> - `--sign` 參數會在打包依賴項後自動對應用程式封裝進行簽名。
> - `--notarize` 參數會在簽名後自動對應用程式封裝進行驗證。

```bash
zapp pkg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 完整範例
以下是一個完整的範例，展示如何使用 `zapp` 來打包依賴項、程式碼簽名、封裝、驗證和加蓋 `MyApp.app`：

```bash
# 依賴項打包
zapp dep --app="MyApp.app"

# 程式碼簽名 / 驗證 / 加蓋
zapp sign --target="MyApp.app"
zapp notarize --profile="key-chain-profile" --target="MyApp.app" --staple

# 建立 pkg/dmg 檔案
zapp pkg --app="MyApp.app" --out="MyApp.pkg"
zapp dmg --app="MyApp.app" --out="MyApp.dmg"

# 為 pkg/dmg 簽名與驗證與加蓋
zapp sign --target="MyApp.app"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
zapp notarize --profile="key-chain-profile" --target="MyApp.dmg" --staple
```
或直接使用簡寫指令
```bash
zapp dep --app="MyApp.app" --sign --notarize --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \ 
  --sign --notarize --profile="key-chain-profile" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple
```

## 授權
[![FOSSA 狀態](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zapp 是根據 [MIT 授權協議](LICENSE) 發布的。

## 支援

如果您遇到任何問題或有疑問，請在 [GitHub 問題追蹤器](https://github.com/ironpark/zapp/issues) 上提交問題。
