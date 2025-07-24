package plist

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"
)

func findPlistPath(path string) (string, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("error accessing path: %v", err)
	}

	if fileInfo.IsDir() {
		if filepath.Ext(path) == ".app" {
			plistPath := filepath.Join(path, "Contents", "Info.plist")
			if _, err := os.Stat(plistPath); err == nil {
				return plistPath, nil
			}
		}
		return "", fmt.Errorf("not a valid .app directory or Info.plist not found")
	}
	if filepath.Base(path) != "Info.plist" {
		return "", fmt.Errorf("not a .plist file")
	}
	return path, nil
}

type AppInfo struct {
	path string
	data map[string]interface{}
}

func (a *AppInfo) Get(key string) (interface{}, error) {
	value, ok := a.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	return value, nil
}

func (a *AppInfo) Version() (string, error) {
	value, err := a.Get("CFBundleShortVersionString")
	if err != nil {
		value, err = a.Get("CFBundleVersion")
		if err != nil {
			return "", err
		}
	}
	return value.(string), nil
}

func (a *AppInfo) BundleID() (string, error) {
	value, err := a.Get("CFBundleIdentifier")
	if err != nil {
		return "", err
	}
	return value.(string), nil
}

func (a *AppInfo) BundleExecutable() (string, error) {
	value, err := a.Get("CFBundleExecutable")
	if err != nil {
		return "", err
	}
	return value.(string), nil
}
func (a *AppInfo) BundleName() (string, error) {
	value, err := a.Get("CFBundleName")
	if err != nil {
		return "", err
	}
	return value.(string), nil
}

func (a *AppInfo) IconFilePath() (string, error) {
	value, err := a.Get("CFBundleIconFile")
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(a.path)
	iconPath := filepath.Join(dir, "Resources", value.(string))
	if !strings.HasSuffix(iconPath, ".icns") {
		iconPath += ".icns"
	}
	_, err = os.Stat(iconPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("icon file not found")
	}
	return iconPath, nil
}

func GetAppInfo(path string) (*AppInfo, error) {
	plistPath, err := findPlistPath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(plistPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plist file: %v", err)
	}
	var plistData map[string]interface{}
	_, err = plist.Unmarshal(data, &plistData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plist: %v", err)
	}
	return &AppInfo{
		path: plistPath,
		data: plistData,
	}, nil
}
