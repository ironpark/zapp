//go:build !darwin

package signing

import (
	"cmp"
	"context"
	"fmt"

	"github.com/ironpark/zapp/pkg/signing/rcodesign"
)

// Select uses the statically linked Rust backend away from macOS.
func Select(c Credentials) (Backend, error) {
	if c.namesKeychainIdentity() {
		return nil, fmt.Errorf("keychain and Apple ID credentials are macOS-only; use --p12-file, --p12-base64 or --pem-file and --api-key-file")
	}
	if err := c.checkP12(); err != nil {
		return nil, err
	}
	if err := rcodesign.Available(); err != nil {
		return nil, err
	}
	opts := rcodesign.Options{APIKeyFile: c.APIKeyFile}
	if !c.namesCertificate() {
		return rcodesign.New(opts), nil
	}
	// rcodesign reads the certificate as PEM from a path, so it is written to
	// a private file for the run and removed by Close.
	key, cert, chain, err := c.certificate()
	if err != nil {
		return nil, err
	}
	bundle, err := pemBundle(key, cert, chain)
	if err != nil {
		return nil, err
	}
	path, remove, err := writeSecretFile(bundle, "certificate.pem")
	if err != nil {
		return nil, err
	}
	opts.PEMFile = path
	source := cmp.Or(c.PEMFile, c.P12File, "PKCS#12 certificate from base64")
	return &decodedCertificate{Backend: rcodesign.New(opts), source: source, remove: remove}, nil
}

// decodedCertificate is an rcodesign backend that owns the certificate file it
// signs with.
type decodedCertificate struct {
	*rcodesign.Backend
	source string // the certificate as the user named it, for logging
	remove func() error
}

// Describe names the certificate the user supplied rather than the copy
// rcodesign reads.
func (d *decodedCertificate) Describe(context.Context, string) (string, error) {
	return d.source, nil
}

// Close removes the certificate file. Closing twice is harmless.
func (d *decodedCertificate) Close() error {
	if d.remove == nil {
		return nil
	}
	err := d.remove()
	d.remove = nil
	return err
}
