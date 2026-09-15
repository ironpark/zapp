# ZAPP
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![GitHub Repo stars](https://img.shields.io/github/stars/ironpark/zapp)](https://github.com/ironpark/zapp/stargazers)

🌐 [English](README.md) | [**한국어**](README.ko.md) | [日本語](README.ja.md) | [简体中文](README.zh-cn.md) | [繁體中文](README.zh-tw.md)
![asd](/docs/demo.gif)

**macOS 앱 배포를 간소화하세요**

`zapp`은 macOS 애플리케이션의 배포 프로세스를 간소화하고 자동화하도록 설계된 강력한 CLI 도구입니다. 종속성 번들링부터 DMG/PKG 생성, 코드 서명, 공증에 이르기까지 배포의 모든 단계를 하나의 도구로 처리합니다.

## ✨ 기능

- [x] DMG 파일 생성
- [x] PKG 파일 생성
- [x] 코드 서명
- [x] 공증 / 스테플링
- [x] plist 수정 (버전)
- [x] 자동 바이너리 종속성 번들링
- [ ] GitHub Actions 지원

## ⚡️ 빠른 시작
#### 🍺 Homebrew 사용
```bash
brew tap ironpark/zapp
brew install zapp
```

#### 🛠️ 소스 코드에서 빌드

```bash
go install github.com/ironpark/zapp@latest
```

## 📖 사용법
### 🔏 코드 서명

> [!TIP]
>
> `--identity` 플래그를 사용하여 인증서를 선택하지 않으면 Zapp은 현재 키체인에서 사용 가능한 인증서를 자동으로 선택합니다.

```bash
zapp sign --target="path/to/target.(app,dmg,pkg)"
```
```bash
zapp sign --identity="Developer ID Application" --target="path/to/target.(app,dmg,pkg)"
```

### 🏷️ 공증 및 스테플링
> [!NOTE]
>
> notarize 명령을 실행할 때 Zapp이 앱 번들 경로를 받으면 자동으로 앱 번들을 압축하고 공증을 시도합니다.

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/target.(app,dmg,pkg)" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/target.(app,dmg,pkg)" --staple
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
> [!TIP]
>
> `dep`, `dmg`, `pkg` 명령어는 `--sign`, `--notarize`, `--staple` 플래그와 함께 사용할 수 있습니다.
> - `--sign` 플래그는 종속성을 번들링한 후 앱 번들에 자동으로 서명합니다.
> - `--notarize` 플래그는 서명 후 앱 번들을 자동으로 공증합니다.

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

`--config dmg.yaml`로 파일·디렉터리·심볼릭 링크와 아이콘 위치를 지정합니다.
같은 필드 구조의 JSON도 지원합니다.

```yaml
version: 1
title: MyApp
window: {width: 720, height: 460}
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
zapp dmg --config dmg.yaml --out dist/MyApp.dmg
# Adjust the two default icons without a config file:
zapp dmg --app MyApp.app --app-position 180,200 --applications-position 540,200
```

- `version: 1`을 지정합니다. 알 수 없는 필드와 중복 키는 오류로 처리합니다.
- 명시한 CLI 옵션이 설정 파일보다 우선하며, 생략한 필드는 기본값을 사용합니다.
- 입력 경로(`app`, `icon`, `background`, `contents`의 파일·디렉터리 키)는 설정 파일
  위치 기준입니다. CLI 경로와 `out`은 현재 작업 디렉터리 기준입니다.
  링크 대상은 상대 경로를 포함해 그대로 보존합니다.
- `contents`는 기본 앱·Applications 구성을 대체합니다. 경로를 키로 사용하고
  값에 `x`, `y` 좌표를 지정합니다. `link: true`이면 심볼릭 링크를 만들고,
  생략하거나 `false`이면 파일·디렉터리를 자동 판별합니다. `name`으로 이미지
  내부 이름을 바꿀 수 있습니다. 대소문자·유니코드 정규화 기준 중복 이름과
  DMG 메타데이터 예약 이름은 사용할 수 없습니다.
- 좌표는 Finder 콘텐츠 영역 좌상단을 기준으로 한 아이콘 중심이며 음수는
  허용하지 않습니다. `--app-position`, `--applications-position`은 기본
  두 항목 배치 전용이며 명시적 `contents`와 함께 사용할 수 없습니다.
- `contents`가 없으면 `app` 또는 `--app`이 필요합니다. `contents`가 있으면
  `app`은 기본 제목과 디스크 아이콘 추출에만 사용합니다. `app`이 없으면
  `title`을 지정해야 하며, `icon`을 생략하면 커스텀 디스크 아이콘을 넣지 않습니다.
- 추가 설정 필드: `out`, `app`, `icon`, `background`, `fs`, `format`.
  PNG 아이콘 변환과 CLI 서명·공증 옵션도 함께 사용할 수 있습니다.
- 현재 `.DS_Store` 단일 노드 용량을 초과하면 오류가 발생합니다.
  이 경우 항목 수나 이름 길이를 줄여야 합니다.

[레이아웃 예제](examples/dmg/layout.yaml)를 참고하세요.

#### 서명 & 공증 & 스테플링과 함께 사용
> [!TIP]
>
> `dep`, `dmg`, `pkg` 명령어는 `--sign`, `--notarize`, `--staple` 플래그와 함께 사용할 수 있습니다.
> - `--sign` 플래그는 종속성을 번들링한 후 앱 번들에 자동으로 서명합니다.
> - `--notarize` 플래그는 서명 후 앱 번들을 자동으로 공증합니다.

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
> [!TIP]
>
> `dep`, `dmg`, `pkg` 명령어는 `--sign`, `--notarize`, `--staple` 플래그와 함께 사용할 수 있습니다.
> - `--sign` 플래그는 종속성을 번들링한 후 앱 번들에 자동으로 서명합니다.
> - `--notarize` 플래그는 서명 후 앱 번들을 자동으로 공증합니다.

```bash
zapp pkg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

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
zapp sign --target="MyApp.app"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
zapp notarize --profile="key-chain-profile" --target="MyApp.dmg" --staple
```
또는 약식 명령을 사용할 수 있습니다.
```bash
zapp dep --app="MyApp.app" --sign --notarize --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \ 
  --sign --notarize --profile="key-chain-profile" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple
```

## 라이선스
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zapp은 [MIT License](LICENSE)에 따라 배포됩니다.

## 지원

문제가 발생하거나 질문이 있는 경우 [GitHub issue tracker](https://github.com/ironpark/zapp/issues)에 이슈를 제출하십시오.
