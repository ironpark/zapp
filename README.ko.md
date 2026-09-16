# ZAPP

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![GitHub Repo stars](https://img.shields.io/github/stars/ironpark/zapp)](https://github.com/ironpark/zapp/stargazers)

🌐 [English](README.md) | [**한국어**](README.ko.md) | [日本語](README.ja.md) | [简体中文](README.zh-cn.md) | [繁體中文](README.zh-tw.md)
![Zapp으로 macOS 앱을 패키징하는 데모](docs/demo.gif)

**macOS 앱 배포를 간소화하세요**

`zapp`은 macOS 앱의 의존성 번들링, DMG/PKG 생성, 코드 서명과 공증을 처리하는 CLI 및 Go 라이브러리입니다. 하나의 `.zapp.yaml`로 반복 가능한 배포 작업을 구성하거나, 개별 명령으로 필요한 작업만 실행할 수 있습니다.

[설치](#설치) · [빠른 시작](#빠른-시작) · [프로젝트 설정](#프로젝트-설정) · [사용법](#-사용법) · [Go 라이브러리](#go-라이브러리)

## ✨ 기능

- [x] DMG 파일 생성
- [x] PKG 파일 생성
- [x] 코드 서명
- [x] 공증 / 스테플링
- [x] plist 수정 (버전)
- [x] 자동 바이너리 종속성 번들링
- [x] 프로젝트 설정 파일과 Go 라이브러리 API

## 설치

### 스크립트로 설치

**macOS / Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/ironpark/zapp/refs/heads/main/scripts/install.ps1 | iex"
```

SHA-256 체크섬 검증 후 최신 릴리스를 설치합니다. macOS/Linux는 `~/.local/bin`에 설치하며, 안내가 나오면 PATH에 추가하세요. Windows는 `%LOCALAPPDATA%\zapp\bin`에 설치하고 사용자 PATH에 추가합니다. 관리자 권한은 필요하지 않습니다.

특정 버전은 환경 변수 `ZAPP_VERSION`에 릴리스 태그를, 설치 경로는 `ZAPP_INSTALL_DIR`에 절대 경로를 지정하세요.

### 🍺 Homebrew 사용

```bash
brew tap ironpark/zapp
brew install --cask zapp
```

### 🛠️ 소스 코드에서 빌드

```bash
go install github.com/ironpark/zapp/cmd/zapp@latest
```

소스 빌드에는 Go 1.27.0 이상이 필요합니다. 위 명령은 macOS에서 사용할 수 있습니다. Linux·Windows에서 서명 기능까지 포함해 빌드하려면 [플랫폼별 빌드 안내](docs/signing.md#building)를 참고하세요.

## 빠른 시작

이미 빌드된 `dist/MyApp.app`이 있는 디렉터리에서 시작하세요. Zapp은 앱 소스를 컴파일하지 않습니다. `init`은 DMG와 PKG 설정을 만들고, `build`는 기본적으로 두 파일을 `dist`에 생성합니다.

의존성 번들링, DMG/PKG 생성, 서명 및 공증을 하나의 `.zapp.yaml`에서 설정합니다.

```sh
zapp init --app dist/MyApp.app
zapp config show
zapp build                  # 설정된 단계 실행
zapp build dmg pkg          # dep 생략; 설정된 서명·공증은 실행
zapp dmg --title "MyApp"    # 프로젝트 제목을 덮어씀
zapp pkg --no-sign --no-notarize
```

`init`이 생성하는 설정은 다음과 같은 형태입니다. `dep`, `sign`, `notarize`를 추가하면 해당 단계도 실행합니다.

```yaml
version: 1
app: dist/MyApp.app
out: dist
# dep: {libs: [/opt/homebrew/lib]}

dmg: {}
pkg: {}
```

## 프로젝트 설정

`.zapp.yaml`에 공통 경로와 실행할 작업을 정의합니다. 전체 옵션은 [주석이 달린 예제](examples/zapp.yaml)를 참고하세요.

### 설정 파일 선택

`dmg`, `pkg`, `dep`는 현재 디렉터리부터 상위 디렉터리까지 `.zapp.yaml`을 찾습니다.

```sh
zapp dmg                              # .zapp.yaml 자동 탐색
zapp dmg --config release/.zapp.yaml   # 설정 파일 직접 선택
zapp dmg --no-config --app MyApp.app   # 설정 파일 없이 실행
zapp config show                      # 적용할 설정 확인
```

`zapp init`은 기본 설정 파일을 생성합니다. 기존 파일을 덮어쓰려면 `--force`가 필요합니다.

### 값 덮어쓰기

같은 옵션을 여러 곳에서 지정하면 다음 순서로 적용합니다.

**CLI 플래그 → `ZAPP_*` 환경 변수 → 설정 파일 → 기본값** (왼쪽이 우선)

환경 변수 이름은 플래그를 대문자로 바꾸고, 하이픈을 밑줄로 바꾼 뒤 `ZAPP_`을 붙입니다.

| CLI 플래그 | 환경 변수 | 값 예시 |
| --- | --- | --- |
| `--app` | `ZAPP_APP` | `dist/MyApp.app` |
| `--title` | `ZAPP_TITLE` | `MyApp` |
| `--out` | `ZAPP_OUT` | `dist/MyApp.dmg` (`dmg` 명령) |
| `--window-width` | `ZAPP_WINDOW_WIDTH` | `720` |
| `--libs` | `ZAPP_LIBS` | `/usr/local/lib,/opt/homebrew/lib` |

### 경로 기준

| 경로를 지정하는 곳 | 상대 경로의 기준 |
| --- | --- |
| 설정 파일 | 설정 파일이 있는 디렉터리 |
| CLI 플래그·환경 변수 | 현재 작업 디렉터리 |

최상위 `out`은 **출력 디렉터리**, `dmg.out`과 `pkg.out`은 **출력 파일 경로**입니다.

```yaml
version: 1
app: dist/MyApp.app
out: dist

dmg:
  out: dist/MyApp-installer.dmg
pkg:
  out: dist/MyApp-installer.pkg
```

### 변수 사용

문자열 값과 `contents`의 키에 다음 변수를 사용할 수 있습니다.

| 변수 | 값 |
| --- | --- |
| `${app}` | 앱 경로 |
| `${app.name}` | 앱 이름 |
| `${app.version}` | 앱 버전 |
| `${env:NAME}` | 환경 변수 `NAME`의 값. 정의되지 않았으면 오류 |

```yaml
dmg:
  title: ${app.name}
  out: dist/${app.name}-${app.version}.dmg
```

### 서명과 공증

`sign:` 또는 `notarize:` 섹션을 추가하면 해당 단계를 활성화합니다. 다음은 macOS 키체인 프로필을 사용하는 설정입니다.

```yaml
sign:
  identity: ${env:ZAPP_IDENTITY}
notarize:
  profile: my-profile
  staple: true
```

- 특정 실행에서 생략하려면 `--no-sign`, `--no-notarize`를 사용합니다.
- 기존 `--sign --notarize --profile ... --staple` 플래그도 사용할 수 있습니다.
- 비밀번호는 `ZAPP_P12_PASSWORD`, `ZAPP_PASSWORD` 환경 변수나 해당 CLI 플래그로 전달합니다. 설정 파일에 `sign.p12Password`, `notarize.password`를 넣을 수 없습니다.
- `zapp config show`는 비밀번호를 출력하지 않습니다.

플랫폼별 인증서와 인증 방법은 [서명·공증 안내](docs/signing.md)를 참고하세요.

### 고급 설정

<details>
<summary>PKG 설정 형식</summary>

| 형식 | 용도와 필드 |
| --- | --- |
| 단축형 | 앱 하나를 패키징. `identifier`, `version`, `installLocation`, `scripts`, `minOS`, `license` 지원 |
| 전체형 | 여러 `components`와 설치 화면용 `distribution` 정의. `choices`로 선택 항목 구성 |

단축형과 전체형 필드는 함께 사용할 수 없습니다.

- 식별자와 버전은 기본적으로 앱의 Info.plist에서 가져옵니다.
- `license`는 파일 경로나 언어별 매핑(`{default: license.txt, ko: license-ko.txt}`)을 받습니다.
- `type: component`는 제품 설치 화면 없이 단일 컴포넌트만 생성합니다.

자세한 내용은 [PKG 엔진 문서](pkg/macpkg/README.md)를 참고하세요.

</details>

<details>
<summary>기존 DMG 설정 파일에서 이전</summary>

평면 DMG 설정도 계속 사용할 수 있지만, 사용 중단 예정 경고가 표시됩니다.

```sh
zapp dmg --config examples/dmg/layout.yaml --out release.dmg
```

이 형식의 출력 경로는 현재 작업 디렉터리 기준입니다. 프로젝트 설정으로 이전하려면 레이아웃 필드를 `dmg:` 아래로 옮기고 공용 `app`은 최상위에 두세요.

</details>

## 📖 사용법

`dep`, `dmg`, `pkg`는 `--sign --notarize --profile "profile" --staple`을 지원합니다. 서명을 설정하면 패키징 전에 앱을 서명하고, 생성한 설치 파일도 서명합니다. 공증·스테플링은 결과물에 적용합니다. 아래 키체인 예제는 macOS용이며, Linux·Windows 인증서 및 API 키 설정은 [서명·공증 안내](docs/signing.md)를 참고하세요.

### 🔏 코드 서명

> [!TIP]
>
> `--identity` 플래그를 사용하여 인증서를 선택하지 않으면 Zapp은 현재 키체인에서 사용 가능한 인증서를 자동으로 선택합니다.

```bash
zapp sign --target="path/to/MyApp.app"
```
```bash
zapp sign --identity="Developer ID Application" --target="path/to/MyApp.app"
```

### 🏷️ 공증 및 스테플링

> [!NOTE]
>
> notarize 명령을 실행할 때 Zapp이 앱 번들 경로를 받으면 자동으로 앱 번들을 압축하고 공증을 시도합니다.

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/MyApp.app" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/MyApp.app" --staple
```

### 🔗 종속성 번들링

> [!NOTE]
>
> 이 프로세스는 애플리케이션 실행 파일의 종속성을 검사하고 필요한 라이브러리를 `/Contents/Frameworks` 내에 포함하며 독립 실행을 가능하게 하기 위해 링크 경로를 수정합니다.

```bash
zapp dep --app="path/to/target.app"
```
#### 라이브러리 검색을 위한 추가 경로

```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```
#### 서명 & 공증 & 스테플링과 함께 사용

```bash
zapp dep --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 💽 DMG 파일 생성

> Zapp을 사용하여 macOS 앱 배포에 일반적으로 사용되는 형식인 DMG 파일을 만들 수 있습니다.
앱 번들에서 아이콘을 자동으로 추출하고, 디스크 아이콘을 합성하고, 앱의 드래그 앤 드롭 설치를 위한 인터페이스를 제공하여 DMG 생성 프로세스를 크게 간소화합니다.

DMG는 외부 도구나 볼륨 마운트 없이 순수 Go로 생성하며 Linux와 macOS에서 사용할 수 있습니다.
기본 파일시스템은 HFS+입니다. `--fs apfs`로 APFS를 선택하거나,
`--fs apfs-case-sensitive`로 대소문자를 구분할 수 있습니다.
APFS 이미지를 열려면 macOS 10.13 이상이 필요합니다.

원본 파일의 FinderInfo와 리소스 포크를 보존합니다. Linux에서는
`user.com.apple.*` 확장 속성을 사용합니다. 볼륨 내부 아이콘은 항상 포함하며,
Linux 확장 속성의 크기·지원 제한으로 저장할 수 없는 경우 호스트 `.dmg` 파일 자체의 아이콘만 생략합니다.

```bash
zapp dmg --app="MyApp.app" --fs apfs
zapp dmg --app="MyApp.app" --fs apfs-case-sensitive --format ulfo
```

압축은 `--format udzo`(기본값, zlib)와 `--format ulfo`(LZFSE)를 모두 지원합니다.
APFS는 암호화하지 않은 단일 볼륨 생성만 지원하며 스냅샷과 기존 이미지 편집은 지원하지 않습니다.

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

`--icon`은 ICNS와 PNG를 지원합니다. PNG는 비율과 투명도를 유지해 ICNS로 변환합니다.
`--out MyApp`처럼 확장자를 생략하면 `MyApp.dmg`를 생성하고, 서명·공증에도 같은 경로를 사용합니다.

#### 커스텀 레이아웃

`.zapp.yaml`의 `dmg.contents`에 넣을 파일과 아이콘 위치를 지정합니다. 아래 예제는 앱, Applications 링크, 안내 문서를 배치합니다.

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

**항목별 설정**

`contents`의 키는 원본 경로입니다. `contents`를 지정하면 기본 앱·Applications 배치를 대체하므로, 필요한 항목을 모두 포함하세요.

| 필드 | 설명 |
| --- | --- |
| `x`, `y` | Finder 콘텐츠 영역 좌상단 기준 아이콘 중심 좌표. 0 이상 |
| `link` | `true`이면 심볼릭 링크 생성. 생략하면 파일·디렉터리 복사 |
| `name` | 이미지 안에서 사용할 이름. 생략하면 원본 이름 사용 |

파일·디렉터리 경로는 설정 파일 기준이며, 링크 대상은 입력한 그대로 유지됩니다. CLI의 `--out`은 현재 작업 디렉터리 기준입니다.

**기본 아이콘 위치만 변경**

앱과 Applications 링크의 위치만 바꾸려면 `contents` 없이 실행합니다.

```sh
zapp dmg --app MyApp.app \
  --app-position 180,200 \
  --applications-position 540,200
```

<details>
<summary>추가 옵션과 제약</summary>

- `--app-position`, `--applications-position`은 명시적 `contents`와 함께 사용할 수 없습니다.
- `contents`가 없으면 `app`이 필요합니다. `contents`가 있으면 `app`은 기본 제목과 디스크 아이콘 추출에 사용합니다. `app`을 생략할 때는 `dmg.title`을 지정하세요.
- `dmg.icon`에 ICNS·PNG, `dmg.background`에 배경 이미지를 지정할 수 있습니다. `fs`, `format`, 서명·공증 옵션도 함께 사용할 수 있습니다.
- `version: 1`이 필요합니다. 알 수 없는 필드와 중복 키는 오류로 처리합니다.
- 이미지 안의 이름은 대소문자·유니코드 정규화 기준으로 고유해야 하며, DMG 메타데이터 예약 이름은 사용할 수 없습니다.
- `.DS_Store` 용량 초과 오류가 발생하면 항목 수나 이름 길이를 줄이세요.

전체 설정은 [프로젝트 예제](examples/zapp.yaml)를 참고하세요. 기존 평면 형식은 [이전 안내](#고급-설정)를 참고하세요.

</details>

#### 서명 & 공증 & 스테플링과 함께 사용

```bash
zapp dmg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```
### 📦 PKG 파일 생성

PKG 생성은 순수 Go [macpkg 패키지](pkg/macpkg/README.md)를 사용하며
`pkgbuild`·`productbuild`가 필요하지 않습니다. 서명·공증은 기존 백엔드를 사용합니다.

> [!TIP]
>
> `--version` 및 `--identifier` 플래그가 설정되지 않은 경우 이러한 값은 제공된 앱 번들의 Info.plist 파일에서 자동으로 검색됩니다.

#### 앱 번들에서 PKG 파일 생성

```bash
zapp pkg --app="path/to/target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="path/to/target.app"
```

#### EULA 파일과 함께 사용

여러 언어로 된 최종 사용자 라이선스 계약 (EULA) 파일을 포함합니다.

```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="path/to/target.app"
```
#### 서명 & 공증 & 스테플링과 함께 사용

```bash
zapp pkg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 📝 Info.plist 편집

앱 번들이나 `.plist` 파일의 값을 읽고 수정합니다. 기존 XML·바이너리 형식과 값의 타입을 유지합니다. 서명된 앱을 수정한 경우 다시 서명해야 하므로, 버전 변경은 서명 전에 수행하세요.

```sh
zapp plist get "MyApp.app" CFBundleVersion
zapp plist set "MyApp.app" CFBundleVersion 1.2.3
zapp plist set "MyApp.app" LSUIElement false
zapp plist delete "MyApp.app" LSUIElement
zapp plist bump "MyApp.app"
zapp plist bump "MyApp.app" --minor
zapp plist bump "MyApp.app" --major
zapp plist bump "MyApp.app" --key CFBundleShortVersionString
```

`bump`는 기본적으로 마지막 버전 구성 요소를 올립니다. `--major`, `--minor`, `--patch`는 지정한 구성 요소를 올리고 뒤의 값을 0으로 초기화합니다. 중첩 키는 `NSAppTransportSecurity.NSAllowsArbitraryLoads`처럼 점으로 구분합니다.

### 전체 예제

다음은 `zapp`을 사용하여 `MyApp.app`의 종속성 번들링, 코드 서명, 패키징, 공증 및 스테플링을 수행하는 방법을 보여주는 완전한 예제입니다.

```bash
# 종속성 번들링

zapp dep --app="MyApp.app"

# 코드 서명 / 공증 / 스테플링

zapp sign --target="MyApp.app"
zapp notarize --profile="key-chain-profile" --target="MyApp.app" --staple

# pkg/dmg 파일 생성

zapp pkg --app="MyApp.app" --out="MyApp.pkg"
zapp dmg --app="MyApp.app" --out="MyApp.dmg"

# pkg/dmg에 대한 코드 서명 / 공증 / 스테플링

zapp sign --target="MyApp.dmg"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
zapp notarize --profile="key-chain-profile" --target="MyApp.dmg" --staple
```
또는 약식 명령을 사용할 수 있습니다.
```bash
zapp dep --app="MyApp.app" --sign --notarize --profile="key-chain-profile" --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple
```

## Go 라이브러리

`github.com/ironpark/zapp`를 임포트해 설정을 불러오고 DMG·PKG를 생성합니다.

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

`zapp.Project`를 직접 구성하거나 `BuildDMG`, `BuildPKG` 등으로 개별 작업을 실행할 수도 있습니다. 결과 경로는 `artifacts.DMG`, `artifacts.PKG`에서 확인합니다.

## 라이선스

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zapp은 [MIT License](LICENSE)에 따라 배포됩니다.

## 지원

문제가 발생하거나 질문이 있는 경우 [GitHub issue tracker](https://github.com/ironpark/zapp/issues)에 이슈를 제출하십시오.
