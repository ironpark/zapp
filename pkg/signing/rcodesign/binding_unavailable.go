//go:build !cgo || (!linux && !windows) || (!amd64 && !arm64)

package rcodesign

import "context"

func Available() error { return ErrUnavailable }

func invoke(context.Context, string, string, Options) error { return ErrUnavailable }
