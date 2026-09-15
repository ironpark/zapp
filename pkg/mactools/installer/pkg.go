package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/internal/fsutil"
	"github.com/ironpark/zapp/pkg/macpkg"
)

type Config struct {
	AppPath         string
	OutputPath      string
	Version         string
	Identifier      string
	InstallLocation string
	LicensePaths    map[string]string
}

func CreatePKG(ctx context.Context, config Config) error {
	// Validate the language codes.
	seenLanguages := make(map[string]bool)
	for lang := range config.LicensePaths {
		if !isValidLanguageCode(lang) {
			return fmt.Errorf("invalid language code: %s", lang)
		}
		normalized := strings.ToLower(lang)
		if seenLanguages[normalized] {
			return fmt.Errorf("duplicate license language: %s", lang)
		}
		seenLanguages[normalized] = true
	}

	tempDir, err := os.MkdirTemp("", "pkg-build")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	componentPkgPath := filepath.Join(tempDir, "component.pkg")
	appPath := filepath.Clean(config.AppPath)
	err = macpkg.BuildComponent(ctx, macpkg.ComponentConfig{
		Root: filepath.Dir(appPath), RootEntry: filepath.Base(appPath),
		InstallLocation: config.InstallLocation, Identifier: config.Identifier,
		Version: config.Version, OutputPath: componentPkgPath,
	})
	if err != nil {
		return fmt.Errorf("build component: %w", err)
	}

	// Create resources directory with lproj folders
	resourcesDir := filepath.Join(tempDir, "Resources")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		return fmt.Errorf("failed to create resources directory: %v", err)
	}

	for lang, sourcePath := range config.LicensePaths {
		lprojDir := filepath.Join(resourcesDir, strings.ToLower(lang)+".lproj")
		if err := os.MkdirAll(lprojDir, 0755); err != nil {
			return fmt.Errorf("failed to create lproj directory for %s: %v", lang, err)
		}
		destPath := filepath.Join(lprojDir, "license.txt")
		if err := fsutil.CopyFile(sourcePath, destPath); err != nil {
			return fmt.Errorf("failed to copy license file for %s: %v", lang, err)
		}
	}

	distribution := &macpkg.Distribution{Title: filepath.Base(appPath), Organization: config.Identifier}
	if len(config.LicensePaths) > 0 {
		distribution.LicenseFile = "license.txt"
	}
	err = macpkg.BuildProduct(ctx, macpkg.ProductConfig{
		Packages: []string{componentPkgPath}, ResourcesDir: resourcesDir,
		OutputPath: config.OutputPath, Distribution: distribution,
	})
	if err != nil {
		return fmt.Errorf("build product: %w", err)
	}

	return nil
}
