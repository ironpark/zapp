package hdiutil

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Format represents the supported DMG formats
type Format string

const (
	UDRW         Format = "UDRW" // Read/write disk image
	UDRO         Format = "UDRO" // Read-only disk image
	UDCO         Format = "UDCO" // Compressed (ADC)
	UDZO         Format = "UDZO" // Compressed (zlib)
	UDBZ         Format = "UDBZ" // Compressed (bzip2)
	UFBI         Format = "UFBI" // Full block, single-partition image
	UDTO         Format = "UDTO" // DVD/CD master
	UDSP         Format = "UDSP" // Sparse disk image
	UDSB         Format = "UDSB" // Sparse bundle disk image
	UDXX         Format = "UDXX" // Compressed (unknown)
	UDIF         Format = "UDIF" // Generic UDIF format
	SPARSEBUNDLE Format = "SPARSEBUNDLE"
)

var supportedFormats = map[Format]bool{
	UDRW:         true,
	UDRO:         true,
	UDCO:         true,
	UDZO:         true,
	UDBZ:         true,
	UFBI:         true,
	UDTO:         true,
	UDSP:         true,
	UDSB:         true,
	UDXX:         true,
	UDIF:         true,
	SPARSEBUNDLE: true,
}

// Create creates a new DMG file
func Create(ctx context.Context, volName, srcFolder string, format Format, outputFile string) error {
	if !supportedFormats[format] {
		return fmt.Errorf("unsupported format: %s", format)
	}
	return runCommand(ctx, "create", "-volname", volName, "-srcfolder", srcFolder, "-ov", "-format", string(format), outputFile)
}

// Convert converts a DMG file from one format to another
func Convert(ctx context.Context, inputFile string, format Format, outputFile string) error {
	if !supportedFormats[format] {
		return fmt.Errorf("unsupported format: %s", format)
	}
	return runCommand(ctx, "convert", inputFile, "-format", string(format), "-o", outputFile)
}

// Attach mounts a DMG file
func Attach(ctx context.Context, dmgPath, mountPoint string) error {
	args := []string{dmgPath}
	if mountPoint != "" {
		args = append(args, "-mountpoint", mountPoint)
	}
	return runCommand(ctx, "attach", args...)
}

// Detach unmounts a DMG file with retry
func Detach(ctx context.Context, target string) error {
	const (
		maxRetries = 5
		retryDelay = time.Second
	)

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		err := runCommand(ctx, "detach", target)
		if err == nil {
			return nil // Success
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
			// Wait before next retry
		}
	}

	return fmt.Errorf("failed to detach after %d attempts: %w", maxRetries, lastErr)
}

// Helper function to run hdiutil commands
func runCommand(ctx context.Context, operation string, args ...string) error {
	cmd := exec.CommandContext(ctx, "hdiutil", append([]string{operation}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hdiutil %s failed: %w, output: %s", operation, err, string(output))
	}
	return nil
}
