//go:build !darwin

package gui

import "context"

func nativeAppIcon(context.Context, string) []byte { return nil }
