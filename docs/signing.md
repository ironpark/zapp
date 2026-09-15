# Signing and notarizing

zapp signs through one of two toolchains. They agree on almost nothing except
the result, so which one runs decides what you have to supply.

| | Apple's tools | rcodesign |
| --- | --- | --- |
| Runs on | macOS only | macOS, Linux, Windows |
| Signing certificate | a keychain identity, by name | a file: PKCS#12 or PEM |
| Installer packages | `productsign`, a separate tool | the same `sign` command |
| Notary credentials | keychain profile, or Apple ID and password and team ID | App Store Connect API key |
| Stapling | `xcrun stapler` | `rcodesign staple` |

## Which one runs

zapp uses Apple's tools on macOS, and rcodesign anywhere else.

Naming a certificate file selects rcodesign whatever the platform, because
Apple's `codesign` cannot read one. That is how a macOS machine with no usable
keychain signs — a CI runner, or an SSH session where the keychain will not
unlock.

Away from macOS with no certificate file, zapp says so rather than failing
obscurely later:

```
signing away from macOS needs a certificate file, because there is no keychain
to take an identity from: pass --p12-file or --pem-file (running on linux)
```

## Signing with Apple's tools

```sh
zapp sign --target MyApp.dmg
zapp sign --target MyApp.dmg --identity "Developer ID Application: Me (TEAMID)"
```

With no `--identity`, zapp picks the first keychain identity matching
`Developer ID Application`, or `Developer ID Installer` for a `.pkg`.

## Signing with rcodesign

Install it first; zapp looks for `rcodesign` on `PATH`.

```sh
cargo install apple-codesign
# or take a release binary from
# https://github.com/indygreg/apple-platform-rs/releases
```

Export your Developer ID certificate and key as a PKCS#12 bundle, then:

```sh
zapp sign --target MyApp.dmg \
  --p12-file developer-id.p12 --p12-password-file ~/.certificate-password
```

`--p12-password` takes the password inline instead, which puts it in the
process list; prefer the file.

The same flags work on the commands that sign what they produce:

```sh
zapp dmg --app MyApp.app --sign --p12-file developer-id.p12 --p12-password-file pw
```

## Notarizing

Apple's tools take a keychain profile, or an Apple ID:

```sh
zapp notarize --target MyApp.dmg --profile my-profile --staple
zapp notarize --target MyApp.dmg --apple-id me@example.com --password app-specific --team-id TEAMID
```

rcodesign talks to the App Store Connect API and so takes an API key:

```sh
rcodesign encode-app-store-connect-api-key -o key.json <issuer-id> <key-id> AuthKey.p8
zapp notarize --target MyApp.dmg --api-key-file key.json --staple
```

An `.app` bundle is archived before it is submitted either way, because the
notary service takes an archive rather than a directory. The ticket is stapled
to the bundle, not to the archive.

## How it is put together

```
pkg/signing/             the Backend interface, Select, and the archiving that
                         notarization needs whichever backend runs
pkg/signing/macos/       codesign, productsign, notarytool, and the keychain
pkg/signing/rcodesign/   rcodesign
```

The backends know nothing of the package above them: each takes the options it
actually needs, and `Select` translates the credentials a command gathered into
whichever backend is going to run. That keeps the dependency one-way and means
neither backend carries fields the other uses.

## What is verified

The rcodesign path was exercised against rcodesign 0.29.0 with a self-signed
certificate: `zapp sign` produced an app bundle that Apple's own
`codesign --verify --deep` accepts, with the hardened runtime flag set.

Notarization against either service needs an Apple account and has not been
exercised here; only the command construction is covered by tests.
