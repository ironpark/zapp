package macpkg

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// AppConfig describes an installer that places a single .app bundle into
// InstallLocation. It is a convenience layer over BuildComponent and
// BuildProduct for the common case of shipping one application.
type AppConfig struct {
	AppPath    string // Path to the .app bundle to install.
	OutputPath string
	Identifier string
	Version    string

	InstallLocation string // Defaults to /Applications.
	ScriptsDir      string
	MinOSVersion    string
	PayloadMode     PayloadMode
	Ownership       Ownership

	Title        string // Installer window title; defaults to the app name.
	Organization string // Defaults to Identifier.

	// License is the license shown before installation. Licenses maps ISO 639-1
	// language codes to per-language files, which win in a matching locale;
	// License is the fallback for every other locale. Both are optional.
	License  string
	Licenses map[string]string
}

// licenseName is the file name every locale directory uses, so Distribution can
// reference a single relative path.
const licenseName = "license.txt"

// BuildApp creates an unsigned product installer for one application bundle.
// Existing output survives failed builds.
func BuildApp(ctx context.Context, c AppConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.AppPath == "" {
		return fmt.Errorf("app path is required")
	}
	appPath := filepath.Clean(c.AppPath)
	info, err := os.Stat(appPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not an app bundle", appPath)
	}
	licenses, err := resolveLicenses(c.License, c.Licenses)
	if err != nil {
		return err
	}
	if c.InstallLocation == "" {
		c.InstallLocation = "/Applications"
	}
	if c.Title == "" {
		c.Title = strings.TrimSuffix(filepath.Base(appPath), ".app")
	}
	if c.Organization == "" {
		c.Organization = c.Identifier
	}

	work, err := os.MkdirTemp("", "zapp-macpkg-app-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()

	component := filepath.Join(work, "component.pkg")
	err = BuildComponent(ctx, ComponentConfig{
		Root:            filepath.Dir(appPath),
		RootEntry:       filepath.Base(appPath),
		OutputPath:      component,
		Identifier:      c.Identifier,
		Version:         c.Version,
		InstallLocation: c.InstallLocation,
		ScriptsDir:      c.ScriptsDir,
		MinOSVersion:    c.MinOSVersion,
		PayloadMode:     c.PayloadMode,
		Ownership:       c.Ownership,
	})
	if err != nil {
		return fmt.Errorf("build component: %w", err)
	}

	distribution := &Distribution{Title: c.Title, Organization: c.Organization, MinOSVersion: c.MinOSVersion}
	var resources string
	if len(licenses) > 0 {
		resources = filepath.Join(work, "Resources")
		if err = writeLicenses(resources, licenses); err != nil {
			return err
		}
		distribution.LicenseFile = licenseName
	}
	err = BuildProduct(ctx, ProductConfig{
		Packages:     []string{component},
		ResourcesDir: resources,
		OutputPath:   c.OutputPath,
		Distribution: distribution,
	})
	if err != nil {
		return fmt.Errorf("build product: %w", err)
	}
	return nil
}

// resolveLicenses keys license files by lower-cased language code, reserving the
// empty key for the non-localized fallback at the Resources root.
func resolveLicenses(fallback string, localized map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(localized)+1)
	if fallback != "" {
		out[""] = fallback
	}
	for lang, path := range localized {
		if !isValidLanguageCode(lang) {
			return nil, fmt.Errorf("invalid license language code: %s", lang)
		}
		if path == "" {
			return nil, fmt.Errorf("missing license path for language: %s", lang)
		}
		code := strings.ToLower(lang)
		if _, ok := out[code]; ok {
			return nil, fmt.Errorf("duplicate license language: %s", lang)
		}
		out[code] = path
	}
	return out, nil
}

func writeLicenses(resources string, licenses map[string]string) error {
	for code, src := range licenses {
		dir, label := resources, "default"
		if code != "" {
			dir, label = filepath.Join(resources, code+".lproj"), code
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		if err := copyFile(src, filepath.Join(dir, licenseName)); err != nil {
			return fmt.Errorf("%s license: %w", label, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
