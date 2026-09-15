# Running zapp on Linux with Darling

zapp is portable Go, but the work it does is macOS work: it drives `hdiutil`,
`codesign`, `otool` and friends. [Darling](https://www.darlinghq.org/) supplies
those tools on Linux, so zapp builds and runs as a native Linux binary and hands
each tool invocation to Darling.

**Verdict: the two things zapp exists for do not work.** Darling cannot create
disk images or packages, and its `codesign` signs nothing. What works is the
metadata handling and the Mach-O tooling. The measurements below were taken
against Darling built from source (commit of September 2026) on x86-64 Linux,
kernel 5.15.

## What works

| Command | Tools it needs | Result |
| --- | --- | --- |
| `zapp plist get` / `set` | none | **Works.** No macOS tool involved. |
| `zapp info` | none | **Works.** |
| `zapp dep` | `otool`, `install_name_tool` | **Works.** Both are real, from cctools, and report correctly. |
| `zapp sign` | `security`, `codesign` | **Refused.** `security find-identity` is real; `codesign` is a stub (see below). |
| `zapp dmg` | `hdiutil` | **Fails.** Darling's `hdiutil` implements only `attach` and `detach`, not `create` or `convert`. |
| `zapp pkg` | `pkgbuild`, `productbuild` | **Fails.** Neither tool exists. |
| `zapp notarize` | `notarytool`, `stapler` | **Fails.** `xcrun` resolves `notarytool` but cannot execute it; `stapler` is absent. |

Tool inventory, as measured:

```
hdiutil            present (attach/detach only)
otool              present
install_name_tool  present
codesign           present (stub)
security           present
xcrun              present
SetFile            present
pkgbuild           missing
productbuild       missing
productsign        missing
stapler            missing
sips               missing
```

`sips` being absent is worth noting: zapp used to shell out to `sips`, `DeRez`,
`Rez` and `SetFile` to attach a custom icon. It now writes the resource fork and
Finder flags itself, so that path no longer depends on tools Darling lacks.

## The codesign stub

Darling's `codesign` prints a notice and **exits zero without signing
anything**:

```console
$ darling shell sh -c 'codesign --force --sign X /usr/bin/otool; echo EXIT=$?'
codesign DID NOT ACTUALLY VERIFY THE SIGNATURE OF ANY CODE THIS IS JUST A STUB
EXIT=0
```

An exit status alone would therefore have zapp report a successful signing of an
unsigned artifact. `codesign.CodeSign` checks the output for that notice and
returns `ErrCodesignStub` instead:

```
codesign is a stub that did not sign anything; signing needs a real macOS
codesign, which Darling does not provide: ./MachO
```

## Building

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o zapp .
```

zapp has no cgo, so this cross compiles from any host and links statically.

Without Darling installed, the tool-driven commands stop with:

```
failed to create dmg: zapp drives the macOS command line tools, which on Linux
come from Darling, but "darling" is not on PATH: exec: "darling": executable
file not found in $PATH
```

## How tool invocation works

On Linux, every external tool goes through one wrapper, in
`pkg/mactools/internal/macexec`:

```
darling shell /bin/sh -c "cd '<translated cwd>' && exec 'hdiutil' 'create' ..."
```

Two translations happen:

- **Paths.** Inside Darling the Linux filesystem is not the root; it is mounted
  at `/Volumes/SystemRoot`. Absolute arguments are rewritten, so
  `/home/u/App.app` is passed as `/Volumes/SystemRoot/home/u/App.app`. Arguments
  of the form `--flag=/path` are rewritten in place.
- **Working directory.** The command runs under `sh -c` purely so the working
  directory can be set, since zapp passes relative paths in places and Darling
  would otherwise resolve them against the container's home directory.

Arguments are single-quoted, so values that are not paths — a signing identity,
a volume name — pass through as data. The logic is a pure function in
`darling.go` with tests that run on any platform; only the decision to use it
sits behind a build tag.

Error messages report the logical command, not the Darling wrapping:

```
failed to create dmg: hdiutil create -volname Fixture -srcfolder /tmp/... -ov
-format UDRW ./Out.dmg failed: exit status 1 (output: Usage: hdiutil <action> ...)
```

## Extended attributes

A disk image's custom icon lives in `com.apple.ResourceFork` and
`com.apple.FinderInfo`. On Linux, unprivileged extended attributes live in the
`user.` namespace, so `xattr_linux.go` writes `user.com.apple.ResourceFork`.

This mapping is **unverified** — `zapp dmg` cannot get far enough on Darling to
exercise it. If Darling maps names differently, change `linuxAttrPrefix` in
`pkg/mactools/dmg/xattr_linux.go`.

## Reproducing the measurements

Darling has no prebuilt packages; it builds from source and wants 16 GB of disk
and 4 GB of RAM. Two things the Ubuntu 22.04 instructions omit:

- `libcap2-bin` is required (cmake fails with `Could NOT find Setcap`). It
  appears in the Debian package list but not the Ubuntu one.
- `dsymutil` is needed; install `llvm-15` and symlink
  `/usr/lib/llvm-15/bin/dsymutil`.

In a container, Darling cannot create its prefix on an overlay filesystem
(`Cannot mount overlay: Invalid argument`). Put the prefix on a real filesystem:

```sh
podman volume create dprefix
podman run -d --name darling --privileged --network host --device /dev/fuse \
  -v dprefix:/dprefix -e DPREFIX=/dprefix/p <image> sleep infinity
```

## If you want disk images on Linux

Darling is not the path. `hdiutil create` is the blocker, and it is closed
source. Options worth considering instead:

- Build the image with a native Linux HFS+/APFS image writer, rather than
  driving `hdiutil` at all. This is what `libdmg-hfsplus` and similar projects
  do, and it would make `zapp dmg` work on Linux without Darling — the
  `.DS_Store`, alias record and icon encoding are already pure Go.
- Keep signing and notarization on macOS, where they have to happen anyway.
