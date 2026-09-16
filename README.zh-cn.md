# ZAPP

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![GitHub Repo stars](https://img.shields.io/github/stars/ironpark/zapp)](https://github.com/ironpark/zapp/stargazers)

🌐 [English](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) | [**简体中文**](README.zh-cn.md) | [繁體中文](README.zh-tw.md)

![使用 Zapp 打包 macOS 应用的演示](docs/demo.gif)

**简化 macOS 应用的发布**

`zapp` 是用于打包和发布 macOS 应用的 CLI 工具和 Go 库。通过一个 `.zapp.yaml` 配置依赖打包、DMG/PKG 创建、签名和公证，也可以按需运行单个命令。

[安装](#安装) · [快速入门](#快速入门) · [项目配置](#项目配置) · [使用方法](#-使用方法) · [Go 库](#go-库)

## ✨ 功能

- [x] 创建 DMG 文件
- [x] 创建 PKG 文件
- [x] 代码签名
- [x] 公证及附加公证票据（Stapling）
- [x] 编辑 plist 和更新版本
- [x] 自动打包二进制依赖
- [x] 声明式项目配置和 Go 库 API

## 安装

### 使用脚本安装

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/scripts/install.ps1 | iex"
```

通过 SHA-256 校验后安装最新发布的版本。macOS/Linux 的安装目录为 `~/.local/bin`，如有提示，请将其添加到 PATH。Windows 的安装目录为 `%LOCALAPPDATA%\zapp\bin`，脚本会将其添加到用户 PATH。无需管理员权限。

将 `ZAPP_VERSION` 设为发布标签可固定版本；将 `ZAPP_INSTALL_DIR` 设为绝对路径可指定安装目录。

### 🍺 使用 Homebrew

```bash
brew tap ironpark/zapp
brew install --cask zapp
```

### 🛠️ 从源代码构建

```bash
go install github.com/ironpark/zapp/cmd/zapp@latest
```

源码构建需要 Go 1.27.0 或更高版本。已包含 Linux 和 Windows（amd64/arm64）的预编译签名库，无需单独下载或编译 Rust。请启用 cgo，并准备 C 编译器：Linux 使用 GCC，Windows 使用 LLVM MinGW。详情参阅[各平台构建说明](docs/signing.md#building)。

## 快速入门

在包含已构建的 `dist/MyApp.app` 的目录中开始。Zapp 不会编译应用源码。`init` 生成 DMG 和 PKG 配置，`build` 默认将两种安装包输出到 `dist`。

用一个 `.zapp.yaml` 配置依赖打包、DMG/PKG 创建、签名和公证。

```sh
zapp init --app dist/MyApp.app
zapp config show
zapp build                  # 运行已配置的步骤
zapp build dmg pkg          # 跳过 dep，仍执行已配置的签名和公证
zapp dmg --title "MyApp"     # 覆盖项目标题
zapp pkg --no-sign --no-notarize
```

生成的配置如下。添加 `dep`、`sign` 或 `notarize` 可启用相应步骤。

```yaml
version: 1
app: dist/MyApp.app
out: dist
# dep: {libs: [/opt/homebrew/lib]}
dmg: {}
pkg: {}
```

## 项目配置

在 `.zapp.yaml` 中定义共用路径和构建步骤。完整选项请参阅[带注释的示例](examples/zapp.yaml)。

### 选择配置文件

`dmg`、`pkg` 和 `dep` 会从当前目录逐级向上查找 `.zapp.yaml`。

```sh
zapp dmg                              # 自动查找 .zapp.yaml
zapp dmg --config release/.zapp.yaml   # 选择配置文件
zapp dmg --no-config --app MyApp.app   # 不使用配置文件
zapp config show                      # 查看生效的配置
```

`zapp init` 创建初始配置文件。覆盖已有文件需要 `--force`。

### 覆盖配置值

同一个选项在多处设置时，左侧来源的优先级更高：

**CLI 参数 → `ZAPP_*` 环境变量 → 配置文件 → 默认值**

环境变量名由参数名转为大写、将连字符替换为下划线，再加上 `ZAPP_` 前缀得到。

| CLI 参数 | 环境变量 | 示例值 |
| --- | --- | --- |
| `--app` | `ZAPP_APP` | `dist/MyApp.app` |
| `--title` | `ZAPP_TITLE` | `MyApp` |
| `--out` | `ZAPP_OUT` | `dist/MyApp.dmg`（用于 `dmg` 命令） |
| `--window-width` | `ZAPP_WINDOW_WIDTH` | `720` |
| `--libs` | `ZAPP_LIBS` | `/usr/local/lib,/opt/homebrew/lib` |

### 路径基准

| 路径的设置位置 | 相对路径的基准 |
| --- | --- |
| 配置文件 | 配置文件所在的目录 |
| CLI 参数或环境变量 | 当前工作目录 |

顶层 `out` 是**输出目录**；`dmg.out` 和 `pkg.out` 是**输出文件路径**。

```yaml
version: 1
app: dist/MyApp.app
out: dist

dmg:
  out: dist/MyApp-installer.dmg
pkg:
  out: dist/MyApp-installer.pkg
```

### 使用变量

字符串值和 `contents` 的键支持以下变量：

| 变量 | 值 |
| --- | --- |
| `${app}` | 应用路径 |
| `${app.name}` | 应用名称 |
| `${app.version}` | 应用版本 |
| `${env:NAME}` | 环境变量 `NAME` 的值；未设置时会报错 |

```yaml
dmg:
  title: ${app.name}
  out: dist/${app.name}-${app.version}.dmg
```

### 启用签名和公证

添加 `sign:` 或 `notarize:` 配置段即可启用对应步骤。以下示例使用 macOS 钥匙串配置：

```yaml
sign:
  identity: ${env:ZAPP_IDENTITY}
notarize:
  profile: my-profile
  staple: true
```

- 使用 `--no-sign` 或 `--no-notarize` 可在某次运行中跳过相应步骤。
- 原有的 `--sign --notarize --profile ... --staple` 参数仍然可用。
- 密码通过 `ZAPP_P12_PASSWORD`、`ZAPP_PASSWORD` 环境变量或对应的 CLI 参数传入。配置文件中不能包含 `sign.p12Password` 或 `notarize.password`。
- `zapp config show` 不会显示密码。

各平台的证书和认证方式请参阅[签名与公证说明](docs/signing.md)。

### 高级配置

<details>
<summary>PKG 配置形式</summary>

| 形式 | 用途和字段 |
| --- | --- |
| 简写形式 | 打包单个应用，支持 `identifier`、`version`、`installLocation`、`scripts`、`minOS` 和 `license` |
| 完整形式 | 定义多个 `components` 和一个 `distribution`，通过 `choices` 配置可选安装项 |

简写形式与完整形式的字段不能混用。

- 标识符和版本默认取自应用的 Info.plist。
- `license` 接受文件路径或语言映射，例如 `{default: license.txt, en: license-en.txt}`。
- `type: component` 只生成单个组件，不包含产品安装界面。

详情参阅 [PKG 引擎文档](pkg/macpkg/README.md)。

</details>

<details>
<summary>迁移旧版 DMG 配置</summary>

旧的平面 DMG 配置仍可使用，但会显示弃用警告：

```sh
zapp dmg --config examples/dmg/layout.yaml --out release.dmg
```

此格式的输出路径仍以工作目录为基准。迁移到项目配置时，将布局字段移到 `dmg:` 下，并将共用的 `app` 保留在顶层。

</details>

## 📖 使用方法

`dep`、`dmg` 和 `pkg` 支持 `--sign --notarize --profile "profile" --staple`。启用签名后，Zapp 会在打包前签名应用，并签名生成的安装包。公证和票据附加操作作用于最终产物。下面的钥匙串示例适用于 macOS；Linux/Windows 的证书和 API 密钥请参阅[签名与公证说明](docs/signing.md)。

### 🔏 代码签名

> [!TIP]
>
> 如果没有通过 `--identity` 指定证书，Zapp 会从当前钥匙串中自动选择可用的证书。

```bash
zapp sign --target="path/to/MyApp.app"
```

```bash
zapp sign --identity="Developer ID Application" --target="path/to/MyApp.app"
```

### 🏷️ 公证与附加票据

> [!NOTE]
>
> 向 `notarize` 传入应用包路径时，Zapp 会自动压缩应用包并提交公证。

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/MyApp.app" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/MyApp.app" --staple
```

### 🔗 依赖打包

> [!NOTE]
>
> 检查应用可执行文件的依赖，将所需库放入 `/Contents/Frameworks`，并修改链接路径，使应用能够独立运行。

```bash
zapp dep --app="path/to/target.app"
```

#### 额外的库搜索路径

```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```

#### 同时签名、公证并附加票据

```bash
zapp dep --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 💽 创建 DMG 文件

Zapp 会从应用包中提取图标、合成磁盘图标，并创建支持拖放安装的 DMG 文件。

DMG 使用纯 Go 实现的 [macfs](pkg/macfs)、[udif](pkg/udif) 和 [lzfse](pkg/lzfse) 包生成，无需 `hdiutil`。直接构建完整镜像，因此构建过程中不会挂载卷。签名和公证使用现有后端。

默认文件系统为 HFS+。使用 `--fs apfs` 选择 APFS，或用 `--fs apfs-case-sensitive` 区分 `App` 和 `app` 这样的名称。两种 APFS 模式都可以在 Linux 或 macOS 上生成，打开时需要 macOS 10.13 或更高版本。它们创建单个未加密的卷，不支持快照或编辑已有镜像。

保留输入文件的 FinderInfo 和资源分支。Linux 上使用 `user.com.apple.*` 命名空间。卷内始终包含图标；如果 Linux 的扩展属性大小或支持情况不允许保存，则跳过为主机上的 `.dmg` 文件附加图标。

```bash
zapp dmg --app="MyApp.app" --fs apfs
zapp dmg --app="MyApp.app" --fs apfs-case-sensitive --format ulfo
```

使用 `--format ulfo` 可将压缩方式从 zlib 改为 LZFSE，得到更小的镜像，但需要 macOS 10.11 或更高版本才能读取。默认的 `udzo` 配合 HFS+ 可由所有 macOS 版本读取。选择 APFS 时，无论压缩格式如何，都需要 macOS 10.13 或更高版本。

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

`--icon` 支持 ICNS 和 PNG。PNG 转换为 ICNS 时会保留宽高比和透明度。使用 `--out MyApp` 省略扩展名时，会生成 `MyApp.dmg`，签名和公证也使用该路径。

#### 自定义布局

在 `.zapp.yaml` 的 `dmg.contents` 中指定文件和图标位置。以下示例放置应用、Applications 链接和使用指南。

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

**条目字段**

`contents` 的键是源路径。显式设置 `contents` 会替换默认的应用与 Applications 布局，因此请包含所有需要的条目。

| 字段 | 说明 |
| --- | --- |
| `pos: [x, y]` | 以 Finder 内容区域左上角为原点的图标中心坐标，必须大于或等于 0 |
| `link` | `true` 表示创建符号链接；省略时复制文件或目录 |
| `name` | 镜像内的名称；默认使用源名称 |

文件和目录路径以配置文件为基准，链接目标按原样保留。CLI 的 `--out` 路径以工作目录为基准。

**仅调整默认图标位置**

如果只需移动应用与 Applications 链接的位置，不要设置 `contents`：

```sh
zapp dmg --app MyApp.app \
  --app-position 180,200 \
  --applications-position 540,200
```

<details>
<summary>额外选项与限制</summary>

- `--app-position` 和 `--applications-position` 不能与显式的 `contents` 一起使用。
- 没有 `contents` 时必须指定 `app`。有 `contents` 时，`app` 用于获取默认标题和磁盘图标。省略 `app` 时请设置 `dmg.title`。
- `dmg.icon` 可指定 ICNS 或 PNG 图像，`dmg.background` 可指定背景图。文件系统、压缩、签名和公证选项也支持自定义布局。
- 必须指定 `version: 1`。未知字段和重复键会被拒绝。
- 镜像内的名称在忽略大小写并进行 Unicode 规范化后必须唯一，且不能使用 DMG 元数据保留名称。
- 如果布局超过 `.DS_Store` 容量，请减少条目数或缩短名称。

完整配置请参阅[项目示例](examples/zapp.yaml)，旧的平面布局请参阅[迁移说明](#高级配置)。

</details>

#### 同时签名、公证并附加票据

```bash
zapp dmg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 📦 创建 PKG 文件

PKG 使用纯 Go 实现的 [macpkg 包](pkg/macpkg/README.md)生成，无需 `pkgbuild` 或 `productbuild`。签名和公证使用现有后端。

> [!TIP]
>
> 如果没有指定 `--version` 和 `--identifier`，将从应用包的 Info.plist 中获取对应值。

#### 从应用包创建 PKG

```bash
zapp pkg --app="path/to/target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="path/to/target.app"
```

#### 添加 EULA 文件

可以包含多种语言的最终用户许可协议（EULA）：

```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="path/to/target.app"
```

#### 同时签名、公证并附加票据

```bash
zapp pkg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 📝 编辑 Info.plist

`zapp plist` 可读写 `.plist` 文件或 `.app` 中的 Info.plist。写入时保留原格式，因此二进制 Info.plist 仍为二进制。修改已签名的应用后需要重新签名，请在签名前完成 plist 编辑。

```bash
zapp plist get "path/to/target.app" CFBundleVersion
zapp plist set "path/to/target.app" CFBundleVersion 1.2.3
zapp plist delete "path/to/target.app" LSUIElement
```

现有键会保留原来的类型，因此布尔值仍为布尔值：

```bash
# 写入布尔值 <false/>；字符串 "false" 会被 macOS 当作 true
zapp plist set "path/to/target.app" LSUIElement false
```

新键的值如果是 `true` 或 `false`，会保存为布尔值；整数保存为整数类型，其他值保存为字符串。因此 `1.0` 这样的版本号仍为字符串。

嵌套值使用点分隔的路径访问。如果键本身含有点号（例如权限键），会先尝试完整匹配键名，再将其解析为路径。

```bash
zapp plist get "path/to/target.app" NSAppTransportSecurity.NSAllowsArbitraryLoads
zapp plist get "entitlements.plist" com.apple.security.app-sandbox
```

#### 递增版本

```bash
zapp plist bump "path/to/target.app"                 # 1.4.2 -> 1.4.3
zapp plist bump "path/to/target.app" --minor         # 1.4.2 -> 1.5.0
zapp plist bump "path/to/target.app" --major         # 1.4.2 -> 2.0.0
zapp plist bump "path/to/target.app" --key CFBundleShortVersionString
```

不带参数时递增最后一段，适合更新构建号。`--major`、`--minor` 和 `--patch` 会递增指定部分，并将后面的部分重置为 0。

### 完整示例

以下示例对 `MyApp.app` 进行依赖打包、代码签名、安装包创建、公证和票据附加：

```bash
# 依赖打包
zapp dep --app="MyApp.app"

# 代码签名、公证与附加票据
zapp sign --target="MyApp.app"
zapp notarize --profile="key-chain-profile" --target="MyApp.app" --staple

# 创建 PKG/DMG 文件
zapp pkg --app="MyApp.app" --out="MyApp.pkg"
zapp dmg --app="MyApp.app" --out="MyApp.dmg"

# 为 PKG/DMG 签名、公证并附加票据
zapp sign --target="MyApp.dmg"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
zapp notarize --profile="key-chain-profile" --target="MyApp.dmg" --staple
```

也可以使用以下简写命令：

```bash
zapp dep --app="MyApp.app" --sign --notarize --profile="key-chain-profile" --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple
```

## Go 库

导入 `github.com/ironpark/zapp` 即可加载项目配置并生成 DMG/PKG 安装包。

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

也可以直接构造 `zapp.Project`，或通过 `BuildDMG`、`BuildPKG` 等方法执行单独操作。输出路径可从 `artifacts.DMG` 和 `artifacts.PKG` 获取。

## 许可证

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zapp 基于 [MIT License](LICENSE) 发布。

## 支持

如有问题或疑问，请在 [GitHub issue tracker](https://github.com/ironpark/zapp/issues) 提交 Issue。
