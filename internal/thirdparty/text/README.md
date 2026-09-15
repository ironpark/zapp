# Vendored golang.org/x/text subset

Source: `golang.org/x/text` **v0.42.0**

Module checksum: `h1:JbOZXgfeCPU9gacVtYliJqOhD+zhrEqK4LfdpmlUZqI=`

Only the runtime source of these packages is included:

- `unicode/norm`: used by `pkg/dsstore` for `norm.NFD.String`.
- `transform`: required by `unicode/norm`.

Upstream generators, tests, and unrelated packages are excluded. Both generated
normalization tables are retained: upstream build constraints select Unicode
15.0.0 before Go 1.27 and Unicode 17.0.0 starting with Go 1.27. This preserves
the pinned module's behavior across the project's supported Go versions.

Local changes to upstream runtime code are limited to:

1. Replacing import paths and canonical import comments with
   `github.com/ironpark/zapp/internal/thirdparty/text/...`.
2. Removing `go:generate` directives, since their generators are not vendored.
3. Omitting three private helpers used only by excluded upstream tests:
   `reorderBuffer.flush`, `isHangulWithoutJamoT`, and `Properties.combinesForward`.

The upstream copyright headers, `LICENSE` (BSD-3-Clause), and `PATENTS` are
retained. Release archives also include these notices.

This is an internal source copy, not Go's module-wide `vendor/` mode. Other
dependencies retain their existing module resolution. `golang.org/x/text` is
not required by `go.mod` or downloaded to build zapp.

## Updating

Copy non-test `.go` files from both packages of the selected upstream version,
excluding files with `//go:build ignore`. Include every Go-version-specific
runtime table, retain all copyright headers, and copy `LICENSE` and `PATENTS`.
Apply the import/generation and test-helper adjustments above, update the version and
checksum in this file, then run the normalization tests, repository tests, and
`golangci-lint run`. Check the upstream package dependency graph for newly
required runtime packages before updating.
