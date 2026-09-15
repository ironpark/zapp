package macpkg

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/pkg/plist"
)

type payloadInfo struct {
	Large  bool  `xml:"large-segmented,attr,omitempty"`
	Files  int   `xml:"numberOfFiles,attr"`
	KBytes int64 `xml:"installKBytes,attr"`
}
type bundleInfo struct {
	Path         string `xml:"path,attr,omitempty"`
	ID           string `xml:"id,attr"`
	ShortVersion string `xml:"CFBundleShortVersionString,attr,omitempty"`
	Version      string `xml:"CFBundleVersion,attr,omitempty"`
}
type bundleRefs struct {
	Bundles []bundleInfo `xml:"bundle"`
}
type scriptInfo struct {
	File    string `xml:"file,attr"`
	Timeout int    `xml:"timeout,attr"`
}
type scriptList struct {
	Pre  *scriptInfo `xml:"preinstall,omitempty"`
	Post *scriptInfo `xml:"postinstall,omitempty"`
}
type packageInfo struct {
	XMLName         xml.Name     `xml:"pkg-info"`
	Overwrite       bool         `xml:"overwrite-permissions,attr"`
	Relocatable     bool         `xml:"relocatable,attr"`
	Identifier      string       `xml:"identifier,attr"`
	Version         string       `xml:"version,attr"`
	Format          string       `xml:"format-version,attr"`
	Generator       string       `xml:"generator-version,attr,omitempty"`
	InstallLocation string       `xml:"install-location,attr,omitempty"`
	Auth            string       `xml:"auth,attr"`
	Conclusion      string       `xml:"postinstall-action,attr"`
	MinOS           string       `xml:"minimumSystemVersion,attr,omitempty"`
	Payload         *payloadInfo `xml:"payload,omitempty"`
	Bundles         []bundleInfo `xml:"bundle"`
	BundleVersion   bundleRefs   `xml:"bundle-version"`
	Upgrade         bundleRefs   `xml:"upgrade-bundle"`
	Update          bundleRefs   `xml:"update-bundle"`
	Atomic          bundleRefs   `xml:"atomic-update-bundle"`
	Strict          bundleRefs   `xml:"strict-identifier"`
	Relocate        bundleRefs   `xml:"relocate"`
	Scripts         *scriptList  `xml:"scripts,omitempty"`
}

// BuildComponent creates an unsigned component package. Input files must remain
// unchanged until the call returns. Existing output survives failed builds.
func BuildComponent(ctx context.Context, c ComponentConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.OutputPath == "" {
		return fmt.Errorf("output path is required")
	}
	if strings.TrimSpace(c.Identifier) == "" || strings.TrimSpace(c.Version) == "" {
		return fmt.Errorf("identifier and version are required")
	}
	for _, value := range []string{c.Identifier, c.Version, c.InstallLocation} {
		if !validXMLText(value) {
			return fmt.Errorf("package metadata contains characters XML cannot represent")
		}
	}
	if c.PayloadMode > Large {
		return fmt.Errorf("invalid payload mode")
	}
	if c.InstallLocation == "" {
		c.InstallLocation = "/"
	}
	if !path.IsAbs(c.InstallLocation) || path.Clean(c.InstallLocation) != c.InstallLocation || strings.ContainsAny(c.InstallLocation, "\x00\\") {
		return fmt.Errorf("install location must be a clean absolute POSIX path")
	}
	if c.Root == "" && (c.ScriptsDir == "" || c.RootEntry != "") {
		return fmt.Errorf("root or scripts directory is required")
	}
	var entries, scripts []fileEntry
	var err error
	if c.Root != "" {
		if err = outputOutside(c.OutputPath, c.Root, c.RootEntry); err != nil {
			return err
		}
		entries, err = collect(ctx, c.Root, c.RootEntry, c.Ownership)
		if err != nil {
			return err
		}
	}
	if c.ScriptsDir != "" {
		if err = outputOutside(c.OutputPath, c.ScriptsDir, ""); err != nil {
			return err
		}
		scripts, err = collect(ctx, c.ScriptsDir, "", c.Ownership)
		if err != nil {
			return err
		}
	}
	large := c.PayloadMode == Large
	for _, e := range entries {
		if e.size >= legacyLimit {
			if c.PayloadMode == Legacy {
				return fmt.Errorf("%s requires large payload mode", e.name)
			}
			large = true
		}
	}
	for _, e := range scripts {
		if e.size >= legacyLimit {
			return fmt.Errorf("script exceeds legacy payload limit: %s", e.name)
		}
	}
	member, minimum := payloadFormat(large)
	c.MinOSVersion, err = resolveMinOS(c.MinOSVersion, minimum)
	if err != nil {
		return fmt.Errorf("payload %w", err)
	}
	info := packageInfo{Overwrite: true, Identifier: c.Identifier, Version: c.Version, Format: "2", Generator: "zapp/macpkg", InstallLocation: c.InstallLocation, Auth: "root", Conclusion: "none", MinOS: c.MinOSVersion}
	if c.Root != "" {
		var kb int64
		for _, e := range entries {
			kb += e.size / 1024
			if e.size%1024 != 0 {
				kb++
			}
		}
		info.Payload = &payloadInfo{Large: large, Files: len(entries), KBytes: kb}
		for _, e := range entries {
			if !e.dir() || !strings.HasSuffix(e.name, ".app") {
				continue
			}
			p := filepath.Join(e.source, "Contents", "Info.plist")
			b, err := readMetadata(p)
			if err != nil {
				return fmt.Errorf("read bundle metadata: %w", err)
			}
			dict, err := plist.ParseDict(b)
			if err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
			id, _ := dict["CFBundleIdentifier"].(string)
			if id == "" {
				return fmt.Errorf("bundle identifier missing in %s", p)
			}
			v, _ := dict["CFBundleVersion"].(string)
			short, _ := dict["CFBundleShortVersionString"].(string)
			if !validXMLText(id) || !validXMLText(v) || !validXMLText(short) {
				return fmt.Errorf("invalid XML text in bundle metadata: %s", p)
			}
			info.Bundles = append(info.Bundles, bundleInfo{Path: "./" + e.name, ID: id, Version: v, ShortVersion: short})
			ref := bundleInfo{ID: id}
			info.BundleVersion.Bundles = append(info.BundleVersion.Bundles, ref)
			info.Upgrade.Bundles = append(info.Upgrade.Bundles, ref)
			info.Strict.Bundles = append(info.Strict.Bundles, ref)
		}
	}
	if len(scripts) > 0 {
		info.Scripts = &scriptList{}
		for _, e := range scripts {
			if e.name != "preinstall" && e.name != "postinstall" {
				continue
			}
			if !e.regular() || e.mode&0111 == 0 {
				return fmt.Errorf("%s must be a regular executable script", e.name)
			}
			s := &scriptInfo{File: "./" + e.name, Timeout: 600}
			if e.name == "preinstall" {
				info.Scripts.Pre = s
			} else {
				info.Scripts.Post = s
			}
		}
		if c.Root == "" && info.Scripts.Pre == nil && info.Scripts.Post == nil {
			return fmt.Errorf("scripts-only package requires preinstall or postinstall")
		}
	}
	return atomicBuild(ctx, c.OutputPath, func(out *os.File, work string) error {
		var archive []archiveEntry
		var staged []*os.File
		defer func() {
			for _, f := range staged {
				_ = f.Close()
			}
		}()
		// stage compresses a payload into the work dir and keeps it open for writeXAR.
		stage := func(name string, es []fileEntry, large bool) error {
			p := filepath.Join(work, name)
			if err := writePayload(ctx, p, es, large); err != nil {
				return err
			}
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			staged = append(staged, f)
			archive = append(archive, archiveEntry{name: name, r: f})
			return nil
		}
		if info.Payload != nil {
			if err := stage(member, entries, large); err != nil {
				return err
			}
			bom, err := makeBOM(entries)
			if err != nil {
				return err
			}
			archive = append(archive, archiveEntry{name: "Bom", r: bytes.NewReader(bom)})
		}
		if len(scripts) > 0 {
			if err := stage("Scripts", scripts, false); err != nil {
				return err
			}
		}
		x, err := xml.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		archive = append(archive, archiveEntry{name: "PackageInfo", r: bytes.NewReader(append([]byte(xml.Header), x...))})
		return writeXAR(ctx, out, work, archive)
	})
}

func outputOutside(output, root, only string) error {
	o, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	r, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	r, err = filepath.EvalSymlinks(r)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(o))
	if err != nil {
		return err
	}
	o = filepath.Join(parent, filepath.Base(o))
	if only != "" {
		r = filepath.Join(r, only)
	}
	rel, err := filepath.Rel(r, o)
	if err != nil {
		return err
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("output must be outside input tree")
	}
	return nil
}

func readMetadata(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, maxMetadata+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxMetadata {
		return nil, fmt.Errorf("metadata exceeds %d bytes: %s", maxMetadata, p)
	}
	return b, nil
}
