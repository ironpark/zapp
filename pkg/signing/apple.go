package signing

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/pkg/mactools/codesign"
	"github.com/ironpark/zapp/pkg/mactools/notarytool"
	"github.com/ironpark/zapp/pkg/mactools/productsign"
	"github.com/ironpark/zapp/pkg/mactools/security"
)

// appleBackend signs with Apple's own tools, taking the certificate from the
// keychain. It only works on macOS.
type appleBackend struct{}

func (appleBackend) Name() string { return "Apple codesign" }

// Sign picks an identity from the keychain and signs with the tool that suits
// the artifact: an installer package is signed by productsign, everything else
// by codesign.
func (appleBackend) Sign(ctx context.Context, path string, creds Credentials) error {
	ext := strings.ToLower(filepath.Ext(path))

	// An installer is signed by a different kind of certificate than an app.
	wanted := "Developer ID Application"
	if ext == ".pkg" {
		wanted = "Developer ID Installer"
	}
	if creds.Identity != "" {
		wanted = creds.Identity
	}

	idt, err := findIdentity(ctx, wanted)
	if err != nil {
		return err
	}

	switch ext {
	case ".pkg":
		return productsign.Sign(ctx, path, idt.String())
	case ".app", ".dmg":
		return codesign.CodeSign(ctx, idt.Fingerprint, path)
	default:
		return fmt.Errorf("%s is not a kind of artifact zapp signs; expected .app, .dmg or .pkg", path)
	}
}

// Identity reports which keychain identity would be used, so a caller can show
// it before signing. It is specific to this backend.
func (appleBackend) Identity(ctx context.Context, path string, creds Credentials) (security.Identity, error) {
	wanted := "Developer ID Application"
	if strings.ToLower(filepath.Ext(path)) == ".pkg" {
		wanted = "Developer ID Installer"
	}
	if creds.Identity != "" {
		wanted = creds.Identity
	}
	return findIdentity(ctx, wanted)
}

// findIdentity returns the first keychain identity whose description contains
// want.
func findIdentity(ctx context.Context, want string) (security.Identity, error) {
	identities, err := security.FindIdentity(ctx, "")
	if err != nil {
		return security.Identity{}, err
	}
	if len(identities) == 0 {
		return security.Identity{}, fmt.Errorf("the keychain holds no signing identities")
	}
	for _, identity := range identities {
		if strings.Contains(identity.String(), want) {
			return identity, nil
		}
	}
	return security.Identity{}, fmt.Errorf("the keychain holds no identity matching %q", want)
}

// Submit uploads through notarytool, storing a temporary keychain profile first
// when the caller gave an Apple ID rather than a profile name.
func (appleBackend) Submit(ctx context.Context, path string, creds Credentials) error {
	profile := creds.Profile
	if profile == "" {
		if creds.AppleID == "" || creds.Password == "" || creds.TeamID == "" {
			return fmt.Errorf("notarizing with Apple's tools needs either a keychain profile " +
				"or all of the Apple ID, password and team ID")
		}
		profile = "temp_profile"
		if err := notarytool.StoreCredentials(ctx, creds.AppleID, creds.Password, creds.TeamID, profile); err != nil {
			return fmt.Errorf("failed to store credentials: %w", err)
		}
	}

	result, err := notarytool.Submit(ctx, path, profile)
	if err != nil {
		return err
	}
	if result.Status == "In Progress" {
		if result, err = notarytool.WaitForCompletion(ctx, result.ID, profile); err != nil {
			return err
		}
	}
	if result.Status != "Accepted" {
		return fmt.Errorf("notarization failed: %s", result.Message)
	}
	return nil
}

func (appleBackend) Staple(ctx context.Context, path string) error {
	if err := notarytool.Staple(ctx, path); err != nil {
		return fmt.Errorf("failed to staple: %w", err)
	}
	stapled, err := notarytool.IsStapled(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to check stapling: %w", err)
	}
	if !stapled {
		return fmt.Errorf("%s is not stapled after notarization", path)
	}
	return nil
}
