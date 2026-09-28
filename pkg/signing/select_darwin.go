package signing

import (
	"errors"

	"github.com/ironpark/zapp/pkg/signing/macos"
)

// Select always uses Apple's signing tools on macOS. A PKCS#12 certificate,
// from a file or base64, or a PEM certificate and key, is imported into a
// temporary keychain for the run; otherwise the identity comes from the user's
// keychains.
func Select(c Credentials) (Backend, error) {
	if c.APIKeyFile != "" {
		return nil, errors.New("macOS notarizes with Apple's notarytool: use --profile or --apple-id, --password and --team-id")
	}
	if err := c.checkP12(); err != nil {
		return nil, err
	}
	opts := macos.Options{Identity: c.Identity, Profile: c.Profile, AppleID: c.AppleID, Password: c.Password, TeamID: c.TeamID}
	switch {
	case c.PEMFile != "":
		p12, password, err := pemToP12(c.PEMFile)
		if err != nil {
			return nil, err
		}
		opts.P12, opts.P12Password = p12, password
	case c.namesP12():
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
