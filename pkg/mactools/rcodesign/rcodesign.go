// Package rcodesign wraps rcodesign, the signing tool from
// https://github.com/indygreg/apple-platform-rs. It signs, notarizes and
// staples Apple artifacts without Apple's tools or a keychain, which is what
// makes it usable away from macOS.
package rcodesign

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	"github.com/ironpark/zapp/pkg/mactools/internal/macexec"
)

// Tool is the executable name. It is looked up on PATH.
const Tool = "rcodesign"

// ErrNotInstalled is returned when rcodesign is not on PATH.
var ErrNotInstalled = errors.New("rcodesign is not installed")

// Credentials locate the signing certificate. rcodesign has no keychain to
// consult away from macOS, so the certificate is named by file.
type Credentials struct {
	P12File         string // PKCS#12 bundle holding the certificate and key
	P12Password     string // the bundle's password, if any
	P12PasswordFile string // a file holding that password, preferred over P12Password
	PEMFile         string // a PEM bundle, as an alternative to PKCS#12
}

// Configured reports whether a certificate was named at all. Without one
// rcodesign signs ad-hoc, which is rarely what a release build wants.
func (c Credentials) Configured() bool {
	return c.P12File != "" || c.PEMFile != ""
}

// args renders the credentials as command line arguments.
func (c Credentials) args() []string {
	var out []string
	if c.P12File != "" {
		out = append(out, "--p12-file", c.P12File)
		switch {
		case c.P12PasswordFile != "":
			out = append(out, "--p12-password-file", c.P12PasswordFile)
		case c.P12Password != "":
			out = append(out, "--p12-password", c.P12Password)
		}
	}
	if c.PEMFile != "" {
		out = append(out, "--pem-file", c.PEMFile)
	}
	return out
}

// Available reports whether rcodesign can be run.
func Available() error {
	if _, err := exec.LookPath(Tool); err != nil {
		return fmt.Errorf("%w: install it from https://github.com/indygreg/apple-platform-rs "+
			"or with `cargo install apple-codesign`: %v", ErrNotInstalled, err)
	}
	return nil
}

// SignOptions are the parts of a signing request beyond the credentials.
type SignOptions struct {
	Entitlements string // path to an entitlements plist
	Runtime      bool   // opt into the hardened runtime
	TeamName     string //
}

// Sign signs the artifact at path in place. rcodesign handles Mach-O binaries,
// app bundles, disk images and installer packages through the one command, so
// unlike Apple's tools there is no separate path for a .pkg.
func Sign(ctx context.Context, path string, creds Credentials, opts SignOptions) error {
	if err := Available(); err != nil {
		return err
	}
	if path == "" {
		return errors.New("a target path is required")
	}

	args := append([]string{"sign"}, creds.args()...)
	if opts.Runtime {
		args = append(args, "--code-signature-flags", "runtime")
	}
	if opts.Entitlements != "" {
		args = append(args, "--entitlements-xml-file", opts.Entitlements)
	}
	if opts.TeamName != "" {
		args = append(args, "--team-name", opts.TeamName)
	}
	// With no output path rcodesign rewrites the input, which is what the
	// callers want.
	args = append(args, path)

	if _, err := macexec.Run(ctx, Tool, args...); err != nil {
		return fmt.Errorf("rcodesign could not sign %s: %w", path, err)
	}
	return nil
}

// NotarizeOptions describe a notarization request. rcodesign talks to the App
// Store Connect API directly, so it takes an API key rather than the Apple ID
// or keychain profile that notarytool accepts.
type NotarizeOptions struct {
	APIKeyFile string // JSON produced by rcodesign encode-app-store-connect-api-key
	Staple     bool   // attach the ticket once the submission is accepted
}

// Notarize uploads path and waits for the result.
func Notarize(ctx context.Context, path string, opts NotarizeOptions) error {
	if err := Available(); err != nil {
		return err
	}
	if opts.APIKeyFile == "" {
		return errors.New("notarizing with rcodesign needs an App Store Connect API key file; " +
			"create one with `rcodesign encode-app-store-connect-api-key`")
	}

	args := []string{"notary-submit", "--api-key-file", opts.APIKeyFile, "--wait"}
	if opts.Staple {
		args = append(args, "--staple")
	}
	args = append(args, path)

	if _, err := macexec.Run(ctx, Tool, args...); err != nil {
		return fmt.Errorf("rcodesign could not notarize %s: %w", path, err)
	}
	return nil
}

// Staple attaches an already issued notarization ticket.
func Staple(ctx context.Context, path string) error {
	if err := Available(); err != nil {
		return err
	}
	if _, err := macexec.Run(ctx, Tool, "staple", path); err != nil {
		return fmt.Errorf("rcodesign could not staple %s: %w", path, err)
	}
	return nil
}
