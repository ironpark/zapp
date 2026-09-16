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
	set func(string) error
}

func stringField(label string, value *string, hint string) field {
	return field{Label: label, Value: *value, Hint: hint, set: func(s string) error { *value = s; return nil }}
}
func choiceField(label string, value *string, choices ...string) field {
	f := stringField(label, value, "Click to cycle options, or Tab then Enter")
	f.Choices = choices
	if *value == "" {
		f.DisplayValue = "Default"
	}
	return f
}

// coord reads an optional layout coordinate, treating an unset one as zero.
func coord(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// contentAxes drives the two selected-item coordinate fields from one
// definition instead of comparing an axis name inside the shared setter.
var contentAxes = []struct {
	name  string
	of    func(zapp.Content) *int
	apply func(n, x, y int) (int, int)
}{
	{"X", func(c zapp.Content) *int { return c.X }, func(n, _, y int) (int, int) { return n, y }},
	{"Y", func(c zapp.Content) *int { return c.Y }, func(n, x, _ int) (int, int) { return x, n }},
}

func boolField(label string, value *bool, hint string) field {
	return field{Label: label, Value: strconv.FormatBool(*value), Hint: hint, Choices: []string{"false", "true"}, set: func(s string) error {
		b, err := strconv.ParseBool(strings.TrimSpace(s))
		if err != nil {
			return fmt.Errorf("%s must be true or false", label)
		}
		*value = b
		return nil
	}}
}
func intField(label string, value *int, low, high int, hint string, displayZero ...int) field {
	f := field{Label: label, Value: strconv.Itoa(*value), Hint: hint, set: func(s string) error {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil || n < low || n > high {
			return fmt.Errorf("%s must be %d–%d", label, low, high)
		}
		*value = n
		return nil
	}}
	if *value == 0 && len(displayZero) > 0 {
		f.DisplayValue = strconv.Itoa(displayZero[0]) + " (default)"
	}
	return f
}

func jsonField[T any](label string, value *T, hint string) field {
	b, _ := json.MarshalIndent(value, "", "  ")
	return field{Label: label, Value: string(b), Hint: hint, Multiline: true, set: func(s string) error {
		var next T
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
	defer g.syncForm()
	g.projectDirty = g.s.Dirty()
	g.fields = nil
	g.active = -1
	if g.enabled() {
		g.fields = g.section().fields(g)
	}
	g.refreshPreview()
}

func (g *editor) projectFields() []field {
	p := g.s.Project
	return []field{
		stringField("App bundle", &p.App, "Path to MyApp.app; relative to the configuration"),
		stringField("Output directory", &p.Out, "Directory used by DMG and PKG"),
	}
}

func (g *editor) dmgFields() []field {
	p := g.s.Project
	c := p.DMG
	var fields []field
	if g.adding {
		fields = append(fields, stringField("New item path", &g.newPath, "File or folder path; click Add file to add"))
	}
	if item, ok := g.selectedContent(); ok {
		fields = append(fields, g.selectedItemFields(c, item)...)
	}
	fields = append(fields,
		stringField("Title", &c.Title, "Blank uses the app name"),
		stringField("Background image", &c.Background, "PNG or JPEG; drawn at its native size"),
		intField("Window width", &c.Window.Width, 0, 32768, fmt.Sprintf("0 = default %d", zapp.DefaultWindowWidth), zapp.DefaultWindowWidth),
		intField("Window height", &c.Window.Height, 0, 32768, fmt.Sprintf("0 = default %d", zapp.DefaultWindowHeight), zapp.DefaultWindowHeight),
		intField("Icon size", &c.IconSize, 0, dmg.MaxIconSize, fmt.Sprintf("0 = %d; otherwise %d–%d", zapp.DefaultIconSize, dmg.MinIconSize, dmg.MaxIconSize), zapp.DefaultIconSize),
		intField("Label size", &c.LabelSize, 0, dmg.MaxLabelSize, fmt.Sprintf("0 = %d; otherwise %d–%d", zapp.DefaultLabelSize, dmg.MinLabelSize, dmg.MaxLabelSize), zapp.DefaultLabelSize),
	)
	if g.dmgAdvanced {
		fields = append(fields,
			stringField("Disk icon", &c.Icon, "ICNS or PNG; not the app icon"),
			choiceField("Filesystem", &c.FS, "", "hfsplus", "apfs", "apfs-case-sensitive"),
			choiceField("Compression", &c.Format, "", "udzo", "ulfo"),
			stringField("Output file", &c.Out, "Blank uses the project output directory"),
			jsonField("Contents (JSON)", &c.Contents, "null = automatic app + Applications layout"),
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
	i, found := layout(p.DMG, p.App).find(g.selected)
	if !found {
		return zapp.Content{}, false
	}
	x, y := i.X, i.Y
	return zapp.Content{X: &x, Y: &y, Name: i.Name, Link: i.Link}, true
}

func (g *editor) selectedItemFields(c *zapp.DMGConfig, item zapp.Content) []field {
	key := g.selected
	fields := []field{{Label: "Selected item name", Value: item.Name, Hint: "Blank uses the source filename", set: func(v string) error {
		g.s.materialize()
		i := c.Contents[key]
		i.Name = v
		c.Contents[key] = i
		return nil
	}}}
	// Both coordinates go through Session.move, so a typed value is clamped to
	// the window exactly like a dragged one.
	for _, axis := range contentAxes {
		fields = append(fields, field{Label: "Selected item " + axis.name, Value: strconv.Itoa(coord(axis.of(item))), Hint: "Icon center in Finder content coordinates", set: func(v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || uint64(n) > dmg.MaxCoordinate {
				return fmt.Errorf("coordinate must be a nonnegative 32-bit integer")
			}
			g.s.materialize()
			i := c.Contents[key]
			x, y := axis.apply(n, coord(i.X), coord(i.Y))
			g.s.move(key, x, y)
			return nil
		}})
	}
	return fields
}

func (g *editor) pkgFields() []field {
	c := g.s.Project.PKG
	fields := []field{
		choiceField("Package type", &c.Type, "", "product", "component"),
		stringField("Output file", &c.Out, "Blank uses the project output directory"),
	}
	if c.Components == nil && c.Distribution == nil {
		return append(fields,
			stringField("Identifier", &c.Identifier, "Blank reads the app Info.plist"),
			stringField("Version", &c.Version, "Blank reads the app Info.plist"),
			stringField("Install location", &c.InstallLocation, "For example /Applications"),
			stringField("Scripts directory", &c.Scripts, "Installer scripts"),
			stringField("Minimum macOS", &c.MinOS, "For example 10.13"),
			jsonField("Licenses (JSON)", &c.License, `{"default":"license.txt","ko":"license-ko.txt"}`),
		)
	}
	return append(fields,
		jsonField("Components (JSON)", &c.Components, "Full-form package components"),
		jsonField("Distribution (JSON)", &c.Distribution, "Installer title, resources, license and choices"),
	)
}

func (g *editor) depFields() []field {
	return []field{jsonField("Library search paths (JSON)", &g.s.Project.Dep.Libs, `["/opt/homebrew/lib", "vendor/lib"]`)}
}

func (g *editor) signFields() []field {
	c := g.s.Project.Sign
	return []field{
		stringField("Signing identity", &c.Identity, "Certificate name or ${env:ZAPP_IDENTITY}"),
		stringField("PKCS#12 certificate", &c.P12File, "Path to .p12 certificate"),
		stringField("PEM certificate", &c.PEMFile, "Path to PEM certificate"),
		stringField("Password file", &c.P12PasswordFile, "File path only; passwords are not stored in this UI"),
	}
}

func (g *editor) notarizeFields() []field {
	c := g.s.Project.Notarize
	return []field{
		stringField("Keychain profile", &c.Profile, "macOS notarytool profile"),
		stringField("Apple ID", &c.AppleID, "Apple account email"),
		stringField("Team ID", &c.TeamID, "Developer team identifier"),
		stringField("API key file", &c.APIKeyFile, "Path to API key configuration"),
		boolField("Staple", &c.Staple, "Click to toggle stapling"),
	}
}
