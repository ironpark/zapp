// Package productsign wraps the productsign(1) tool, which signs installer
// packages. Unlike codesign it cannot sign in place, so Sign writes to a
// temporary file and swaps it over the original.
package signing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironpark/zapp/internal/macexec"
)

// runProductsign signs the installer package at path with identity, replacing it with the
// signed copy.
func runProductsign(ctx context.Context, path, identity string) error {
	if identity == "" || path == "" {
		return fmt.Errorf("identity and path are required")
	}

	// Sign into the package's own directory: os.Rename cannot move across
	// filesystems, and the system temp directory is often a separate volume.
	tempDir, err := os.MkdirTemp(filepath.Dir(path), ".zapp-pkg-signing-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	signedPath := filepath.Join(tempDir, filepath.Base(path))
	if _, err := macexec.Run(ctx, "productsign", "--sign", identity, path, signedPath); err != nil {
		return fmt.Errorf("failed to sign pkg: %w", err)
	}

	if err := os.Rename(signedPath, path); err != nil {
		return fmt.Errorf("failed to replace original pkg with signed one: %w", err)
	}
	return nil
}
