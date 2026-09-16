package gui

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
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

var tabNames = []string{"Project", "DMG", "PKG", "Dependencies", "Signing", "Notarization"}

func (g *editor) enabled() bool {
	p := g.s.Project
	switch g.tab {
	case 1:
		return p.DMG != nil
	case 2:
		return p.PKG != nil
	case 3:
		return p.Dep != nil
	case 4:
		return p.Sign != nil
	case 5:
		return p.Notarize != nil
	}
	return true
}

func (g *editor) toggle() {
	g.s.checkpoint()
	p := g.s.Project
	// Keep disabled values in memory so toggling a section on again restores
	// its fields. Only enabled sections are persisted.
	switch g.tab {
	case 1:
		if p.DMG != nil {
			g.disabled.DMG = p.DMG
			p.DMG = nil
		} else {
			p.DMG = g.disabled.DMG
			if p.DMG == nil {
				p.DMG = &zapp.DMGConfig{}
			}
		}
	case 2:
		if p.PKG != nil {
			g.disabled.PKG = p.PKG
			p.PKG = nil
		} else {
			p.PKG = g.disabled.PKG
			if p.PKG == nil {
				p.PKG = &zapp.PKGConfig{}
			}
		}
	case 3:
		if p.Dep != nil {
			g.disabled.Dep = p.Dep
			p.Dep = nil
		} else {
			p.Dep = g.disabled.Dep
			if p.Dep == nil {
				p.Dep = &zapp.DepConfig{}
			}
		}
	case 4:
		if p.Sign != nil {
			g.disabled.Sign = p.Sign
			p.Sign = nil
		} else {
			p.Sign = g.disabled.Sign
			if p.Sign == nil {
				p.Sign = &zapp.SignConfig{}
			}
		}
	case 5:
		if p.Notarize != nil {
			g.disabled.Notarize = p.Notarize
			p.Notarize = nil
		} else {
			p.Notarize = g.disabled.Notarize
			if p.Notarize == nil {
				p.Notarize = &zapp.NotarizeConfig{}
			}
		}
	}
	g.selected = ""
	g.rebuild()
}

func (g *editor) rebuild() {
	defer g.syncForm()
	g.projectDirty = g.s.Dirty()
	g.fields = nil
	g.active = -1
	p := g.s.Project
	if !g.enabled() {
		return
	}
	switch g.tab {
	case 0:
		g.fields = []field{stringField("App bundle", &p.App, "Path to MyApp.app; relative to the configuration"), stringField("Output directory", &p.Out, "Directory used by DMG and PKG")}
	case 1:
		c := p.DMG
		if g.adding {
			g.fields = append(g.fields, stringField("New item path", &g.newPath, "File or folder path; click Add file to add"))
		}
		item, ok := c.Contents[g.selected]
		if !ok && g.selected != "" {
			if i, found := layout(c, p.App).find(g.selected); found {
				x, y := i.X, i.Y
				item, ok = zapp.Content{X: &x, Y: &y, Name: i.Name, Link: i.Link}, true
			}
		}
		if ok {
			key := g.selected
			g.fields = append(g.fields, field{Label: "Selected item name", Value: item.Name, Hint: "Blank uses the source filename", set: func(v string) error {
				g.s.materialize()
				i := c.Contents[key]
				i.Name = v
				c.Contents[key] = i
				return nil
			}})
			for _, axis := range []string{"X", "Y"} {
				n := 0
				if axis == "X" && item.X != nil {
					n = *item.X
				}
				if axis == "Y" && item.Y != nil {
					n = *item.Y
				}
				g.fields = append(g.fields, field{Label: "Selected item " + axis, Value: strconv.Itoa(n), Hint: "Icon center in Finder content coordinates", set: func(v string) error {
					n, err := strconv.Atoi(v)
					if err != nil || n < 0 || uint64(n) > 0xffffffff {
						return fmt.Errorf("coordinate must be a nonnegative 32-bit integer")
					}
					g.s.materialize()
					i := c.Contents[key]
					if axis == "X" {
						i.X = &n
					} else {
						i.Y = &n
					}
					c.Contents[key] = i
					return nil
				}})
			}
		}
		g.fields = append(g.fields,
			stringField("Title", &c.Title, "Blank uses the app name"),
			stringField("Background image", &c.Background, "PNG or JPEG; drawn at its native size"),
			intField("Window width", &c.Window.Width, 0, 32768, "0 = default 640", 640),
			intField("Window height", &c.Window.Height, 0, 32768, "0 = default 480", 480),
			intField("Icon size", &c.IconSize, 0, 512, "0 = 128; otherwise 16–512", 128),
			intField("Label size", &c.LabelSize, 0, 16, "0 = 14; otherwise 10–16", 14),
		)
		if g.dmgAdvanced {
			g.fields = append(g.fields,
				stringField("Disk icon", &c.Icon, "ICNS or PNG; not the app icon"),
				choiceField("Filesystem", &c.FS, "", "hfsplus", "apfs", "apfs-case-sensitive"),
				choiceField("Compression", &c.Format, "", "udzo", "ulfo"),
				stringField("Output file", &c.Out, "Blank uses the project output directory"),
				jsonField("Contents (JSON)", &c.Contents, "null = automatic app + Applications layout"),
			)
		}

	case 2:
		c := p.PKG
		g.fields = []field{choiceField("Package type", &c.Type, "", "product", "component"), stringField("Output file", &c.Out, "Blank uses the project output directory")}
		if c.Components == nil && c.Distribution == nil {
			g.fields = append(g.fields, stringField("Identifier", &c.Identifier, "Blank reads the app Info.plist"), stringField("Version", &c.Version, "Blank reads the app Info.plist"), stringField("Install location", &c.InstallLocation, "For example /Applications"), stringField("Scripts directory", &c.Scripts, "Installer scripts"), stringField("Minimum macOS", &c.MinOS, "For example 10.13"), jsonField("Licenses (JSON)", &c.License, `{"default":"license.txt","ko":"license-ko.txt"}`))
		} else {
			g.fields = append(g.fields, jsonField("Components (JSON)", &c.Components, "Full-form package components"), jsonField("Distribution (JSON)", &c.Distribution, "Installer title, resources, license and choices"))
		}
	case 3:
		g.fields = []field{jsonField("Library search paths (JSON)", &p.Dep.Libs, `["/opt/homebrew/lib", "vendor/lib"]`)}
	case 4:
		c := p.Sign
		g.fields = []field{stringField("Signing identity", &c.Identity, "Certificate name or ${env:ZAPP_IDENTITY}"), stringField("PKCS#12 certificate", &c.P12File, "Path to .p12 certificate"), stringField("PEM certificate", &c.PEMFile, "Path to PEM certificate"), stringField("Password file", &c.P12PasswordFile, "File path only; passwords are not stored in this UI")}
	case 5:
		c := p.Notarize
		g.fields = []field{stringField("Keychain profile", &c.Profile, "macOS notarytool profile"), stringField("Apple ID", &c.AppleID, "Apple account email"), stringField("Team ID", &c.TeamID, "Developer team identifier"), stringField("API key file", &c.APIKeyFile, "Path to API key configuration"), boolField("Staple", &c.Staple, "Click to toggle stapling")}
	}
	g.refreshPreview()
}
