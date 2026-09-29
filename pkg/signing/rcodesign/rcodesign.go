// Package rcodesign binds the statically linked apple-codesign Rust library.
package rcodesign

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/pkg/signing/notarylog"
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

	// EntitlementsFile replaces the main executable's entitlements; empty
	// keeps the ones it has. Left out of the request when empty, so a library
	// that predates it still takes every other request.
	EntitlementsFile string `json:"entitlements_file,omitempty"`

	// NotarizeTimeoutSecs bounds the wait for the notary's verdict; zero is
	// the library's ten minutes. Left out when zero, like EntitlementsFile.
	NotarizeTimeoutSecs uint64 `json:"notarize_timeout_secs,omitempty"`
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

	opts := b.opts
	if !strings.EqualFold(filepath.Ext(path), ".app") {
		opts.EntitlementsFile = "" // only an app's executable has entitlements
	}
	if err := invoke(ctx, "sign", path, opts); err != nil {
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
		return fmt.Errorf("rcodesign could not notarize %s: %w", path, readableLog(err))
	}
	return nil
}

// readableLog renders the notary log a rejection carries, as JSON after
// "; notary log: ", as the issues it lists.
func readableLog(err error) error {
	verdict, log, ok := strings.Cut(err.Error(), "; notary log: ")
	if !ok {
		return err
	}
	if summary := notarylog.Summary([]byte(log)); summary != "" {
		return errors.New(verdict + ": " + summary)
	}
	return err
}

// Staple attaches an already issued notarization ticket.
func (b *Backend) Staple(ctx context.Context, path string) error {
	if err := invoke(ctx, "staple", path, b.opts); err != nil {
		return fmt.Errorf("rcodesign could not staple %s: %w", path, err)
	}
	return nil
}

// Verify checks the code digests and CMS signature of the Mach-O at path, or
// of every Mach-O in the bundle at path. It reads no credentials. A bundle's
// sealed resources are not checked.
func Verify(ctx context.Context, path string) error {
	return invoke(ctx, "verify", path, Options{})
}
