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
brew install --cask zapp
```

#### 🛠️ 從源碼建立

```bash
go install github.com/ironpark/zapp/cmd/zapp@latest
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

## 專案設定與函式庫 API

使用一個 `.zapp.yaml` 設定相依套件打包、DMG/PKG 建立、簽署與公證。

```sh
go install github.com/ironpark/zapp/cmd/zapp@latest
zapp init --app dist/MyApp.app
zapp config show
zapp build                  # dep → sign(app) → dmg/pkg → sign → notarize → staple
zapp build dmg pkg          # 只執行選定的打包步驟
zapp dmg --title "MyApp"    # 覆寫專案標題
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

`dmg`、`pkg` 與 `dep` 會從工作目錄逐層向上尋找 `.zapp.yaml`。使用 `--config 路徑` 指定檔案，或以 `--no-config` 忽略設定檔。優先順序為 **CLI 參數 > `ZAPP_*` 環境變數 > 設定檔 > 預設值**。將參數名稱改為大寫並把連字號換成底線即為環境變數名稱，例如 `ZAPP_APP`、`ZAPP_TITLE`、`ZAPP_OUT`、`ZAPP_WINDOW_WIDTH`、`ZAPP_LIBS`（以逗號分隔）。設定檔中的路徑相對於設定檔所在目錄，CLI 與環境變數中的路徑則相對於工作目錄。最上層的 `out` 是目錄，而 `dmg.out` 與 `pkg.out` 是檔名。

`${env:NAME}`、`${app}`、`${app.name}` 與 `${app.version}` 可用於字串值與 `contents` 的鍵。引用未設定的環境變數會發生錯誤。存在 `sign:` 與 `notarize:` 時會自動執行對應步驟，可用 `--no-sign` 與 `--no-notarize` 略過。現有的 `--sign --notarize --profile ... --staple` 指令稿仍可正常運作。設定檔中不允許出現密碼鍵（`sign.p12Password`、`notarize.password`），請透過 `ZAPP_P12_PASSWORD` / `ZAPP_PASSWORD` 或對應的 CLI 參數提供。`config show` 不會輸出密碼。未加 `--force` 時 `init` 不會覆寫既有檔案。

PKG 簡寫形式支援 `identifier`、`version`、`installLocation`、`scripts`、`minOS` 與 `license`（路徑，或 `{default: 路徑, en: 路徑, ...}`）。識別碼與版本預設取自 Info.plist。完整形式支援 `components`，以及帶有可選項 `choices` 的 `distribution`；簡寫欄位與完整形式欄位不能混用。`type: component` 只會產生單一元件，不含產品安裝畫面。請參閱[附註解的專案範例](examples/zapp.yaml)與 [PKG 引擎文件](pkg/macpkg/README.md)。

舊的扁平 DMG 設定檔仍可透過 `zapp dmg --config examples/dmg/layout.yaml --out release.dmg` 使用，此時會顯示即將淘汰的警告。其輸出路徑同樣仍相對於工作目錄。將版面配置欄位移到 `dmg:` 之下，並把共用的 `app` 放在最上層，即可完成移轉。

現在可以匯入模組根目錄，執行檔已移至 `cmd/zapp`：

```go
project, err := zapp.Load(".zapp.yaml")
if err != nil { return err }
project.DMG.Title = "MyApp"
plan, err := project.Resolve(zapp.WithClock(time.Unix(1700000000, 0)))
if err != nil { return err }
artifacts, err := plan.Build(ctx, zapp.StepDMG, zapp.StepPKG)
```

請匯入 `github.com/ironpark/zapp`。你也可以直接建構 `zapp.Project`。`Plan.BundleDeps`、`BuildDMG`、`BuildPKG`、`Sign` 與 `Notarize` 可分別執行各項作業。除非提供 `WithLogger`，否則不會輸出任何紀錄。失敗會被包裝為 `*zapp.StepError`，並支援 `errors.As` / `errors.Is`。`WithClock` 會固定產生的 DMG 中繼資料與 PKG 時間戳記；不過要取得可重現的映像檔，來源檔案及其中繼資料也必須保持一致。
