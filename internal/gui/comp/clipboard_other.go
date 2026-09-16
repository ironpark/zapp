//go:build !darwin

package comp

import "github.com/atotto/clipboard"

func readClipboard() (string, error) { return clipboard.ReadAll() }
func writeClipboard(s string) error  { return clipboard.WriteAll(s) }
