//go:build !cgo || (!linux && !windows) || (!amd64 && !arm64)

package rcodesign

import "context"

func Available() error { return ErrUnavailable }
func invoke(ctx context.Context, operation, path string, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrUnavailable
}
