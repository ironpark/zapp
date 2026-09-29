package macho

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	cmdCodeSignature   = 0x1d
	cmdVersionMinMacOS = 0x24
	cmdBuildVersion    = 0x32

	fileTypeObject  = 1
	fileTypeExecute = 2
	fileTypeDSYM    = 0xa

	cpuX86_64 = 0x01000007
	cpuARM64  = 0x0100000c
)

// Image is what one architecture of a Mach-O file records about itself and
// its code signature.
type Image struct {
	// Arch is arm64, x86_64 or the CPU type in hex.
	Arch string
	// Executable is set for a program, as opposed to a library or bundle.
	Executable bool
	// Unloaded is set for a relocatable object or a dSYM's debug symbols,
	// which are linked or read but never loaded, and so never signed.
	Unloaded bool
	// MinOS is the oldest macOS it runs on, as 12.0; empty when unrecorded.
	MinOS string
	// Signature is nil when the image is unsigned.
	Signature *Signature
}

// ReadImages reports each architecture of the Mach-O file at path.
func ReadImages(path string) ([]Image, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	images, err := readImages(buf)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return images, nil
}

func readImages(buf []byte) ([]Image, error) {
	slices, err := slices(buf)
	if err != nil {
		return nil, err
	}
	var out []Image
	for _, s := range slices {
		commands, err := s.commands(buf)
		if err != nil {
			return nil, err
		}
		fileType := s.order.Uint32(buf[s.start+12:])
		img := Image{Arch: arch(s.order.Uint32(buf[s.start+4:])), Executable: fileType == fileTypeExecute, Unloaded: fileType == fileTypeObject || fileType == fileTypeDSYM}
		for _, c := range commands {
			switch c.cmd {
			case cmdBuildVersion, cmdVersionMinMacOS:
				// LC_BUILD_VERSION: platform, then minos; LC_VERSION_MIN_MACOSX:
				// version first.
				at := c.at + 8
				if c.cmd == cmdBuildVersion {
					at += 4
				}
				if at+4 <= c.at+int(c.size) && img.MinOS == "" {
					img.MinOS = version(s.order.Uint32(buf[at:]))
				}
			case cmdCodeSignature:
				if c.size < 16 {
					return nil, fmt.Errorf("code signature load command is too small")
				}
				off := s.start + int(s.order.Uint32(buf[c.at+8:]))
				size := int(s.order.Uint32(buf[c.at+12:]))
				if off+size > len(buf) || size < 12 {
					return nil, fmt.Errorf("code signature runs past the end of the file")
				}
				sig, err := ParseSignature(buf[off : off+size])
				if err != nil {
					return nil, err
				}
				img.Signature = sig
			}
		}
		out = append(out, img)
	}
	return out, nil
}

func arch(cpu uint32) string {
	switch cpu {
	case cpuARM64:
		return "arm64"
	case cpuX86_64:
		return "x86_64"
	}
	return fmt.Sprintf("cpu %#x", cpu)
}

// version renders a packed X.Y.Z version, xxxx.yy.zz in nibbles.
func version(v uint32) string {
	s := fmt.Sprintf("%d.%d", v>>16, v>>8&0xff)
	if patch := v & 0xff; patch != 0 {
		s += fmt.Sprintf(".%d", patch)
	}
	return s
}

// Code signature blobs and the slots that hold them.
const (
	magicSuperBlob     = 0xfade0cc0
	magicCodeDirectory = 0xfade0c02
	magicEntitlements  = 0xfade7171
	magicCMS           = 0xfade0b01

	slotCodeDirectory = 0
	slotEntitlements  = 5
	slotCMS           = 0x10000
	slotTicket        = 0x10002

	flagAdHoc   = 0x2
	flagRuntime = 0x10000
)

// Signature is what a code signature says about how it was made. The same
// superblob signs a Mach-O image or, at the end of the image, a disk image.
type Signature struct {
	// Signed is set when there is a code directory: a disk image may carry a
	// stapled ticket and nothing else.
	Signed     bool
	Identifier string
	TeamID     string
	// AdHoc is set for a signature with no certificate behind it.
	AdHoc bool
	// Runtime is set when the hardened runtime is on.
	Runtime bool
	// Entitlements is the entitlements plist, empty when there is none.
	Entitlements []byte
	// CMS is the certificate-based signature, empty when ad hoc.
	CMS []byte
	// Stapled is set when a notarization ticket is attached; only a disk
	// image carries one in its signature.
	Stapled bool
}

// ParseSignature reads an embedded signature superblob.
func ParseSignature(b []byte) (*Signature, error) {
	be := binary.BigEndian
	if len(b) < 12 || be.Uint32(b) != magicSuperBlob {
		return nil, fmt.Errorf("code signature is not a signature superblob")
	}
	count := int(be.Uint32(b[8:]))
	if 12+count*8 > len(b) {
		return nil, fmt.Errorf("code signature index runs past its end")
	}
	blob := func(off uint32, magic uint32) ([]byte, bool) {
		if int(off)+8 > len(b) || be.Uint32(b[off:]) != magic {
			return nil, false
		}
		n := be.Uint32(b[off+4:])
		if n < 8 || int(off)+int(n) > len(b) {
			return nil, false
		}
		return b[off+8 : off+n], true
	}
	sig := &Signature{}
	for i := range count {
		slot, off := be.Uint32(b[12+i*8:]), be.Uint32(b[16+i*8:])
		switch slot {
		case slotCodeDirectory:
			cd, ok := blob(off, magicCodeDirectory)
			if !ok {
				return nil, fmt.Errorf("code directory is malformed")
			}
			if err := sig.readCodeDirectory(append(make([]byte, 8), cd...)); err != nil {
				return nil, err
			}
			sig.Signed = true
		case slotEntitlements:
			sig.Entitlements, _ = blob(off, magicEntitlements)
		case slotCMS:
			sig.CMS, _ = blob(off, magicCMS)
		case slotTicket:
			sig.Stapled = true
		}
	}
	return sig, nil
}

// readCodeDirectory reads a code directory, cd, including its 8-byte blob
// header.
func (s *Signature) readCodeDirectory(cd []byte) error {
	be := binary.BigEndian
	if len(cd) < 44 {
		return fmt.Errorf("code directory is too short")
	}
	version, flags := be.Uint32(cd[8:]), be.Uint32(cd[12:])
	s.AdHoc, s.Runtime = flags&flagAdHoc != 0, flags&flagRuntime != 0
	s.Identifier = cstring(cd, be.Uint32(cd[20:]))
	// scatterOffset follows spare2 at 44; teamOffset follows it.
	if version >= 0x20200 && len(cd) >= 52 {
		if off := be.Uint32(cd[48:]); off != 0 {
			s.TeamID = cstring(cd, off)
		}
	}
	return nil
}

func cstring(b []byte, off uint32) string {
	if int(off) >= len(b) {
		return ""
	}
	end := int(off)
	for end < len(b) && b[end] != 0 {
		end++
	}
	return string(b[off:end])
}
