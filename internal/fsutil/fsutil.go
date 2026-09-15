// Package fsutil holds the filesystem helpers shared across zapp.
//
// Paths are resolved through os.Stat throughout, so a symlink is treated as
// whatever it points at: copying one produces a regular file holding the
// target's contents.
package fsutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// IsDir reports whether path is a directory, following symlinks.
func IsDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}

// SameFile reports whether a and b are the same file on disk. A missing b is
// reported as not the same rather than as an error, so callers can use it to
// guard a copy that would otherwise truncate its own source.
func SameFile(a, b string) (bool, error) {
	aInfo, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bInfo, err := os.Stat(b)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return os.SameFile(aInfo, bInfo), nil
}

// CopyFile copies a regular file from src to dst, creating dst's parent
// directories and preserving the source's permission bits. Preserving the mode
// matters for anything copied out of an app bundle: an executable that loses
// its exec bit cannot be launched from the copy.
func CopyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("%s is a directory", src)
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	if err := dstFile.Sync(); err != nil {
		return err
	}
	return dstFile.Close()
}

// CopyDir recursively copies the directory src to dst, preserving the
// permission bits of every file and directory it creates.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		isDir, err := IsDir(srcPath)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", srcPath, err)
		}
		if isDir {
			err = CopyDir(srcPath, dstPath)
		} else {
			err = CopyFile(srcPath, dstPath)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
