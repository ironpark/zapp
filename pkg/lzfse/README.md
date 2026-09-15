# lzfse

Compress data in Apple's LZFSE format in Go, without CGO or Apple's libraries.
Compression only: these streams are written to be read by macOS, which already
has a decoder. `zapp dmg --format ulfo` uses this package.

```go
compressed := lzfse.Encode(nil, data)
```

Reuse an `Encoder` when compressing many buffers. It holds the match finder's
history table, which is the bulk of the working set, and it may not be used
from two goroutines at once.

```go
e := lzfse.NewEncoder()
for _, chunk := range chunks {
    out = e.Encode(out[:0], chunk)
}
```

## Format

A stream is a sequence of blocks ending with an end-of-stream marker. Data that
compression would not shrink is stored in a plain block rather than made larger,
so `Encode` never returns much more than it was given.

Compressed blocks carry the data as a run of literals and a list of (literal
count, match length, match distance) triples, each coded with finite state
entropy against frequency tables stored in the block header. Literals are coded
as four interleaved streams. Both payloads are written from the last symbol to
the first, which is what lets a decoder read them forwards.

## Correctness

The output is checked against the reference implementation at
[github.com/lzfse/lzfse](https://github.com/lzfse/lzfse): build a decoder from
it that reads a stream on standard input, then

```
LZFSE_REFERENCE_DECODER=/path/to/decoder go test ./pkg/lzfse
```

The `udif` package tests go further on macOS, where `hdiutil` reads back a disk
image compressed with this package and recomputes its checksums, which means
Apple's own decoder has decompressed every chunk of it.

## Performance

Roughly 160 MB/s on an M-series core, producing streams within a fraction of a
percent of the size the reference encoder produces.

Building with `GOEXPERIMENT=simd` swaps in a match extension that compares a
vector at a time. It is not currently a win; `matchlen_simd.go` records what was
measured and why.
