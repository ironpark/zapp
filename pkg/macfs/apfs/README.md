# APFS image writer

This package writes a single, unencrypted APFS volume in a raw container. It
uses 4096-byte blocks, a complete initial checkpoint, allocation bitmaps,
object maps, and multi-level filesystem/extent trees. File data is streamed;
metadata is planned in memory. It does not read, edit, encrypt, or snapshot
existing filesystems.

```go
volume := apfs.Volume{
    Name: "My App",
    Root: &apfs.Node{
        Mode: fs.ModeDir | 0755,
        Children: []*apfs.Node{
            {Name: "Read me.txt", Mode: 0644, Data: apfs.Bytes([]byte("Hello"))},
            {Name: "Applications", Mode: fs.ModeSymlink, LinkTarget: "/Applications"},
        },
    },
}
image, err := apfs.Plan(ctx, volume)
// Handle err, then stream image.WriteTo(ctx, output).
```

The default ignores case. Set `CaseSensitive: true` to distinguish `a` from
`A`; both modes are normalization insensitive. Names and comparison hashes
use Unicode 9.0.0, independent of the Go runtime's Unicode version. Names must
be valid UTF-8 and at most 255 bytes. Symbolic link targets must fit within
macOS's 1024-byte path limit (including the terminating zero).

`AssignIDs` validates and numbers nodes before layout, so a caller can build
metadata referring to those IDs. `Plan` repeats validation and numbering;
changing data does not change IDs. Sources must remain unchanged until writing
finishes. Supply a fixed `Created` and unchanged inputs for reproducible output.
Directories default to mode 0755 when permission bits are omitted; regular
files preserve their explicit permission bits, including mode 0000.

The on-disk structures follow [Apple File System Reference](https://developer.apple.com/support/downloads/Apple-File-System-Reference.pdf).
Compatibility tests additionally check format details not fully described in
that reference, including fixed-key B-tree table capacity and space-manager
free-queue limits. FinderInfo is an extended attribute, not an inode extension.

Unicode tables are generated from the Unicode Consortium's 9.0.0
`UnicodeData.txt` and `CaseFolding.txt` by `internal/genunicode/main.py`.
Their license is included in [UNICODE-LICENSE.txt](UNICODE-LICENSE.txt).
