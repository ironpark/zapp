package signing

import (
	"errors"

	"github.com/ironpark/zapp/pkg/signing/macos"
)

// Select always uses Apple's signing tools on macOS. A PKCS#12 certificate,
// from a file or base64, is imported into a temporary keychain for the run;
// otherwise the identity comes from the user's keychains.
func Select(c Credentials) (Backend, error) {
	if c.PEMFile != "" || c.APIKeyFile != "" {
		return nil, errors.New("macOS uses Apple's signing tools: sign with a keychain identity or a PKCS#12 " +
			"certificate (--p12-file or --p12-base64); notarize with --profile or --apple-id, --password and --team-id")
	}
	if err := c.checkP12(); err != nil {
		return nil, err
	}
	opts := macos.Options{Identity: c.Identity, Profile: c.Profile, AppleID: c.AppleID, Password: c.Password, TeamID: c.TeamID}
	if c.namesP12() {
		p12, err := c.p12()
		if err != nil {
			return nil, err
		}
		password, err := c.p12Password()
		if err != nil {
			return nil, err
		}
		opts.P12, opts.P12Password = p12, password
	}
	return macos.New(opts), nil
}
