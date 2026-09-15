package pkg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironpark/zapp/internal/fsutil"
	"github.com/ironpark/zapp/pkg/mactools/internal/macexec"
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
	for lang := range config.LicensePaths {
		if !isValidLanguageCode(lang) {
			return fmt.Errorf("invalid language code: %s", lang)
		}
	}

	tempDir, err := os.MkdirTemp("", "pkg-build")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	componentPkgPath := filepath.Join(tempDir, "component.pkg")
	_, err = macexec.Run(ctx, "pkgbuild",
		"--root", filepath.Dir(config.AppPath),
		"--install-location", config.InstallLocation,
		"--identifier", config.Identifier,
		"--version", config.Version,
		componentPkgPath)
	if err != nil {
		return fmt.Errorf("pkgbuild failed: %w", err)
	}

	// Create resources directory with lproj folders
	resourcesDir := filepath.Join(tempDir, "Resources")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		return fmt.Errorf("failed to create resources directory: %v", err)
	}

	for lang, sourcePath := range config.LicensePaths {
		lprojDir := filepath.Join(resourcesDir, lang+".lproj")
		if err := os.MkdirAll(lprojDir, 0755); err != nil {
			return fmt.Errorf("failed to create lproj directory for %s: %v", lang, err)
		}
		destPath := filepath.Join(lprojDir, "license.txt")
		if err := fsutil.CopyFile(sourcePath, destPath); err != nil {
			return fmt.Errorf("failed to copy license file for %s: %v", lang, err)
		}
	}

	builder := NewDistributionBuilder()
	builder.Title = filepath.Base(config.AppPath)
	builder.Organization = config.Identifier
	builder.Identifier = config.Identifier
	builder.Version = config.Version
	builder.AddLicense("license.txt")
	builder.AddChoice("choice1", false, config.Identifier)
	distributionContent := builder.Build()

	distributionPath := filepath.Join(tempDir, "distribution.xml")
	err = os.WriteFile(distributionPath, []byte(distributionContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to create distribution.xml: %v", err)
	}

	_, err = macexec.Run(ctx, "productbuild",
		"--distribution", distributionPath,
		"--package-path", tempDir,
		"--resources", resourcesDir,
		config.OutputPath)
	if err != nil {
		return fmt.Errorf("productbuild failed: %w", err)
	}

	return nil
}
