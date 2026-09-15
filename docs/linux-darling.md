# Running zapp on Linux with Darling

zapp itself is portable Go, but the work it does is macOS work: it drives
`hdiutil`, `codesign`, `otool` and friends, which only exist on Darwin.
[Darling](https://www.darlinghq.org/) supplies those tools on Linux, so zapp
builds and runs natively on Linux and hands each tool invocation to Darling.

**Nothing here has been tried against Darling.** It was developed on macOS,
where Darling does not run. What *was* verified is everything up to the point
Darling takes over: a Linux binary was built and run in an x86-64 container, and
the sections below distinguish what is known to work from what is not.

Treat the Darling parts as a starting point, not a supported configuration.

## What works on Linux without Darling

Some commands touch no macOS tool at all, and work on a plain Linux box. Both of
these were run in an Alpine container against a fixture app bundle:

```console
$ zapp plist get ./Fixture.app CFBundleIdentifier
dev.zapp.fixture
$ zapp plist set ./Fixture.app CFBundleShortVersionString 7.7.7
Value set successfully
$ zapp info
[Build Info]
...
```

`zapp dmg` gets surprisingly far too: the icon is decoded and resized, the
Finder window settings and the background alias record are encoded, and only
then does it need `hdiutil`. All of that is Go, and all of it ran on Linux.

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
see https://docs.darlinghq.org/installing-software.html
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
  of the form `--flag=/path` are rewritten in place. Relative paths are left
  alone.
- **Working directory.** The command runs under `sh -c` purely so the working
  directory can be set, since zapp passes relative paths in places and Darling
  would otherwise resolve them against the container's home directory.

Arguments are single-quoted, so values that are not paths — a signing identity,
a volume name — pass through as data.

The logic is in `darling.go` and is covered by tests that run on any platform;
only the decision to use it is behind a build tag (`exec_linux.go` versus
`exec_darwin.go`).

## Extended attributes

A disk image's custom icon lives in two extended attributes,
`com.apple.ResourceFork` and `com.apple.FinderInfo`. zapp writes them directly
rather than shelling out. On Linux, unprivileged extended attributes live in the
`user.` namespace, so `xattr_linux.go` writes `user.com.apple.ResourceFork`.

**Assumption to verify:** that this is the mapping Darling uses when a macOS
program reads the same attribute. If Darling maps names differently, adjust
`linuxAttrPrefix` in `pkg/mactools/dmg/xattr_linux.go`.

## What to check on a Darling host

In rough order of how likely each is to work:

| Command | Tools it needs | Notes |
| --- | --- | --- |
| `zapp dep` | `otool`, `install_name_tool` | Both are cctools, which is open source, so these are the most likely to be present. |
| `zapp dmg` | `hdiutil` | Darling documents attaching and detaching disk images. Creating one, and writing into the attached volume, is the open question — see below. |
| `zapp pkg` | `pkgbuild`, `productbuild` | Closed-source Apple tools. |
| `zapp sign` | `codesign`, `security`, `productsign` | Needs a working keychain as well as the tools. |
| `zapp notarize` | `xcrun notarytool`, `stapler` | Also needs network access and Apple credentials. |

### The mount visibility question

`zapp dmg` attaches the image it just created, writes the Finder window settings
and the background image into the mounted volume, then detaches. Under Darling,
`hdiutil` runs inside a container, and a mount it makes there may not be visible
to zapp, which is a native Linux process outside it.

If that is the case, `zapp dmg` reports:

```
the volume attached at <path> appears empty from this process, so its contents
cannot be customised
```

rather than quietly producing a disk image with no background and no window
layout. If you hit this, the options are to run zapp itself inside Darling
(`darling shell zapp dmg ...`, using a darwin build), or to teach the dmg
package to do the in-volume work through Darling as well.

### Reporting results

If you try this, the useful things to record are: which of the table's commands
ran at all, whether the mount is visible, and whether the custom icon survives
being read back by a macOS program.
