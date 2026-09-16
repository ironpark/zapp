# ZAPP

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![GitHub Repo stars](https://img.shields.io/github/stars/ironpark/zapp)](https://github.com/ironpark/zapp/stargazers)

🌐 [English](README.md) | [한국어](README.ko.md) | [**日本語**](README.ja.md) | [简体中文](README.zh-cn.md) | [繁體中文](README.zh-tw.md)

![Zapp で macOS アプリをパッケージ化するデモ](docs/demo.gif)

**macOS アプリの配布をシンプルに**

`zapp` は macOS アプリのパッケージ作成と配布に使える CLI・Go ライブラリです。一つの `.zapp.yaml` で依存関係のバンドル、DMG/PKG 作成、署名、公証を設定でき、必要な操作だけを個別のコマンドで実行することもできます。

[インストール](#インストール) · [クイックスタート](#クイックスタート) · [プロジェクト設定](#プロジェクト設定) · [使い方](#-使い方) · [Go ライブラリ](#go-ライブラリ)

## ✨ 機能

- [x] DMG ファイルの作成
- [x] PKG ファイルの作成
- [x] コード署名
- [x] 公証・チケットの添付（ステープル）
- [x] plist の編集・バージョン更新
- [x] バイナリの依存関係の自動バンドル
- [x] 宣言的なプロジェクト設定と Go ライブラリ API

## インストール

### スクリプトでインストール

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/scripts/install.ps1 | iex"
```

SHA-256 チェックサムを検証してから最新リリースをインストールします。macOS/Linux のインストール先は `~/.local/bin` です。案内が表示された場合は PATH に追加してください。Windows では `%LOCALAPPDATA%\zapp\bin` にインストールし、ユーザーの PATH に追加します。管理者権限は不要です。

バージョンを固定するには `ZAPP_VERSION` にリリースタグを指定します。インストール先を変更するには `ZAPP_INSTALL_DIR` に絶対パスを指定します。

### 🍺 Homebrew を使用

```bash
brew tap ironpark/zapp
brew install --cask zapp
```

### 🛠️ ソースコードからビルド

```bash
go install github.com/ironpark/zapp/cmd/zapp@latest
```

ソースからのビルドには Go 1.27.0 以降が必要です。Linux・Windows（amd64/arm64）用のビルド済み署名ライブラリを同梱しているため、別途ダウンロードしたり Rust をビルドしたりする必要はありません。cgo を有効にし、Linux では GCC、Windows では LLVM MinGW を用意してください。詳細は[プラットフォーム別のビルド手順](docs/signing.md#building)を参照してください。

## クイックスタート

ビルド済みの `dist/MyApp.app` があるディレクトリで実行します。Zapp はアプリのソースをコンパイルしません。`init` は DMG と PKG の設定を作成し、`build` は既定で両方のインストーラを `dist` に出力します。

依存関係のバンドル、DMG/PKG 作成、署名、公証を一つの `.zapp.yaml` で設定します。

```sh
zapp init --app dist/MyApp.app
zapp config show
zapp build                  # 設定した手順を実行
zapp build dmg pkg          # dep を省略。設定済みの署名・公証は実行
zapp dmg --title "MyApp"     # プロジェクトのタイトルを上書き
zapp pkg --no-sign --no-notarize
```

生成される設定は次のような形式です。`dep`、`sign`、`notarize` を追加すると、対応する手順も有効になります。

```yaml
version: 1
app: dist/MyApp.app
out: dist
# dep: {libs: [/opt/homebrew/lib]}
dmg: {}
pkg: {}
```

## プロジェクト設定

`.zapp.yaml` に共通のパスと実行する手順を定義します。すべての設定項目は[注釈付きの例](examples/zapp.yaml)を参照してください。

### 設定ファイルの選択

`dmg`、`pkg`、`dep` は現在のディレクトリから上位ディレクトリへ `.zapp.yaml` を探します。

```sh
zapp dmg                              # .zapp.yaml を自動検索
zapp dmg --config release/.zapp.yaml   # 設定ファイルを指定
zapp dmg --no-config --app MyApp.app   # 設定ファイルを使わずに実行
zapp config show                      # 適用される設定を確認
```

`zapp init` は初期設定ファイルを作成します。既存ファイルの上書きには `--force` が必要です。

### 値の上書き

同じ設定を複数の場所で指定した場合、左側の指定を優先します。

**CLI フラグ → `ZAPP_*` 環境変数 → 設定ファイル → 既定値**

環境変数名は、フラグ名を大文字にし、ハイフンをアンダースコアに置き換え、先頭に `ZAPP_` を付けたものです。

| CLI フラグ | 環境変数 | 値の例 |
| --- | --- | --- |
| `--app` | `ZAPP_APP` | `dist/MyApp.app` |
| `--title` | `ZAPP_TITLE` | `MyApp` |
| `--out` | `ZAPP_OUT` | `dist/MyApp.dmg`（`dmg` コマンドの場合） |
| `--window-width` | `ZAPP_WINDOW_WIDTH` | `720` |
| `--libs` | `ZAPP_LIBS` | `/usr/local/lib,/opt/homebrew/lib` |

### パスの基準

| パスの指定場所 | 相対パスの基準 |
| --- | --- |
| 設定ファイル | 設定ファイルがあるディレクトリ |
| CLI フラグ・環境変数 | 現在の作業ディレクトリ |

最上位の `out` は**出力ディレクトリ**、`dmg.out` と `pkg.out` は**出力ファイルのパス**です。

```yaml
version: 1
app: dist/MyApp.app
out: dist

dmg:
  out: dist/MyApp-installer.dmg
pkg:
  out: dist/MyApp-installer.pkg
```

### 変数の使用

文字列の値と `contents` のキーには、次の変数を使えます。

| 変数 | 値 |
| --- | --- |
| `${app}` | アプリのパス |
| `${app.name}` | アプリ名 |
| `${app.version}` | アプリのバージョン |
| `${env:NAME}` | 環境変数 `NAME` の値。未設定の場合はエラー |

```yaml
dmg:
  title: ${app.name}
  out: dist/${app.name}-${app.version}.dmg
```

### 署名と公証の有効化

`sign:` または `notarize:` セクションを追加すると、その手順を有効にします。次の例は macOS のキーチェーンプロファイルを使用します。

```yaml
sign:
  identity: ${env:ZAPP_IDENTITY}
notarize:
  profile: my-profile
  staple: true
```

- 特定の実行で手順を省略するには `--no-sign`、`--no-notarize` を使います。
- 既存の `--sign --notarize --profile ... --staple` フラグも使えます。
- パスワードは `ZAPP_P12_PASSWORD`、`ZAPP_PASSWORD` 環境変数、または対応する CLI フラグで渡します。設定ファイルに `sign.p12Password`、`notarize.password` は記述できません。
- `zapp config show` はパスワードを表示しません。

プラットフォームごとの証明書と認証方法は[署名・公証の説明](docs/signing.md)を参照してください。

### 詳細設定

<details>
<summary>PKG の設定形式</summary>

| 形式 | 用途とフィールド |
| --- | --- |
| 短縮形 | 一つのアプリをパッケージ化。`identifier`、`version`、`installLocation`、`scripts`、`minOS`、`license` に対応 |
| 完全形 | 複数の `components` と `distribution` を定義し、`choices` で選択項目を構成 |

短縮形と完全形のフィールドは混在させられません。

- 識別子とバージョンは、既定でアプリの Info.plist から取得します。
- `license` にはファイルのパス、または `{default: license.txt, en: license-en.txt}` のような言語別のマッピングを指定できます。
- `type: component` は製品インストーラの画面を持たない単一コンポーネントを作成します。

詳細は [PKG エンジンのドキュメント](pkg/macpkg/README.md)を参照してください。

</details>

<details>
<summary>従来の DMG 設定からの移行</summary>

フラットな DMG 設定も引き続き使えますが、非推奨の警告が表示されます。

```sh
zapp dmg --config examples/dmg/layout.yaml --out release.dmg
```

この形式の出力パスは、従来どおり作業ディレクトリを基準とします。プロジェクト設定に移行するには、レイアウトのフィールドを `dmg:` の下へ移し、共通の `app` を最上位に残してください。

</details>

## 📖 使い方

`dep`、`dmg`、`pkg` は `--sign --notarize --profile "profile" --staple` に対応しています。署名を有効にすると、パッケージ作成前にアプリへ署名し、生成したインストーラにも署名します。公証とステープルは生成物に適用します。以下のキーチェーンの例は macOS 用です。Linux・Windows の証明書と API キーについては[署名・公証の説明](docs/signing.md)を参照してください。

### 🔏 コード署名

> [!TIP]
>
> `--identity` で証明書を指定しない場合、Zapp は現在のキーチェーンから利用可能な証明書を自動的に選択します。

```bash
zapp sign --target="path/to/MyApp.app"
```

```bash
zapp sign --identity="Developer ID Application" --target="path/to/MyApp.app"
```

### 🏷️ 公証とステープル

> [!NOTE]
>
> `notarize` にアプリバンドルのパスを渡すと、自動的に圧縮して公証を申請します。

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/MyApp.app" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/MyApp.app" --staple
```

### 🔗 依存関係のバンドル

> [!NOTE]
>
> アプリの実行ファイルの依存関係を検査し、必要なライブラリを `/Contents/Frameworks` に含め、単体で実行できるようにリンクパスを変更します。

```bash
zapp dep --app="path/to/target.app"
```

#### ライブラリの追加検索パス

```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```

#### 署名・公証・ステープルを実行

```bash
zapp dep --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 💽 DMG ファイルの作成

Zapp は、アプリバンドルからアイコンを抽出し、ディスクアイコンを合成して、ドラッグ＆ドロップでインストールできる DMG を作成します。

DMG は [macfs](pkg/macfs)、[udif](pkg/udif)、[lzfse](pkg/lzfse) を使い、Go だけで生成します。`hdiutil` は不要です。イメージ全体を直接組み立てるため、ビルド中にボリュームをマウントしません。署名と公証には既存のバックエンドを使用します。

既定のファイルシステムは HFS+ です。APFS には `--fs apfs`、`App` と `app` のように大文字・小文字を区別するには `--fs apfs-case-sensitive` を使います。どちらも Linux または macOS で生成でき、開くには macOS 10.13 以降が必要です。暗号化されていない単一ボリュームを作成します。スナップショットや既存イメージの編集には対応していません。

入力ファイルの FinderInfo とリソースフォークを保持します。Linux では `user.com.apple.*` 名前空間を使います。ボリューム内には常にアイコンを含めますが、Linux の拡張属性の容量やサポート状況により保存できない場合、ホスト側の `.dmg` ファイルへのアイコン設定は省略します。

```bash
zapp dmg --app="MyApp.app" --fs apfs
zapp dmg --app="MyApp.app" --fs apfs-case-sensitive --format ulfo
```

`--format ulfo` を指定すると、zlib の代わりに LZFSE で圧縮してイメージを小さくできます。読み取りには macOS 10.11 以降が必要です。既定の `udzo` と HFS+ の組み合わせは、すべての macOS バージョンで読み取れます。APFS を選んだ場合は、圧縮形式にかかわらず macOS 10.13 以降が必要です。

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

`--icon` は ICNS と PNG に対応します。PNG は縦横比と透明度を保持して ICNS に変換します。`--out MyApp` のように拡張子を省略すると `MyApp.dmg` を作成し、署名と公証も同じパスに対して実行します。

#### カスタムレイアウト

`.zapp.yaml` の `dmg.contents` にファイルとアイコンの位置を指定します。次の例では、アプリ、Applications へのリンク、ガイドを配置します。

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
zapp dmg --config .zapp.yaml --out dist/MyApp.dmg
```

**項目の設定**

`contents` のキーは元のパスです。`contents` は既定のアプリ・Applications の配置を置き換えるため、必要な項目をすべて含めてください。

| フィールド | 説明 |
| --- | --- |
| `x`, `y` | Finder のコンテンツ領域の左上を基準とするアイコン中心の座標。0 以上 |
| `link` | `true` でシンボリックリンクを作成。省略するとファイル・ディレクトリをコピー |
| `name` | イメージ内での名前。省略すると元の名前を使用 |

ファイルとディレクトリのパスは設定ファイルを基準とします。リンク先は入力したとおりに保持します。CLI の `--out` は作業ディレクトリを基準とします。

**既定のアイコン位置だけを変更**

アプリと Applications リンクの位置だけを変更するには、`contents` を指定せずに実行します。

```sh
zapp dmg --app MyApp.app \
  --app-position 180,200 \
  --applications-position 540,200
```

<details>
<summary>追加オプションと制約</summary>

- `--app-position` と `--applications-position` は、明示的な `contents` と併用できません。
- `contents` がなければ `app` が必要です。`contents` がある場合、`app` から既定のタイトルとディスクアイコンを取得します。`app` を省略する場合は `dmg.title` を指定してください。
- `dmg.icon` に ICNS・PNG、`dmg.background` に背景画像を指定できます。ファイルシステム、圧縮、署名、公証のオプションも使えます。
- `version: 1` を指定してください。不明なフィールドや重複したキーはエラーになります。
- イメージ内の名前は、大文字・小文字と Unicode 正規化を考慮して重複してはいけません。DMG メタデータの予約名も使えません。
- `.DS_Store` の容量を超える場合は、項目数を減らすか名前を短くしてください。

設定全体は[プロジェクトの例](examples/zapp.yaml)、従来のフラットな形式については[移行手順](#詳細設定)を参照してください。

</details>

#### 署名・公証・ステープルを実行

```bash
zapp dmg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 📦 PKG ファイルの作成

PKG は Go 製の [macpkg パッケージ](pkg/macpkg/README.md)で生成します。`pkgbuild` や `productbuild` は不要です。署名と公証には既存のバックエンドを使用します。

> [!TIP]
>
> `--version` と `--identifier` を指定しない場合、アプリバンドルの Info.plist から値を取得します。

#### アプリバンドルから PKG を作成

```bash
zapp pkg --app="path/to/target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="path/to/target.app"
```

#### EULA ファイルを追加

複数の言語のエンドユーザー使用許諾契約（EULA）を含められます。

```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="path/to/target.app"
```

#### 署名・公証・ステープルを実行

```bash
zapp pkg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 📝 Info.plist の編集

`zapp plist` は `.plist` ファイル、または `.app` 内の Info.plist を読み書きします。元の形式を保持するため、バイナリ形式の Info.plist はバイナリのままです。署名済みのアプリを変更した場合は再署名が必要なので、plist の編集は署名前に行ってください。

```bash
zapp plist get "path/to/target.app" CFBundleVersion
zapp plist set "path/to/target.app" CFBundleVersion 1.2.3
zapp plist delete "path/to/target.app" LSUIElement
```

既存の値の型を保持するため、真偽値は真偽値のままです。

```bash
# 真偽値の <false/> を書き込む。文字列 "false" は macOS では true と解釈される
zapp plist set "path/to/target.app" LSUIElement false
```

新しいキーでは `true`・`false` は真偽値、整数は整数型、それ以外は文字列になります。`1.0` のようなバージョンは文字列のままです。

入れ子の値はドット区切りのパスで指定します。エンタイトルメントのキーのように名前自体にドットを含む場合は、パスとして解釈する前にキー全体の一致を確認します。

```bash
zapp plist get "path/to/target.app" NSAppTransportSecurity.NSAllowsArbitraryLoads
zapp plist get "entitlements.plist" com.apple.security.app-sandbox
```

#### バージョンを上げる

```bash
zapp plist bump "path/to/target.app"                 # 1.4.2 -> 1.4.3
zapp plist bump "path/to/target.app" --minor         # 1.4.2 -> 1.5.0
zapp plist bump "path/to/target.app" --major         # 1.4.2 -> 2.0.0
zapp plist bump "path/to/target.app" --key CFBundleShortVersionString
```

フラグを省略すると、ビルド番号に適した最後の要素を増やします。`--major`、`--minor`、`--patch` は指定した要素を増やし、それ以降の要素を 0 に戻します。

### 一連の実行例

`MyApp.app` の依存関係のバンドル、署名、パッケージ作成、公証、ステープルを実行する例です。

```bash
# 依存関係のバンドル
zapp dep --app="MyApp.app"

# コード署名・公証・ステープル
zapp sign --target="MyApp.app"
zapp notarize --profile="key-chain-profile" --target="MyApp.app" --staple

# PKG・DMG を作成
zapp pkg --app="MyApp.app" --out="MyApp.pkg"
zapp dmg --app="MyApp.app" --out="MyApp.dmg"

# PKG・DMG の署名・公証・ステープル
zapp sign --target="MyApp.dmg"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
zapp notarize --profile="key-chain-profile" --target="MyApp.dmg" --staple
```

次のように短縮して実行することもできます。

```bash
zapp dep --app="MyApp.app" --sign --notarize --profile="key-chain-profile" --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple
```

## Go ライブラリ

`github.com/ironpark/zapp` をインポートして、設定を読み込み、DMG・PKG を作成します。

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

`zapp.Project` を直接構成したり、`BuildDMG`、`BuildPKG` などで個別の操作を実行したりすることもできます。出力パスは `artifacts.DMG`、`artifacts.PKG` で確認できます。

## ライセンス

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zapp は [MIT License](LICENSE) の下で公開されています。

## サポート

問題や質問がある場合は、[GitHub の Issue](https://github.com/ironpark/zapp/issues) でお知らせください。
