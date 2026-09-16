package gui

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
	"github.com/ironpark/zapp/pkg/dmg"
)

// field binds reusable input presentation to a project-specific setter.
type field struct {
	comp.InputSpec
	set    func(string) error
	picker pickMode
	// mayNotExist marks a path the build creates, so Validate does not require
	// it to be on disk already.
	mayNotExist bool
}

func stringField(label string, value *string, hint string) field {
	return field{Label: label, Value: *value, Hint: hint, Placeholder: fieldPlaceholders[label], set: func(s string) error { *value = s; return nil }}
}
func choiceField(label string, value *string, choices ...string) field {
	f := stringField(label, value, "Click to choose, or press Enter")
	f.Choices = choices
	if *value == "" {
		f.DisplayValue = "Default"
	}
	return f
}

// contentAxes drives the two selected-item coordinate fields from one
// definition instead of comparing an axis name inside the shared setter.
var contentAxes = []struct {
	name  string
	of    func(zapp.Content) int
	apply func(n, x, y int) (int, int)
}{
	{"X", func(c zapp.Content) int { return positionCoord(c.Pos, 0) }, func(n, _, y int) (int, int) { return n, y }},
	{"Y", func(c zapp.Content) int { return positionCoord(c.Pos, 1) }, func(n, x, _ int) (int, int) { return x, n }},
}

func boolField(label string, value *bool, hint string) field {
	return field{Boolean: true, Label: label, Value: strconv.FormatBool(*value), Hint: hint, Choices: []string{"false", "true"}, set: func(s string) error {
		b, err := strconv.ParseBool(strings.TrimSpace(s))
		if err != nil {
			return fmt.Errorf("%s must be true or false", label)
		}
		*value = b
		return nil
	}}
}

// intField accepts 0 as "use def", so the stepper's own minimum (stepMin) is
// passed separately from the range the setter accepts.
func intField(label string, value *int, high, def, stepMin int, hint string) field {
	f := field{Label: label, Value: strconv.Itoa(*value), Hint: hint, Number: &comp.NumberSpec{Min: stepMin, Max: high, Step: 1, Default: def}, set: func(s string) error {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n < 0 || n > high {
			return fmt.Errorf("%s must be 0–%d", label, high)
		}
		*value = n
		return nil
	}}
	if *value == 0 {
		f.DisplayValue = strconv.Itoa(def) + " (default)"
	}
	return f
}

func jsonField[T any](label string, value *T, hint string) field {
	b, _ := json.MarshalIndent(value, "", "  ")
	text := string(b)
	if text == "null" {
		text = ""
	}
	return field{Label: label, Value: text, Placeholder: "Optional · JSON", Hint: hint, Multiline: true, set: func(s string) error {
		var next T
		if strings.TrimSpace(s) == "" {
			*value = next
			return nil
		}
		dec := json.NewDecoder(strings.NewReader(s))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&next); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			return fmt.Errorf("%s must contain one JSON value", label)
		}
		*value = next
		return nil
	}}
}

func (g *editor) rebuild() {
	g.rebuildFields()
	g.refreshDerived()
}

func (g *editor) rebuildFields() {
	g.restoreLive()
	defer g.syncForm()
	g.projectDirty = g.s.Dirty()
	g.fields = nil
	g.active = -1
	g.choiceOpen = false
	if g.enabled() {
		g.fields = g.section().fields(g)
	}
	g.inspectorStart = len(g.fields)
	if g.tab == tabDMG && g.enabled() {
		if _, ok := g.s.layout().find(g.selected); !ok {
			g.selected = ""
		}
		if item, ok := g.selectedContent(); ok {
			g.fields = append(g.fields, g.selectedItemFields(g.s.Project.DMG, item)...)
		}
	}
}

func (g *editor) refreshDerived() {
	// Both derived caches read the layout minus item coordinates, so one
	// signature keeps a drag or an arrow-key nudge from re-resolving the
	// project and re-stat'ing every item path.
	if sig := g.derivedSignature(); sig != g.previewSig {
		g.previewSig = sig
		g.refreshPreview()
		g.refreshItemKinds()
	}
}

func (g *editor) projectFields() []field {
	p := g.s.Project
	outDir := pathField("Output directory", &p.Out, "Directory used by DMG and PKG", pickFolder)
	outDir.mayNotExist = true
	return []field{
		pathField("App bundle", &p.App, "Path to MyApp.app; relative to the configuration", pickApp),
		outDir,
	}
}

func (g *editor) dmgFields() []field {
	if g.dmgYAML {
		return []field{g.yamlField()}
	}
	return g.dmgFormFields()
}

func (g *editor) dmgFormFields() []field {
	p := g.s.Project
	c := p.DMG
	var fields []field
	height := intField("Window height", &c.Window.Height, 32768, zapp.DefaultWindowHeight, 0, fmt.Sprintf("0 = default %d", zapp.DefaultWindowHeight))
	height.SameRow = true
	fields = append(fields,
		stringField("Title", &c.Title, "Blank uses the app name"),
		pathField("Background image", &c.Background, "PNG or JPEG; drawn at its native size", pickImage),
		intField("Window width", &c.Window.Width, 32768, zapp.DefaultWindowWidth, 0, fmt.Sprintf("0 = default %d", zapp.DefaultWindowWidth)),
		height,
		intField("Icon size", &c.IconSize, dmg.MaxIconSize, zapp.DefaultIconSize, dmg.MinIconSize, fmt.Sprintf("0 = %d; otherwise %d–%d", zapp.DefaultIconSize, dmg.MinIconSize, dmg.MaxIconSize)),
		intField("Label size", &c.LabelSize, dmg.MaxLabelSize, zapp.DefaultLabelSize, dmg.MinLabelSize, fmt.Sprintf("0 = %d; otherwise %d–%d", zapp.DefaultLabelSize, dmg.MinLabelSize, dmg.MaxLabelSize)),
	)
	if g.dmgAdvanced {
		fields = append(fields,
			pathField("Disk icon", &c.Icon, "ICNS or PNG; not the app icon", pickIcon),
			choiceField("Filesystem", &c.FS, "", "hfsplus", "apfs", "apfs-case-sensitive"),
			choiceField("Compression", &c.Format, "", "udzo", "ulfo"),
			pathField("Output file", &c.Out, "Blank uses the project output directory", pickSave),
		)
	}
	return fields
}

// selectedContent resolves the selected layout item, falling back to the
// automatic layout for a project that has not materialized its contents yet.
func (g *editor) selectedContent() (zapp.Content, bool) {
	p := g.s.Project
	if item, ok := p.DMG.Contents[g.selected]; ok {
		return item, true
	}
	if g.selected == "" {
		return zapp.Content{}, false
	}
	i, found := g.s.layout().find(g.selected)
	if !found {
		return zapp.Content{}, false
	}
	x, y := i.X, i.Y
	return zapp.Content{Pos: &zapp.Position{x, y}, Name: i.Name, Link: i.Link, Icon: i.Icon}, true
}

// itemIconFieldIndex is the position of the "Item icon" field within
// selectedItemFields, after Name, X and Y. Callers that focus or decorate that
// field use this instead of a bare literal.
const itemIconFieldIndex = 3

func (g *editor) selectedItemFields(c *zapp.DMGConfig, item zapp.Content) []field {
	key := g.selected
	fields := []field{{Label: "Name", Value: item.Name, Hint: "Blank uses the source filename", Placeholder: "Source filename", set: func(v string) error {
		g.s.materialize()
		i := c.Contents[key]
		i.Name = v
		c.Contents[key] = i
		return nil
	}}}
	// Both coordinates go through Session.move, so a typed value is clamped to
	// the window exactly like a dragged one.
	for _, axis := range contentAxes {
		fields = append(fields, field{Label: axis.name, Value: strconv.Itoa(axis.of(item)), Hint: "Icon center (px)", Number: &comp.NumberSpec{Min: 0, Max: int(dmg.MaxCoordinate), Step: 1}, set: func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || uint64(n) > dmg.MaxCoordinate {
				return fmt.Errorf("coordinate must be a nonnegative 32-bit integer")
			}
			g.s.materialize()
			i := c.Contents[key]
			x, y := axis.apply(n, positionCoord(i.Pos, 0), positionCoord(i.Pos, 1))
			g.s.move(key, x, y)
			return nil
		}})
	}
	icon := pathField("Item icon", &item.Icon, "Blank uses the original icon", pickItemIcon)
	icon.set = func(value string) error {
		g.s.materialize()
		i := c.Contents[key]
		i.Icon = value
		c.Contents[key] = i
		return nil
	}
	icon.Placeholder = "PNG, JPG or ICNS"
	if !item.Link {
		fields = append(fields, icon)
	}
	fields[2].SameRow = true
	return fields
}

func (g *editor) pkgFields() []field {
	c := g.s.Project.PKG
	fields := []field{
		choiceField("Package type", &c.Type, "", "product", "component"),
		pathField("Output file", &c.Out, "Blank uses the project output directory", pickSave),
	}
	if !c.HasFullForm() {
		fields = append(fields,
			stringField("Identifier", &c.Identifier, "Blank reads the app Info.plist"),
			stringField("Version", &c.Version, "Blank reads the app Info.plist"),
			stringField("Install location", &c.InstallLocation, "For example /Applications"),
			pathField("Scripts directory", &c.Scripts, "Installer scripts", pickFolder),
			stringField("Minimum macOS", &c.MinOS, "For example 10.13"),
			jsonField("Licenses", &c.License, `{"default":"license.txt","ko":"license-ko.txt"}`),
		)
		fields[3].SameRow = true // Identifier and version.
		fields[6].SameRow = true // Scripts and minimum macOS.
		return fields
	}
	return append(fields,
		jsonField("Components", &c.Components, "Full-form package components"),
		jsonField("Distribution", &c.Distribution, "Installer title, resources, license and choices"),
	)
}

func (g *editor) depFields() []field {
	c := g.s.Project.Dep
	return []field{{Label: "Library search paths", Value: strings.Join(c.Libs, "\n"),
		Multiline: true, Height: 180, Placeholder: "/opt/homebrew/lib", Hint: "One directory per line; blank uses automatic discovery",
		set: func(value string) error {
			var paths []string
			for _, line := range strings.Split(value, "\n") {
				if path := strings.TrimSpace(line); path != "" {
					paths = append(paths, path)
				}
			}
			c.Libs = paths
			return nil
		}}}
}

func (g *editor) signFields() []field {
	c := g.s.Project.Sign
	return []field{
		stringField("Signing identity", &c.Identity, "macOS Keychain certificate name or ${env:ZAPP_IDENTITY}"),
		pathField("PKCS#12 certificate", &c.P12File, "Windows / Linux: .p12 certificate for rcodesign", pickFile),
		pathField("PEM certificate", &c.PEMFile, "Windows / Linux: PEM certificate for rcodesign", pickFile),
		pathField("Password file", &c.P12PasswordFile, "File path only; passwords are not stored in this UI", pickFile),
	}
}

func (g *editor) notarizeFields() []field {
	c := g.s.Project.Notarize
	return []field{
		stringField("Keychain profile", &c.Profile, "macOS: saved notarytool credentials (recommended)"),
		stringField("Apple ID", &c.AppleID, "macOS: requires Team ID and a runtime password"),
		stringField("Team ID", &c.TeamID, "Developer team identifier"),
		pathField("API key file", &c.APIKeyFile, "Windows / Linux: rcodesign API key JSON", pickFile),
		boolField("Staple", &c.Staple, "Attach the notarization ticket after approval"),
	}
}

// fieldPlaceholders holds the greyed-out example shown in an empty input.
var fieldPlaceholders = map[string]string{
	"App bundle": "MyApp.app", "Output directory": "dist",
	"Title": "App name", "Background image": "background.png",
	"Disk icon": "volume.icns", "Output file": "Automatic output path",
	"Identifier": "com.example.myapp", "Version": "1.0.0",
	"Install location": "/Applications", "Scripts directory": "scripts",
	"Minimum macOS": "10.13", "Signing identity": "Developer ID Application: …",
	"PKCS#12 certificate": "certificate.p12", "PEM certificate": "certificate.pem",
	"Password file": "password.txt", "Keychain profile": "notary-profile",
	"Apple ID": "name@example.com", "Team ID": "ABCDEFGHIJ", "API key file": "api-key.json",
}

func positionCoord(pos *zapp.Position, axis int) int {
	if pos == nil {
		return 0
	}
	return pos[axis]
}
