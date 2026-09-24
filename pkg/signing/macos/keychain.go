package macos

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ironpark/zapp/internal/macexec"
)

// tempKeychain holds an imported PKCS#12 certificate for one signing run. It
// follows the steps CI uses to import a certificate secret
// (apple-actions/import-codesign-certs): a keychain of its own, unlocked, with
// Apple's signing tools allowed to use the private key without a prompt, and
// added to the user's search list so the tools can build the certificate chain
// from what the bundle carries. close undoes all of it.
type tempKeychain struct {
	dir, path string
	listed    bool // whether path was added to the user's search list
}

// importP12 creates a temporary keychain holding the certificate and key in
// p12. On failure nothing is left behind.
func importP12(ctx context.Context, p12 []byte, password string) (*tempKeychain, error) {
	dir, err := os.MkdirTemp("", "zapp-keychain-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	k := &tempKeychain{dir: dir, path: filepath.Join(dir, "signing.keychain-db")}
	if err := k.populate(ctx, p12, password); err != nil {
		// The cleanup reads k itself, not a return value: a `return nil, err`
		// would clear a named result before a deferred cleanup could use it.
		return nil, errors.Join(err, k.close(context.WithoutCancel(ctx)))
	}
	return k, nil
}

// populate creates, unlocks and fills the keychain, then lists it.
func (k *tempKeychain) populate(ctx context.Context, p12 []byte, password string) error {
	// The keychain's own password only ever unlocks this throwaway keychain.
	secret := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	keychainPassword := hex.EncodeToString(secret)

	// security imports from a path. The file sits in the private temp
	// directory and goes as soon as the import is done.
	p12Path := filepath.Join(k.dir, "certificate.p12")
	if err := os.WriteFile(p12Path, p12, 0o600); err != nil {
		return err
	}
	defer func() { _ = os.Remove(p12Path) }()

	steps := [][]string{
		{"create-keychain", "-p", keychainPassword, k.path},
		// Keep it unlocked for the whole run rather than the default five minutes.
		{"set-keychain-settings", "-lut", "21600", k.path},
		{"unlock-keychain", "-p", keychainPassword, k.path},
		{"import", p12Path, "-k", k.path, "-f", "pkcs12", "-P", password,
			"-T", "/usr/bin/codesign", "-T", "/usr/bin/productsign", "-T", "/usr/bin/security"},
		// Without this, codesign fails with errSecInternalComponent in a non-GUI session.
		{"set-key-partition-list", "-S", "apple-tool:,apple:,codesign:", "-s", "-k", keychainPassword, k.path},
	}
	for _, args := range steps {
		if _, err := security(ctx, args...); err != nil {
			if args[0] == "import" {
				return fmt.Errorf("could not import the PKCS#12 certificate (check its password): %w", err)
			}
			return err
		}
	}

	list, err := searchList(ctx)
	if err != nil {
		return err
	}
	if _, err := security(ctx, append([]string{"list-keychains", "-d", "user", "-s", k.path}, list...)...); err != nil {
		return err
	}
	k.listed = true
	return nil
}

// close takes the keychain off the user's search list and deletes it. It
// removes only its own entry from the list as it stands now, so a change the
// user made to the list meanwhile survives.
func (k *tempKeychain) close(ctx context.Context) error {
	var errs []error
	if k.listed {
		list, err := searchList(ctx)
		if err == nil {
			list = slices.DeleteFunc(list, func(p string) bool { return samePath(p, k.path) })
			_, err = security(ctx, append([]string{"list-keychains", "-d", "user", "-s"}, list...)...)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("could not remove the temporary keychain %s from the search list: %w", k.path, err))
		} else {
			k.listed = false
		}
	}
	if _, err := os.Stat(k.path); err == nil {
		if _, err := security(ctx, "delete-keychain", k.path); err != nil {
			errs = append(errs, err)
		}
	}
	if err := os.RemoveAll(k.dir); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// searchList returns the user's keychain search list.
func searchList(ctx context.Context) ([]string, error) {
	out, err := security(ctx, "list-keychains", "-d", "user")
	if err != nil {
		return nil, err
	}
	return parseKeychainList(out), nil
}

// parseKeychainList reads the output of `security list-keychains`: one
// quoted path per line.
func parseKeychainList(out string) []string {
	var list []string
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		if p := strings.Trim(strings.TrimSpace(scanner.Text()), `"`); p != "" {
			list = append(list, p)
		}
	}
	return list
}

// samePath compares keychain paths as security reports them, which may
// resolve symlinks such as /var → /private/var.
func samePath(a, b string) bool {
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// security runs the security tool. Its arguments carry passwords, so a
// failure reports the subcommand and the tool's own output, never the
// command line.
func security(ctx context.Context, args ...string) (string, error) {
	out, err := macexec.Run(ctx, "security", args...)
	if err != nil {
		if msg := macexec.Output(err); msg != "" {
			return out, fmt.Errorf("security %s failed: %s", args[0], msg)
		}
		return out, fmt.Errorf("security %s failed: %w", args[0], errors.Unwrap(err))
	}
	return out, nil
}
