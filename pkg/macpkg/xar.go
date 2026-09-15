package macpkg

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

const maxTOC = 32 << 20
const maxMetadata = 16 << 20

type xarSum struct {
	Style string `xml:"style,attr"`
	Value string `xml:",chardata"`
}
type xarEncoding struct {
	Style string `xml:"style,attr"`
}
type xarData struct {
	Length    int64       `xml:"length"`
	Offset    int64       `xml:"offset"`
	Size      int64       `xml:"size"`
	Encoding  xarEncoding `xml:"encoding"`
	Archived  xarSum      `xml:"archived-checksum"`
	Extracted xarSum      `xml:"extracted-checksum"`
}
type xarFile struct {
	ID    int        `xml:"id,attr"`
	Name  string     `xml:"name"`
	Type  string     `xml:"type"`
	Mode  string     `xml:"mode,omitempty"`
	UID   int        `xml:"uid"`
	GID   int        `xml:"gid"`
	Mtime string     `xml:"mtime,omitempty"`
	Data  *xarData   `xml:"data,omitempty"`
	Files []*xarFile `xml:"file"`
}
type xarTOCSum struct {
	Style  string `xml:"style,attr"`
	Offset int64  `xml:"offset"`
	Size   int64  `xml:"size"`
}
type xarDocument struct {
	XMLName xml.Name `xml:"xar"`
	TOC     struct {
		Checksum xarTOCSum  `xml:"checksum"`
		Files    []*xarFile `xml:"file"`
	} `xml:"toc"`
}
type archiveEntry struct {
	name   string
	source string // Opened only while copying this member.
	mode   string
	mtime  string
	r      io.Reader // nil for directories
	raw    *xarData  // Already encoded XAR member, copied without recompression.
}

func writeXAR(ctx context.Context, out io.Writer, work string, entries []archiveEntry) error {
	heap, err := os.CreateTemp(work, "heap-*")
	if err != nil {
		return err
	}
	defer func() { _ = heap.Close() }()
	// Buffer the heap and track the write offset directly; both the member
	// copies and the final spill are otherwise syscall-bound.
	hw := bufio.NewWriterSize(heap, 1<<20)
	buf := make([]byte, 1<<20)
	pos := int64(20)
	if _, err = hw.Write(make([]byte, 20)); err != nil {
		return err
	}
	doc := xarDocument{}
	doc.TOC.Checksum = xarTOCSum{"sha1", 0, 20}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	nodes := map[string]*xarFile{}
	id := 0
	var addDir func(string) (*xarFile, error)
	add := func(name string, n *xarFile) error {
		parent := path.Dir(name)
		if parent == "." {
			doc.TOC.Files = append(doc.TOC.Files, n)
		} else {
			p, err := addDir(parent)
			if err != nil {
				return err
			}
			p.Files = append(p.Files, n)
		}
		nodes[name] = n
		return nil
	}
	addDir = func(name string) (*xarFile, error) {
		if n := nodes[name]; n != nil {
			if n.Type != "directory" {
				return nil, fmt.Errorf("XAR parent is a file: %s", name)
			}
			return n, nil
		}
		id++
		n := &xarFile{ID: id, Name: path.Base(name), Type: "directory", Mode: "0755"}
		return n, add(name, n)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !validArchivePath(e.name) || seen[e.name] {
			return fmt.Errorf("invalid or duplicate XAR path %q", e.name)
		}
		seen[e.name] = true
		if e.r == nil && e.source == "" {
			n, err := addDir(e.name)
			if err != nil {
				return err
			}
			if e.mode != "" {
				n.Mode = e.mode
			}
			continue
		}
		if nodes[e.name] != nil {
			return fmt.Errorf("duplicate XAR path %s", e.name)
		}
		id++
		n := &xarFile{ID: id, Name: path.Base(e.name), Type: "file", Mode: e.mode, Mtime: e.mtime}
		if n.Mode == "" {
			n.Mode = "0644"
		}
		offset := pos
		if e.raw != nil {
			d := *e.raw
			n.Data = &d
			h, err := checksumHash(d.Archived.Style)
			if err != nil {
				return err
			}
			count, err := io.CopyBuffer(io.MultiWriter(hw, h), contextReader{ctx, e.r}, buf)
			if err != nil {
				return err
			}
			pos += count
			if count != d.Length || !equalSum(h, d.Archived) {
				return fmt.Errorf("invalid archived data for %s", e.name)
			}
			d.Offset = offset
		} else {
			var src *os.File
			if e.source != "" {
				src, err = os.Open(e.source)
				if err != nil {
					return err
				}
				e.r = src
			}
			h := sha1.New()
			count, copyErr := io.CopyBuffer(io.MultiWriter(hw, h), contextReader{ctx, e.r}, buf)
			if src != nil {
				_ = src.Close()
			}
			if copyErr != nil {
				return copyErr
			}
			pos += count
			sum := xarSum{"sha1", hex.EncodeToString(h.Sum(nil))}
			n.Data = &xarData{Length: count, Size: count, Offset: offset, Encoding: xarEncoding{"application/octet-stream"}, Archived: sum, Extracted: sum}
		}
		if err := add(e.name, n); err != nil {
			return err
		}
	}
	x, err := xml.Marshal(doc)
	if err != nil {
		return err
	}
	x = append([]byte(xml.Header), x...)
	if len(x) > maxTOC {
		return fmt.Errorf("XAR TOC exceeds %d bytes", maxTOC)
	}
	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(x); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	sum := sha1.Sum(compressed.Bytes())
	if err = hw.Flush(); err != nil {
		return err
	}
	if _, err = heap.WriteAt(sum[:], 0); err != nil {
		return err
	}
	header := make([]byte, 28)
	copy(header, "xar!")
	be.PutUint16(header[4:], 28)
	be.PutUint16(header[6:], 1)
	be.PutUint64(header[8:], uint64(compressed.Len()))
	be.PutUint64(header[16:], uint64(len(x)))
	be.PutUint32(header[24:], 1)
	if _, err = out.Write(header); err != nil {
		return err
	}
	if _, err = out.Write(compressed.Bytes()); err != nil {
		return err
	}
	if _, err = heap.Seek(0, 0); err != nil {
		return err
	}
	_, err = io.CopyBuffer(out, contextReader{ctx, heap}, buf)
	return err
}

func checksumHash(style string) (hash.Hash, error) {
	switch style {
	case "sha1":
		return sha1.New(), nil
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	case "md5":
		return md5.New(), nil
	}
	return nil, fmt.Errorf("unsupported XAR checksum %q", style)
}
func equalSum(h hash.Hash, s xarSum) bool {
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), strings.TrimSpace(s.Value))
}

type xarArchive struct {
	f       *os.File
	heap    int64
	entries map[string]*xarFile
}

func openXAR(ctx context.Context, p string) (*xarArchive, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = f.Close()
		}
	}()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	header := make([]byte, 28)
	if _, err = io.ReadFull(f, header); err != nil {
		return nil, err
	}
	if string(header[:4]) != "xar!" || be.Uint16(header[6:]) != 1 || be.Uint16(header[4:]) != 28 {
		return nil, fmt.Errorf("unsupported XAR header")
	}
	cl, ul := be.Uint64(header[8:]), be.Uint64(header[16:])
	if cl > maxTOC || ul > maxTOC || cl == 0 || ul == 0 || st.Size() < 28+int64(cl) {
		return nil, fmt.Errorf("invalid XAR TOC size")
	}
	compressed := make([]byte, int(cl))
	if _, err = io.ReadFull(contextReader{ctx, f}, compressed); err != nil {
		return nil, err
	}
	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	x, err := io.ReadAll(io.LimitReader(zr, int64(ul)+1))
	_ = zr.Close()
	if err != nil {
		return nil, err
	}
	if len(x) != int(ul) {
		return nil, fmt.Errorf("XAR TOC length mismatch")
	}
	var doc xarDocument
	if err = xml.Unmarshal(x, &doc); err != nil {
		return nil, err
	}
	a := &xarArchive{f: f, heap: 28 + int64(cl), entries: map[string]*xarFile{}}
	cs := doc.TOC.Checksum
	styles := map[uint32]string{1: "sha1", 2: "md5", 3: "sha256", 4: "sha512"}
	if styles[be.Uint32(header[24:])] != cs.Style || cs.Style == "" {
		return nil, fmt.Errorf("unsupported XAR TOC checksum")
	}
	h, err := checksumHash(cs.Style)
	if err != nil {
		return nil, err
	}
	if cs.Offset < 0 || cs.Size != int64(h.Size()) || cs.Offset > st.Size()-a.heap-cs.Size {
		return nil, fmt.Errorf("invalid XAR checksum range")
	}
	want := make([]byte, h.Size())
	if _, err = f.ReadAt(want, a.heap+cs.Offset); err != nil {
		return nil, err
	}
	h.Write(compressed)
	if !bytes.Equal(want, h.Sum(nil)) {
		return nil, fmt.Errorf("XAR TOC checksum mismatch")
	}
	var walk func([]*xarFile, string, int) error
	walk = func(nodes []*xarFile, parent string, depth int) error {
		if depth > 128 {
			return fmt.Errorf("XAR nesting exceeds limit")
		}
		for _, n := range nodes {
			if !validArchivePath(n.Name) || strings.Contains(n.Name, "/") {
				return fmt.Errorf("invalid XAR member name %q", n.Name)
			}
			name := n.Name
			if parent != "" {
				name = parent + "/" + name
			}
			if a.entries[name] != nil {
				return fmt.Errorf("duplicate XAR member %q", name)
			}
			a.entries[name] = n
			if n.Type == "directory" {
				if n.Data != nil {
					return fmt.Errorf("directory has data")
				}
				if err := walk(n.Files, name, depth+1); err != nil {
					return err
				}
				continue
			}
			if n.Type != "file" || n.Data == nil || len(n.Files) != 0 {
				return fmt.Errorf("unsupported XAR entry %s", name)
			}
			d := n.Data
			if d.Offset < 0 || d.Length < 0 || d.Size < 0 || d.Offset > st.Size()-a.heap-d.Length {
				return fmt.Errorf("XAR data outside archive: %s", name)
			}
			if d.Offset < cs.Offset+cs.Size && d.Offset+d.Length > cs.Offset {
				return fmt.Errorf("XAR data overlaps TOC checksum")
			}
			if d.Encoding.Style != "application/octet-stream" && d.Encoding.Style != "application/x-gzip" {
				return fmt.Errorf("unsupported XAR encoding %q", d.Encoding.Style)
			}
			if _, err := checksumHash(d.Archived.Style); err != nil {
				return err
			}
			if _, err := checksumHash(d.Extracted.Style); err != nil {
				return err
			}
			if n.Mode != "" {
				if _, err := strconv.ParseUint(n.Mode, 8, 16); err != nil {
					return fmt.Errorf("invalid XAR mode")
				}
			}
		}
		return nil
	}
	if err = walk(doc.TOC.Files, "", 0); err != nil {
		return nil, err
	}
	ok = true
	return a, nil
}
func (a *xarArchive) raw(n *xarFile) io.Reader {
	return io.NewSectionReader(a.f, a.heap+n.Data.Offset, n.Data.Length)
}
func (a *xarArchive) read(ctx context.Context, name string, limit int64) ([]byte, error) {
	n := a.entries[name]
	if n == nil || n.Data == nil {
		return nil, fmt.Errorf("missing XAR member %s", name)
	}
	d := n.Data
	if d.Size > limit || d.Length > limit {
		return nil, fmt.Errorf("XAR member too large: %s", name)
	}
	b, err := io.ReadAll(contextReader{ctx, a.raw(n)})
	if err != nil {
		return nil, err
	}
	h, _ := checksumHash(d.Archived.Style)
	h.Write(b)
	if !equalSum(h, d.Archived) {
		return nil, fmt.Errorf("XAR archived checksum mismatch: %s", name)
	}
	if d.Encoding.Style == "application/x-gzip" {
		zr, err := zlib.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		b, err = io.ReadAll(io.LimitReader(zr, limit+1))
		_ = zr.Close()
		if err != nil {
			return nil, err
		}
	}
	if int64(len(b)) != d.Size {
		return nil, fmt.Errorf("XAR extracted size mismatch")
	}
	h, _ = checksumHash(d.Extracted.Style)
	h.Write(b)
	if !equalSum(h, d.Extracted) {
		return nil, fmt.Errorf("XAR extracted checksum mismatch")
	}
	return b, nil
}
