package macpkg

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const legacyLimit int64 = 8 << 30
const segmentSize int64 = 1 << 30

type fileEntry struct {
	name, source, link string
	mode, uid, gid     uint32
	mtime              uint32
	size               int64
	ino, nlink         uint32
	checksum           uint32
	info               fs.FileInfo
}

func (e fileEntry) regular() bool { return e.mode&0170000 == 0100000 }
func (e fileEntry) dir() bool     { return e.mode&0170000 == 0040000 }

func validArchivePath(p string) bool {
	// fs.ValidPath covers non-empty, unrooted, clean, and no "." / ".." elements.
	return fs.ValidPath(p) && p != "." && validXMLText(p) && !strings.ContainsAny(p, "\x00\\")
}

func validXMLText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if r < 0x20 || r == 0xfffe || r == 0xffff {
			return false
		}
	}
	return true
}

func collect(ctx context.Context, root, only string, ownership Ownership) ([]fileEntry, error) {
	if ownership > PreserveOwnership {
		return nil, fmt.Errorf("invalid ownership mode")
	}
	if only != "" && (!validArchivePath(only) || strings.Contains(only, "/")) {
		return nil, fmt.Errorf("RootEntry must be an immediate child name")
	}
	st, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("root must be a real directory: %s", root)
	}
	var entries []fileEntry
	type inodeKey struct{ dev, ino uint64 }
	inodes := map[inodeKey]uint32{}
	counts := map[uint32]uint32{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if only != "" && rel != "." && rel != only && !strings.HasPrefix(rel, only+"/") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if rel != "." && !validArchivePath(rel) {
			return fmt.Errorf("invalid payload path %q", rel)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		s, err := payloadMetadata(p, info, ownership)
		if err != nil {
			return err
		}
		e := fileEntry{name: rel, source: p, info: info, mode: uint32(s.Mode), mtime: uint32(info.ModTime().Unix())}
		if info.ModTime().Unix() < 0 || info.ModTime().Unix() > 1<<32-1 {
			return fmt.Errorf("mtime out of range: %s", p)
		}
		if ownership == PreserveOwnership {
			e.uid = s.Uid
			e.gid = s.Gid
		}
		if e.uid > 0777777 || e.gid > 0777777 {
			return fmt.Errorf("uid/gid exceeds odc CPIO range: %s", p)
		}
		switch {
		case info.Mode().IsRegular():
			e.size = info.Size()
		case info.IsDir():
		case info.Mode()&os.ModeSymlink != 0:
			e.link, err = os.Readlink(p)
			if err != nil {
				return err
			}
			if !utf8.ValidString(e.link) || strings.ContainsRune(e.link, 0) {
				return fmt.Errorf("invalid symlink target: %s", p)
			}
			e.size = int64(len(e.link))
		default:
			return fmt.Errorf("unsupported special file: %s", p)
		}
		key := inodeKey{uint64(s.Dev), s.Ino}
		if e.regular() {
			e.ino = inodes[key]
		}
		if e.ino == 0 {
			e.ino = uint32(len(entries) + 1)
			if e.regular() {
				inodes[key] = e.ino
			}
		}
		counts[e.ino]++
		if len(entries) >= 0777776 {
			return fmt.Errorf("too many payload entries for odc CPIO")
		}
		entries = append(entries, e)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if only != "" && len(entries) == 1 {
		return nil, fmt.Errorf("RootEntry %q not found", only)
	}
	for i := range entries {
		entries[i].nlink = counts[entries[i].ino]
	}
	return entries, nil
}
