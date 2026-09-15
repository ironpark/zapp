package signing

import (
	"fmt"

	"github.com/ironpark/zapp/pkg/signing/macos"
)

// Select always uses Apple's signing tools on macOS.
func Select(c Credentials) (Backend, error) {
	if c.namesCertificateFile() {
		return nil, fmt.Errorf("macOS uses Apple's signing tools: import the certificate into Keychain and use --identity; notarize with --profile or --apple-id, --password and --team-id")
	}
	return macos.New(macos.Options{Identity: c.Identity, Profile: c.Profile, AppleID: c.AppleID, Password: c.Password, TeamID: c.TeamID}), nil
}
