package codesign

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// ErrCodesignFailed is returned when the codesign command fails.
var ErrCodesignFailed = errors.New("codesign command failed")

// Options holds the configuration for the CodeSign function.
type Options struct {
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
type Option func(*Options)

// WithKeyChain sets the keychain.
func WithKeyChain(keychain string) Option {
	return func(o *Options) {
		o.KeyChain = keychain
	}
}

// WithEntitlements sets the entitlements file path.
func WithEntitlements(path string) Option {
	return func(o *Options) {
		o.Entitlements = path
	}
}

// WithForce sets the force flag.
func WithForce(force bool) Option {
	return func(o *Options) {
		o.Force = force
	}
}

// WithVerbose sets the verbose flag.
func WithVerbose(verbose bool) Option {
	return func(o *Options) {
		o.Verbose = verbose
	}
}

// WithDeepSign sets the deep sign flag.
func WithDeepSign(deep bool) Option {
	return func(o *Options) {
		o.DeepSign = deep
	}
}

// WithPreserveMetadata sets the metadata to preserve.
func WithPreserveMetadata(metadata ...string) Option {
	return func(o *Options) {
		o.PreserveMetadata = metadata
	}
}

// WithRequirements sets the requirements file path.
func WithRequirements(path string) Option {
	return func(o *Options) {
		o.Requirements = path
	}
}

// WithTimestamp sets the timestamp server URL.
func WithTimestamp(url string) Option {
	return func(o *Options) {
		o.Timestamp = url
	}
}

// CodeSign performs code signing on the specified file.
func CodeSign(ctx context.Context, identityName, filePath string, opts ...Option) error {
	if identityName == "" || filePath == "" {
		return errors.New("identity name and file path are required")
	}

	options := &Options{
		IdentityName: identityName,
		FilePath:     filePath,
		Force:        true, // Set force as default
		Runtime:      true, // Set runtime as default
		DeepSign:     true,
	}

	for _, opt := range opts {
		opt(options)
	}

	args := buildArgs(options)
	cmd := exec.CommandContext(ctx, "codesign", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("%w: %v (output: %s)", ErrCodesignFailed, err, output)
	}

	return nil
}

func buildArgs(options *Options) []string {
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
