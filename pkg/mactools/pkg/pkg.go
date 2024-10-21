package pkg

import (
	"fmt"
	"github.com/ironpark/zapp/pkg/fsutil"
	"os"
	"os/exec"
	"path/filepath"
)

type Config struct {
	AppPath         string
	OutputPath      string
	Version         string
	Identifier      string
	InstallLocation string
	LicensePaths    map[string]string
}

func CreatePKG(config Config) error {
	// 언어 코드 유효성 검사
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
	cmd := exec.Command("pkgbuild",
		"--root", filepath.Dir(config.AppPath),
		"--install-location", config.InstallLocation,
		"--identifier", config.Identifier,
		"--version", config.Version,
		componentPkgPath)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pkgbuild failed: %v\nOutput: %s", err, output)
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
		if err := fsutil.CopyFileAnyway(sourcePath, destPath); err != nil {
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

	cmd = exec.Command("productbuild",
		"--distribution", distributionPath,
		"--package-path", tempDir,
		"--resources", resourcesDir,
		config.OutputPath)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("productbuild failed: %v\nOutput: %s", err, output)
	}

	return nil
}
