// Package macos signs and notarizes with Apple's own tools: codesign for apps
// and disk images, productsign for installer packages, notarytool and stapler
// for notarization, and security to pick the certificate out of the keychain.
//
// It only works on macOS, and only where those tools are installed.
package macos

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Options are the credentials Apple's tools take. The certificate comes from
// the keychain, so it is named rather than supplied.
type Options struct {
	// Identity names a keychain identity. Empty means the best matching
	// Developer ID is chosen for the artifact being signed.
	Identity string

	// Profile is a stored notarytool keychain profile. When it is empty the
	// Apple ID trio below is used to create a temporary one.
	Profile  string
	AppleID  string
	Password string
	TeamID   string
}

// Backend signs with Apple's tools.
type Backend struct {
	opts Options
}

// New returns a backend that signs with the given credentials.
func New(opts Options) *Backend { return &Backend{opts: opts} }

func (b *Backend) Name() string { return "Apple codesign" }

// wantedIdentity is the identity description to look for when signing path. An
// installer is signed by a different kind of certificate than an app.
func (b *Backend) wantedIdentity(path string) string {
	if b.opts.Identity != "" {
		return b.opts.Identity
	}
	if strings.EqualFold(filepath.Ext(path), ".pkg") {
		return "Developer ID Installer"
	}
	return "Developer ID Application"
}

// Describe reports the keychain identity that would be used, masked so it is
// safe to log.
func (b *Backend) Describe(ctx context.Context, path string) (string, error) {
	identity, err := b.identity(ctx, path)
	if err != nil {
		return "", err
	}
	return identity.SecureString(), nil
}

// identity finds the keychain identity to sign path with.
func (b *Backend) identity(ctx context.Context, path string) (Identity, error) {
	want := b.wantedIdentity(path)

	identities, err := listIdentities(ctx, "")
	if err != nil {
		return Identity{}, err
	}
	if len(identities) == 0 {
		return Identity{}, fmt.Errorf("the keychain holds no signing identities")
	}
	for _, identity := range identities {
		if strings.Contains(identity.String(), want) {
			return identity, nil
		}
	}
	return Identity{}, fmt.Errorf("the keychain holds no identity matching %q", want)
}

// Sign signs with the tool that suits the artifact: an installer package is
// signed by productsign, everything else by codesign.
func (b *Backend) Sign(ctx context.Context, path string) error {
	identity, err := b.identity(ctx, path)
	if err != nil {
		return err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pkg":
		return runProductsign(ctx, path, identity.String())
	case ".app", ".dmg":
		return runCodesign(ctx, identity.Fingerprint, path)
	default:
		return fmt.Errorf("%s is not a kind of artifact zapp signs; expected .app, .dmg or .pkg", path)
	}
}

// Submit uploads through notarytool, storing a temporary keychain profile first
// when the caller gave an Apple ID rather than a profile name.
func (b *Backend) Submit(ctx context.Context, path string) error {
	profile := b.opts.Profile
	if profile == "" {
		if b.opts.AppleID == "" || b.opts.Password == "" || b.opts.TeamID == "" {
			return fmt.Errorf("notarizing with Apple's tools needs either a keychain profile " +
				"or all of the Apple ID, password and team ID")
		}
		profile = "temp_profile"
		if err := notaryStoreCredentials(ctx, b.opts.AppleID, b.opts.Password, b.opts.TeamID, profile); err != nil {
			return fmt.Errorf("failed to store credentials: %w", err)
		}
	}

	result, err := notarySubmit(ctx, path, profile)
	if err != nil {
		return err
	}
	if result.Status == "In Progress" {
		if result, err = notaryWait(ctx, result.ID, profile); err != nil {
			return err
		}
	}
	if result.Status != "Accepted" {
		return fmt.Errorf("notarization failed: %s", result.Message)
	}
	return nil
}

func (b *Backend) Staple(ctx context.Context, path string) error {
	if err := notaryStaple(ctx, path); err != nil {
		return fmt.Errorf("failed to staple: %w", err)
	}
	stapled, err := notaryIsStapled(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to check stapling: %w", err)
	}
	if !stapled {
		return fmt.Errorf("%s is not stapled after notarization", path)
	}
	return nil
}
