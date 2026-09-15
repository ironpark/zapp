// Package macho reads and rewrites the load commands that record what a Mach-O
// binary links against: the install name it publishes, the libraries it loads,
// and the runpaths it searches. It covers what otool -L, otool -D, otool -l and
// install_name_tool are used for, without leaving the process.
//
// Only the load command region is touched. Like install_name_tool, rewriting
// invalidates an existing code signature; the caller is expected to sign
// afterwards.
package macho

import (
	"encoding/binary"
	"fmt"
)

// Magic numbers identifying a Mach-O file or a universal binary wrapping
// several of them.
const (
	magic32    = 0xfeedface
	magic64    = 0xfeedfacf
	fatMagic   = 0xcafebabe
	fatMagic64 = 0xcafebabf
)

// Load commands. The high bit marks commands the dynamic linker must
// understand; it is part of the value stored in the file.
const (
	reqDyld = 0x80000000

	cmdSegment32     = 0x01
	cmdSegment64     = 0x19
	cmdLoadDylib     = 0x0c
	cmdIDDylib       = 0x0d
	cmdLoadWeakDylib = 0x18 | reqDyld
	cmdReexportDylib = 0x1f | reqDyld
	cmdUpwardDylib   = 0x23 | reqDyld
	cmdRPath         = 0x1c | reqDyld
)

// isDylibLoad reports whether cmd records a library the binary links against,
// as opposed to its own install name.
func isDylibLoad(cmd uint32) bool {
	switch cmd {
	case cmdLoadDylib, cmdLoadWeakDylib, cmdReexportDylib, cmdUpwardDylib:
		return true
	}
	return false
}

// header sizes, in bytes
const (
	machHeaderSize32 = 28
	machHeaderSize64 = 32 // the 64-bit header has four reserved bytes
	loadCommandSize  = 8  // cmd and cmdsize
	dylibCommandSize = 24 // through compatibility_version
	rpathCommandSize = 12 // through path offset
	fatHeaderSize    = 8
	fatArchSize      = 20
	fatArchSize64    = 32
)

// slice is one architecture's Mach-O image within a file. A thin file has one;
// a universal binary has several.
type slice struct {
	start int              // offset of the image within the file
	order binary.ByteOrder //
	is64  bool             //
}

// headerSize is the size of this image's Mach-O header.
func (s slice) headerSize() int {
	if s.is64 {
		return machHeaderSize64
	}
	return machHeaderSize32
}

// ncmds and sizeofcmds live at fixed offsets in both header layouts.
func (s slice) ncmds(buf []byte) uint32      { return s.order.Uint32(buf[s.start+16:]) }
func (s slice) sizeofcmds(buf []byte) uint32 { return s.order.Uint32(buf[s.start+20:]) }

func (s slice) setNcmds(buf []byte, v uint32)      { s.order.PutUint32(buf[s.start+16:], v) }
func (s slice) setSizeofcmds(buf []byte, v uint32) { s.order.PutUint32(buf[s.start+20:], v) }

// loadCommand locates one load command within an image.
type loadCommand struct {
	cmd  uint32
	size uint32
	at   int // offset of the command within the file
}

// slices enumerates the Mach-O images in buf, rejecting anything that is not a
// Mach-O or universal binary.
func slices(buf []byte) ([]slice, error) {
	if len(buf) < fatHeaderSize {
		return nil, fmt.Errorf("file is too short to be Mach-O (%d bytes)", len(buf))
	}
	switch binary.BigEndian.Uint32(buf) {
	case fatMagic, fatMagic64:
		return fatSlices(buf)
	}
	s, err := thinSlice(buf, 0)
	if err != nil {
		return nil, err
	}
	return []slice{s}, nil
}

// fatSlices reads the architecture table of a universal binary. The table is
// always big endian, whatever the images inside it are.
func fatSlices(buf []byte) ([]slice, error) {
	is64 := binary.BigEndian.Uint32(buf) == fatMagic64
	count := int(binary.BigEndian.Uint32(buf[4:]))
	entry := fatArchSize
	if is64 {
		entry = fatArchSize64
	}
	if fatHeaderSize+count*entry > len(buf) {
		return nil, fmt.Errorf("universal binary claims %d architectures, which do not fit in %d bytes", count, len(buf))
	}

	out := make([]slice, 0, count)
	for i := range count {
		// The offset field follows cputype and cpusubtype in both layouts; in
		// the 64-bit layout it is 64 bits wide.
		at := fatHeaderSize + i*entry + 8
		var off uint64
		if is64 {
			off = binary.BigEndian.Uint64(buf[at:])
		} else {
			off = uint64(binary.BigEndian.Uint32(buf[at:]))
		}
		if off > uint64(len(buf)) {
			return nil, fmt.Errorf("architecture %d starts past the end of the file", i)
		}
		s, err := thinSlice(buf, int(off))
		if err != nil {
			return nil, fmt.Errorf("architecture %d: %w", i, err)
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("universal binary contains no architectures")
	}
	return out, nil
}

// thinSlice identifies the image starting at off.
func thinSlice(buf []byte, off int) (slice, error) {
	if off+machHeaderSize32 > len(buf) {
		return slice{}, fmt.Errorf("Mach-O header at %d runs past the end of the file", off)
	}
	be := binary.BigEndian.Uint32(buf[off:])
	le := binary.LittleEndian.Uint32(buf[off:])
	switch {
	case le == magic64:
		return slice{start: off, order: binary.LittleEndian, is64: true}, nil
	case le == magic32:
		return slice{start: off, order: binary.LittleEndian, is64: false}, nil
	case be == magic64:
		return slice{start: off, order: binary.BigEndian, is64: true}, nil
	case be == magic32:
		return slice{start: off, order: binary.BigEndian, is64: false}, nil
	}
	return slice{}, fmt.Errorf("not a Mach-O image: magic %#08x", le)
}

// commands walks the load commands of one image.
func (s slice) commands(buf []byte) ([]loadCommand, error) {
	at := s.start + s.headerSize()
	end := at + int(s.sizeofcmds(buf))
	if end > len(buf) {
		return nil, fmt.Errorf("load commands run past the end of the file")
	}

	n := int(s.ncmds(buf))
	out := make([]loadCommand, 0, n)
	for i := range n {
		if at+loadCommandSize > end {
			return nil, fmt.Errorf("load command %d runs past the load command region", i)
		}
		size := s.order.Uint32(buf[at+4:])
		if size < loadCommandSize || at+int(size) > end {
			return nil, fmt.Errorf("load command %d has an invalid size of %d", i, size)
		}
		out = append(out, loadCommand{cmd: s.order.Uint32(buf[at:]), size: size, at: at})
		at += int(size)
	}
	return out, nil
}

// lcString reads the NUL-terminated string a load command carries at the offset
// recorded in its header. Mach-O stores these as an offset from the start of
// the command, with the text padded to the command's size.
func (s slice) lcString(buf []byte, c loadCommand, offsetField int) (string, error) {
	if int(c.size) < offsetField+4 {
		return "", fmt.Errorf("load command %#x is too small to hold a string offset", c.cmd)
	}
	off := int(s.order.Uint32(buf[c.at+offsetField:]))
	if off < loadCommandSize || off >= int(c.size) {
		return "", fmt.Errorf("load command %#x has a string offset of %d, outside the command", c.cmd, off)
	}
	text := buf[c.at+off : c.at+int(c.size)]
	for i, b := range text {
		if b == 0 {
			return string(text[:i]), nil
		}
	}
	return string(text), nil
}
