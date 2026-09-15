//go:build !darwin

package signing

import (
	"fmt"
	"github.com/ironpark/zapp/pkg/signing/rcodesign"
)

// Select uses the statically linked Rust backend away from macOS.
func Select(c Credentials) (Backend, error) {
	if c.Identity != "" || c.Profile != "" || c.AppleID != "" || c.Password != "" || c.TeamID != "" {
		return nil, fmt.Errorf("keychain and Apple ID credentials are macOS-only; use --p12-file or --pem-file and --api-key-file")
	}
	if err := rcodesign.Available(); err != nil {
		return nil, err
	}
	return rcodesign.New(rcodesign.Options{P12File: c.P12File, P12Password: c.P12Password, P12PasswordFile: c.P12PasswordFile, PEMFile: c.PEMFile, APIKeyFile: c.APIKeyFile}), nil
}
