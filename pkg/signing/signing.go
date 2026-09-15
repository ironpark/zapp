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
	"runtime"
	"strings"

	"github.com/ironpark/zapp/pkg/signing/macos"
	"github.com/ironpark/zapp/pkg/signing/rcodesign"
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

// rcodesignOptions is the subset of the credentials rcodesign understands. It
// is the one place that knows how they map across.
func (c Credentials) rcodesignOptions() rcodesign.Options {
	return rcodesign.Options{
		P12File:         c.P12File,
		P12Password:     c.P12Password,
		P12PasswordFile: c.P12PasswordFile,
		PEMFile:         c.PEMFile,
		APIKeyFile:      c.APIKeyFile,
	}
}

// namesCertificateFile reports whether the caller supplied file-based
// credentials, which only rcodesign can use.
func (c Credentials) namesCertificateFile() bool {
	opts := c.rcodesignOptions()
	return opts.Configured() || opts.APIKeyFile != ""
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

// Select returns the backend that suits the platform and the credentials.
//
// Apple's tools are preferred on macOS, where they are present and understand
// the keychain. A caller that names a certificate file has asked for rcodesign
// whatever the platform, since Apple's codesign cannot read one; that is how a
// macOS build machine with no usable keychain, such as a CI runner, signs.
func Select(creds Credentials) (Backend, error) {
	if !creds.namesCertificateFile() {
		if runtime.GOOS == "darwin" {
			return macos.New(macos.Options{
				Identity: creds.Identity,
				Profile:  creds.Profile,
				AppleID:  creds.AppleID,
				Password: creds.Password,
				TeamID:   creds.TeamID,
			}), nil
		}
		return nil, fmt.Errorf("signing away from macOS needs a certificate file, "+
			"because there is no keychain to take an identity from: "+
			"pass --p12-file or --pem-file (running on %s)", runtime.GOOS)
	}
	if err := rcodesign.Available(); err != nil {
		return nil, err
	}
	return rcodesign.New(creds.rcodesignOptions()), nil
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
