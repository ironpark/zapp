# macpkg

Build unsigned macOS component and product installer packages in Go, on macOS
or Linux. No CGO, Apple command-line tools, or third-party packaging library is
used at runtime. `zapp pkg` uses this package; signing and notarization continue
through zapp's existing signing backends.

## Component and product packages

```go
ctx := context.Background()

err := macpkg.BuildComponent(ctx, macpkg.ComponentConfig{
    Root:            "./dist",
    RootEntry:       "MyApp.app", // Include this child, not its siblings.
    Identifier:      "com.example.myapp",
    Version:         "1.2.3",
    InstallLocation: "/Applications",
    OutputPath:      "./MyApp-component.pkg",
})
if err != nil {
    return err
}

return macpkg.BuildProduct(ctx, macpkg.ProductConfig{
    Packages:     []string{"./MyApp-component.pkg"},
    OutputPath:   "./MyApp.pkg",
    ResourcesDir: "./installer-resources",
    Distribution: &macpkg.Distribution{
        Title:       "MyApp",
        LicenseFile: "license.txt",
    },
})
```

Import `github.com/ironpark/zapp/pkg/macpkg`. Resource layouts may include
`en.lproj/license.txt`, `ko.lproj/license.txt`, or a top-level `license.txt`.
Omit `LicenseFile` when there is no license. Files referenced by a license must
exist in the resources directory or at least one locale directory.

With no `RootEntry`, all contents of `Root` are installed relative to
`InstallLocation` (default `/`). `RootEntry` is an immediate child name, not an
arbitrary path. Output must be outside the selected input tree. Input files must
remain unchanged throughout the build. `.app` directories need a valid
`Contents/Info.plist` and `CFBundleIdentifier`; bundle versions are included in
package metadata, and app upgrades use the specified install location without
automatic relocation.

## Scripts and multiple components

Set `ScriptsDir` to include executable `preinstall` and/or `postinstall` scripts
and their auxiliary files. Entry scripts must be regular executable files;
their Installer timeout is 600 seconds. Leave `Root` empty for a scripts-only
package, which must contain at least one entry script. Script execution occurs
only during installation, never while building a package.

`ProductConfig.Packages` accepts multiple flat component packages, including
Apple-created components. Base names and package identifiers must be unique.
Product packages cannot themselves be supplied as components. Archived member
bytes are copied without decompressing and recompressing the payload.

An empty `Distribution.Choices` installs every component without customization.
For explicit choices, set `Selected: true` for choices selected by default:

```go
Distribution: &macpkg.Distribution{
    Title: "MyApp Suite",
    Choices: []macpkg.Choice{
        {ID: "app", Title: "Application", Visible: true, Selected: true,
            PackageIDs: []string{"com.example.myapp"}},
        {ID: "extras", Title: "Extras", Visible: true, Selected: false,
            PackageIDs: []string{"com.example.extras"}},
    },
},
```

Every component must belong to exactly one generated choice. For more advanced
Installer logic, supply `DistributionXML` instead of `Distribution`. XML is
preserved, with syntax and local package bindings checked. Each package needs a
`pkg-ref` definition whose ID matches its `PackageInfo` and whose text names its
archive, e.g. `#MyApp-component.pkg`. Remote references are rejected. Caller
JavaScript is neither executed nor semantically validated by macpkg. Custom XML
must declare an adequate `os-version min` when components require an OS newer
than 10.9. Resources support regular files and directories, not symlinks.

## Formats and limits

* XAR v1 container, zlib-compressed XML TOC, SHA-1 TOC/member integrity checksums.
  These checksums describe the archive format, not a signing or trust mechanism.
* gzip-compressed odc CPIO payload and scripts; binary BOM includes POSIX `cksum`,
  multi-page path trees, and `Size64` for file sizes above 32 bits.
* `PayloadMode: Auto` (default) selects `Large` when any file is at least 8 GiB.
  `Legacy` rejects those files. `Large` uses `LargeSegmentedPayload`, with regular
  files split into at most 1 GiB records. It works with smaller files too.
* Default minimum OS is 10.9 for legacy payloads, 12.0 for large payloads. An
  explicitly lower version is rejected. Products inherit the highest component
  minimum. Large payload mode does not require a newer compression algorithm.
* File contents, POSIX permissions, whole-second modification times, symlinks,
  and hard links within the selected input are preserved. Ownership defaults to
  UID/GID 0 (`root:wheel`); use `PreserveOwnership` to retain source IDs. Linux
  UID/GID numbers are carried literally, not mapped to macOS account names.
* Extended attributes, ACLs, resource forks, device nodes, sockets, and FIFOs
  are outside this version's metadata support. Extended metadata is omitted;
  special file types are rejected. Symlink targets are stored without following
  them. Source-side AppleDouble files, if present, are ordinary input files.
* odc CPIO has 18-bit UID/GID, name-length, and inode fields. Inputs exceeding
  these limits are rejected, including too many files or large-file segments.
  Timestamps must fit unsigned 32-bit seconds. XAR TOCs are limited to 32 MiB;
  PackageInfo, Distribution, and bundle plist metadata to 16 MiB.
* File data is streamed with bounded buffers. Metadata memory scales with file
  count. Temporary disk holds compressed payloads and the assembled XAR heap;
  allow space for several copies of the compressed output. Existing output is
  replaced only after successful construction, sync, and close. Cancellation and
  failures remove temporary files.

This is a core build API, not full option compatibility with Apple's CLIs.
Package signing, notarization, latest Apple compression codecs, bundle component
plists, and automatic bundle relocation are not implemented here.

## Verification

```sh
CGO_ENABLED=0 go test ./pkg/macpkg ./pkg/mactools/installer
```

On macOS, normal tests also use `pkgutil --expand-full`, `lsbom`, `pkgbuild`,
`productbuild`, and the read-only `installer -showChoicesXML` operation. The
Installer check needs access to system volume information (a restrictive sandbox
may prevent it). No normal test installs software or accesses signing keys.

```sh
# Requires roughly 10 GiB free space; creates and restores >8 GiB plus a hard link.
MACPKG_LARGE_TEST=1 go test -run '^TestLargeFileIntegration$' -timeout 10m ./pkg/macpkg

# Run as root ONLY in a disposable macOS VM. Tests first install, scripts,
# app upgrade/removal of obsolete files, then forgets its test receipt.
MACPKG_INSTALL_TEST=1 go test -run '^TestInstallInDisposableVM$' ./pkg/macpkg

# Optional: use an already configured Developer ID Installer identity.
MACPKG_SIGN_IDENTITY='Developer ID Installer: Example (...)' \
  go test -run '^TestProductsignCompatibility$' ./pkg/macpkg
```

`MACPKG_ARTIFACT_DIR=/some/directory` exports component/product fixtures during
tests. This enables testing Linux-created packages with Apple tools on macOS.
The CI workflow runs both platforms and checks the Linux artifact on macOS;
large-file tests are available by manual workflow dispatch.

Format references: [Apple XAR source](https://github.com/apple-oss-distributions/xar)
and the installed `pkgbuild(1)`, `productbuild(1)`, and `lsbom(1)` manuals. BOM and
large-payload layouts are checked against locally generated Apple fixtures.
