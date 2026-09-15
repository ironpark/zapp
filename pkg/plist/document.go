package plist

import (
	"fmt"
	"io"
	"os"

	"github.com/ironpark/zapp/internal/fsutil"
)

// Document is a property list read from a file, remembering enough about it to
// be written back the way it arrived. Edit Root, then call Save.
type Document struct {
	// Path is the file the document was read from.
	Path string
	// Root is the dictionary at the top of the file, free to be modified.
	Root map[string]any

	binary bool
	mode   os.FileMode
}

// Open reads the property list at path, which must hold a dictionary.
func Open(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	root, err := ParseDict(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &Document{Path: path, Root: root, binary: isBinary(data), mode: info.Mode().Perm()}, nil
}

// Save writes the document back over the file it came from, in the format it
// was read in -- rewriting a binary Info.plist as XML would invalidate the
// signature of a bundle holding it -- keeping the file's permissions, and
// replacing it atomically so an interrupted write cannot truncate it.
func (d *Document) Save() error {
	var data []byte
	var err error
	if d.binary {
		data, err = MarshalBinary(d.Root)
	} else {
		data, err = MarshalXML(d.Root)
	}
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", d.Path, err)
	}
	return fsutil.WriteFileAtomic(d.Path, data, d.mode)
}
