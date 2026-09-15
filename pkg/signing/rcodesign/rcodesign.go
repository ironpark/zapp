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

	"github.com/ironpark/zapp/internal/macexec"
)

// Tool is the executable name. It is looked up on PATH.
const Tool = "rcodesign"

// ErrNotInstalled is returned when rcodesign is not on PATH.
var ErrNotInstalled = errors.New("rcodesign is not installed")

// Options are the credentials rcodesign takes. It has no keychain to consult,
// so the certificate is named by file, and it reaches the notary service
// directly, so it authenticates with an App Store Connect API key.
type Options struct {
	P12File         string // PKCS#12 bundle holding the certificate and key
	P12Password     string // the bundle's password, if any
	P12PasswordFile string // a file holding that password, preferred over P12Password
	PEMFile         string // a PEM bundle, as an alternative to PKCS#12

	APIKeyFile string // JSON from rcodesign encode-app-store-connect-api-key
}

// Configured reports whether a certificate was named at all. Without one
// rcodesign signs ad-hoc, which is rarely what a release build wants.
func (c Options) Configured() bool {
	return c.P12File != "" || c.PEMFile != ""
}

// args renders the credentials as command line arguments.
func (c Options) args() []string {
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

// Backend signs with rcodesign.
type Backend struct {
	opts Options
}

// New returns a backend that signs with the given credentials.
func New(opts Options) *Backend { return &Backend{opts: opts} }

func (b *Backend) Name() string { return Tool }

// Describe reports the certificate file that will be used.
func (b *Backend) Describe(context.Context, string) (string, error) {
	switch {
	case b.opts.P12File != "":
		return b.opts.P12File, nil
	case b.opts.PEMFile != "":
		return b.opts.PEMFile, nil
	default:
		return "", errors.New("no signing certificate was given; pass --p12-file or --pem-file")
	}
}

// Sign signs the artifact at path in place. rcodesign handles Mach-O binaries,
// app bundles, disk images and installer packages through the one command, so
// unlike Apple's tools there is no separate path for a .pkg.
func (b *Backend) Sign(ctx context.Context, path string) error {
	if path == "" {
		return errors.New("a target path is required")
	}
	if !b.opts.Configured() {
		return errors.New("no signing certificate was given; pass --p12-file or --pem-file")
	}

	args := append([]string{"sign"}, b.opts.args()...)
	// Opt into the hardened runtime, which notarization requires.
	args = append(args, "--code-signature-flags", "runtime")
	// With no output path rcodesign rewrites the input, which is what the
	// callers want.
	args = append(args, path)

	if _, err := macexec.Run(ctx, Tool, args...); err != nil {
		return fmt.Errorf("rcodesign could not sign %s: %w", path, err)
	}
	return nil
}

// Submit uploads path and waits for the verdict. Stapling is left to Staple so
// that the ticket lands on the bundle rather than on the archive submitted.
func (b *Backend) Submit(ctx context.Context, path string) error {
	if b.opts.APIKeyFile == "" {
		return errors.New("notarizing with rcodesign needs an App Store Connect API key file; " +
			"create one with `rcodesign encode-app-store-connect-api-key`")
	}
	if _, err := macexec.Run(ctx, Tool, "notary-submit",
		"--api-key-file", b.opts.APIKeyFile, "--wait", path); err != nil {
		return fmt.Errorf("rcodesign could not notarize %s: %w", path, err)
	}
	return nil
}

// Staple attaches an already issued notarization ticket.
func (b *Backend) Staple(ctx context.Context, path string) error {
	if _, err := macexec.Run(ctx, Tool, "staple", path); err != nil {
		return fmt.Errorf("rcodesign could not staple %s: %w", path, err)
	}
	return nil
}
