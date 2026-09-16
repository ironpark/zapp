# macOS filesystem image writers

- [`hfsplus`](hfsplus): HFS+ writer (previously `pkg/hfsplus`).
- [`apfs`](apfs): APFS writer, with case-insensitive and case-sensitive volumes.

Both packages plan a volume from a file tree and stream its raw bytes without
CGO, mounting, or Apple tools. [`dmg`](../dmg) supplies Finder metadata and wraps
the filesystem with [`udif`](../udif) compression. Import HFS+ from
`github.com/ironpark/zapp/pkg/macfs/hfsplus`; its API is otherwise unchanged.

```sh
CGO_ENABLED=0 go test ./pkg/macfs/... ./pkg/dmg ./pkg/udif ./cmd/zapp
```

Tests run format checks on every platform. On macOS they also use `hdiutil`,
`fsck_apfs`, and a small Carbon test oracle to verify mounted data and resolve
Finder background aliases. CI mounts Linux-generated images on macOS.
