// Package signing presents one way to sign, notarize and staple Apple
// artifacts, over two toolchains that agree on almost nothing else.
//
// On macOS the work is done by Apple's own tools, which take a signing identity
// from the keychain and notarize through notarytool. A PKCS#12 or PEM
// certificate is imported into a temporary keychain for the run, the way CI
// imports a certificate secret. Everywhere else it is done by rcodesign, which has no
// keychain to consult and so takes a certificate file, and which talks to the
// App Store Connect API directly and so takes an API key rather than an Apple
// ID.
//
// The backends live below this package and know nothing of it: each takes the
// options it actually needs, and Select translates the credentials a caller
// gathered into whichever of them is going to run.
package signing

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/ironpark/zapp/pkg/archive"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Credentials carry everything either toolchain might need. A caller fills in
// the ones it was given; Select works out which toolchain that implies and
// hands it the subset it understands.
type Credentials struct {
	// Identity names a keychain identity, for Apple's tools. An empty value
	// means the best matching Developer ID is chosen.
	Identity string

	// P12File and P12Base64 supply a PKCS#12 certificate and private key: as a
	// file, or as base64 text, which is how CI secrets usually hold one. Both
	// toolchains take either: macOS imports it into a temporary keychain, and
	// rcodesign is handed it unpacked as PEM.
	P12File         string
	P12Base64       string
	P12Password     string
	P12PasswordFile string

	// PEMFile names a PEM certificate and private key. rcodesign reads it
	// directly; macOS repackages it as PKCS#12 for a temporary keychain.
	PEMFile string

	// Entitlements names a plist of entitlements to sign an app's main
	// executable with. Empty keeps the entitlements it already carries.
	Entitlements string

	// Profile, or the Apple ID trio, authenticate notarytool.
	Profile  string
	AppleID  string
	Password string
	TeamID   string

	// APIKeyFile is an App Store Connect API key as rcodesign's
	// encode-app-store-connect-api-key writes it. Every platform reads it.
	APIKeyFile string

	// NotarizeTimeout bounds the wait for the notary's verdict. Zero leaves
	// it to the toolchain.
	NotarizeTimeout time.Duration
}

// namesP12 reports whether c supplies a PKCS#12 certificate, in either form.
func (c Credentials) namesP12() bool { return c.P12File != "" || c.P12Base64 != "" }

// checkP12 rejects PKCS#12 options that cannot mean anything together: the
// certificate given twice, a password with no certificate for it to open, or
// a PEM certificate alongside it.
func (c Credentials) checkP12() error {
	if c.P12File != "" && c.P12Base64 != "" {
		return errors.New("pass the PKCS#12 certificate once: --p12-file or --p12-base64, not both")
	}
	if !c.namesP12() && (c.P12Password != "" || c.P12PasswordFile != "") {
		return errors.New("a PKCS#12 password was given without --p12-file or --p12-base64")
	}
	if c.namesP12() && c.PEMFile != "" {
		return errors.New("pass one certificate: --pem-file or a PKCS#12 certificate, not both")
	}
	return nil
}

// p12 returns the PKCS#12 bundle's bytes, read from its file or decoded from
// base64. Whitespace inside the base64 is ignored, since tools such as
// `openssl base64` wrap their output.
func (c Credentials) p12() ([]byte, error) {
	if c.P12Base64 == "" {
		return os.ReadFile(c.P12File)
	}
	data, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(c.P12Base64), ""))
	if err != nil {
		return nil, fmt.Errorf("--p12-base64 is not valid base64: %w", err)
	}
	return data, nil
}

// p12Password returns the bundle's password. A password file takes precedence
// and only its first line is used, as rcodesign reads one.
func (c Credentials) p12Password() (string, error) {
	if c.P12PasswordFile == "" {
		return c.P12Password, nil
	}
	data, err := os.ReadFile(c.P12PasswordFile)
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(string(data), "\n")
	return strings.TrimSuffix(line, "\r"), nil
}

// writeSecretFile writes data to a file only the current user can read, for a
// tool that takes a path, and returns a function that removes it.
func writeSecretFile(data []byte, name string) (string, func() error, error) {
	dir, err := os.MkdirTemp("", "zapp-secret-")
	if err != nil {
		return "", nil, err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	return path, func() error { return os.RemoveAll(dir) }, nil
}

// Close releases whatever a backend holds for its run: the temporary keychain
// a PKCS#12 import creates on macOS, or the decoded certificate file
// elsewhere. It is a no-op for backends that hold nothing, including ones a
// caller supplied, which stay the caller's to release.
func Close(b Backend) error {
	if c, ok := b.(io.Closer); ok {
		return c.Close()
	}
	return nil
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
		if err := archive.Zip(ctx, path, submitPath); err != nil {
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
