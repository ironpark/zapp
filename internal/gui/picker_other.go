//go:build !darwin && !linux && !windows

package gui

import (
	"context"
	"fmt"
	"runtime"
)

func choosePath(context.Context, pickMode, string, string) (string, error) {
	return "", fmt.Errorf("file picker is unavailable on %s; enter a path directly", runtime.GOOS)
}
