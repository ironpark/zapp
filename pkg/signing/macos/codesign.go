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

// Options holds the configuration for the CodeSign function.
type codesignOptions struct {
	IdentityName     string
	FilePath         string
	Entitlements     string
	Force            bool
	Verbose          bool
	DeepSign         bool
	Runtime          bool
	PreserveMetadata []string
	Requirements     string
	Timestamp        string
	KeyChain         string
}

// Option is a function that modifies Options.
type codesignOption func(*codesignOptions)

// runCodesign signs the file with Apple's codesign.
func runCodesign(ctx context.Context, identityName, filePath string) error {
	if identityName == "" || filePath == "" {
		return errors.New("identity name and file path are required")
	}

	options := &codesignOptions{
		IdentityName: identityName,
		FilePath:     filePath,
		Force:        true, // Set force as default
		Runtime:      true, // Set runtime as default
		DeepSign:     true,
	}

	if _, err := macexec.Run(ctx, "codesign", buildCodesignArgs(options)...); err != nil {
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

func buildCodesignArgs(options *codesignOptions) []string {
	args := []string{"--sign", options.IdentityName}

	if options.Entitlements != "" {
		args = append(args, "--entitlements", options.Entitlements)
	}
	if options.Force {
		args = append(args, "--force")
	}
	if options.Verbose {
		args = append(args, "--verbose")
	}
	if options.DeepSign {
		args = append(args, "--deep")
	}
	if options.Runtime {
		args = append(args, "--options=runtime")
	}
	for _, metadata := range options.PreserveMetadata {
		args = append(args, "--preserve-metadata="+metadata)
	}
	if options.Requirements != "" {
		args = append(args, "--requirements", options.Requirements)
	}
	if options.Timestamp != "" {
		args = append(args, "--timestamp", options.Timestamp)
	}
	if options.KeyChain != "" {
		args = append(args, "--keychain", options.KeyChain)
	}
	args = append(args, options.FilePath)
	return args
}
