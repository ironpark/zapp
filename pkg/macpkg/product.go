package macpkg

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type productPackage struct {
	name    string
	info    packageInfo
	archive *xarArchive
}
type distRef struct {
	ID         string `xml:"id,attr"`
	Version    string `xml:"version,attr,omitempty"`
	KBytes     int64  `xml:"installKBytes,attr,omitempty"`
	Conclusion string `xml:"onConclusion,attr,omitempty"`
	File       string `xml:",chardata"`
}
type distChoice struct {
	ID          string    `xml:"id,attr"`
	Title       string    `xml:"title,attr,omitempty"`
	Description string    `xml:"description,attr,omitempty"`
	Visible     bool      `xml:"visible,attr"`
	Selected    bool      `xml:"start_selected,attr"`
	Refs        []distRef `xml:"pkg-ref"`
}
type distLicense struct {
	File string `xml:"file,attr"`
	MIME string `xml:"mime-type,attr"`
}
type distLine struct {
	Choice string `xml:"choice,attr"`
}
type distXML struct {
	XMLName      xml.Name `xml:"installer-gui-script"`
	Spec         string   `xml:"minSpecVersion,attr"`
	Title        string   `xml:"title"`
	Organization string   `xml:"organization,omitempty"`
	Domains      struct {
		Local    string `xml:"enable_localSystem,attr"`
		User     string `xml:"enable_currentUserHome,attr"`
		Anywhere string `xml:"enable_anywhere,attr"`
	} `xml:"domains"`
	Options struct {
		Customize string `xml:"customize,attr"`
		Scripts   string `xml:"require-scripts,attr"`
		External  string `xml:"allow-external-scripts,attr"`
	} `xml:"options"`
	License *distLicense `xml:"license,omitempty"`
	OS      struct {
		Versions struct {
			Min string `xml:"min,attr"`
		} `xml:"allowed-os-versions>os-version"`
	} `xml:"volume-check"`
	Lines   []distLine   `xml:"choices-outline>line"`
	Choices []distChoice `xml:"choice"`
	Refs    []distRef    `xml:"pkg-ref"`
}

// indexPackages maps components by identifier and returns the highest minimum
// macOS version any of them requires.
func indexPackages(pkgs []productPackage) (map[string]productPackage, string, error) {
	byID := map[string]productPackage{}
	minimum := baseMinOS
	for _, p := range pkgs {
		byID[p.info.Identifier] = p
		if p.info.MinOS == "" {
			continue
		}
		cmp, err := compareVersion(p.info.MinOS, minimum)
		if err != nil {
			return nil, "", err
		}
		if cmp > 0 {
			minimum = p.info.MinOS
		}
	}
	return byID, minimum, nil
}

func distributionXML(d Distribution, pkgs []productPackage) ([]byte, error) {
	for _, value := range []string{d.Title, d.Organization} {
		if !validXMLText(value) {
			return nil, fmt.Errorf("Distribution contains characters XML cannot represent")
		}
	}
	byID, minimum, err := indexPackages(pkgs)
	if err != nil {
		return nil, err
	}
	d.MinOSVersion, err = resolveMinOS(d.MinOSVersion, minimum)
	if err != nil {
		return nil, fmt.Errorf("product %w", err)
	}
	x := distXML{Spec: "1", Title: d.Title, Organization: d.Organization}
	x.Domains.Local = "true"
	x.Domains.User = "false"
	x.Domains.Anywhere = "false"
	x.Options.Customize = "never"
	x.Options.Scripts = "true"
	x.Options.External = "no"
	x.OS.Versions.Min = d.MinOSVersion
	if d.LicenseFile != "" {
		if !validArchivePath(d.LicenseFile) {
			return nil, fmt.Errorf("invalid license path")
		}
		x.License = &distLicense{d.LicenseFile, "text/plain"}
	}
	if len(d.Choices) == 0 {
		for i, p := range pkgs {
			d.Choices = append(d.Choices, Choice{ID: fmt.Sprintf("component%d", i+1), Selected: true, PackageIDs: []string{p.info.Identifier}})
		}
	}
	seen := map[string]bool{}
	referenced := map[string]bool{}
	for _, c := range d.Choices {
		for _, value := range []string{c.ID, c.Title, c.Description} {
			if !validXMLText(value) {
				return nil, fmt.Errorf("choice contains characters XML cannot represent")
			}
		}
		if c.ID == "" || seen[c.ID] {
			return nil, fmt.Errorf("empty or duplicate choice ID %q", c.ID)
		}
		seen[c.ID] = true
		if c.Visible {
			x.Options.Customize = "allow"
		}
		dc := distChoice{ID: c.ID, Title: c.Title, Description: c.Description, Visible: c.Visible, Selected: c.Selected}
		if len(c.PackageIDs) == 0 {
			return nil, fmt.Errorf("choice %s has no packages", c.ID)
		}
		for _, id := range c.PackageIDs {
			if _, ok := byID[id]; !ok {
				return nil, fmt.Errorf("unknown package ID %s", id)
			}
			if referenced[id] {
				return nil, fmt.Errorf("package %s belongs to multiple choices", id)
			}
			referenced[id] = true
			dc.Refs = append(dc.Refs, distRef{ID: id})
		}
		x.Lines = append(x.Lines, distLine{c.ID})
		x.Choices = append(x.Choices, dc)
	}
	for _, p := range pkgs {
		if !referenced[p.info.Identifier] {
			return nil, fmt.Errorf("unreferenced component %s", p.name)
		}
		kb := int64(0)
		if p.info.Payload != nil {
			kb = p.info.Payload.KBytes
		}
		x.Refs = append(x.Refs, distRef{ID: p.info.Identifier, Version: p.info.Version, KBytes: kb, Conclusion: "none", File: "#" + url.PathEscape(p.name)})
	}
	b, err := xml.MarshalIndent(x, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), b...), nil
}

// validateDistribution preserves caller XML but verifies its package bindings
// and explicit OS bounds. Installer evaluates any caller-supplied JavaScript.
func validateDistribution(data []byte, pkgs []productPackage) ([]string, error) {
	if len(data) > maxMetadata {
		return nil, fmt.Errorf("Distribution exceeds size limit")
	}
	byID, minimum, err := indexPackages(pkgs)
	if err != nil {
		return nil, err
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	roots := 0
	seen := map[string]bool{}
	hasOS := false
	var licenses []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots != 1 || t.Name.Local != "installer-gui-script" {
					return nil, fmt.Errorf("invalid Distribution root")
				}
			}
			depth++
			attrs := map[string]string{}
			for _, a := range t.Attr {
				attrs[a.Name.Local] = a.Value
			}
			if t.Name.Local == "license" {
				p := attrs["file"]
				if !validArchivePath(p) {
					return nil, fmt.Errorf("invalid license path")
				}
				licenses = append(licenses, p)
			}
			if t.Name.Local == "os-version" {
				m := attrs["min"]
				cmp, err := compareVersion(m, minimum)
				if err != nil || cmp < 0 {
					return nil, fmt.Errorf("Distribution OS minimum must be at least %s", minimum)
				}
				hasOS = true
			}
			if t.Name.Local == "pkg-ref" {
				id := attrs["id"]
				p, ok := byID[id]
				if !ok {
					return nil, fmt.Errorf("Distribution references unknown package %s", id)
				}
				var text string
				if err := dec.DecodeElement(&text, &t); err != nil {
					return nil, err
				}
				depth--
				text = strings.TrimSpace(text)
				if text != "" {
					name, err := url.PathUnescape(strings.TrimPrefix(text, "#"))
					if err != nil || name != p.name {
						return nil, fmt.Errorf("package reference %q does not match %s", text, p.name)
					}
					if seen[id] {
						return nil, fmt.Errorf("duplicate package definition %s", id)
					}
					seen[id] = true
				}
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return nil, fmt.Errorf("text outside Distribution root")
			}
		case xml.Directive:
			return nil, fmt.Errorf("XML directives are not supported")
		}
	}
	if roots != 1 || depth != 0 {
		return nil, fmt.Errorf("invalid Distribution document")
	}
	for id := range byID {
		if !seen[id] {
			return nil, fmt.Errorf("missing package definition %s", id)
		}
	}
	cmp, _ := compareVersion(minimum, baseMinOS)
	if cmp > 0 && !hasOS {
		return nil, fmt.Errorf("Distribution must declare macOS %s minimum", minimum)
	}
	return licenses, nil
}

// BuildProduct embeds flat component packages without recompressing their
// members. Resources may contain regular files and directories, including lproj.
func BuildProduct(ctx context.Context, c ProductConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.OutputPath == "" {
		return fmt.Errorf("output path is required")
	}
	if len(c.Packages) == 0 {
		return fmt.Errorf("at least one component is required")
	}
	if c.Distribution != nil && len(c.DistributionXML) > 0 {
		return fmt.Errorf("Distribution and DistributionXML are mutually exclusive")
	}
	var pkgs []productPackage
	var archive []archiveEntry
	names, ids := map[string]bool{}, map[string]bool{}
	for _, p := range c.Packages {
		name := filepath.Base(p)
		if !validArchivePath(name) || names[name] || name == "Resources" || name == "Distribution" {
			return fmt.Errorf("invalid or duplicate component name %q", name)
		}
		names[name] = true
		a, err := openXAR(ctx, p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		defer func() { _ = a.f.Close() }()
		b, err := a.read(ctx, "PackageInfo", maxMetadata)
		if err != nil {
			return err
		}
		var info packageInfo
		if err = xml.Unmarshal(b, &info); err != nil {
			return err
		}
		if info.Identifier == "" || info.Version == "" || info.Format != "2" || ids[info.Identifier] {
			return fmt.Errorf("invalid or duplicate component identifier %q", info.Identifier)
		}
		ids[info.Identifier] = true
		if info.Payload != nil {
			payload, floor := payloadFormat(info.Payload.Large)
			if info.Payload.Large {
				if info.MinOS, err = resolveMinOS(info.MinOS, floor); err != nil {
					return fmt.Errorf("invalid large payload minimum OS")
				}
			}
			if a.entries[payload] == nil || a.entries[payload].Data == nil || a.entries["Bom"] == nil || a.entries["Bom"].Data == nil {
				return fmt.Errorf("component lacks payload or BOM")
			}
		}
		if a.entries["Distribution"] != nil {
			return fmt.Errorf("nested products are not component packages")
		}
		pkgs = append(pkgs, productPackage{name, info, a})
		archive = append(archive, archiveEntry{name: name})
		for member, n := range a.entries {
			e := archiveEntry{name: name + "/" + member, mode: n.Mode, mtime: n.Mtime}
			if n.Data != nil {
				e.r = a.raw(n)
				e.raw = n.Data
			}
			archive = append(archive, e)
		}
	}
	data := c.DistributionXML
	if len(data) == 0 {
		d := Distribution{}
		if c.Distribution != nil {
			d = *c.Distribution
		}
		if d.Title == "" {
			d.Title = strings.TrimSuffix(filepath.Base(c.OutputPath), filepath.Ext(c.OutputPath))
		}
		var err error
		data, err = distributionXML(d, pkgs)
		if err != nil {
			return err
		}
	}
	licenses, err := validateDistribution(data, pkgs)
	if err != nil {
		return err
	}
	resources := map[string]bool{}
	if c.ResourcesDir != "" {
		if err := outputOutside(c.OutputPath, c.ResourcesDir, ""); err != nil {
			return err
		}
		files, err := collect(ctx, c.ResourcesDir, "", RootWheel)
		if err != nil {
			return err
		}
		for _, e := range files {
			if e.name == "." {
				continue
			}
			if !e.dir() && !e.regular() {
				return fmt.Errorf("resource must be a regular file or directory: %s", e.name)
			}
			a := archiveEntry{name: "Resources/" + e.name, mode: fmt.Sprintf("%04o", e.mode&07777)}
			if e.regular() {
				a.source = e.source
				resources[e.name] = true
			}
			archive = append(archive, a)
		}
	}
	for _, license := range licenses {
		found := resources[license]
		for name := range resources {
			if found {
				break
			}
			dir, base, ok := strings.Cut(name, "/")
			found = ok && strings.HasSuffix(dir, ".lproj") && base == license
		}
		if !found {
			return fmt.Errorf("license resource missing: %s", license)
		}
	}
	archive = append(archive, archiveEntry{name: "Distribution", r: bytes.NewReader(data)})
	return atomicBuild(ctx, c.OutputPath, func(out *os.File, work string) error { return writeXAR(ctx, out, work, archive) })
}
