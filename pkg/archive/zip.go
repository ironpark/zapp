// Package archive writes the ZIP archives zapp distributes and submits for
// notarization.
package archive

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Zip archives source, a file or directory, as target, the way
// `ditto -c -k --keepParent` does: entries are named from source's own name,
// Unix permissions are kept, and symbolic links are stored as links. A
// framework's Versions/Current is a link, and copying what it points to
// instead would break the bundle's code signature.
//
// The archive is written beside target and renamed into place, so a failed
// or cancelled run never leaves a partial archive under target's name.
func Zip(ctx context.Context, source, target string) (err error) {
	source = filepath.Clean(source)
	if _, err := os.Lstat(source); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+"-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(out.Name())
		}
	}()
	w := zip.NewWriter(out)
	err = filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return add(w, source, path)
	})
	err = errors.Join(err, w.Close(), out.Close())
	if err != nil {
		return err
	}
	return os.Rename(out.Name(), target)
}

func add(w *zip.Writer, source, path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(filepath.Dir(source), path)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(rel)
	switch {
	case info.IsDir():
		header.Name += "/"
		_, err = w.CreateHeader(header)
		return err
	case info.Mode()&fs.ModeSymlink != 0:
		// A link's entry holds its target, as unzip and ditto expect.
		link, err := os.Readlink(path)
		if err != nil {
			return err
		}
		entry, err := w.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.WriteString(entry, filepath.ToSlash(link))
		return err
	case !info.Mode().IsRegular():
		return errors.New(path + " is neither a file, a directory nor a link")
	}
	header.Method = zip.Deflate
	entry, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(entry, f)
	return err
}
