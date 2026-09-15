package signing

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func createZip(source, target string) (err error) {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, zipfile.Close()) }()

	archive := zip.NewWriter(zipfile)
	defer func() { err = errors.Join(err, archive.Close()) }()

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			rel, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(filepath.Join(baseDir, rel))
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		_, err = io.Copy(writer, file)
		return err
	})
}
