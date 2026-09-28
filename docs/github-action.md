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
| `api-key` | App Store Connect API key JSON from `rcodesign encode-app-store-connect-api-key` | All |
| `api-key-id`, `api-issuer-id`, `api-private-key` | The same key as its three parts, the last being the `AuthKey_*.p8` contents | All |
| `apple-id`, `app-password`, `team-id` | Notarize with an Apple ID and app-specific password | macOS |

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
| `version` | The zapp version used |

The same paths are listed in the job summary.

## Examples

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

Install zapp only, and run its commands directly:

```yaml
- uses: ironpark/zapp/setup@v1
- run: zapp plist bump --patch MyApp.app/Contents/Info.plist
```
