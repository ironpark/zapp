// Package macpkg builds unsigned macOS flat installer packages without Apple
// tools or CGO. Creation is supported on macOS and Linux; installation is macOS-only.
package macpkg

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PayloadMode selects the legacy or macOS 12 segmented CPIO representation.
type PayloadMode uint8

const (
	Auto PayloadMode = iota // Select Large when a file is at least 8 GiB.
	Legacy
	Large // Requires macOS 12.0 or later.
)

// Ownership controls numeric owners recorded in the payload and BOM.
type Ownership uint8

const (
	RootWheel Ownership = iota
	PreserveOwnership
)

// ComponentConfig describes one installable payload and its optional scripts.
type ComponentConfig struct {
	Root string // Contents become relative to InstallLocation; empty for scripts-only.
	// RootEntry optionally includes just this immediate child of Root. It is useful
	// for packaging one app without staging or including its siblings.
	RootEntry       string
	OutputPath      string
	Identifier      string
	Version         string
	InstallLocation string // Defaults to /.
	ScriptsDir      string
	MinOSVersion    string
	PayloadMode     PayloadMode
	Ownership       Ownership
}

// ProductConfig combines components and installer UI resources into a product.
type ProductConfig struct {
	OutputPath      string
	Packages        []string // Flat component package paths, with unique base names and IDs.
	ResourcesDir    string
	Distribution    *Distribution
	DistributionXML []byte // Mutually exclusive with Distribution.
}

// Distribution describes the supported Installer UI and requirements.
type Distribution struct {
	Title        string
	Organization string
	MinOSVersion string
	LicenseFile  string   // Relative to Resources or its locale .lproj directories.
	Choices      []Choice // Empty selects all components without customization.
}

// Choice groups components into one selectable item. Selected must be set
// explicitly for initially selected items when providing a custom choice list.
type Choice struct {
	ID          string
	Title       string
	Description string
	Visible     bool
	Selected    bool
	PackageIDs  []string
}

// baseMinOS is the floor for any flat package; largeMinOS is the floor once the
// macOS 12 segmented payload format is used.
const (
	baseMinOS  = "10.9"
	largeMinOS = "12.0"
)

// payloadFormat maps the payload representation to its XAR member name and the
// macOS version that can read it, so the rule lives in exactly one place.
func payloadFormat(large bool) (member, minOS string) {
	if large {
		return "LargeSegmentedPayload", largeMinOS
	}
	return "Payload", baseMinOS
}

// resolveMinOS defaults an unset minimum to floor and rejects anything below it.
func resolveMinOS(requested, floor string) (string, error) {
	if requested == "" {
		return floor, nil
	}
	cmp, err := compareVersion(requested, floor)
	if err != nil {
		return "", err
	}
	if cmp < 0 {
		return "", fmt.Errorf("requires macOS %s or later", floor)
	}
	return requested, nil
}

func versionParts(v string) ([]uint64, error) {
	parts := strings.Split(v, ".")
	if len(parts) > 3 {
		return nil, fmt.Errorf("invalid OS version %q", v)
	}
	out := make([]uint64, 3)
	for i, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return nil, fmt.Errorf("invalid OS version %q", v)
		}
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

func compareVersion(a, b string) (int, error) {
	x, err := versionParts(a)
	if err != nil {
		return 0, err
	}
	y, err := versionParts(b)
	if err != nil {
		return 0, err
	}
	for i := range x {
		if x[i] < y[i] {
			return -1, nil
		}
		if x[i] > y[i] {
			return 1, nil
		}
	}
	return 0, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func atomicBuild(ctx context.Context, output string, build func(*os.File, string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if output == "" {
		return fmt.Errorf("output path is required")
	}
	dir := filepath.Dir(output)
	work, err := os.MkdirTemp("", "zapp-macpkg-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	f, err := os.CreateTemp(dir, ".macpkg-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = build(f, work); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = f.Chmod(0644); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), output)
}
