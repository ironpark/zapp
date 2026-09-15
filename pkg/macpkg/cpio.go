package macpkg

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
)

// BOM checksums are POSIX cksum (non-reflected CRC-32 with a length suffix),
// not the reflected CRC used by gzip.
var cksumTable = func() (t [256]uint32) {
	for i := range t {
		c := uint32(i) << 24
		for range 8 {
			if c&0x80000000 != 0 {
				c = c<<1 ^ 0x04c11db7
			} else {
				c <<= 1
			}
		}
		t[i] = c
	}
	return
}()

type posixSum struct {
	crc uint32
	n   uint64
}

func (s *posixSum) Write(p []byte) (int, error) {
	for _, b := range p {
		s.crc = s.crc<<8 ^ cksumTable[byte(s.crc>>24)^b]
	}
	s.n += uint64(len(p))
	return len(p), nil
}
func (s posixSum) Sum32() uint32 {
	for n := s.n; n != 0; n >>= 8 {
		s.crc = s.crc<<8 ^ cksumTable[byte(s.crc>>24)^byte(n)]
	}
	return ^s.crc
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, e := w.w.Write(p)
	w.n += int64(n)
	return n, e
}

func cpioHeader(w io.Writer, e fileEntry, name string, size int64, ino uint32) error {
	if size < 0 || size >= legacyLimit || len(name)+1 > 0777777 || ino > 0777777 {
		return fmt.Errorf("CPIO field overflow for %q", name)
	}
	_, err := fmt.Fprintf(w, "070707%06o%06o%06o%06o%06o%06o%06o%011o%06o%011o%s\x00", 0, ino, e.mode, e.uid, e.gid, e.nlink, 0, e.mtime, len(name)+1, size, name)
	return err
}

// writeRegular streams one regular file into the CPIO stream, splitting it into
// segments when large, and returns its POSIX cksum.
func writeRegular(ctx context.Context, w io.Writer, e *fileEntry, name string, chunks int64, group []uint32, large bool, buf []byte) (uint32, error) {
	in, err := os.Open(e.source)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return 0, err
	}
	if !os.SameFile(st, e.info) || st.Size() != e.size || !st.Mode().IsRegular() {
		return 0, fmt.Errorf("input changed: %s", e.source)
	}
	sum := &posixSum{}
	remaining := e.size
	for c := int64(0); c < chunks; c++ {
		size := remaining
		if large && size > segmentSize {
			size = segmentSize
		}
		if err := cpioHeader(w, *e, name, size, group[c]); err != nil {
			return 0, err
		}
		n, err := io.CopyBuffer(io.MultiWriter(w, sum), io.LimitReader(contextReader{ctx, in}, size), buf)
		if err != nil {
			return 0, err
		}
		if n != size {
			return 0, io.ErrUnexpectedEOF
		}
		remaining -= size
	}
	if st, err = in.Stat(); err != nil {
		return 0, err
	}
	if st.Size() != e.size || !st.ModTime().Equal(e.info.ModTime()) {
		return 0, fmt.Errorf("input changed: %s", e.source)
	}
	return sum.Sum32(), nil
}

func writePayload(ctx context.Context, dest string, entries []fileEntry, large bool) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	// flate flushes its bit-writer every few hundred bytes; buffer to keep the
	// payload from turning into millions of tiny writes.
	bw := bufio.NewWriterSize(f, 1<<20)
	gz := gzip.NewWriter(bw)
	gz.OS = 3
	w := &countingWriter{w: gz}
	next := uint32(1)
	ids := map[uint32][]uint32{}
	buf := make([]byte, 256<<10)
	for i := range entries {
		e := &entries[i]
		if err := ctx.Err(); err != nil {
			return err
		}
		name := "."
		if e.name != "." {
			name = "./" + e.name
		}
		chunks := int64(1)
		if large && e.regular() && e.size > segmentSize {
			chunks = (e.size-1)/segmentSize + 1
		}
		group := ids[e.ino]
		if group == nil {
			if uint64(next)+uint64(chunks) > 0777777 {
				return fmt.Errorf("too many CPIO segments")
			}
			for range chunks {
				group = append(group, next)
				next++
			}
			ids[e.ino] = group
		}
		if !e.regular() {
			if err := cpioHeader(w, *e, name, e.size, group[0]); err != nil {
				return err
			}
			if _, err := io.WriteString(w, e.link); err != nil {
				return err
			}
			if e.link != "" {
				s := posixSum{}
				s.Write([]byte(e.link))
				e.checksum = s.Sum32()
			}
			continue
		}
		sum, err := writeRegular(ctx, w, e, name, chunks, group, large, buf)
		if err != nil {
			return err
		}
		e.checksum = sum
	}
	if err := cpioHeader(w, fileEntry{nlink: 1}, "TRAILER!!!", 0, next); err != nil {
		return err
	}
	if _, err := w.Write(make([]byte, (512-w.n%512)%512)); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	return f.Close()
}
