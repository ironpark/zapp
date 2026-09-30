//go:build !darwin

package signing

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

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
	if err := checkAPIKey(c.APIKeyFile); err != nil {
		return nil, err
	}
	if err := rcodesign.Available(); err != nil {
		return nil, err
	}
	opts := rcodesign.Options{APIKeyFile: c.APIKeyFile, EntitlementsFile: c.Entitlements, NotarizeTimeoutSecs: uint64(c.NotarizeTimeout.Round(time.Second) / time.Second)}
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

// checkAPIKey rejects an API key JSON rcodesign cannot read before it fails
// with "invalid unified api key": its private_key must be the base64 DER of
// the .p8, not the PEM itself.
func checkAPIKey(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var key struct {
		PrivateKey string `json:"private_key"`
	}
	if err := json.Unmarshal(data, &key); err != nil {
		return fmt.Errorf("%s is not an App Store Connect API key JSON: %w", path, err)
	}
	if strings.Contains(key.PrivateKey, "-----BEGIN") {
		return fmt.Errorf("%s holds the private key as PEM; rcodesign needs its base64 DER, as `rcodesign encode-app-store-connect-api-key` writes it", path)
	}
	return nil
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
