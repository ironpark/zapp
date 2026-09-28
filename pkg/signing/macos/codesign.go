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

// runCodesign signs the files with Apple's codesign.
// A non-empty keychain restricts the identity search to that keychain, and a
// non-empty entitlements file replaces the files' own entitlements.
func runCodesign(ctx context.Context, identityName, keychain, entitlements string, paths ...string) error {
	if identityName == "" || len(paths) == 0 {
		return errors.New("identity name and file path are required")
	}

	if _, err := macexec.Run(ctx, "codesign", codesignArgs(identityName, keychain, entitlements, paths...)...); err != nil {
		return fmt.Errorf("%w: %v%s", errCodesignFailed, err, hint(macexec.Output(err)))
	}

	return nil
}

// signApp signs an app bundle and the code inside it, from the inside out.
// The entitlements file, if any, is the app's own; nested code keeps the
// entitlements it was built with.
func signApp(ctx context.Context, identityName, keychain, entitlements, app string) error {
	groups, err := nestedCode(app)
	if err != nil {
		return fmt.Errorf("could not list the code inside %s: %w", app, err)
	}
	for _, paths := range groups {
		if err := runCodesign(ctx, identityName, keychain, "", paths...); err != nil {
			return err
		}
	}
	return runCodesign(ctx, identityName, keychain, entitlements, app)
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
// artifact zapp signs wants: replace any existing signature, with a secure
// timestamp and the hardened runtime that notarization requires. Re-signing
// would drop the entitlements the code was built with, so they are carried
// over unless an entitlements file replaces them.
func codesignArgs(identityName, keychain, entitlements string, paths ...string) []string {
	args := []string{"--sign", identityName}
	if keychain != "" {
		args = append(args, "--keychain", keychain)
	}
	args = append(args, "--force", "--timestamp", "--options=runtime")
	if entitlements != "" {
		args = append(args, "--entitlements", entitlements)
	} else {
		args = append(args, "--preserve-metadata=entitlements")
	}
	return append(args, paths...)
}
