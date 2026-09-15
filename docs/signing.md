# Signing and notarizing

The operating system chooses the backend at build time.

| Host | Backend | Signing credentials | Notarization credentials |
| --- | --- | --- | --- |
| macOS arm64 / amd64 | `pkg/signing/macos`: Apple tools | Keychain identity | Keychain profile or Apple ID, password, team ID |
| Linux arm64 / amd64 | Statically linked `apple-codesign` Rust library | PKCS#12 or PEM certificate and private key | App Store Connect API key JSON |
| Windows arm64 / amd64 | Statically linked `apple-codesign` Rust library | PKCS#12 or PEM certificate and private key | App Store Connect API key JSON |

macOS always uses `codesign`, `productsign`, `notarytool`, and `stapler`.
Passing `--p12-file`, `--pem-file`, or `--api-key-file` on macOS returns an
error explaining which Apple credentials to use. Import signing certificates
into Keychain first. The macOS binary does not link or import rcodesign.

Windows and Linux release builds include the Rust signing implementation in
the zapp executable. No separate rcodesign executable or Rust installation is
required at runtime. System libraries (such as glibc on Linux) are still used;
"static" refers to the Rust binding, not to every operating-system dependency.

## macOS

```sh
zapp sign --target MyApp.dmg
zapp sign --target MyApp.dmg --identity "Developer ID Application: Me (TEAMID)"
zapp notarize --target MyApp.dmg --profile my-profile --staple
```

With no `--identity`, zapp picks the first keychain identity matching
`Developer ID Application`, or `Developer ID Installer` for a `.pkg`.

## Windows and Linux

Export your Developer ID certificate and private key as a PKCS#12 bundle:

```sh
zapp sign --target MyApp.dmg \
  --p12-file developer-id.p12 --p12-password-file certificate-password.txt
zapp dmg --app MyApp.app --sign \
  --p12-file developer-id.p12 --p12-password-file certificate-password.txt
zapp notarize --target MyApp.dmg --api-key-file key.json --staple
```

`--pem-file` accepts a PEM bundle containing the certificate and private key.
A password file takes precedence over `--p12-password`. With neither option,
an empty PKCS#12 password is used; the library does not prompt on stdin.
The password file's first line is used. Prefer it to putting a password in the
shell's command line. Signing requires a private key; certificate-only PEM
inputs are rejected instead of producing ad-hoc signatures.

`key.json` uses the upstream apple-codesign/App Store Connect unified API key
format. Existing key files created with `rcodesign encode-app-store-connect-api-key`
remain usable. Signing uses the hardened runtime flag and Apple's timestamp
service. Notarization waits for acceptance for up to 600 seconds before returning
an error. Stapling an existing ticket does not require a signing certificate.

An `.app` bundle is zipped before submission; its ticket is stapled to the
original bundle. Calls into Rust are synchronous. Cancellation is checked before
entry; an in-flight Rust operation completes before the Go call returns, so it
cannot continue modifying the target after return.

## Building

macOS needs only Go and Apple's tools:

```sh
CGO_ENABLED=0 go build ./cmd/zapp
```

Windows and Linux signing builds need Go, Python 3, Rust 1.98.0, and a C toolchain:

| Build target | Rust target | C compiler |
| --- | --- | --- |
| `linux_amd64` | `x86_64-unknown-linux-gnu` | GCC for x86-64 Linux |
| `linux_arm64` | `aarch64-unknown-linux-gnu` | GCC for arm64 Linux |
| `windows_amd64` | `x86_64-pc-windows-gnullvm` | LLVM MinGW `x86_64-w64-mingw32-clang` |
| `windows_arm64` | `aarch64-pc-windows-gnullvm` | LLVM MinGW `aarch64-w64-mingw32-clang` |

Run on the target host, with the compiler on `PATH`:

```sh
python scripts/build-native.py linux_amd64 --test
# or windows_amd64, linux_arm64, windows_arm64
```

The script builds the pinned Rust sources with `cargo --locked`, copies the
static archive into `libcodesign/lib/<os>_<arch>/`, runs tests if requested,
and links zapp into `dist/<os>_<arch>/`, stamping the version with the same
ldflags GoReleaser uses. Set `CC` for cross compilation and omit `--test`
unless the host can execute target binaries.

A Windows/Linux `CGO_ENABLED=0` build can run commands that do not need signing;
signing and notarization return `ErrUnavailable`. A CGO build requires the static
archive to be built first. Unsupported operating systems/architectures do not
fall back to an external rcodesign executable.

`.github/workflows/signing.yaml` builds and tests all four targets on native
runners and checks the Apple-only macOS build. The release workflow consumes
those archives alongside GoReleaser's macOS archives, and every archive appears
in the release's single checksums file.

Windows and Linux binaries link the Rust library through cgo, so they are built
on their own runners rather than by GoReleaser, whose OSS distribution cannot
adopt a binary it did not build itself (`builder: prebuilt` is Pro-only).
`scripts/package-native.py` assembles those archives, but takes their contents,
format and name from `.goreleaser.yaml` via `scripts/goreleaser_config.py`, so
the two halves of a release cannot drift apart; editing the archive layout in
`.goreleaser.yaml` is enough, and changing its `name_template` fails packaging
with an explicit message. Each Windows/Linux archive additionally carries the
Rust dependency license notices, which the macOS archive has no need of.

Windows DMG creation writes the icon into the image, but cannot attach Finder
metadata to the host `.dmg` file or import macOS extended attributes from source
files. Windows PKG creation uses Windows file IDs to preserve hard links and
rejects `PreserveOwnership`, since Windows does not supply Unix uid/gid values.

## Implementation and validation

- `pkg/signing/select_darwin.go`: Apple-only backend selection.
- `pkg/signing/rcodesign`: Go/C binding, credential validation and errors.
- `libcodesign`: Rust static library using apple-codesign 0.29.0 directly.
- Rust tests sign amd64 and arm64 Mach-O fixtures with an ephemeral certificate,
  verify their signatures and hardened runtime flags, and check FFI error ownership.
  These tests disable network timestamping.
- Native Go tests exercise the C ABI with an empty `PATH` and check cancellation
  before entry. They do not require an external rcodesign executable.

Live Apple notarization and timestamping require network/service access and
appropriate credentials and are not exercised by these offline tests.
