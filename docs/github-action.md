# GitHub Action

`ironpark/zapp` builds a project with `zapp build` on any GitHub-hosted runner:
macOS, Linux or Windows, x86_64 or arm64. `ironpark/zapp/setup` only installs
zapp, for workflows that run `zapp` commands themselves.

```yaml
- id: zapp
  uses: ironpark/zapp@v1
  with:
    certificate: ${{ secrets.CERTIFICATE_P12_BASE64 }}
    certificate-password: ${{ secrets.CERTIFICATE_PASSWORD }}
    api-key: ${{ secrets.ASC_API_KEY }}
- uses: softprops/action-gh-release@v2
  with:
    files: ${{ steps.zapp.outputs.dmg }}
```

The action downloads the zapp release matching the ref it is used at, checks it
against the release checksums and caches it for the job: `@v1` follows the
newest 1.x release and `@v1.2.0` stays on 1.2.0. Set `version` to choose
another release.

## Project and steps

Without further inputs the action builds every section of the `.zapp.yaml`
discovered from `working-directory`, in deployment order: dependencies, app
signing, DMG, PKG, installer signing and notarization. Inputs override the
project the way the matching `ZAPP_*` variables do on the command line; an
empty input leaves the project's value alone.

| Input | Meaning |
| --- | --- |
| `working-directory` | Directory to build in. Default `.` |
| `config` | Project file, instead of discovering `.zapp.yaml` |
| `steps` | Sections to build, from `dep dmg pkg`. Without a project file this alone chooses what to build |
| `app` | App bundle, overriding the project's `app` |
| `sign` / `notarize` | `false` skips the step; `true` runs it even if the project does not enable it |
| `staple` | `true` or `false`, overriding the project |
| `zip` | `true` archives the notarized, stapled app as a ZIP; `false` skips the project's `zip` |
| `appcast` | `false` skips the project's Sparkle `appcast`; `true` runs it with named `steps` (zapp 1.3.0+) |
| `checksums` | `true` writes `SHA256SUMS` for the ZIP, DMG and PKG; `false` skips the project's `checksums` (zapp 1.3.0+) |
| `upload-url` | Endpoint to send artifacts to, added to the project's `upload`; `${file.name}` is each file's name |
| `upload-method`, `upload-field` | `PUT` (default) or `POST`, and the form field of a POST |
| `upload-headers` | Headers sent with each upload, one `Name: value` per line |
| `github-release` | Tag of a GitHub release to add the artifacts to, created if missing; `github-token` needs `contents: write` (zapp 1.3.0+) |
| `upload-artifacts` | Artifacts to send to `upload-url` and `github-release`: `zip`, `dmg`, `pkg`, `checksums`, `appcast`, separated by spaces |
| `upload` | `false` skips every upload |
| `args` | Further `zapp build` options, one per line, such as `--title=My App` |
| `version` | zapp release to use, such as `1.2.0` or `latest`; `local` uses a `zapp` already on `PATH` |

## Credentials

Pass secrets through inputs; the action hands them to zapp through environment
variables, never the command line, and writes the ones zapp reads from a file
to a private temporary directory deleted when the step ends.

| Input | Used for | Runners |
| --- | --- | --- |
| `certificate` | Developer ID certificate as base64 PKCS#12 (`base64 -i certificate.p12`) | All |
| `certificate-password` | Its password | All |
| `pem` | A PEM bundle of certificate and private key, instead of `certificate` | All |
| `identity` | A keychain identity; with `certificate`, which identity in it to use | macOS |
| `entitlements` | An entitlements plist in the repository to sign the app with; by default it keeps its own (zapp 1.3.0+) | All |
| `api-key` | App Store Connect API key JSON from `rcodesign encode-app-store-connect-api-key` | All |
| `api-key-id`, `api-issuer-id`, `api-private-key` | The same key as its three parts, the last being the `AuthKey_*.p8` contents | All |
| `apple-id`, `app-password`, `team-id` | Notarize with an Apple ID and app-specific password | macOS |
| `sparkle-key` | Sparkle's private EdDSA key from `generate_keys -x`, to sign the project's appcast | All |

An App Store Connect API key is the one notarization credential every runner
accepts. Create a Team Key with the Developer role under **Users and Access →
Integrations → App Store Connect API**, and keep its issuer ID, key ID and
`.p8` file, which can be downloaded only once. See
[signing and notarization](signing.md) for how each host signs.

## Outputs

| Output | Value |
| --- | --- |
| `app` | Absolute path of the app bundle packaged |
| `dmg` | Absolute path of the DMG, empty if none was built |
| `pkg` | Absolute path of the PKG, empty if none was built |
| `zip` | Absolute path of the app ZIP, empty if none was built |
| `checksums` | Absolute path of `SHA256SUMS`, empty if none was written |
| `appcast` | Absolute path of the Sparkle appcast, empty if none was written |
| `zip-url`, `dmg-url`, `pkg-url`, `checksums-url`, `appcast-url` | Where each artifact was uploaded, without the query string |
| `version` | The zapp version used |

On Windows the paths use forward slashes (`D:/a/project/MyApp.dmg`), which bash
and PowerShell steps both accept. The same paths are listed in the job summary.

## Examples

`zip`, `upload-*` and the matching outputs need zapp 1.2.0 or later.

Build unsigned installers for pull requests and signed, notarized ones for
tags, from an app built earlier in the workflow:

```yaml
jobs:
  package:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/download-artifact@v4
        with:
          name: MyApp.app
          path: build/MyApp.app
      - id: zapp
        uses: ironpark/zapp@v1
        with:
          app: build/MyApp.app
          sign: ${{ startsWith(github.ref, 'refs/tags/') }}
          notarize: ${{ startsWith(github.ref, 'refs/tags/') }}
          certificate: ${{ secrets.CERTIFICATE_P12_BASE64 }}
          certificate-password: ${{ secrets.CERTIFICATE_PASSWORD }}
          api-key-id: ${{ secrets.ASC_KEY_ID }}
          api-issuer-id: ${{ secrets.ASC_ISSUER_ID }}
          api-private-key: ${{ secrets.ASC_PRIVATE_KEY }}
      - uses: actions/upload-artifact@v4
        with:
          name: installers
          path: |
            ${{ steps.zapp.outputs.dmg }}
            ${{ steps.zapp.outputs.pkg }}
```

Ship the notarized app as a ZIP to your own server, with a token from a
secret; with `steps` named, `zip` and `upload-url` add their steps:

```yaml
- id: zapp
  uses: ironpark/zapp@v1
  with:
    app: build/MyApp.app
    steps: dmg
    zip: true
    sign: true
    notarize: true
    staple: true
    certificate: ${{ secrets.CERTIFICATE_P12_BASE64 }}
    certificate-password: ${{ secrets.CERTIFICATE_PASSWORD }}
    api-key: ${{ secrets.ASC_API_KEY }}
    upload-url: https://releases.example.com/${{ github.ref_name }}/${file.name}
    upload-headers: |
      Authorization: Bearer ${{ secrets.RELEASE_TOKEN }}
- run: echo "Published ${{ steps.zapp.outputs.zip-url }}"
```

Attach the notarized ZIP, the DMG and their checksums to the release of the
tag being built:

```yaml
on:
  push:
    tags: ['v*']
permissions:
  contents: write
jobs:
  release:
    runs-on: macos-latest
    steps:
      # ... build the app
      - uses: ironpark/zapp@v1
        with:
          app: build/MyApp.app
          steps: dmg
          zip: true
          checksums: true
          sign: true
          notarize: true
          staple: true
          certificate: ${{ secrets.CERTIFICATE_P12_BASE64 }}
          certificate-password: ${{ secrets.CERTIFICATE_PASSWORD }}
          api-key: ${{ secrets.ASC_API_KEY }}
          github-release: ${{ github.ref_name }}
```

See [distributing and uploading](distribution.md) for the project's `zip`,
`checksums`, `appcast` and `upload` sections.

Install zapp only, and run its commands directly:

```yaml
- uses: ironpark/zapp/setup@v1
- run: zapp plist bump --patch MyApp.app/Contents/Info.plist
```
