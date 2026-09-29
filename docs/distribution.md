# Distributing and uploading

A project can finish a build by archiving the notarized app as a ZIP, listing
the checksums of what it built, publishing it in a Sparkle appcast, and sending
the artifacts to an HTTP endpoint or a GitHub release: your own server, a presigned S3 or
R2 URL, or any service that accepts a PUT or a multipart POST.

```yaml
zip:
  out: dist/${app.name}-${app.version}.zip   # default: <out>/<app name>.zip

checksums:
  out: dist/SHA256SUMS            # default: <out>/SHA256SUMS

upload:
  - url: https://releases.example.com/${app.name}/${app.version}/${file.name}
    method: PUT                   # PUT (file as the body) or POST (multipart)
    field: file                   # form field of a POST
    headers:
      Authorization: Bearer ${env:RELEASE_TOKEN}
    artifacts: [zip, dmg]         # default: every artifact the build made
```

## Order

```
dep → sign app → [zip: notarize app → staple → archive] → DMG → PKG
    → sign installers → notarize installers → [checksums] → [appcast] → [upload]
```

With a `zip` section and `staple: true`, the app is notarized and stapled
before it is archived and packaged, so the app inside the ZIP, DMG and PKG is
stapled and Gatekeeper accepts it offline. Without stapling, the ZIP itself
is submitted for notarization, so the app is archived only once.

The archive keeps Unix permissions and stores symbolic links as links, as
`ditto -c -k --keepParent` does, so a framework's `Versions/Current` link does
not break the app's code signature.

Windows file systems keep no execute bits, so on Windows zapp gives them back
in the ZIP, DMG and PKG alike: a file is executable when it sits in a
bundle's `MacOS` directory, is a Mach-O binary or starts with `#!`. Other
files get `0644` and directories `0755`, and a link's target is written with
`/` whatever the host's separator.

A framework's `Versions/Current`, and every entry beside `Versions`, must be
a symbolic link: the code signature seals them as links. Git on Windows checks
a link out as a text file holding its target unless `core.symlinks` is true
(which needs Developer Mode), and copying or unpacking with a tool that
follows links leaves a copy in its place. Either breaks the signature on the
Mac, so zapp checks the app's frameworks before it signs or packages the app,
and stops with the entries at fault. Build such an app on macOS or Linux, or
bring it over with its links, as a tar or a `ditto` archive.

## Checksums

`checksums` writes the SHA-256 of the ZIP, DMG and PKG the build made, one
line per file by name, as `shasum -a 256` prints them. They are taken last,
from the stapled artifacts people download. Check a download with:

```sh
shasum -a 256 -c SHA256SUMS --ignore-missing
```

Uploads send the list with the artifacts, as `checksums`.

## Sparkle appcasts

`appcast` adds the build to the feed a [Sparkle](https://sparkle-project.org)
app checks for updates, signing the download with the app's EdDSA key:

```yaml
appcast:
  url: https://dl.example.com/${app.version}/${file.name}   # where the download will be
  artifact: zip                   # zip (default) or dmg
  feed: https://example.com/appcast.xml   # the published feed to extend
  releaseNotes: https://example.com/notes/${app.version}.html
  out: dist/appcast.xml           # default: <out>/appcast.xml
upload:
  - url: https://dl.example.com/${app.version}/${file.name}
    artifacts: [zip]
  - url: https://example.com/${file.name}
    headers:
      Authorization: Bearer ${env:RELEASE_TOKEN}
    artifacts: [appcast]
```

- The release is read from the app's Info.plist: `CFBundleVersion`, which
  Sparkle compares, `CFBundleShortVersionString` and `LSMinimumSystemVersion`.
- The private key comes from `ZAPP_SPARKLE_KEY` or a `keyFile`, in the base64
  form Sparkle's `generate_keys -x` exports; the project cannot hold it. It is
  checked against the app's `SUPublicEDKey` before anything is built, since an
  update signed with another key is one the app refuses. The signature is the
  one Sparkle's `sign_update` makes.
- The new release goes first. The rest of the feed, a published `feed` URL or
  path, or else the `out` written last time, is kept as it was, except a
  release of the same `CFBundleVersion`, which is replaced. A feed URL that
  answers 404 starts a new appcast.
- `url` is written into the feed, not checked: upload the artifact there, as
  above, or name the GitHub release download URL,
  `https://github.com/OWNER/REPO/releases/download/TAG/${file.name}`.
- Uploads send the appcast as the `appcast` artifact. Endpoints, and the
  artifacts within one, are sent in order, so list the appcast after the
  download it points to, and a failed download upload never publishes it.

## Uploads

- `upload` is a list; each endpoint receives the artifacts it names.
- `${file.name}` is each file's name, URL-encoded. The other project
  variables, such as `${app.version}`, work as anywhere else.
- A PUT sends the file as the body with a matching `Content-Type`; a POST
  sends it as one multipart field. Both send a `Content-Length`.
- Network errors and 408, 429 and 5xx responses are retried twice, after 2 and
  4 seconds. Any other response outside 2xx fails the build and quotes the
  start of the server's answer.
- An endpoint that names an artifact the build did not make skips it; an
  endpoint that receives nothing fails.

### Credentials

A header whose name suggests a credential (containing `auth`, `cookie`,
`token`, `secret`, `key`, `password` or `signature`) must be read from the
environment with `${env:NAME}`; a literal value is rejected when the project
is loaded. `zapp config show` prints such headers as `<redacted>` and drops
the user information and query string of upload URLs, where presigned URLs
keep their signatures. Logs and outputs leave them out as well.

## GitHub releases

An upload with `github` instead of `url` adds the artifacts to a GitHub
release as assets:

```yaml
upload:
  - github:
      repo: me/my-app             # default: $GITHUB_REPOSITORY
      tag: v${app.version}        # default: the tag a GitHub Actions run was started for
      draft: true                 # only for a release zapp creates
    artifacts: [zip, dmg, checksums]
```

- The release is found by its tag, drafts included, and created when there is
  none; `draft` applies to that case only. Create the tag first, or GitHub
  makes it from the default branch when the release is published.
- An asset of the same name is replaced, so a build can be run again.
- The token comes from `GITHUB_TOKEN` or `GH_TOKEN` and needs write access to
  the repository's contents; the build stops before building anything when
  neither is set. `GITHUB_API_URL` points zapp at GitHub Enterprise Server.
- The reported URL is the asset's download URL. GitHub replaces spaces in an
  asset's name with dots.

## Command line

```sh
zapp build                    # runs zip and upload when the project has them
zapp build dmg zip checksums upload   # choose the steps
zapp build --no-upload        # everything but the upload
zapp build --zip --upload-url 'https://example.com/${file.name}' \
  --upload-header "Authorization: Bearer $TOKEN" dmg
```

| Flag | Environment | Meaning |
| --- | --- | --- |
| `--zip` | `ZAPP_ZIP` | Archive the app (`--zip=false` skips the project's `zip`) |
| `--appcast` | `ZAPP_APPCAST` | `--appcast=false` skips the project's `appcast`; its settings live in the project |
| — | `ZAPP_SPARKLE_KEY` | Sparkle's private EdDSA key, base64 |
| `--checksums` | `ZAPP_CHECKSUMS` | List the artifacts' SHA-256 (`--checksums=false` skips the project's `checksums`) |
| `--upload-url` | `ZAPP_UPLOAD_URL` | Adds an endpoint to the project's |
| `--upload-method` | `ZAPP_UPLOAD_METHOD` | `PUT` or `POST` |
| `--upload-field` | `ZAPP_UPLOAD_FIELD` | Form field of a POST |
| `--upload-header` | `ZAPP_UPLOAD_HEADER` | `"Name: value"`, repeatable; the variable holds one per line |
| `--github-release` | `ZAPP_GITHUB_RELEASE` | Adds a GitHub release, by tag, to the project's uploads |
| `--github-repo` | `ZAPP_GITHUB_REPO` | Its `owner/name` (default: `$GITHUB_REPOSITORY`) |
| `--upload-artifacts` | `ZAPP_UPLOAD_ARTIFACTS` | `zip`, `dmg`, `pkg`, `checksums`, `appcast`, for the endpoints above |
| `--no-upload` | `ZAPP_NO_UPLOAD` | Skip every upload |

Headers given on the command line or in the environment are runtime values,
so they may be literal. When steps are named, `--zip`, `--checksums`,
`--appcast`, `--upload-url` and `--github-release` add their steps to them.

`--artifacts FILE` writes `zip=`, `checksums=` and `appcast=` next to the other
paths, and the URL each artifact was uploaded to as `zip-url=`, `dmg-url=`,
`pkg-url=`, `checksums-url=` and `appcast-url=`.

`zapp upload` sends existing files with the same flags, to an endpoint or a
release (`zapp upload --github-release v1.2.0 MyApp.zip`):

```sh
zapp upload --upload-url 'https://example.com/${file.name}' \
  --upload-header "Authorization: Bearer $TOKEN" MyApp.zip MyApp.dmg
```
