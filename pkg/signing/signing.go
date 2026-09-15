// Package signing presents one way to sign, notarize and staple Apple
// artifacts, over two toolchains that agree on almost nothing else.
//
// On macOS the work is done by Apple's own tools, which take a signing identity
// from the keychain and notarize through notarytool. Everywhere else it is done
// by rcodesign, which has no keychain to consult and so takes a certificate
// file, and which talks to the App Store Connect API directly and so takes an
// API key rather than an Apple ID.
//
// The backends live below this package and know nothing of it: each takes the
// options it actually needs, and Select translates the credentials a caller
// gathered into whichever of them is going to run.
package signing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Credentials carry everything either toolchain might need. A caller fills in
// the ones it was given; Select works out which toolchain that implies and
// hands it the subset it understands.
type Credentials struct {
	// Identity names a keychain identity, for Apple's tools. An empty value
	// means the best matching Developer ID is chosen.
	Identity string

	// P12File and PEMFile name a signing certificate by file, for rcodesign.
	P12File         string
	P12Password     string
	P12PasswordFile string
	PEMFile         string

	// Profile, or the Apple ID trio, authenticate notarytool.
	Profile  string
	AppleID  string
	Password string
	TeamID   string

	// APIKeyFile is an App Store Connect API key, for rcodesign.
	APIKeyFile string
}

// namesCertificateFile reports whether c carries credentials only rcodesign
// understands: a certificate held in a file rather than a keychain.
func (c Credentials) namesCertificateFile() bool {
	return c.P12File != "" || c.PEMFile != "" || c.P12Password != "" ||
		c.P12PasswordFile != "" || c.APIKeyFile != ""
}

// namesKeychainIdentity reports whether c carries credentials only Apple's
// tools understand: a keychain identity, or an Apple ID for notarytool.
func (c Credentials) namesKeychainIdentity() bool {
	return c.Identity != "" || c.Profile != "" || c.AppleID != "" ||
		c.Password != "" || c.TeamID != ""
}

// Backend signs, notarizes and staples through one toolchain. A backend is
// built with the credentials it needs, so the methods take only what varies per
// call.
type Backend interface {
	// Name identifies the toolchain, for logging.
	Name() string
	// Describe reports which credential will be used to sign path, in a form
	// safe to log.
	Describe(ctx context.Context, path string) (string, error)
	// Sign signs the artifact at path in place.
	Sign(ctx context.Context, path string) error
	// Submit uploads the artifact for notarization and waits for the verdict.
	Submit(ctx context.Context, path string) error
	// Staple attaches an issued notarization ticket to the artifact.
	Staple(ctx context.Context, path string) error
}

// Notarize submits path and, if asked, staples the resulting ticket.
//
// An app bundle is archived first: the notary service takes an archive, not a
// directory. The ticket is then stapled to the bundle itself rather than to the
// archive, which is thrown away.
func Notarize(ctx context.Context, b Backend, path string, staple bool) error {
	submitPath := path
	if strings.EqualFold(filepath.Ext(path), ".app") {
		tempDir, err := os.MkdirTemp("", "zapp-notary-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer func() { _ = os.RemoveAll(tempDir) }()

		submitPath = filepath.Join(tempDir, filepath.Base(path)+".zip")
		if err := createZip(path, submitPath); err != nil {
			return fmt.Errorf("failed to archive %s: %w", path, err)
		}
	}

	if err := b.Submit(ctx, submitPath); err != nil {
		return err
	}
	if !staple {
		return nil
	}
	return b.Staple(ctx, path)
}
