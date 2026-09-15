# icns

Pure Go reader and writer for Apple ICNS icon families, with no external dependencies.

```go
family, err := icns.Decode(reader)
if err != nil {
    return err
}
img, err := family.HighestResolution()
if err != nil {
    return err
}

out := icns.NewICNS()
if err := out.Add(img); err != nil {
    return err
}
return icns.Encode(writer, out)
```

## API and supported formats

- `Decode` / `Encode` read and write an ordered family, preserving unknown
  elements, duplicate type codes, metadata, nested payloads and unsupported
  image data byte for byte. `Decode` reads exactly the declared family length;
  image validation is deferred until extraction.
- `Image(type)` extracts a specific four-byte type. `HighestResolution()` and
  `ByResolution(size)` select by physical pixel dimensions, preferring full
  color at equal sizes. Unsupported encodings are skipped; corrupt supported
  images return an error. An unsupported-only family returns `ErrUnsupported`.
- `Add(image)` chooses a format for the image's dimensions. `AddWithType(type,
  image)` selects a slot explicitly, including Retina variants. Neither resizes
  the image. Both snapshot the pixels, replace duplicate entries of that type,
  generate any required mask, and remove a stale `TOC ` element.
- `Element.Nested()` reads a nested family's payload without its outer header.
  Nested icons are not included in the parent's resolution selection.

| Encoding | Reading | Writing |
| --- | --- | --- |
| PNG | All modern image slots | `ic07`–`ic14`, `icp6`, `icsB`, `sb24`, `SB24` |
| RGB RLE + separate 8-bit mask | `is32`, `il32`, `ih32`, `it32`, `icp4`, `icp5` | Same |
| Uncompressed interleaved XRGB | RGB slots, alpha ignored | No; writes RLE |
| ARGB RLE | All modern slots with `ARGB` signature | `ic04`, `ic05`, `icsb` |
| 1-bit monochrome, with/without mask | All classic types | Same |
| Fixed 4/8-bit Macintosh palettes | All classic types | Same, quantized with 1-bit alpha |
| JPEG 2000 (JP2 or codestream) | Requires an `image.RegisterFormat` decoder | Preserves existing payloads |

PNG/ARGB/JPEG 2000 alpha takes precedence over external masks. Missing legacy
masks are treated as opaque; malformed present masks return errors. Mask
position in the family does not matter. `it32` has a four-byte prefix, ignored
when decoding. ARGB output includes a trailing zero byte for the ARM rendering
workaround documented by icns-analysis.

`Add` uses RGB and masks at 16/32/48 pixels, PNG at 128/256/512/1024 pixels,
and `ic12` at 64 pixels (32 points at 2x density). It also accepts 16x12, 18,
24 and 36 pixel icons. Use `AddWithType` to include both normal and Retina
representations. `icp6` is supported for explicit use and reading, but is not
selected automatically because standalone and app-bundle rendering differ.
No API promises compatibility with a particular historical macOS release.

Families are limited to 64 MiB and 4096 elements. PNG/registered-decoder
image dimensions are checked against the slot before decoding pixels. RLE
packets must fit within their channel. `ErrFormat`, `ErrUnsupported`, and
`ErrNoImage` support `errors.Is`; I/O errors are propagated or wrapped.

## References and tests

The format and compatibility decisions follow:

- [mdsteele/rust-icns](https://github.com/mdsteele/rust-icns): type layout,
  palettes, RLE and independent golden ICNS/PNG fixtures.
- [relikd/icns-analysis](https://github.com/relikd/icns-analysis): signatures,
  masks, uncompressed XRGB, nested families and macOS rendering behavior.

`testdata/icns` and `testdata/png` are from rust-icns (retrieved 2026-09-15).
Its MIT notice is in `testdata/LICENSE.rust-icns`; the same notice covers the
reference palette data. Tests compare decoded pixels against those independent
PNG fixtures, exercise every supported type, malformed data, I/O failures,
unknown-element preservation, and fuzz the parser and RLE codec.

```sh
go test ./pkg/icns ./cmd/dmg
go test ./pkg/icns -run '^$' -fuzz '^FuzzDecode$' -fuzztime 10s
go test ./pkg/icns -run '^$' -fuzz '^FuzzRLE$' -fuzztime 10s
```
