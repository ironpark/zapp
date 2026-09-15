# ZAPP
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![GitHub Repo stars](https://img.shields.io/github/stars/ironpark/zapp)](https://github.com/ironpark/zapp/stargazers)

🌐 [English](README.md) | [한국어](README.ko.md) | [**日本語**](README.ja.md) | [简体中文](README.zh-cn.md) | [繁體中文](README.zh-tw.md)

![asd](/docs/demo.gif)

**macOSアプリのデプロイを簡素化**

`zapp`は、macOSアプリケーションのデプロイプロセスを効率化し、自動化するために設計された強力なCLIツールです。依存関係のバンドルからDMG/PKGの作成、コード署名、およびノータライズまで、デプロイのすべての段階を1つのツールで処理します。

## ✨ 特徴

- [x] DMGファイルの作成
- [x] PKGファイルの作成
- [x] コード署名
- [x] ノータライズ/ステープル
- [x] plistの変更（バージョン）
- [x] バイナリ依存関係の自動バンドル
- [ ] GitHub Actionsのサポート

## ⚡️ クイックスタート
#### 🍺 Homebrewを使用
```bash
brew tap ironpark/zapp
brew install zapp
```

#### 🛠️ ソースコードからビルド

```bash
go install github.com/ironpark/zapp/cmd/zapp@latest
```

## 📖 使い方
### 🔏 コード署名

> [!TIP]
>
> `--identity`フラグを使用して証明書を選択しない場合、Zappは現在のキーチェーンから利用可能な証明書を自動的に選択します。

```bash
zapp sign --target="path/to/target.(app,dmg,pkg)"
```
```bash
zapp sign --identity="Developer ID Application" --target="path/to/target.(app,dmg,pkg)"
```

### 🏷️ ノータライズとステープル
> [!NOTE]
>
> notarizeコマンドを実行する際、Zappがアプリバンドルのパスを受け取ると、自動的にアプリバンドルを圧縮し、ノータライズを試みます。

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/target.(app,dmg,pkg)" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/target.(app,dmg,pkg)" --staple
```

### 🔗 依存関係のバンドル
> [!NOTE]
> 
> このプロセスでは、アプリケーション実行ファイルの依存関係を検査し、必要なライブラリを`/Contents/Frameworks`内に含め、スタンドアロン実行を可能にするためにリンクパスを変更します。

```bash
zapp dep --app="path/to/target.app"
```
#### ライブラリを検索するための追加パス
```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```
#### 署名、ノータライズ、ステープル付き
> [!TIP]
>
> `dep`、`dmg`、`pkg`コマンドは、`--sign`、`--notarize`、および`--staple`フラグとともに使用できます。
> - `--sign`フラグは、依存関係のバンドル後にアプリバンドルを自動的に署名します。
> - `--notarize`フラグは、署名後にアプリバンドルを自動的にノータライズします。

```bash
zapp dep --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 💽 DMGファイルの作成

> Zappは、macOSアプリの配布によく使用される形式であるDMGファイルの作成に使用できます。
アプリバンドルからアイコンを自動的に抽出し、ディスクアイコンを合成し、アプリのドラッグアンドドロップインストール用のインターフェースを提供することで、DMG作成プロセスを大幅に簡素化します。

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
#### 署名、ノータライズ、ステープル付き
> [!TIP]
>
> `dep`、`dmg`、`pkg`コマンドは、`--sign`、`--notarize`、および`--staple`フラグとともに使用できます。
> - `--sign`フラグは、依存関係のバンドル後にアプリバンドルを自動的に署名します。
> - `--notarize`フラグは、署名後にアプリバンドルを自動的にノータライズします。

```bash
zapp dmg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```
### 📦 PKGファイルの作成

> [!TIP]
> 
> `--version`および`--identifier`フラグが設定されていない場合、これらの値は、提供されたアプリバンドルのInfo.plistファイルから自動的に取得されます。

#### アプリバンドルからPKGファイルを作成する
```bash
zapp pkg --app="path/to/target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="path/to/target.app"
```

#### EULAファイル付き

複数の言語でエンドユーザーライセンス契約（EULA）ファイルを含めます。

```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="path/to/target.app" 
```
#### 署名、ノータライズ、ステープル付き
> [!TIP]
>
> `dep`、`dmg`、`pkg`コマンドは、`--sign`、`--notarize`、および`--staple`フラグとともに使用できます。
> - `--sign`フラグは、依存関係のバンドル後にアプリバンドルを自動的に署名します。
> - `--notarize`フラグは、署名後にアプリバンドルを自動的にノータライズします。

```bash
zapp pkg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 完全な例
以下は、`zapp`を使用して、`MyApp.app`の依存関係のバンドル、コード署名、パッケージング、ノータライズ、およびステープルを行う方法を示す完全な例です。

```bash
# 依存関係のバンドル
zapp dep --app="MyApp.app"

# コード署名 / ノータライズ / ステープル
zapp sign --target="MyApp.app"
zapp notarize --profile="key-chain-profile" --target="MyApp.app" --staple

# pkg/dmgファイルの作成
zapp pkg --app="MyApp.app" --out="MyApp.pkg"
zapp dmg --app="MyApp.app" --out="MyApp.dmg"

# pkg/dmgのコード署名 / ノータライズ / ステープル
zapp sign --target="MyApp.app"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
zapp notarize --profile="key-chain-profile" --target="MyApp.dmg" --staple
```
または、短縮コマンドを使用するだけです
```bash
zapp dep --app="MyApp.app" --sign --notarize --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \ 
  --sign --notarize --profile="key-chain-profile" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple
```

## ライセンス
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zappは[MIT License](LICENSE)の下でリリースされています。

## サポート

問題が発生した場合や質問がある場合は、[GitHub issue tracker](https://github.com/ironpark/zapp/issues)にissueを提出してください。

## プロジェクト設定とライブラリ API

依存関係のバンドル、DMG/PKG 作成、署名、公証を一つの `.zapp.yaml` で設定します。

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
