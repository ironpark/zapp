package macos

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ironpark/zapp/internal/macexec"
)

// errCodesignFailed is returned when the codesign command fails.
var errCodesignFailed = errors.New("codesign command failed")

// runCodesign signs the file with Apple's codesign.
func runCodesign(ctx context.Context, identityName, filePath string) error {
	if identityName == "" || filePath == "" {
		return errors.New("identity name and file path are required")
	}

	if _, err := macexec.Run(ctx, "codesign", codesignArgs(identityName, filePath)...); err != nil {
		return fmt.Errorf("%w: %v%s", errCodesignFailed, err, hint(macexec.Output(err)))
	}

	return nil
}

// hint expands codesign error codes that give no indication of their cause.
func hint(output string) string {
	if strings.Contains(output, "errSecInternalComponent") {
		return "\nhint: codesign could not reach the signing key. This usually means the keychain is locked" +
			" or unreachable from a non-GUI session (SSH, CI). Try:\n" +
			"  security unlock-keychain ~/Library/Keychains/login.keychain-db\n" +
			"  security set-key-partition-list -S apple-tool:,apple:,codesign: -s ~/Library/Keychains/login.keychain-db"
	}
	return ""
}

// codesignArgs renders the codesign invocation. The flags are the ones every
// artifact zapp signs wants: replace any existing signature, sign nested code,
// and opt into the hardened runtime that notarization requires.
func codesignArgs(identityName, filePath string) []string {
	return []string{
		"--sign", identityName,
		"--force",
		"--deep",
		"--options=runtime",
		filePath,
	}
}
