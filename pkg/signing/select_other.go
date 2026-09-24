//go:build !darwin

package signing

import (
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
	opts := rcodesign.Options{P12File: c.P12File, P12Password: c.P12Password, P12PasswordFile: c.P12PasswordFile, PEMFile: c.PEMFile, APIKeyFile: c.APIKeyFile}
	if c.P12Base64 == "" {
		return rcodesign.New(opts), nil
	}
	// rcodesign reads the certificate from a path, so a base64 certificate is
	// written to a private file for the run and removed by Close.
	p12, err := c.p12()
	if err != nil {
		return nil, err
	}
	path, remove, err := writeSecretFile(p12, "certificate.p12")
	if err != nil {
		return nil, err
	}
	opts.P12File = path
	return &decodedCertificate{Backend: rcodesign.New(opts), remove: remove}, nil
}

// decodedCertificate is an rcodesign backend that owns the certificate file it
// signs with.
type decodedCertificate struct {
	*rcodesign.Backend
	remove func() error
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
