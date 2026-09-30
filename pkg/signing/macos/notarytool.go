package macos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ironpark/zapp/internal/macexec"
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

// submissionResult represents the result of a notarization submission.
type submissionResult struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Message        string `json:"message"`
	SubmissionTime string `json:"submissionTime"`
}

// notaryStoreCredentials stores the Apple ID credentials for notarization.
func notaryStoreCredentials(ctx context.Context, appleID, password, teamID, profileName string) error {
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

// apiKey is an App Store Connect API key as rcodesign's
// encode-app-store-connect-api-key writes it.
type apiKey struct {
	IssuerID   string `json:"issuer_id"`
	KeyID      string `json:"key_id"`
	PrivateKey string `json:"private_key"`
}

func readAPIKey(path string) (apiKey, error) {
	var key apiKey
	data, err := os.ReadFile(path)
	if err != nil {
		return key, err
	}
	if err := json.Unmarshal(data, &key); err != nil {
		return key, fmt.Errorf("%s is not an App Store Connect API key JSON: %w", path, err)
	}
	if key.IssuerID == "" || key.KeyID == "" || key.PrivateKey == "" {
		return key, errors.New(path + " needs issuer_id, key_id and private_key; " +
			"create it with `rcodesign encode-app-store-connect-api-key`")
	}
	return key, nil
}

// p8 is the private key as the .p8 file App Store Connect hands out, which
// notarytool reads. rcodesign's JSON holds it as base64 DER; a PEM key, as
// earlier versions of the GitHub Action wrote, is taken as it is.
func (k apiKey) p8() ([]byte, error) {
	if strings.Contains(k.PrivateKey, "-----BEGIN") {
		return []byte(k.PrivateKey), nil
	}
	der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(k.PrivateKey), ""))
	if err != nil {
		return nil, fmt.Errorf("private_key is neither PEM nor base64 DER: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// notaryStoreAPIKey stores an App Store Connect API key for notarization.
// notarytool reads the private key from a .p8 file, which exists only while
// it copies the key into the keychain profile.
func notaryStoreAPIKey(ctx context.Context, path, profileName string) error {
	key, err := readAPIKey(path)
	if err != nil {
		return err
	}
	data, err := key.p8()
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	dir, err := os.MkdirTemp("", "zapp-notary-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	p8 := filepath.Join(dir, "AuthKey_"+key.KeyID+".p8")
	if err := os.WriteFile(p8, data, 0o600); err != nil {
		return err
	}
	_, err = xcrun(ctx,
		"notarytool", "store-credentials", profileName,
		"--key", p8,
		"--key-id", key.KeyID,
		"--issuer", key.IssuerID,
	)
	if err != nil {
		return fmt.Errorf("storing credentials failed: %w", err)
	}
	return nil
}

// notarySubmit submits a file for notarization.
// A positive timeout bounds the wait.
func notarySubmit(ctx context.Context, filePath, keychainProfile string, timeout time.Duration) (*submissionResult, error) {
	var result submissionResult
	err := xcrunJSON(ctx, &result, append([]string{
		"notarytool", "submit", filePath,
		"--keychain-profile", keychainProfile,
		"--wait",
	}, timeoutArgs(timeout)...)...)
	if err != nil {
		return nil, fmt.Errorf("notarization submission failed: %w", err)
	}
	return &result, nil
}

// notaryWait waits for the notarization process to complete.
// A positive timeout bounds the wait.
func notaryWait(ctx context.Context, submissionID, keychainProfile string, timeout time.Duration) (*submissionResult, error) {
	var result submissionResult
	err := xcrunJSON(ctx, &result, append([]string{
		"notarytool", "wait", submissionID,
		"--keychain-profile", keychainProfile,
	}, timeoutArgs(timeout)...)...)
	if err != nil {
		return nil, fmt.Errorf("wait for notarization failed: %w", err)
	}
	return &result, nil
}

// timeoutArgs renders notarytool's --timeout, in whole seconds, rounded up
// so a short timeout does not become none.
func timeoutArgs(timeout time.Duration) []string {
	if timeout <= 0 {
		return nil
	}
	secs := (timeout + time.Second - 1) / time.Second
	return []string{"--timeout", strconv.FormatInt(int64(secs), 10) + "s"}
}

// notaryStaple staples the notarization ticket to the file.
func notaryStaple(ctx context.Context, filePath string) error {
	if _, err := xcrun(ctx, "stapler", "staple", filePath); err != nil {
		return fmt.Errorf("stapling failed: %w", err)
	}
	return nil
}

// notaryIsStapled checks if the file has been stapled.
func notaryIsStapled(ctx context.Context, filePath string) (bool, error) {
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

// notaryLog fetches the log the notary kept for a submission, as JSON.
func notaryLog(ctx context.Context, submissionID, keychainProfile string) (string, error) {
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
