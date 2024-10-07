# ZAPP

[English](README.md) | [**한국어**](README.ko.md) | [日本語](README.ja.md)

**macOS 앱 배포를 간소화하세요**

Zapp은 macOS 애플리케이션의 배포 프로세스를 간소화하기 위해 설계된 강력한 CLI 도구입니다. Zapp을 사용하면 DMG 및 PKG 파일을 손쉽게 생성하고, 코드 서명, 앱 공증, plist 파일 수정 작업을 수행할 수 있습니다.

## 기능

- [x] DMG 파일 생성
- [x] PKG 파일 생성
- [x] 코드 서명
- [x] 공증 / 스테이플링
    - [ ] 공증 재시도
- [ ] plist 수정 (버전)
- [x] 자동 바이너리 종속성 번들링
- [ ] GitHub Actions 지원

## 설치
Homebrew 사용
```bash
brew tap ironpark/zapp
brew install zapp
```
Using Go
```bash
go install github.com/ironpark/zapp@latest
```

## 사용법

### 전체 예시
다음은 zapp을 사용하여 종속성 번들링, 코드 서명, 패키징, 공증, 스테이플링을 수행하는 완전한 예제입니다:
```bash
zapp dep --app="MyApp.app"
zapp sign --target="MyApp.app"
zapp pkg --out="MyApp.pkg" --app="MyApp.app"
zapp sign --target="MyApp.pkg"
zapp notarize --profile="key-chain-profile" --target="MyApp.pkg" --staple
```


### Dependency Bundling
dep 명령어는 어플리케이션이 독립적으로 실행될 수 있도록 애플리케이션 실행 파일의 종속성을 검사하고, 필요한 라이브러리를 `/Contents/Frameworks` 내에 복사한뒤 링크 경로를 수정합니다.
```bash
zapp dep --app="path/to/target.app"
```
추가적으로 라이브러리를 검색할 경로를 추가할 수 있습니다.
```bash
zapp dep --app="path/to/target.app" --libs="/usr/local/lib" --libs="/opt/homebrew/Cellar/ffmpeg/7.0.2/lib"
```


### DMG 파일 생성

> Zapp은 macOS 앱 배포에 사용되는 일반적인 형식인 DMG 파일을 생성하는 데 사용할 수 있습니다.
앱 번들에서 아이콘을 자동으로 추출하고, 디스크 아이콘을 합성하고, 앱을 드래그 앤 드롭으로 설치할 수 있는 인터페이스를 제공하여 DMG 생성 프로세스를 크게 간소화합니다.


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

### PKG 파일 생성
> `--version` 및 `--identifier` 플래그가 설정되지 않은 경우 제공된 앱 번들의 Info.plist 파일에서 이 값이 자동으로 검색됩니다.

```bash
zapp pkg --app="path/to/target.app"
```

```bash
zapp pkg --out="MyApp.pkg" --version="1.2.3" --identifier="com.example.myapp" --app="path/to/target.app"
```

#### With EULA Files

여러 언어로 된 최종 사용자 라이선스 계약(EULA) 파일을 포함하려면 `--eula` 플래그에 ',' 로 구분된 언어 코드 및 파일 경로를 전달하십시오.

```bash
zapp pkg --eula=en:eula_en.txt,es:eula_es.txt,fr:eula_fr.txt --app="path/to/target.app" 
```
### 코드 서명

인증서를 선택할 때 `--identity` 플래그를 사용하지 않는 경우, `zapp` 은 현재 키체인에서 사용 가능한 인증서를 자동으로 선택합니다.
```bash
zapp sign --target="path/to/target.(app,dmg,pkg)"
```
```bash
zapp sign --identity="Developer ID Application" --target="path/to/target.(app,dmg,pkg)"
```

### 공증 및 스테이플링
> 공증 명령(notarize)을 실행할 때 앱 번들 경로를 받으면 `zapp`은 자동으로 앱 번들을 압축하고 공증을 시도합니다.

```bash
zapp notarize --profile="key-chain-profile" --target="path/to/target.(app,dmg,pkg)" --staple
```

```bash
zapp notarize --apple-id="your@email.com" --password="pswd" --team-id="XXXXX" --target="path/to/target.(app,dmg,pkg)" --staple
```

## Advanced Usage

(TODO: Add advanced usage examples)

## License

Zapp is released under the [MIT License](LICENSE).

## Support

If you encounter any issues or have questions, please file an issue on the [GitHub issue tracker](https://github.com/ironpark/zapp/issues).
