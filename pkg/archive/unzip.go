package archive

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Unzip extracts the archive at source into dir, keeping the permissions and
// symbolic links Zip stores. An entry that would land outside dir is an
// error.
func Unzip(ctx context.Context, source, dir string) error {
	r, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	for _, f := range r.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := extract(f, dir); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	return nil
}

func extract(f *zip.File, dir string) error {
	name := filepath.FromSlash(strings.TrimSuffix(f.Name, "/"))
	if name == "" || !filepath.IsLocal(name) {
		return errors.New("entry names a path outside the archive")
	}
	path := filepath.Join(dir, name)
	mode := f.Mode()
	if mode.IsDir() {
		return os.MkdirAll(path, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	if mode&fs.ModeSymlink != 0 {
		target, err := io.ReadAll(io.LimitReader(rc, 4096))
		if err != nil {
			return err
		}
		return os.Symlink(filepath.FromSlash(string(target)), path)
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm()|0o200)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, rc)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		// OpenFile's mode passes through the umask.
		err = os.Chmod(path, mode.Perm())
	}
	return err
}
