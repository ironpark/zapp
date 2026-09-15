// Package appbundle reads the metadata an .app bundle records in its
// Info.plist.
package appbundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/pkg/plist"
)

func findPlistPath(path string) (string, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("error accessing %s: %w", path, err)
	}

	if fileInfo.IsDir() {
		if filepath.Ext(path) == ".app" {
			plistPath := filepath.Join(path, "Contents", "Info.plist")
			if _, err := os.Stat(plistPath); err == nil {
				return plistPath, nil
			}
		}
		return "", fmt.Errorf("%s is not an .app bundle containing Contents/Info.plist", path)
	}
	if filepath.Base(path) != "Info.plist" {
		return "", fmt.Errorf("%s is neither an .app bundle nor an Info.plist", path)
	}
	return path, nil
}

// Info is the Info.plist of an app bundle.
type Info struct {
	path string
	data map[string]any
}

func (a *Info) Get(key string) (any, error) {
	value, ok := a.data[key]
	if !ok {
		return nil, fmt.Errorf("%s has no %s key", a.path, key)
	}
	return value, nil
}

// GetString returns a string-valued key. Info.plist contents are attacker- or
// build-tool-controlled, so a key holding a non-string value is reported as an
// error rather than asserted.
func (a *Info) GetString(key string) (string, error) {
	value, err := a.Get(key)
	if err != nil {
		return "", err
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s is not a string (got %T)", key, value)
	}
	return str, nil
}

func (a *Info) Version() (string, error) {
	value, err := a.GetString("CFBundleShortVersionString")
	if err != nil {
		return a.GetString("CFBundleVersion")
	}
	return value, nil
}

func (a *Info) BundleID() (string, error) {
	return a.GetString("CFBundleIdentifier")
}

func (a *Info) BundleExecutable() (string, error) {
	return a.GetString("CFBundleExecutable")
}

func (a *Info) BundleName() (string, error) {
	return a.GetString("CFBundleName")
}

func (a *Info) IconFilePath() (string, error) {
	iconFile, err := a.GetString("CFBundleIconFile")
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(a.path)
	iconPath := filepath.Join(dir, "Resources", iconFile)
	if !strings.HasSuffix(iconPath, ".icns") {
		iconPath += ".icns"
	}
	if _, err := os.Stat(iconPath); errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%s names %q as its icon, but %s does not exist",
			a.path, iconFile, iconPath)
	}
	return iconPath, nil
}

// Open locates and reads the Info.plist of an .app bundle, or reads an
// Info.plist given directly.
func Open(path string) (*Info, error) {
	plistPath, err := findPlistPath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(plistPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", plistPath, err)
	}
	plistData, err := plist.ParseDict(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", plistPath, err)
	}
	return &Info{
		path: plistPath,
		data: plistData,
	}, nil
}
