// Package signing presents one way to sign, notarize and staple Apple
// artifacts, over two toolchains that agree on almost nothing else.
//
// On macOS the work is done by Apple's own tools, which take a signing identity
// from the keychain and notarize through notarytool. Everywhere else it is done
// by rcodesign, which has no keychain to consult and so takes a certificate
// file, and which talks to the App Store Connect API directly and so takes an
// API key rather than an Apple ID.
//
// Because the credentials differ, so do the fields a caller fills in; Select
// picks the backend that matches what it was given, and each backend reports
// plainly when something it needs is missing.
package signing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ironpark/zapp/pkg/mactools/rcodesign"
)

// Credentials carry everything either toolchain might need. A caller fills in
// the ones its toolchain uses; Select decides which that is.
type Credentials struct {
	// Identity names a keychain identity, for Apple's tools. An empty value
	// means the best matching Developer ID is chosen.
	Identity string

	// Certificate names a signing certificate by file, for rcodesign.
	Certificate rcodesign.Credentials

	// Keychain profile or Apple ID credentials, for notarytool.
	Profile  string
	AppleID  string
	Password string
	TeamID   string

	// APIKeyFile is an App Store Connect API key, for rcodesign.
	APIKeyFile string
}

// usesCertificateFile reports whether the caller supplied file-based
// credentials, which only rcodesign can use.
func (c Credentials) usesCertificateFile() bool {
	return c.Certificate.Configured() || c.APIKeyFile != ""
}

// Backend signs, notarizes and staples through one toolchain.
type Backend interface {
	// Name identifies the toolchain, for logging.
	Name() string
	// Sign signs the artifact at path in place.
	Sign(ctx context.Context, path string, creds Credentials) error
	// Submit uploads the artifact for notarization and waits for the verdict.
	Submit(ctx context.Context, path string, creds Credentials) error
	// Staple attaches an issued notarization ticket to the artifact.
	Staple(ctx context.Context, path string) error
}

// Notarize submits path and, if asked, staples the resulting ticket.
//
// An app bundle is archived first: the notary service takes an archive, not a
// directory. The ticket is then stapled to the bundle itself rather than to the
// archive, which is thrown away.
func Notarize(ctx context.Context, b Backend, path string, creds Credentials, staple bool) error {
	submitPath := path
	if strings.EqualFold(filepath.Ext(path), ".app") {
		tempDir, err := os.MkdirTemp("", "zapp-notary-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		submitPath = filepath.Join(tempDir, filepath.Base(path)+".zip")
		if err := createZip(path, submitPath); err != nil {
			return fmt.Errorf("failed to archive %s: %w", path, err)
		}
	}

	if err := b.Submit(ctx, submitPath, creds); err != nil {
		return err
	}
	if !staple {
		return nil
	}
	return b.Staple(ctx, path)
}

// Select returns the backend that suits the platform and the credentials.
//
// Apple's tools are preferred on macOS, where they are present and understand
// the keychain. A caller that names a certificate file has asked for rcodesign
// whatever the platform, since Apple's codesign cannot read one; that is how a
// macOS build machine with no usable keychain, such as a CI runner, signs.
func Select(creds Credentials) (Backend, error) {
	if runtime.GOOS == "darwin" && !creds.usesCertificateFile() {
		return appleBackend{}, nil
	}
	if runtime.GOOS != "darwin" && !creds.usesCertificateFile() {
		return nil, fmt.Errorf("signing away from macOS needs a certificate file, "+
			"because there is no keychain to take an identity from: "+
			"pass --p12-file or --pem-file (running on %s)", runtime.GOOS)
	}
	if err := rcodesign.Available(); err != nil {
		return nil, err
	}
	return rcodesignBackend{}, nil
}
