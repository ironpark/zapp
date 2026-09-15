package macpkg

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha1"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
)

// Change a TOC and recompute its checksum so malformed member tests reach the
// member validator, rather than merely failing the envelope integrity check.
func rewriteTOC(t *testing.T, b []byte, change func(*xarDocument)) []byte {
	t.Helper()
	oldLength := int(be.Uint64(b[8:]))
	z, err := zlib.NewReader(bytes.NewReader(b[28 : 28+oldLength]))
	if err != nil {
		t.Fatal(err)
	}
	var d xarDocument
	if err = xml.NewDecoder(z).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	change(&d)
	x, err := xml.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	if _, err := w.Write(x); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	header := bytes.Clone(b[:28])
	be.PutUint64(header[8:], uint64(compressed.Len()))
	be.PutUint64(header[16:], uint64(len(x)))
	heap := bytes.Clone(b[28+oldLength:])
	sum := sha1.Sum(compressed.Bytes())
	copy(heap, sum[:])
	out := append(header, compressed.Bytes()...)
	return append(out, heap...)
}

func TestXARRejectsMalformedMembers(t *testing.T) {
	_, c := fixture(t)
	if err := BuildComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(c.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*xarDocument){
		"traversal":        func(d *xarDocument) { d.TOC.Files[0].Name = "../escape" },
		"duplicate":        func(d *xarDocument) { d.TOC.Files = append(d.TOC.Files, d.TOC.Files[0]) },
		"negative offset":  func(d *xarDocument) { d.TOC.Files[0].Data.Offset = -1 },
		"past end":         func(d *xarDocument) { d.TOC.Files[0].Data.Length = 1 << 62 },
		"checksum overlap": func(d *xarDocument) { d.TOC.Files[0].Data.Offset = 0 },
		"symlink":          func(d *xarDocument) { d.TOC.Files[0].Type = "symlink" },
		"file children":    func(d *xarDocument) { d.TOC.Files[0].Files = []*xarFile{{Name: "child", Type: "directory"}} },
		"bad encoding":     func(d *xarDocument) { d.TOC.Files[0].Data.Encoding.Style = "unknown" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "bad.pkg")
			put(t, p, rewriteTOC(t, b, change), 0644)
			a, err := openXAR(context.Background(), p)
			if err == nil {
				if err := a.f.Close(); err != nil {
					t.Fatal(err)
				}
				t.Fatal("accepted malformed XAR")
			}
		})
	}
}

func TestProductRejectsDamagedPayloadAtomically(t *testing.T) {
	dir, c := fixture(t)
	ctx := context.Background()
	if err := BuildComponent(ctx, c); err != nil {
		t.Fatal(err)
	}
	a, err := openXAR(ctx, c.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	offset := a.heap + a.entries["Payload"].Data.Offset
	if err := a.f.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(c.OutputPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	b := []byte{0}
	if _, err = f.ReadAt(b, offset); err != nil {
		t.Fatal(err)
	}
	b[0] ^= 0xff
	if _, err = f.WriteAt(b, offset); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "product.pkg")
	put(t, output, []byte("old"), 0644)
	if err := BuildProduct(ctx, ProductConfig{OutputPath: output, Packages: []string{c.OutputPath}}); err == nil {
		t.Fatal("accepted damaged component")
	}
	b, err = os.ReadFile(output)
	if err != nil || string(b) != "old" {
		t.Fatal("replaced product on failed import")
	}
}

func FuzzOpenXAR(f *testing.F) {
	f.Add([]byte("xar!"))
	f.Add(make([]byte, 28))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		p := filepath.Join(t.TempDir(), "input.pkg")
		put(t, p, b, 0644)
		a, err := openXAR(context.Background(), p)
		if err == nil {
			defer func() { _ = a.f.Close() }()
			_, _ = a.read(context.Background(), "PackageInfo", maxMetadata)
		}
	})
}
