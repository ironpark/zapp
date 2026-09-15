package signing

import (
	"context"

	"github.com/ironpark/zapp/pkg/mactools/rcodesign"
)

// rcodesignBackend signs with rcodesign, which needs neither Apple's tools nor
// a keychain and so is what runs away from macOS.
type rcodesignBackend struct{}

func (rcodesignBackend) Name() string { return "rcodesign" }

// Sign handles every artifact kind through the one command: rcodesign understands
// Mach-O binaries, bundles, disk images and installer packages alike.
func (rcodesignBackend) Sign(ctx context.Context, path string, creds Credentials) error {
	return rcodesign.Sign(ctx, path, creds.Certificate, rcodesign.SignOptions{
		Runtime: true,
	})
}

// Submit uploads and waits. Stapling is left to Staple so the ticket lands on
// the bundle rather than on the archive that was submitted.
func (rcodesignBackend) Submit(ctx context.Context, path string, creds Credentials) error {
	return rcodesign.Notarize(ctx, path, rcodesign.NotarizeOptions{APIKeyFile: creds.APIKeyFile})
}

func (rcodesignBackend) Staple(ctx context.Context, path string) error {
	return rcodesign.Staple(ctx, path)
}
