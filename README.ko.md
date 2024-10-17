# ZAPP
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_shield&issueType=license)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironpark/zapp)](https://goreportcard.com/report/github.com/ironpark/zapp)
[![codebeat badge](https://codebeat.co/badges/6b004587-036c-4324-bc97-c2e76d58b474)](https://codebeat.co/projects/github-com-ironpark-zapp-main)
![GitHub Repo stars](https://img.shields.io/github/stars/ironpark/zapp)


🌐 [English](README.md) | [**한국어**](README.ko.md) | [日本語](README.ja.md)

![asd](/docs/demo.gif)

**macOS 앱 배포를 간소화하세요**

`zapp`은 macOS 애플리케이션의 배포 과정을 간소화하고 자동화하기 위해 설계된 강력한 CLI 도구입니다. 의존성 번들링부터 DMG/PKG 생성, 코드 서명, 공증에 이르기까지 배포의 모든 단계를 하나의 도구로 처리합니다.

## ✨ 기능

- [x] DMG 파일 생성
- [x] PKG 파일 생성
- [x] 코드 서명
- [x] 공증 / 스테이플링
- [ ] plist 수정 (버전)
- [x] 자동 바이너리 의존성 번들링
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
> `--identity` 플래그를 사용하여 인증서를 선택하지 않으면, Zapp은 현재 키체인에서 사용 가능한 인증서를 자동으로 선택합니다.

```bash
zapp sign --target="path/to/target.(app,dmg,pkg)"
```
```bash
zapp sign --identity="Developer ID Application" --target="path/to/target.(app,dmg,pkg)"
```

### 🏷️ 공증 & 스테이플링
> [!NOTE]
>
> 공증 명령을 실행할 때 Zapp이 앱 번들 경로를 받으면, 자동으로 앱 번들을 압축하고 공증을 시도합니다.

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/target.(app,dmg,pkg)" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/target.(app,dmg,pkg)" --staple
```

### 🔗 의존성 번들링
> [!NOTE]
>
> 이 과정은 애플리케이션 실행 파일의 의존성을 검사하고, 필요한 라이브러리를 `/Contents/Frameworks` 내에 포함시키며, 독립 실행을 위해 링크 경로를 수정합니다.

```bash
zapp dep --app="path/to/target.app"
```
#### 라이브러리 검색을 위한 추가 경로
```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```
#### 서명 & 공증 & 스테이플링 포함
> [!TIP]
>
> `dep`, `dmg`, `pkg` 명령어는 `--sign`, `--notarize`, `--staple` 플래그와 함께 사용할 수 있습니다.
> - `--sign` 플래그는 의존성 번들링 후 앱 번들을 자동으로 서명합니다.
> - `--notarize` 플래그는 서명 후 앱 번들을 자동으로 공증합니다.

```bash
zapp dep --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 💽 DMG 파일 생성

> Zapp을 사용하여 macOS 앱 배포에 일반적으로 사용되는 DMG 파일을 생성할 수 있습니다.
> 앱 번들에서 아이콘을 자동으로 추출하고, 디스크 아이콘을 합성하며, 앱의 드래그 앤 드롭 설치를 위한 인터페이스를 제공하여 DMG 생성 과정을 크게 간소화합니다.

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
#### 서명 & 공증 & 스테이플링 포함
> [!TIP]
>
> `dep`, `dmg`, `pkg` 명령어는 `--sign`, `--notarize`, `--staple` 플래그와 함께 사용할 수 있습니다.
> - `--sign` 플래그는 의존성 번들링 후 앱 번들을 자동으로 서명합니다.
> - `--notarize` 플래그는 서명 후 앱 번들을 자동으로 공증합니다.

```bash
zapp dmg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 📦 PKG 파일 생성

> [!TIP]
>
> `--version`과 `--identifier` 플래그가 설정되지 않은 경우, 이 값들은 제공된 앱 번들의 Info.plist 파일에서 자동으로 가져옵니다.

#### 앱 번들에서 PKG 파일 생성
```bash
zapp pkg --app="path/to/target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="path/to/target.app"
```

#### EULA 파일 포함

여러 언어로 된 최종 사용자 라이선스 계약(EULA) 파일 포함:

```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="path/to/target.app" 
```
#### 서명 & 공증 & 스테이플링 포함
> [!TIP]
>
> `dep`, `dmg`, `pkg` 명령어는 `--sign`, `--notarize`, `--staple` 플래그와 함께 사용할 수 있습니다.
> - `--sign` 플래그는 의존성 번들링 후 앱 번들을 자동으로 서명합니다.
> - `--notarize` 플래그는 서명 후 앱 번들을 자동으로 공증합니다.

```bash
zapp pkg --app="path/to/target.app" --sign --notarize --profile "profile" --staple
```

### 전체 예시
다음은 `zapp`을 사용하여 `MyApp.app`의 의존성 번들링, 코드 서명, 패키징, 공증, 스테이플링을 수행하는 방법을 보여주는 완전한 예시입니다:

```bash
# 의존성 번들링
zapp dep --app="MyApp.app"

# 코드 서명 / 공증 / 스테이플링
zapp sign --target="MyApp.app"
zapp notarize --profile="key-chain-profile" --target="MyApp.app" --staple

# pkg/dmg 파일 생성
zapp pkg --app="MyApp.app" --out="MyApp.pkg"
zapp dmg --app="MyApp.app" --out="MyApp.dmg"

# pkg/dmg에 대한 코드 서명 / 공증 / 스테이플링
zapp sign --target="MyApp.app"
zapp sign --target="MyApp.pkg"

zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
zapp notarize --profile="key-chain-profile" --target="MyApp.dmg" --staple
```
또는 간단히 축약 명령어를 사용하세요
```bash
zapp dep --app="MyApp.app" --sign --notarize --staple

zapp pkg --out="MyApp.pkg" --app="MyApp.app" \ 
  --sign --notarize --profile="key-chain-profile" --staple

zapp dmg --out="MyApp.dmg" --app="MyApp.app" \
  --sign --notarize --profile="key-chain-profile" --staple
```

## 라이선스
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fironpark%2Fzapp.svg?type=large&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fironpark%2Fzapp?ref=badge_large&issueType=license)

Zapp은 [MIT 라이선스](LICENSE)에 따라 배포됩니다.

## 지원

문제가 발생하거나 질문이 있으면 [GitHub 이슈 트래커](https://github.com/ironpark/zapp/issues)에 이슈를 제기해 주세요.