package notarytool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

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
	args := []string{
		"notarytool",
		"store-credentials",
		profileName,
		"--apple-id", appleID,
		"--password", password,
		"--team-id", teamID,
	}

	cmd := exec.CommandContext(ctx, "xcrun", args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("storing credentials failed: %w, stderr: %s", err, errBuf.String())
	}

	return nil
}

// Submit submits a file for notarization.
func Submit(ctx context.Context, filePath, keychainProfile string) (*SubmissionResult, error) {
	args := []string{
		"notarytool", "submit",
		filePath,
		"--keychain-profile", keychainProfile,
		"--wait",
		"--output-format", "json",
	}

	cmd := exec.CommandContext(ctx, "xcrun", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("notarization submission failed: %w, stderr: %s", err, errBuf.String())
	}

	var result SubmissionResult
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse submission result: %w", err)
	}

	return &result, nil
}

// WaitForCompletion waits for the notarization process to complete.
func WaitForCompletion(ctx context.Context, submissionID, keychainProfile string) (*SubmissionResult, error) {
	args := []string{
		"notarytool", "wait",
		submissionID,
		"--keychain-profile", keychainProfile,
		"--output-format", "json",
	}

	cmd := exec.CommandContext(ctx, "xcrun", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("wait for notarization failed: %w, stderr: %s", err, errBuf.String())
	}

	var result SubmissionResult
	if err := json.Unmarshal(outBuf.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse wait result: %w", err)
	}

	return &result, nil
}

// Staple staples the notarization ticket to the file.
func Staple(ctx context.Context, filePath string) error {
	args := []string{
		"stapler", "staple",
		filePath,
	}

	cmd := exec.CommandContext(ctx, "xcrun", args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stapling failed: %w, stderr: %s", err, errBuf.String())
	}

	return nil
}

// IsStapled checks if the file has been stapled.
func IsStapled(ctx context.Context, filePath string) (bool, error) {
	args := []string{
		"stapler", "validate",
		filePath,
	}

	cmd := exec.CommandContext(ctx, "xcrun", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		if strings.Contains(errBuf.String(), "The validate action failed") {
			return false, nil
		}
		return false, fmt.Errorf("validation check failed: %w, stderr: %s", err, errBuf.String())
	}

	return strings.Contains(outBuf.String(), "The validate action worked!"), nil
}

// GetNotarizationLog
func GetNotarizationLog(ctx context.Context, submissionID, keychainProfile string) (string, error) {
	args := []string{
		"notarytool", "log",
		submissionID,
		"--keychain-profile", keychainProfile,
		"--output-format", "json",
	}

	cmd := exec.CommandContext(ctx, "xcrun", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("getting notarization log failed: %w, stderr: %s", err, errBuf.String())
	}

	return outBuf.String(), nil
}
