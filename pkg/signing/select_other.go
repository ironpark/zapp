//go:build !darwin

package signing

import (
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
	opts := rcodesign.Options{PEMFile: c.PEMFile, APIKeyFile: c.APIKeyFile}
	if !c.namesP12() {
		return rcodesign.New(opts), nil
	}
	// rcodesign cannot read PKCS#12 in the encryption most tools now write, so
	// the certificate is unpacked to a private PEM file for the run and removed
	// by Close.
	p12, err := c.p12()
	if err != nil {
		return nil, err
	}
	password, err := c.p12Password()
	if err != nil {
		return nil, err
	}
	bundle, err := p12ToPEM(p12, password)
	if err != nil {
		return nil, err
	}
	path, remove, err := writeSecretFile(bundle, "certificate.pem")
	if err != nil {
		return nil, err
	}
	opts.PEMFile = path
	source := c.P12File
	if source == "" {
		source = "PKCS#12 certificate from base64"
	}
	return &decodedCertificate{Backend: rcodesign.New(opts), source: source, remove: remove}, nil
}

// decodedCertificate is an rcodesign backend that owns the certificate file it
// signs with.
type decodedCertificate struct {
	*rcodesign.Backend
	source string // the certificate as the user named it, for logging
	remove func() error
}

// Describe names the certificate the user supplied rather than the unpacked
// copy rcodesign reads.
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
