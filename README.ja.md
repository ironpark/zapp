# ZAPP
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![codebeat badge](https://codebeat.co/badges/6b004587-036c-4324-bc97-c2e76d58b474)](https://codebeat.co/projects/github-com-ironpark-zapp-main)
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
- [ ] plistの変更（バージョン）
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
go install github.com/ironpark/zapp@latest
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
