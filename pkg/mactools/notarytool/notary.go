package notarytool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ironpark/zapp/pkg/mactools/internal/macexec"
)

// xcrun invokes a tool from the active Xcode toolchain.
func xcrun(ctx context.Context, args ...string) (string, error) {
	return macexec.Run(ctx, "xcrun", args...)
}

// xcrunJSON invokes a notarytool subcommand that reports JSON and decodes it.
func xcrunJSON(ctx context.Context, result any, args ...string) error {
	output, err := xcrun(ctx, append(args, "--output-format", "json")...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(output), result); err != nil {
		return fmt.Errorf("failed to parse %s result: %w", args[1], err)
	}
	return nil
}

// SubmissionResult represents the result of a notarization submission.
type SubmissionResult struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	Message         string `json:"message"`
	SubmissionTime  string `json:"submissionTime"`
	keychainProfile string `json:"-"`
}

func (r SubmissionResult) GetLog(ctx context.Context) (string, error) {
	msg, err := GetNotarizationLog(ctx, r.ID, r.keychainProfile)
	if err != nil {
		return "", fmt.Errorf("getting notarization log failed: %w", err)
	}
	return msg, nil
}

// StoreCredentials stores the Apple ID credentials for notarization.
func StoreCredentials(ctx context.Context, appleID, password, teamID, profileName string) error {
	_, err := xcrun(ctx,
		"notarytool", "store-credentials", profileName,
		"--apple-id", appleID,
		"--password", password,
		"--team-id", teamID,
	)
	if err != nil {
		return fmt.Errorf("storing credentials failed: %w", err)
	}
	return nil
}

// Submit submits a file for notarization.
func Submit(ctx context.Context, filePath, keychainProfile string) (*SubmissionResult, error) {
	var result SubmissionResult
	err := xcrunJSON(ctx, &result,
		"notarytool", "submit", filePath,
		"--keychain-profile", keychainProfile,
		"--wait",
	)
	if err != nil {
		return nil, fmt.Errorf("notarization submission failed: %w", err)
	}
	return &result, nil
}

// WaitForCompletion waits for the notarization process to complete.
func WaitForCompletion(ctx context.Context, submissionID, keychainProfile string) (*SubmissionResult, error) {
	var result SubmissionResult
	err := xcrunJSON(ctx, &result,
		"notarytool", "wait", submissionID,
		"--keychain-profile", keychainProfile,
	)
	if err != nil {
		return nil, fmt.Errorf("wait for notarization failed: %w", err)
	}
	return &result, nil
}

// Staple staples the notarization ticket to the file.
func Staple(ctx context.Context, filePath string) error {
	if _, err := xcrun(ctx, "stapler", "staple", filePath); err != nil {
		return fmt.Errorf("stapling failed: %w", err)
	}
	return nil
}

// IsStapled checks if the file has been stapled.
func IsStapled(ctx context.Context, filePath string) (bool, error) {
	output, err := xcrun(ctx, "stapler", "validate", filePath)
	if err != nil {
		// An unstapled file is a normal answer, not a failure.
		if strings.Contains(macexec.Output(err), "The validate action failed") {
			return false, nil
		}
		return false, fmt.Errorf("validation check failed: %w", err)
	}
	return strings.Contains(output, "The validate action worked!"), nil
}

// GetNotarizationLog
func GetNotarizationLog(ctx context.Context, submissionID, keychainProfile string) (string, error) {
	output, err := xcrun(ctx,
		"notarytool", "log", submissionID,
		"--keychain-profile", keychainProfile,
		"--output-format", "json",
	)
	if err != nil {
		return "", fmt.Errorf("getting notarization log failed: %w", err)
	}
	return output, nil
}
