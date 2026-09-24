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
	"sync"
)

// Options are the credentials Apple's tools take. The certificate comes from
// the keychain: the user's own, or a temporary one holding a supplied PKCS#12.
type Options struct {
	// Identity names a keychain identity. Empty means the best matching
	// Developer ID is chosen for the artifact being signed.
	Identity string

	// P12 is a PKCS#12 certificate and private key to sign with instead of the
	// user's keychains. It is imported into a temporary keychain on first use
	// and removed by Close.
	P12         []byte
	P12Password string

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

	// mu guards resolved, which caches the keychain lookup so that describing
	// and then signing an artifact does not enumerate the keychain twice, and
	// keychain, the temporary keychain holding opts.P12 once it is imported.
	mu       sync.Mutex
	resolved map[string]Identity
	keychain *tempKeychain
}

// New returns a backend that signs with the given credentials.
func New(opts Options) *Backend {
	return &Backend{opts: opts, resolved: map[string]Identity{}}
}

func (b *Backend) Name() string { return "Apple codesign" }

// wantedIdentity is the identity description to look for when signing an
// artifact with the given extension. An installer is signed by a different kind
// of certificate than an app.
func (b *Backend) wantedIdentity(ext string) string {
	if b.opts.Identity != "" {
		return b.opts.Identity
	}
	if ext == ".pkg" {
		return "Developer ID Installer"
	}
	return "Developer ID Application"
}

// extOf normalises an artifact's extension, which is what both the identity
// choice and the choice of signing tool turn on.
func extOf(path string) string { return strings.ToLower(filepath.Ext(path)) }

// Describe reports the keychain identity that would be used, masked so it is
// safe to log.
func (b *Backend) Describe(ctx context.Context, path string) (string, error) {
	identity, err := b.identity(ctx, extOf(path))
	if err != nil {
		return "", err
	}
	return identity.SecureString(), nil
}

// identity finds the keychain identity to sign an artifact of the given
// extension with. The result is cached: a signing run describes the identity
// before using it, and enumerating the keychain means a subprocess each time.
func (b *Backend) identity(ctx context.Context, ext string) (Identity, error) {
	want := b.wantedIdentity(ext)

	b.mu.Lock()
	defer b.mu.Unlock()
	if identity, ok := b.resolved[want]; ok {
		return identity, nil
	}

	keychain, err := b.keychainPath(ctx)
	if err != nil {
		return Identity{}, err
	}
	identities, err := listIdentities(ctx, keychain)
	if err != nil {
		return Identity{}, err
	}
	if len(identities) == 0 {
		if keychain != "" {
			return Identity{}, fmt.Errorf("the PKCS#12 certificate holds no valid signing identity; " +
				"a Developer ID certificate also needs Apple's Developer ID intermediate certificate installed")
		}
		return Identity{}, fmt.Errorf("the keychain holds no signing identities")
	}
	for _, identity := range identities {
		if strings.Contains(identity.String(), want) {
			b.resolved[want] = identity
			return identity, nil
		}
	}
	return Identity{}, fmt.Errorf("the keychain holds no identity matching %q", want)
}

// keychainPath imports opts.P12 the first time it is needed and returns the
// temporary keychain's path, or "" to search the user's keychains. The caller
// holds b.mu.
func (b *Backend) keychainPath(ctx context.Context) (string, error) {
	if len(b.opts.P12) == 0 {
		return "", nil
	}
	if b.keychain == nil {
		k, err := importP12(ctx, b.opts.P12, b.opts.P12Password)
		if err != nil {
			return "", err
		}
		b.keychain = k
	}
	return b.keychain.path, nil
}

// Close deletes the temporary keychain a PKCS#12 certificate was imported
// into and takes it off the user's search list. It runs even when the signing
// context was cancelled, so a cancelled build does not leave it behind.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.keychain == nil {
		return nil
	}
	err := b.keychain.close(context.Background())
	b.keychain = nil
	b.resolved = map[string]Identity{}
	return err
}

// Sign signs with the tool that suits the artifact: an installer package is
// signed by productsign, everything else by codesign.
func (b *Backend) Sign(ctx context.Context, path string) error {
	ext := extOf(path)
	identity, err := b.identity(ctx, ext)
	if err != nil {
		return err
	}
	b.mu.Lock()
	keychain := ""
	if b.keychain != nil {
		keychain = b.keychain.path
	}
	b.mu.Unlock()
	switch ext {
	case ".pkg":
		return runProductsign(ctx, path, identity.String(), keychain)
	case ".app", ".dmg":
		return runCodesign(ctx, identity.Fingerprint, path, keychain)
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
