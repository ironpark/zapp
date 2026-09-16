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
brew install --cask zapp
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
zapp build dmg pkg          # パッケージ手順だけを選んで実行
zapp dmg --title "MyApp"    # プロジェクトのタイトルを上書き
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

`dmg`、`pkg`、`dep` は作業ディレクトリから上位へさかのぼって `.zapp.yaml` を探します。`--config パス` でファイルを指定し、`--no-config` で設定ファイルを無視します。優先順位は **CLI フラグ > `ZAPP_*` 環境変数 > 設定ファイル > 既定値** です。フラグ名を大文字にしてハイフンをアンダースコアに置き換えたものが環境変数名になります。たとえば `ZAPP_APP`、`ZAPP_TITLE`、`ZAPP_OUT`、`ZAPP_WINDOW_WIDTH`、`ZAPP_LIBS`（カンマ区切り）です。設定ファイル内のパスは設定ファイルのあるディレクトリ基準、CLI と環境変数のパスは作業ディレクトリ基準です。最上位の `out` はディレクトリで、`dmg.out` と `pkg.out` はファイル名です。

`${env:NAME}`、`${app}`、`${app.name}`、`${app.version}` は文字列の値と `contents` のキーで使えます。未設定の環境変数を参照するとエラーになります。`sign:` と `notarize:` があるとその手順が自動的に実行され、`--no-sign` と `--no-notarize` で省略できます。既存の `--sign --notarize --profile ... --staple` を使ったスクリプトはそのまま動作します。パスワードのキー（`sign.p12Password`、`notarize.password`）は設定ファイルに書けません。`ZAPP_P12_PASSWORD` / `ZAPP_PASSWORD` か対応する CLI フラグで渡してください。`config show` はパスワードを出力しません。`init` は `--force` なしに既存ファイルを上書きしません。

PKG の短縮形は `identifier`、`version`、`installLocation`、`scripts`、`minOS`、`license`（パス、または `{default: パス, en: パス, ...}`）に対応します。識別子とバージョンは Info.plist から既定値を取得します。完全形は `components` と、選択可能な `choices` を持つ `distribution` に対応し、短縮形と完全形のフィールドを混在させることはできません。`type: component` は製品インストーラの画面を持たない単一コンポーネントを作成します。[注釈付きのプロジェクト例](examples/zapp.yaml)と [PKG エンジンのドキュメント](pkg/macpkg/README.md)を参照してください。

従来のフラットな DMG 設定ファイルも `zapp dmg --config examples/dmg/layout.yaml --out release.dmg` の形で引き続き使用できますが、非推奨の警告が表示されます。出力先も従来どおり作業ディレクトリ基準です。レイアウトの項目を `dmg:` の下へ移し、共通の `app` を最上位に置けば移行は完了です。

モジュールのルートをインポートできるようになり、実行ファイルは `cmd/zapp` へ移動しました。

```go
project, err := zapp.Load(".zapp.yaml")
if err != nil { return err }
project.DMG.Title = "MyApp"
plan, err := project.Resolve(zapp.WithClock(time.Unix(1700000000, 0)))
if err != nil { return err }
artifacts, err := plan.Build(ctx, zapp.StepDMG, zapp.StepPKG)
```

`github.com/ironpark/zapp` をインポートしてください。`zapp.Project` をコードから直接組み立てることもできます。`Plan.BundleDeps`、`BuildDMG`、`BuildPKG`、`Sign`、`Notarize` は各操作を個別に実行します。`WithLogger` を渡さない限りログは出力されません。失敗は `*zapp.StepError` でラップされ、`errors.As` / `errors.Is` に対応します。`WithClock` は生成される DMG のメタデータと PKG のタイムスタンプを固定します。ただし再現可能なイメージにするには、元のファイルとそのメタデータも同一である必要があります。
