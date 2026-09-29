//go:build !darwin

package verify

import (
	"context"
	"errors"

	"github.com/ironpark/zapp/pkg/signing/rcodesign"
)

// deepApp checks the code digests and signature of every binary in the
// bundle with the signature library, the part of codesign --verify that
// needs no macOS. Gatekeeper exists only there.
func deepApp(ctx context.Context, r *Report, path string) {
	switch err := rcodesign.Verify(ctx, path); {
	case errors.Is(err, rcodesign.ErrUnavailable):
		r.add("signature integrity", Skip, "this build of zapp has no signature library")
	case err != nil:
		r.add("signature integrity", Fail, "%v", err)
	default:
		r.add("signature integrity", Pass, "every binary's code digests and signature; sealed resources are checked on macOS")
	}
	gatekeeper(r)
}

func deepDMG(_ context.Context, r *Report, _ string) {
	r.add("signature integrity", Skip, "checked on macOS")
	gatekeeper(r)
}

func deepPKG(_ context.Context, r *Report, _ string) {
	r.add("signature integrity", Skip, "checked on macOS")
	gatekeeper(r)
}

func gatekeeper(r *Report) {
	r.add("Gatekeeper", Skip, "only macOS has Gatekeeper")
}
