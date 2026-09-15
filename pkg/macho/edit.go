package macho

import (
	"fmt"
	"os"
	"path/filepath"
)

// SetID sets the install name a dylib publishes, the equivalent of
// install_name_tool -id.
func SetID(path, name string) error {
	return edit(path, func(s slice, buf []byte) ([]command, bool, error) {
		cmds, err := s.decode(buf)
		if err != nil {
			return nil, false, err
		}
		changed := false
		for i := range cmds {
			if cmds[i].cmd == cmdIDDylib && cmds[i].str != name {
				cmds[i].str = name
				changed = true
			}
		}
		return cmds, changed, nil
	})
}

// ChangeDependency repoints a load command from one install name to another,
// the equivalent of install_name_tool -change. A name that does not appear is
// left alone, as install_name_tool leaves it.
func ChangeDependency(path, old, new string) error {
	return edit(path, func(s slice, buf []byte) ([]command, bool, error) {
		cmds, err := s.decode(buf)
		if err != nil {
			return nil, false, err
		}
		changed := false
		for i := range cmds {
			if isDylibLoad(cmds[i].cmd) && cmds[i].str == old && old != new {
				cmds[i].str = new
				changed = true
			}
		}
		return cmds, changed, nil
	})
}

// AddRPath appends a runpath, the equivalent of install_name_tool -add_rpath.
// A runpath the binary already has is left as it is, rather than added twice.
func AddRPath(path, rpath string) error {
	return edit(path, func(s slice, buf []byte) ([]command, bool, error) {
		cmds, err := s.decode(buf)
		if err != nil {
			return nil, false, err
		}
		for _, c := range cmds {
			if c.cmd == cmdRPath && c.str == rpath {
				return cmds, false, nil
			}
		}
		return append(cmds, command{cmd: cmdRPath, str: rpath, hasStr: true}), true, nil
	})
}

// command is one load command, held either as the bytes it already had or, for
// the kinds this package rewrites, as its fixed fields plus its string.
type command struct {
	cmd    uint32
	raw    []byte // the command exactly as it was, for kinds left untouched
	fixed  []byte // the fields between the string offset and the string itself
	str    string
	hasStr bool
}

// carriesString reports whether cmd is one of the kinds stored as a string.
func carriesString(cmd uint32) bool {
	return cmd == cmdIDDylib || isDylibLoad(cmd) || cmd == cmdRPath
}

// decode reads the load commands of one image. Commands this package does not
// rewrite keep their original bytes, so re-encoding cannot perturb them.
func (s slice) decode(buf []byte) ([]command, error) {
	found, err := s.commands(buf)
	if err != nil {
		return nil, err
	}
	out := make([]command, 0, len(found))
	for _, c := range found {
		if !carriesString(c.cmd) {
			out = append(out, command{cmd: c.cmd, raw: clone(buf[c.at : c.at+int(c.size)])})
			continue
		}
		str, err := s.lcString(buf, c, 8)
		if err != nil {
			return nil, err
		}
		// A dylib command carries three more fields after the string offset; an
		// rpath command carries none. Keeping the bytes verbatim covers both.
		off := int(s.order.Uint32(buf[c.at+8:]))
		out = append(out, command{
			cmd:    c.cmd,
			fixed:  clone(buf[c.at+loadCommandSize+4 : c.at+off]),
			str:    str,
			hasStr: true,
		})
	}
	return out, nil
}

// size is how many bytes the command occupies once encoded, rounded up so the
// next command stays aligned.
func (c command) size(align int) int {
	if !c.hasStr {
		return len(c.raw)
	}
	n := loadCommandSize + 4 + len(c.fixed) + len(c.str) + 1
	if r := n % align; r != 0 {
		n += align - r
	}
	return n
}

// encode writes the command into dst, which must be exactly its size.
func (c command) encode(s slice, dst []byte) {
	if !c.hasStr {
		copy(dst, c.raw)
		return
	}
	for i := range dst {
		dst[i] = 0
	}
	strOff := loadCommandSize + 4 + len(c.fixed)
	s.order.PutUint32(dst[0:], c.cmd)
	s.order.PutUint32(dst[4:], uint32(len(dst)))
	s.order.PutUint32(dst[8:], uint32(strOff))
	copy(dst[loadCommandSize+4:], c.fixed)
	copy(dst[strOff:], c.str)
}

func clone(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// edit applies fn to every architecture of the file and writes it back only if
// something changed. Writing goes through a temporary file so a failure part
// way cannot leave a half-rewritten binary behind.
func edit(path string, fn func(slice, []byte) ([]command, bool, error)) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	buf, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	images, err := slices(buf)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	changed := false
	for i, s := range images {
		cmds, did, err := fn(s, buf)
		if err != nil {
			return fmt.Errorf("%s: architecture %d: %w", path, i, err)
		}
		if !did {
			continue
		}
		if err := s.writeCommands(buf, cmds); err != nil {
			return fmt.Errorf("%s: architecture %d: %w", path, i, err)
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return writeBack(path, buf, info.Mode().Perm())
}

// writeCommands lays the load commands back out. The whole region is rewritten
// rather than patched in place so that a name may grow: the commands after it
// shift, which is safe because a load command addresses its own string by an
// offset from itself, and nothing outside the header moves.
func (s slice) writeCommands(buf []byte, cmds []command) error {
	align := 4
	if s.is64 {
		align = 8
	}

	total := 0
	for _, c := range cmds {
		total += c.size(align)
	}

	start := s.start + s.headerSize()
	oldEnd := start + int(s.sizeofcmds(buf))
	existing, err := s.commands(buf)
	if err != nil {
		return err
	}
	limit, err := s.headerLimit(buf, existing)
	if err != nil {
		return err
	}
	if start+total > limit {
		return fmt.Errorf("the load commands need %d bytes but only %d are free before the first section; "+
			"relink with -headerpad_max_install_names to make space", total, limit-start)
	}

	at := start
	for _, c := range cmds {
		n := c.size(align)
		c.encode(s, buf[at:at+n])
		at += n
	}
	// Clear whatever a longer previous layout left behind.
	for i := at; i < oldEnd; i++ {
		buf[i] = 0
	}

	s.setNcmds(buf, uint32(len(cmds)))
	s.setSizeofcmds(buf, uint32(total))
	return nil
}

// writeBack replaces path's contents atomically.
func writeBack(path string, buf []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".zapp-macho-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()

	if _, err := tmp.Write(buf); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// headerLimit is the file offset the load commands may not grow past: the
// nearest thing the linker has already placed after them.
func (s slice) headerLimit(buf []byte, commands []loadCommand) (int, error) {
	limit := -1
	// consider takes the section's own file offset, before it is placed within
	// the file: a zero there means the section holds no file data, as with
	// __bss, and says nothing about where the header may grow to.
	consider := func(sectionOffset uint32) {
		if sectionOffset == 0 {
			return
		}
		at := s.start + int(sectionOffset)
		if limit < 0 || at < limit {
			limit = at
		}
	}

	for _, c := range commands {
		switch c.cmd {
		case cmdSegment64:
			// nsects sits after the 16-byte name and four 64-bit fields.
			if int(c.size) < 72 {
				continue
			}
			nsects := int(s.order.Uint32(buf[c.at+64:]))
			for i := range nsects {
				// A 64-bit section is 80 bytes; its file offset follows the
				// 16-byte name, the 16-byte segment name, addr and size.
				at := c.at + 72 + i*80
				if at+48 > len(buf) {
					return 0, fmt.Errorf("segment claims more sections than fit in the file")
				}
				consider(s.order.Uint32(buf[at+48:]))
			}
		case cmdSegment32:
			if int(c.size) < 56 {
				continue
			}
			nsects := int(s.order.Uint32(buf[c.at+48:]))
			for i := range nsects {
				// A 32-bit section is 68 bytes; its file offset follows the two
				// names, addr and size.
				at := c.at + 56 + i*68
				if at+40 > len(buf) {
					return 0, fmt.Errorf("segment claims more sections than fit in the file")
				}
				consider(s.order.Uint32(buf[at+40:]))
			}
		}
	}
	if limit < 0 {
		return 0, fmt.Errorf("could not determine how much header padding is available")
	}
	return limit, nil
}
