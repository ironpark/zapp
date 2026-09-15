# ZAPP
[![FOSSA 状态](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go 报告卡](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![GitHub 仓库星标数](https://img.shields.io/github/stars/ironpark/zapp)](https://github.com/ironpark/zapp/stargazers)


🌐 [English](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) | [**简体中文**](README.zh-cn.md) | [繁體中文](README.zh-tw.md)

![asd](/docs/demo.gif)

**简化你的 macOS 应用部署**

`zapp` 是一个强大的 CLI 工具，旨在简化和自动化 macOS 应用程序的部署流程。它在一个工具中处理所有部署阶段，从依赖捆绑到 DMG/PKG 创建、代码签名和公证。

## ✨ 功能

- [x] 创建 DMG 文件
- [x] 创建 PKG 文件
- [x] 代码签名
- [x] 公证 / 附加
- [x] 修改 plist（版本）
- [x] 自动捆绑二进制依赖
- [ ] 支持 GitHub Actions

## ⚡️ 快速入门
#### 🍺 使用 Homebrew
```bash
brew tap ironpark/zapp
brew install zapp
```

#### 🛠️ 从源代码构建

```bash
go install github.com/ironpark/zapp/cmd/zapp@latest
```

## 📖 使用方法
### 🔏 代码签名

> [!TIP]
>
> 如果未使用 `--identity` 标志选择证书，Zapp 会自动从当前钥匙串中选择一个可用的证书。

```bash
zapp sign --target="目标路径.(app,dmg,pkg)"
```
```bash
zapp sign --identity="开发者 ID 应用程序" --target="目标路径.(app,dmg,pkg)"
```

### 🏷️ 公证与附加
> [!NOTE]
>
> 执行 notarize 命令时，如果 Zapp 收到应用包路径，它会自动压缩应用包并尝试进行公证。

```bash
zapp notarize --profile="钥匙串配置文件" --target="目标路径.(app,dmg,pkg)" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="目标路径.(app,dmg,pkg)" --staple
```

### 🔗 依赖捆绑
> [!NOTE]
> 
> 此过程会检查应用程序可执行文件的依赖项，将必要的库包含在 `/Contents/Frameworks` 中，并修改链接路径以实现独立运行。

```bash
zapp dep --app="目标路径.target.app"
```
#### 指定额外的库搜索路径
```bash
zapp dep --app="目标路径.target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```
#### 捆绑后自动签名、公证和附加
> [!TIP]
>
> `dep`、`dmg`、`pkg` 命令可以与 `--sign`、`--notarize` 和 `--staple` 标志一起使用。
> - `--sign` 标志会在捆绑依赖项后自动对应用包进行签名。
> - `--notarize` 标志会在签名后自动对应用包进行公证。

```bash
zapp dep --app="目标路径.target.app" --sign --notarize --profile "配置文件" --staple
```

### 💽 创建 DMG 文件

> Zapp 可用于创建 DMG 文件，这是分发 macOS 应用的常见格式。
它通过自动从应用包中提取图标、合成磁盘图标并提供拖放安装界面，大大简化了 DMG 创建过程。

```bash
zapp dmg --app="目标路径.target.app"
```

```bash
zapp dmg --title="我的应用" \ 
  --app="目标路径.target.app" \
  --icon="目标路径.icon.icns" \
  --bg="目标路径.background.png" \ 
  --out="MyApp.dmg"
```
#### 捆绑后自动签名、公证和附加
> [!TIP]
>
> `dep`、`dmg`、`pkg` 命令可以与 `--sign`、`--notarize` 和 `--staple` 标志一起使用。
> - `--sign` 标志会在捆绑依赖项后自动对应用包进行签名。
> - `--notarize` 标志会在签名后自动对应用包进行公证。

```bash
zapp dmg --app="目标路径.target.app" --sign --notarize --profile "配置文件" --staple
```

### 📦 创建 PKG 文件

> [!TIP]
> 
> 如果未设置 `--version` 和 `--identifier` 标志，这些值将从提供的应用包的 Info.plist 文件中自动获取

#### 从应用包创建 PKG 文件
```bash
zapp pkg --app="目标路径.target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="目标路径.target.app"
```

#### 包含多语言 EULA 文件
```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="目标路径.target.app" 
```
#### 捆绑后自动签名、公证和附加
> [!TIP]
>
> `dep`、`dmg`、`pkg` 命令可以与 `--sign`、`--notarize` 和 `--staple` 标志一起使用。
> - `--sign` 标志会在捆绑依赖项后自动对应用包进行签名。
> - `--notarize` 标志会在签名后自动对应用包进行公证。

```bash
zapp pkg --app="目标路径.target.app" --sign --notarize --profile "配置文件" --staple
```

### 完整示例
以下是一个完整示例，展示如何使用 `zapp` 对 `MyApp.app` 进行依赖捆绑、代码签名、打包、公证和附加：

```bash
# 依赖捆绑
zapp dep --app="MyApp.app"

# 代码签名 / 公证 / 附加
zapp sign --target="MyApp.app"
zapp notarize --profile="钥匙串配置文件" --target="MyApp.app" --staple

# 创建 pkg/dmg 文件
zapp pkg --app="MyApp.app" --out="MyApp.pkg"
zapp dmg --app="MyApp.app" --out="MyApp.dmg"

# 对 pkg/dmg 进行代码签名 / 公证 / 附加
zapp sign --target="MyApp.app"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="钥匙串配置文件" --target="MyApp.pkg" --staple
zapp notarize --profile="钥匙串配置文件" --target="MyApp.dmg" --staple
```
或者直接使用简写命令
```bash
zapp dep --app="MyApp.app" --sign --notarize --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \ 
  --sign --notarize --profile="配置文件" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="配置文件" --staple
```

## 许可证
[![FOSSA 状态](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zapp 根据 [MIT 许可证](LICENSE) 发布。

## 支持

如果你遇到任何问题或有疑问，请在 [GitHub 问题跟踪器](https://github.com/ironpark/zapp/issues) 中提交问题。

## 项目配置与库 API

使用一个 `.zapp.yaml` 配置依赖打包、DMG/PKG 创建、签名和公证。

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
