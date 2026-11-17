package macos

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ironpark/zapp/internal/macexec"
)

// The API key is read in the JSON rcodesign writes, and a file missing a part
// says how to make a complete one.
func TestReadAPIKey(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	key, err := readAPIKey(write("key.json", `{"issuer_id":"issuer","key_id":"ABC123","private_key":"-----BEGIN PRIVATE KEY-----\nx\n-----END PRIVATE KEY-----\n"}`))
	if err != nil {
		t.Fatal(err)
	}
	if key.IssuerID != "issuer" || key.KeyID != "ABC123" || !strings.HasPrefix(key.PrivateKey, "-----BEGIN PRIVATE KEY-----") {
		t.Fatalf("key = %+v", key)
	}
	if _, err := readAPIKey(write("partial.json", `{"issuer_id":"issuer"}`)); err == nil || !strings.Contains(err.Error(), "encode-app-store-connect-api-key") {
		t.Fatalf("partial key: %v", err)
	}
	if _, err := readAPIKey(write("bad.json", `AuthKey`)); err == nil {
		t.Fatal("accepted a non-JSON key")
	}
}

// notarytool gets the key as the PEM .p8 whichever way the JSON holds it:
// base64 DER as rcodesign writes it, or PEM as earlier actions did.
func TestAPIKeyP8(t *testing.T) {
	der := []byte{0x30, 0x03, 0x02, 0x01, 0x01}
	want := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	for _, private := range []string{base64.StdEncoding.EncodeToString(der), "MAMC\nAQE=", want} {
		got, err := apiKey{PrivateKey: private}.p8()
		if err != nil || string(got) != want {
			t.Fatalf("p8(%q) = %q, %v", private, got, err)
		}
	}
	if _, err := (apiKey{PrivateKey: "not a key!"}).p8(); err == nil {
		t.Fatal("accepted a key neither PEM nor base64")
	}
}

func TestTimeoutArgs(t *testing.T) {
	for timeout, want := range map[time.Duration]string{0: "", time.Hour: "--timeout 3600s", 1500 * time.Millisecond: "--timeout 2s"} {
		if got := strings.Join(timeoutArgs(timeout), " "); got != want {
			t.Errorf("timeoutArgs(%s) = %q, want %q", timeout, got, want)
		}
	}
}

// fakeTools replaces the command runner for a test, answering each call with
// respond and recording its command line.
func fakeTools(t *testing.T, respond func(args []string) (string, error)) *[][]string {
	t.Helper()
	var calls [][]string
	saved := runTool
	t.Cleanup(func() { runTool = saved })
	runTool = func(ctx context.Context, name string, args ...string) (string, error) {
		if name != "xcrun" {
			t.Fatalf("ran %s, want xcrun", name)
		}
		calls = append(calls, slices.Clone(args))
		if err := ctx.Err(); err != nil {
			return "", &macexec.Error{Name: name, Args: args, Err: err}
		}
		return respond(args)
	}
	return &calls
}

func writeAPIKey(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "key.json")
	key := `{"issuer_id":"ISSUER","key_id":"KEY123","private_key":"MAMCAQE="}`
	if err := os.WriteFile(path, []byte(key), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// argAfter returns the value args gives flag, or "".
func argAfter(args []string, flag string) string {
	if i := slices.Index(args, flag); i >= 0 && i+1 < len(args) {
		return args[i+1]
	}
	return ""
}

// Every way of authenticating is passed to notarytool as flags: nothing is
// stored as a keychain profile, so nothing is left behind in the keychain.
func TestSubmitPassesCredentialsWithoutStoringThem(t *testing.T) {
	for name, tc := range map[string]struct {
		opts Options
		want []string
	}{
		"profile": {Options{Profile: "release", AppleID: "a@example.com", Password: "pw", TeamID: "T"},
			[]string{"--keychain-profile", "release"}},
		"apple id": {Options{AppleID: "a@example.com", Password: "pw", TeamID: "TEAM"},
			[]string{"--apple-id", "a@example.com", "--password", "pw", "--team-id", "TEAM"}},
		"api key": {Options{APIKeyFile: "key.json", AppleID: "a@example.com", Password: "pw", TeamID: "T"},
			[]string{"--key", "*", "--key-id", "KEY123", "--issuer", "ISSUER"}},
	} {
		t.Run(name, func(t *testing.T) {
			if tc.opts.APIKeyFile != "" {
				tc.opts.APIKeyFile = writeAPIKey(t)
			}
			var p8 string
			calls := fakeTools(t, func(args []string) (string, error) {
				if p8 = argAfter(args, "--key"); p8 != "" {
					data, err := os.ReadFile(p8)
					if err != nil || !strings.HasPrefix(string(data), "-----BEGIN PRIVATE KEY-----") {
						t.Errorf("notarytool's .p8 = %q, %v", data, err)
					}
				}
				return `{"id":"S1","status":"Accepted"}`, nil
			})
			if err := New(tc.opts).Submit(t.Context(), "Demo.zip"); err != nil {
				t.Fatal(err)
			}
			if len(*calls) != 1 {
				t.Fatalf("calls = %q, want one submit", *calls)
			}
			want := slices.Concat([]string{"notarytool", "submit", "Demo.zip"}, tc.want,
				[]string{"--wait", "--output-format", "json"})
			if i := slices.Index(want, "*"); i >= 0 {
				want[i] = p8
			}
			if got := (*calls)[0]; !slices.Equal(got, want) {
				t.Fatalf("submit = %q\nwant     %q", got, want)
			}
			if p8 != "" {
				if _, err := os.Stat(filepath.Dir(p8)); !os.IsNotExist(err) {
					t.Fatalf("the .p8 outlived the submission: %v", err)
				}
			}
		})
	}
}

// A submission still in progress is waited out, and a rejected one's log read,
// with the same credentials as the submission.
func TestSubmitWaitsAndReadsLogWithSameCredentials(t *testing.T) {
	calls := fakeTools(t, func(args []string) (string, error) {
		switch args[1] {
		case "submit":
			return `{"id":"S1","status":"In Progress"}`, nil
		case "wait":
			return `{"id":"S1","status":"Invalid","message":"rejected"}`, nil
		default:
			return `{}`, nil
		}
	})
	err := New(Options{AppleID: "a@example.com", Password: "pw", TeamID: "TEAM"}).Submit(t.Context(), "Demo.zip")
	if err == nil || !strings.Contains(err.Error(), "S1 ended Invalid") {
		t.Fatalf("err = %v", err)
	}
	var verbs []string
	for _, call := range *calls {
		verbs = append(verbs, call[1])
		if argAfter(call, "--apple-id") != "a@example.com" || argAfter(call, "--team-id") != "TEAM" {
			t.Errorf("%s lacks the Apple ID: %q", call[1], call)
		}
	}
	if !slices.Equal(verbs, []string{"submit", "wait", "log"}) {
		t.Fatalf("notarytool calls = %q", verbs)
	}
}

// A failed or cancelled submission still removes the key's .p8, and its error
// never shows the app-specific password.
func TestSubmitFailureCleansUpAndHidesPassword(t *testing.T) {
	var p8 string
	fakeTools(t, func(args []string) (string, error) {
		p8 = argAfter(args, "--key")
		return "", &macexec.Error{Name: "xcrun", Args: args, Stderr: "HTTP 401", Err: errors.New("exit status 1")}
	})
	if err := New(Options{APIKeyFile: writeAPIKey(t)}).Submit(t.Context(), "Demo.zip"); err == nil {
		t.Fatal("a failed submission succeeded")
	}
	if _, err := os.Stat(filepath.Dir(p8)); p8 == "" || !os.IsNotExist(err) {
		t.Fatalf("the .p8 %q outlived a failed submission: %v", p8, err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := New(Options{AppleID: "a@example.com", Password: "secret-pw", TeamID: "TEAM"}).Submit(ctx, "Demo.zip")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want cancellation", err)
	}
	if strings.Contains(err.Error(), "secret-pw") || !strings.Contains(err.Error(), "--password <redacted>") {
		t.Fatalf("error shows the password: %v", err)
	}
}

func TestSubmitNeedsCredentials(t *testing.T) {
	calls := fakeTools(t, func([]string) (string, error) { return "", nil })
	if err := New(Options{AppleID: "a@example.com"}).Submit(t.Context(), "Demo.zip"); err == nil {
		t.Fatal("submitted without credentials")
	}
	if len(*calls) != 0 {
		t.Fatalf("ran %q", *calls)
	}
}
