// Package rcodesign binds the statically linked apple-codesign Rust library.
package rcodesign

import (
	"context"
	"errors"
	"fmt"
)

const Tool = "rcodesign"

// ErrUnavailable means this build does not contain the Rust static library.
var ErrUnavailable = errors.New("rcodesign static binding is unavailable in this build")

// errNoCertificate is returned when no signing certificate was named. rcodesign
// would sign ad-hoc instead, which is rarely what a release build wants.
var errNoCertificate = errors.New("no signing certificate was given; pass --p12-file, --p12-base64 or --pem-file")

// Options are the credentials rcodesign takes. It has no keychain to consult,
// so the certificate is named by file, and it reaches the notary service
// directly, so it authenticates with an App Store Connect API key.
//
// The certificate is always PEM: the library's PKCS#12 reader only
// understands legacy encryption, so callers unpack PKCS#12 themselves.
type Options struct {
	PEMFile string `json:"pem_file"` // the private key and certificate chain

	APIKeyFile string `json:"api_key_file"` // JSON from rcodesign encode-app-store-connect-api-key
}

// Configured reports whether a certificate was named at all. Without one
// rcodesign signs ad-hoc, which is rarely what a release build wants.
func (c Options) Configured() bool {
	return c.PEMFile != ""
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
	if !b.opts.Configured() {
		return "", errNoCertificate
	}
	return b.opts.PEMFile, nil
}

// Sign signs the artifact at path in place. rcodesign handles Mach-O binaries,
// app bundles, disk images and installer packages through the one command, so
// unlike Apple's tools there is no separate path for a .pkg.
func (b *Backend) Sign(ctx context.Context, path string) error {
	if path == "" {
		return errors.New("a target path is required")
	}
	if !b.opts.Configured() {
		return errNoCertificate
	}

	if err := invoke(ctx, "sign", path, b.opts); err != nil {
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
	if err := invoke(ctx, "submit", path, b.opts); err != nil {
		return fmt.Errorf("rcodesign could not notarize %s: %w", path, err)
	}
	return nil
}

// Staple attaches an already issued notarization ticket.
func (b *Backend) Staple(ctx context.Context, path string) error {
	if err := invoke(ctx, "staple", path, b.opts); err != nil {
		return fmt.Errorf("rcodesign could not staple %s: %w", path, err)
	}
	return nil
}
