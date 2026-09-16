# ZAPP

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![GitHub Repo stars](https://img.shields.io/github/stars/ironpark/zapp)](https://github.com/ironpark/zapp/stargazers)

🌐 [English](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) | [简体中文](README.zh-cn.md) | [**繁體中文**](README.zh-tw.md)

![使用 Zapp 封裝 macOS 應用程式的示範](docs/demo.gif)

**簡化 macOS 應用程式的發布**

`zapp` 是用於封裝和發布 macOS 應用程式的 CLI 工具和 Go 程式庫。透過一個 `.zapp.yaml` 設定相依項目封裝、DMG/PKG 建立、簽章和公證，也可以依需要執行單一命令。

[安裝](#安裝) · [快速入門](#快速入門) · [專案設定](#專案設定) · [使用方法](#-使用方法) · [Go 程式庫](#go-程式庫)

## ✨ 功能

- [x] 建立 DMG 檔案
- [x] 建立 PKG 檔案
- [x] 程式碼簽章
- [x] 公證及附加公證票據（Stapling）
- [x] 編輯 plist 和更新版本
- [x] 自動封裝二進位相依項目
- [x] 宣告式專案設定和 Go 程式庫 API

## 安裝

### 使用指令碼安裝

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/scripts/install.ps1 | iex"
```

透過 SHA-256 驗證後安裝最新發布的版本。macOS/Linux 的安裝目錄為 `~/.local/bin`，如有提示，請將其新增到 PATH。Windows 的安裝目錄為 `%LOCALAPPDATA%\zapp\bin`，指令碼會將其新增到使用者 PATH。無需管理員權限。

將 `ZAPP_VERSION` 設為發布標籤可固定版本；將 `ZAPP_INSTALL_DIR` 設為絕對路徑可指定安裝目錄。

### 🍺 使用 Homebrew

```bash
brew tap ironpark/zapp
brew install --cask zapp
```

### 🛠️ 從原始碼建置

```bash
go install github.com/ironpark/zapp/cmd/zapp@latest
```

原始碼建置需要 Go 1.27.0 或更高版本。已包含 Linux 和 Windows（amd64/arm64）的預編譯簽章程式庫，無需單獨下載或編譯 Rust。請啟用 cgo，並準備 C 編譯器：Linux 使用 GCC，Windows 使用 LLVM MinGW。詳情參閱[各平台建置說明](docs/signing.md#building)。

## 快速入門

在包含已建置的 `dist/MyApp.app` 的目錄中開始。Zapp 不會編譯應用程式原始碼。`init` 產生 DMG 和 PKG 設定，`build` 預設將兩種安裝套件輸出到 `dist`。

用一個 `.zapp.yaml` 設定相依項目封裝、DMG/PKG 建立、簽章和公證。

```sh
zapp init --app dist/MyApp.app
zapp config show
zapp build                  # 執行已設定的步驟
zapp build dmg pkg          # 跳過 dep，仍執行已設定的簽章和公證
zapp dmg --title "MyApp"     # 覆蓋專案標題
zapp pkg --no-sign --no-notarize
```

產生的設定如下。新增 `dep`、`sign` 或 `notarize` 可啟用相應步驟。

```yaml
version: 1
app: dist/MyApp.app
out: dist
# dep: {libs: [/opt/homebrew/lib]}
dmg: {}
pkg: {}
```

## 專案設定

在 `.zapp.yaml` 中定義共用路徑和建置步驟。完整選項請參閱[附註解的範例](examples/zapp.yaml)。

### 選擇設定檔

`dmg`、`pkg` 和 `dep` 會從目前目錄逐級向上尋找 `.zapp.yaml`。

```sh
zapp dmg                              # 自動尋找 .zapp.yaml
zapp dmg --config release/.zapp.yaml   # 選擇設定檔
zapp dmg --no-config --app MyApp.app   # 不使用設定檔
zapp config show                      # 檢視生效的設定
```

`zapp init` 建立初始設定檔。覆蓋已有檔案需要 `--force`。

### 覆蓋設定值

同一個選項在多處設定時，左側來源的優先順序更高：

**CLI 參數 → `ZAPP_*` 環境變數 → 設定檔 → 預設值**

環境變數名由參數名轉為大寫、將連字元替換為底線，再加上 `ZAPP_` 前綴得到。

| CLI 參數 | 環境變數 | 範例值 |
| --- | --- | --- |
| `--app` | `ZAPP_APP` | `dist/MyApp.app` |
| `--title` | `ZAPP_TITLE` | `MyApp` |
| `--out` | `ZAPP_OUT` | `dist/MyApp.dmg`（用於 `dmg` 命令） |
| `--window-width` | `ZAPP_WINDOW_WIDTH` | `720` |
| `--libs` | `ZAPP_LIBS` | `/usr/local/lib,/opt/homebrew/lib` |

### 路徑基準

| 路徑的設定位置 | 相對路徑的基準 |
| --- | --- |
| 設定檔 | 設定檔所在的目錄 |
| CLI 參數或環境變數 | 目前工作目錄 |

頂層 `out` 是**輸出目錄**；`dmg.out` 和 `pkg.out` 是**輸出檔案路徑**。

```yaml
version: 1
app: dist/MyApp.app
out: dist

dmg:
  out: dist/MyApp-installer.dmg
pkg:
  out: dist/MyApp-installer.pkg
```

### 使用變數

字串值和 `contents` 的鍵支援以下變數：

| 變數 | 值 |
| --- | --- |
| `${app}` | 應用程式路徑 |
| `${app.name}` | 應用程式名稱 |
| `${app.version}` | 應用程式版本 |
| `${env:NAME}` | 環境變數 `NAME` 的值；未設定時會回報錯誤 |

```yaml
dmg:
  title: ${app.name}
  out: dist/${app.name}-${app.version}.dmg
```

### 啟用簽章和公證

新增 `sign:` 或 `notarize:` 設定段即可啟用對應步驟。以下範例使用 macOS 鑰匙圈設定：

```yaml
sign:
  identity: ${env:ZAPP_IDENTITY}
notarize:
  profile: my-profile
  staple: true
```

- 使用 `--no-sign` 或 `--no-notarize` 可在某次執行中跳過相應步驟。
- 原有的 `--sign --notarize --profile ... --staple` 參數仍然可用。
- 密碼透過 `ZAPP_P12_PASSWORD`、`ZAPP_PASSWORD` 環境變數或對應的 CLI 參數傳入。設定檔中不能包含 `sign.p12Password` 或 `notarize.password`。
- `zapp config show` 不會顯示密碼。

各平台的憑證和認證方式請參閱[簽章與公證說明](docs/signing.md)。

### 高階設定

<details>
<summary>PKG 設定形式</summary>

| 形式 | 用途和欄位 |
| --- | --- |
| 簡寫形式 | 封裝單一應用程式，支援 `identifier`、`version`、`installLocation`、`scripts`、`minOS` 和 `license` |
| 完整形式 | 定義多個 `components` 和一個 `distribution`，透過 `choices` 設定可選安裝項 |

簡寫形式與完整形式的欄位不能混用。

- 識別碼和版本預設取自應用程式的 Info.plist。
- `license` 接受檔案路徑或語言對應表，例如 `{default: license.txt, en: license-en.txt}`。
- `type: component` 只產生單一元件，不包含產品安裝介面。

詳情參閱 [PKG 引擎文件](pkg/macpkg/README.md)。

</details>

<details>
<summary>遷移舊版 DMG 設定</summary>

舊的平面 DMG 設定仍可使用，但會顯示棄用警告：

```sh
zapp dmg --config examples/dmg/layout.yaml --out release.dmg
```

此格式的輸出路徑仍以工作目錄為基準。遷移到專案設定時，將配置欄位移到 `dmg:` 下，並將共用的 `app` 保留在頂層。

</details>

## 📖 使用方法

`dep`、`dmg` 和 `pkg` 支援 `--sign --notarize --profile "profile" --staple`。啟用簽章後，Zapp 會在封裝前對應用程式簽章，並對產生的安裝套件簽章。公證和票據附加操作作用於最終產物。下面的鑰匙圈範例適用於 macOS；Linux/Windows 的憑證和 API 金鑰請參閱[簽章與公證說明](docs/signing.md)。

### 🔏 程式碼簽章

> [!TIP]
>
> 如果沒有透過 `--identity` 指定憑證，Zapp 會從目前鑰匙圈中自動選擇可用的憑證。

```bash
zapp sign --target="path/to/MyApp.app"
```

```bash
zapp sign --identity="Developer ID Application" --target="path/to/MyApp.app"
```

### 🏷️ 公證與附加票據

> [!NOTE]
>
> 向 `notarize` 傳入應用程式套件路徑時，Zapp 會自動壓縮應用程式套件並提交公證。

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/MyApp.app" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/MyApp.app" --staple
```

### 🔗 相依項目封裝

> [!NOTE]
>
> 檢查應用程式可執行檔案的相依項目，將所需程式庫放入 `/Contents/Frameworks`，並修改連結路徑，使應用程式能夠獨立執行。

```bash
zapp dep --app="path/to/target.app"
```

#### 額外的程式庫搜尋路徑

```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```

#### 同時簽章、公證並附加票據

```bash
zapp dep --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 💽 建立 DMG 檔案

Zapp 會從應用程式套件中擷取圖示、合成磁碟圖示，並建立支援拖放安裝的 DMG 檔案。

DMG 使用純 Go 實現的 [macfs](pkg/macfs)、[udif](pkg/udif) 和 [lzfse](pkg/lzfse) 套件產生，無需 `hdiutil`。直接建置完整映像檔，因此建置過程中不會掛載磁碟區。簽章和公證使用現有後端。

預設檔案系統為 HFS+。使用 `--fs apfs` 選擇 APFS，或用 `--fs apfs-case-sensitive` 區分 `App` 和 `app` 這樣的名稱。兩種 APFS 模式都可以在 Linux 或 macOS 上產生，開啟時需要 macOS 10.13 或更高版本。它們建立單一未加密的磁碟區，不支援快照或編輯已有映像檔。

保留輸入檔案的 FinderInfo 和資源分支（resource fork）。Linux 上使用 `user.com.apple.*` 命名空間。磁碟區內始終包含圖示；如果 Linux 的延伸屬性大小或支援情況不允許儲存，則跳過為主機上的 `.dmg` 檔案附加圖示。

```bash
zapp dmg --app="MyApp.app" --fs apfs
zapp dmg --app="MyApp.app" --fs apfs-case-sensitive --format ulfo
```

使用 `--format ulfo` 可將壓縮方式從 zlib 改為 LZFSE，得到更小的映像檔，但需要 macOS 10.11 或更高版本才能讀取。預設的 `udzo` 配合 HFS+ 可由所有 macOS 版本讀取。選擇 APFS 時，無論壓縮格式如何，都需要 macOS 10.13 或更高版本。

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

`--icon` 支援 ICNS 和 PNG。PNG 轉換為 ICNS 時會保留寬高比和透明度。使用 `--out MyApp` 省略副檔名時，會產生 `MyApp.dmg`，簽章和公證也使用該路徑。

#### 自訂配置

在 `.zapp.yaml` 的 `dmg.contents` 中指定檔案和圖示位置。以下範例放置應用程式、Applications 連結和使用指南。

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

**項目欄位**

`contents` 的鍵是來源路徑。明確設定 `contents` 會替換預設的應用程式與 Applications 配置，因此請包含所有需要的項目。

| 欄位 | 說明 |
| --- | --- |
| `pos: [x, y]` | 以 Finder 內容區域左上角為原點的圖示中心座標，必須大於或等於 0 |
| `link` | `true` 表示建立符號連結；省略時複製檔案或目錄 |
| `name` | 映像檔內的名稱；預設使用來源名稱 |

檔案和目錄路徑以設定檔為基準，連結目標按原樣保留。CLI 的 `--out` 路徑以工作目錄為基準。

**僅調整預設圖示位置**

如果只需移動應用程式與 Applications 連結的位置，不要設定 `contents`：

```sh
zapp dmg --app MyApp.app \
  --app-position 180,200 \
  --applications-position 540,200
```

<details>
<summary>額外選項與限制</summary>

- `--app-position` 和 `--applications-position` 不能與明確的 `contents` 一起使用。
- 沒有 `contents` 時必須指定 `app`。有 `contents` 時，`app` 用於取得預設標題和磁碟圖示。省略 `app` 時請設定 `dmg.title`。
- `dmg.icon` 可指定 ICNS 或 PNG 影像，`dmg.background` 可指定背景圖。檔案系統、壓縮、簽章和公證選項也支援自訂配置。
- 必須指定 `version: 1`。未知欄位和重複鍵會被拒絕。
- 映像檔內的名稱在忽略大小寫並進行 Unicode 正規化後必須唯一，且不能使用 DMG 後設資料保留名稱。
- 如果配置超過 `.DS_Store` 容量，請減少項目數或縮短名稱。

完整設定請參閱[專案範例](examples/zapp.yaml)，舊的平面配置請參閱[遷移說明](#高階設定)。

</details>

#### 同時簽章、公證並附加票據

```bash
zapp dmg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 📦 建立 PKG 檔案

PKG 使用純 Go 實現的 [macpkg 套件](pkg/macpkg/README.md)產生，無需 `pkgbuild` 或 `productbuild`。簽章和公證使用現有後端。

> [!TIP]
>
> 如果沒有指定 `--version` 和 `--identifier`，將從應用程式套件的 Info.plist 中取得對應值。

#### 從應用程式套件建立 PKG

```bash
zapp pkg --app="path/to/target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="path/to/target.app"
```

#### 新增 EULA 檔案

可以包含多種語言的使用者授權合約（EULA）：

```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="path/to/target.app"
```

#### 同時簽章、公證並附加票據

```bash
zapp pkg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 📝 編輯 Info.plist

`zapp plist` 可讀寫 `.plist` 檔案或 `.app` 中的 Info.plist。寫入時保留原格式，因此二進位 Info.plist 仍為二進位。修改已簽章的應用程式後需要重新簽章，請在簽章前完成 plist 編輯。

```bash
zapp plist get "path/to/target.app" CFBundleVersion
zapp plist set "path/to/target.app" CFBundleVersion 1.2.3
zapp plist delete "path/to/target.app" LSUIElement
```

現有鍵會保留原來的型別，因此布林值仍為布林值：

```bash
# 寫入布林值 <false/>；字串 "false" 會被 macOS 當作 true
zapp plist set "path/to/target.app" LSUIElement false
```

新鍵的值如果是 `true` 或 `false`，會儲存為布林值；整數儲存為整數型別，其他值儲存為字串。因此 `1.0` 這樣的版本號仍為字串。

巢狀值使用點分隔的路徑存取。如果鍵本身含有點號（例如權限鍵），會先嘗試完整比對鍵名，再將其解析為路徑。

```bash
zapp plist get "path/to/target.app" NSAppTransportSecurity.NSAllowsArbitraryLoads
zapp plist get "entitlements.plist" com.apple.security.app-sandbox
```

#### 遞增版本

```bash
zapp plist bump "path/to/target.app"                 # 1.4.2 -> 1.4.3
zapp plist bump "path/to/target.app" --minor         # 1.4.2 -> 1.5.0
zapp plist bump "path/to/target.app" --major         # 1.4.2 -> 2.0.0
zapp plist bump "path/to/target.app" --key CFBundleShortVersionString
```

不帶參數時遞增最後一段，適合更新建置號。`--major`、`--minor` 和 `--patch` 會遞增指定部分，並將後面的部分重設為 0。

### 完整範例

以下範例對 `MyApp.app` 進行相依項目封裝、程式碼簽章、安裝套件建立、公證和票據附加：

```bash
# 相依項目封裝
zapp dep --app="MyApp.app"

# 程式碼簽章、公證與附加票據
zapp sign --target="MyApp.app"
zapp notarize --profile="key-chain-profile" --target="MyApp.app" --staple

# 建立 PKG/DMG 檔案
zapp pkg --app="MyApp.app" --out="MyApp.pkg"
zapp dmg --app="MyApp.app" --out="MyApp.dmg"

# 為 PKG/DMG 簽章、公證並附加票據
zapp sign --target="MyApp.dmg"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
zapp notarize --profile="key-chain-profile" --target="MyApp.dmg" --staple
```

也可以使用以下簡寫命令：

```bash
zapp dep --app="MyApp.app" --sign --notarize --profile="key-chain-profile" --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple
```

## Go 程式庫

匯入 `github.com/ironpark/zapp` 即可載入專案設定並產生 DMG/PKG 安裝套件。

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

也可以直接建立 `zapp.Project`，或透過 `BuildDMG`、`BuildPKG` 等方法執行單獨操作。輸出路徑可從 `artifacts.DMG` 和 `artifacts.PKG` 取得。

## 授權條款

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zapp 基於 [MIT License](LICENSE) 發布。

## 支援

如有問題，請在 [GitHub issue tracker](https://github.com/ironpark/zapp/issues) 提交 Issue。
