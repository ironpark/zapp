package signing

import "github.com/ironpark/zapp/pkg/signing/macos"

// Select always uses Apple's signing tools on macOS. A PKCS#12 certificate,
// from a file or base64, or a PEM certificate and key, is imported into a
// temporary keychain for the run; otherwise the identity comes from the user's
// keychains.
func Select(c Credentials) (Backend, error) {
	if err := c.checkP12(); err != nil {
		return nil, err
	}
	opts := macos.Options{Identity: c.Identity, Entitlements: c.Entitlements, Profile: c.Profile, AppleID: c.AppleID, Password: c.Password, TeamID: c.TeamID, APIKeyFile: c.APIKeyFile, NotarizeTimeout: c.NotarizeTimeout}
	if c.namesCertificate() {
		key, cert, chain, err := c.certificate()
		if err != nil {
			return nil, err
		}
		if opts.P12, opts.P12Password, err = p12Bundle(key, cert, chain); err != nil {
			return nil, err
		}
	}
	return macos.New(opts), nil
}
