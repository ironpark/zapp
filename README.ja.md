# ZAPP

[English](README.md) | [한국어](README.ko.md) | [**日本語**](README.ja.md)

**macOSアプリのデプロイメントを簡素化**

`ZAPP`は、macOSアプリケーションのデプロイメントプロセスを合理化するために設計された強力なCLIツールです。Zappを使用すると、DMGファイルやPKGファイルの作成、コード署名、アプリの公証、plistファイルの修正作業を簡単に行うことができます。

## 機能

- [x] DMGファイルの作成
- [x] PKGファイルの作成
- [x] コード署名
- [x] 公証 / ステープリング
  - [ ] 公証の再試行
- [ ] plistの修正（バージョン）
- [x] 自動バイナリ依存関係のバンドル
- [ ] GitHub Actionsのサポート

## インストール
Homebrewを使用
```bash
brew tap ironpark/zapp
brew install zapp
```
Goを使用
```bash
go install github.com/ironpark/zapp@latest
```

## 使用方法

### 完全な例
以下は、zappを使用して依存関係のバンドル、コード署名、パッケージング、公証、ステープリングを行う完全な例です：
```bash
zapp dep --app="MyApp.app"
zapp sign --target="MyApp.app"
zapp pkg --out="MyApp.pkg" --app="MyApp.app"
zapp sign --target="MyApp.pkg"
zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
```

### 依存関係のバンドル
dep コマンドは、アプリケーションが独立して実行できるように、アプリケーション実行ファイルの依存関係を検査し、必要なライブラリを `/Contents/Frameworks` 内にコピーしてリンクパスを修正します。
```bash
zapp dep --app="path/to/target.app"
```
追加でライブラリを検索するパスを指定することもできます。
```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```

### DMGファイルの作成

> Zappは、macOSアプリの配布によく使用されるDMGファイルの作成に使用できます。
アプリバンドルからアイコンを自動的に抽出し、ディスクアイコンを合成し、アプリをドラッグ＆ドロップでインストールできるインターフェースを提供することで、DMG作成プロセスを大幅に簡素化します。

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

### PKGファイルの作成
> `--version` と `--identifier` フラグが設定されていない場合、提供されたアプリバンドルのInfo.plistファイルからこれらの値が自動的に取得されます。

```bash
zapp pkg --app="path/to/target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="path/to/target.app"
```

#### EULAファイルの追加

複数の言語のエンドユーザーライセンス契約（EULA）ファイルを含めるには、`--eula` フラグに ',' で区切られた言語コードとファイルパスを渡します。

```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="path/to/target.app" 
```

### コード署名

証明書を選択する際、`--identity` フラグを使用しない場合、`zapp` は現在のキーチェーンで利用可能な証明書を自動的に選択します。
```bash
zapp sign --target="path/to/target.(app,dmg,pkg)"
```
```bash
zapp sign --identity="Developer ID Application" --target="path/to/target.(app,dmg,pkg)"
```

### 公証とステープリング
> 公証コマンド（notarize）を実行する際にアプリバンドルのパスを受け取ると、`zapp` は自動的にアプリバンドルを圧縮し、公証を試みます。

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/target.(app,dmg,pkg)" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/target.(app,dmg,pkg)" --staple
```

## 高度な使用方法

（TODO: 高度な使用例を追加）

## ライセンス

Zappは[MITライセンス](LICENSE)の下でリリースされています。

## サポート

問題が発生した場合や質問がある場合は、[GitHubのイシュートラッカー](https://github.com/ironpark/zapp/issues)に問題を報告してください。