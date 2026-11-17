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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ironpark/zapp/internal/macexec"
)

// runTool runs a command line tool; tests replace it to see the command lines
// built without running Apple's tools.
var runTool = macexec.Run

// xcrun invokes a tool from the active Xcode toolchain. notarytool can be
// handed an app-specific password, so a failure's command line never shows it.
func xcrun(ctx context.Context, args ...string) (string, error) {
	output, err := runTool(ctx, "xcrun", args...)
	if cmdErr, ok := errors.AsType[*macexec.Error](err); ok {
		cmdErr.Args = redacted(cmdErr.Args)
	}
	return output, err
}

// redacted copies args with the value of every --password hidden.
func redacted(args []string) []string {
	args = slices.Clone(args)
	for i := 1; i < len(args); i++ {
		if args[i-1] == "--password" {
			args[i] = "<redacted>"
		}
	}
	return args
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

// notaryAuth returns the arguments that authenticate every notarytool call
// with opts, and a cleanup that removes whatever they need on disk. Nothing is
// stored in a keychain: a profile the caller stored is named, and an App Store
// Connect API key or the Apple ID trio is passed to each call, the key as a
// .p8 file that lasts until cleanup. cleanup is never nil.
func notaryAuth(opts Options) (args []string, cleanup func(), err error) {
	cleanup = func() {}
	switch {
	case opts.Profile != "":
		return []string{"--keychain-profile", opts.Profile}, cleanup, nil
	case opts.APIKeyFile != "":
		return notaryAPIKey(opts.APIKeyFile)
	case opts.AppleID != "" && opts.Password != "" && opts.TeamID != "":
		return []string{"--apple-id", opts.AppleID, "--password", opts.Password, "--team-id", opts.TeamID}, cleanup, nil
	default:
		return nil, cleanup, errors.New("notarizing with Apple's tools needs a keychain profile, " +
			"an App Store Connect API key, or all of the Apple ID, password and team ID")
	}
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

// notaryAPIKey authenticates with an App Store Connect API key. notarytool
// reads the private key from a .p8 file, which is private to the user and
// removed by cleanup.
func notaryAPIKey(path string) (args []string, cleanup func(), err error) {
	cleanup = func() {}
	key, err := readAPIKey(path)
	if err != nil {
		return nil, cleanup, err
	}
	data, err := key.p8()
	if err != nil {
		return nil, cleanup, fmt.Errorf("%s: %w", path, err)
	}
	dir, err := os.MkdirTemp("", "zapp-notary-")
	if err != nil {
		return nil, cleanup, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	p8 := filepath.Join(dir, "AuthKey_"+key.KeyID+".p8")
	if err := os.WriteFile(p8, data, 0o600); err != nil {
		cleanup()
		return nil, func() {}, err
	}
	return []string{"--key", p8, "--key-id", key.KeyID, "--issuer", key.IssuerID}, cleanup, nil
}

// notarySubmit submits a file for notarization.
// A positive timeout bounds the wait.
func notarySubmit(ctx context.Context, filePath string, auth []string, timeout time.Duration) (*submissionResult, error) {
	var result submissionResult
	err := xcrunJSON(ctx, &result, slices.Concat(
		[]string{"notarytool", "submit", filePath}, auth, []string{"--wait"}, timeoutArgs(timeout))...)
	if err != nil {
		return nil, fmt.Errorf("notarization submission failed: %w", err)
	}
	return &result, nil
}

// notaryWait waits for the notarization process to complete.
// A positive timeout bounds the wait.
func notaryWait(ctx context.Context, submissionID string, auth []string, timeout time.Duration) (*submissionResult, error) {
	var result submissionResult
	err := xcrunJSON(ctx, &result, slices.Concat(
		[]string{"notarytool", "wait", submissionID}, auth, timeoutArgs(timeout))...)
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
func notaryLog(ctx context.Context, submissionID string, auth []string) (string, error) {
	output, err := xcrun(ctx, slices.Concat(
		[]string{"notarytool", "log", submissionID}, auth, []string{"--output-format", "json"})...)
	if err != nil {
		return "", fmt.Errorf("getting notarization log failed: %w", err)
	}
	return output, nil
}
