//go:build !darwin

package gui

import (
	"context"
	"fmt"
)

func chooseNativePath(context.Context, pickMode, string, string) (string, error) {
	return "", fmt.Errorf("native macOS picker unavailable")
}
