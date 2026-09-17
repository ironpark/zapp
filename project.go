// Package zapp resolves project files and orchestrates macOS deployment engines.
package zapp

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/goccy/go-yaml"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
)

type Project struct {
	Version  int             `json:"version"`
	App      string          `json:"app,omitempty"`
	Out      string          `json:"out,omitempty"`
	Sign     *SignConfig     `json:"sign,omitempty"`
	Notarize *NotarizeConfig `json:"notarize,omitempty"`
	Dep      *DepConfig      `json:"dep,omitempty"`
	DMG      *DMGConfig      `json:"dmg,omitempty"`
	PKG      *PKGConfig      `json:"pkg,omitempty"`
	dir      string
	legacy   bool
}
type SignConfig struct {
	Identity        string `json:"identity,omitempty"`
	P12File         string `json:"p12File,omitempty"`
	PEMFile         string `json:"pemFile,omitempty"`
	P12PasswordFile string `json:"p12PasswordFile,omitempty"`
	// Passwords are runtime-only. The strict decoder rejects their file keys.
	P12Password string `json:"-"`
}
type NotarizeConfig struct {
	Profile    string `json:"profile,omitempty"`
	AppleID    string `json:"appleId,omitempty"`
	TeamID     string `json:"teamId,omitempty"`
	APIKeyFile string `json:"apiKeyFile,omitempty"`
	Staple     bool   `json:"staple,omitempty"`
	Password   string `json:"-"`
}
type DepConfig struct {
	Libs []string `json:"libs,omitempty"`
}
type Window struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}
type Content struct {
	Icon string    `json:"icon,omitempty"`
	Pos  *Position `json:"pos" yaml:"pos,flow"`
	Link bool      `json:"link,omitempty"`
	Name string    `json:"name,omitempty"`
}
type DMGConfig struct {
	Title                string             `json:"title,omitempty"`
	Icon                 string             `json:"icon,omitempty"`
	Background           string             `json:"background,omitempty"`
	FS                   string             `json:"fs,omitempty"`
	Format               string             `json:"format,omitempty"`
	Window               Window             `json:"window"`
	IconSize             int                `json:"iconSize,omitempty"`
	LabelSize            int                `json:"labelSize,omitempty"`
	Out                  string             `json:"out,omitempty"`
	Contents             map[string]Content `json:"contents,omitempty"`
	UseOriginalIcon      bool               `json:"-"`
	AppPosition          *[2]int            `json:"-"`
	ApplicationsPosition *[2]int            `json:"-"`
}

// DMG layout defaults, applied wherever a field is left at its zero value.
const (
	DefaultWindowWidth  = 640
	DefaultWindowHeight = 480
	DefaultIconSize     = 128
	DefaultLabelSize    = 14
)

// Metrics returns the window and icon geometry with defaults applied, leaving
// the config untouched. Resolve and the GUI preview share one definition of
// what an unset dimension means.
func (c *DMGConfig) Metrics() (width, height, iconSize, labelSize int) {
	width, height, iconSize, labelSize = c.Window.Width, c.Window.Height, c.IconSize, c.LabelSize
	if width == 0 {
		width = DefaultWindowWidth
	}
	if height == 0 {
		height = DefaultWindowHeight
	}
	if iconSize == 0 {
		iconSize = DefaultIconSize
	}
	if labelSize == 0 {
		labelSize = DefaultLabelSize
	}
	return
}

// DefaultPositions returns the automatic placement used when contents are not
// listed: the app bundle left of centre and the Applications link right of it,
// both on the same baseline.
func (c *DMGConfig) DefaultPositions() (appX, linkX, y int) {
	width, height, iconSize, labelSize := c.Metrics()
	y = int(float64(height)/2-float64(iconSize)/2) + labelSize
	appX = int(float64(width)/3 - float64(iconSize)/2)
	linkX = int(float64(width)/3*2 + float64(iconSize)/2)
	return
}

type License map[string]string

func (l *License) UnmarshalYAML(data []byte) error {
	var s string
	if err := yaml.UnmarshalWithOptions(data, &s, yaml.Strict()); err == nil {
		*l = License{"default": s}
		return nil
	}
	var m map[string]string
	if err := yaml.UnmarshalWithOptions(data, &m, yaml.Strict()); err != nil {
		return err
	}
	*l = m
	return nil
}

type PKGConfig struct {
	Identifier      string        `json:"identifier,omitempty"`
	Version         string        `json:"version,omitempty"`
	InstallLocation string        `json:"installLocation,omitempty"`
	Scripts         string        `json:"scripts,omitempty"`
	MinOS           string        `json:"minOS,omitempty"`
	License         License       `json:"license,omitempty"`
	Out             string        `json:"out,omitempty"`
	Type            string        `json:"type,omitempty"`
	Components      []Component   `json:"components,omitempty"`
	Distribution    *Distribution `json:"distribution,omitempty"`
}

// HasShortForm reports whether any single-component field is set, and
// HasFullForm whether the components/distribution form is in use. The two
// forms cannot be mixed, so validation and editors share one definition of
// which fields belong to each.
func (c *PKGConfig) HasShortForm() bool {
	return c.Identifier != "" || c.Version != "" || c.InstallLocation != "" || c.Scripts != "" || c.MinOS != "" || c.License != nil
}
func (c *PKGConfig) HasFullForm() bool {
	return c.Components != nil || c.Distribution != nil
}

type Component struct {
	ID              string `json:"id"`
	Root            string `json:"root,omitempty"`
	Entry           string `json:"entry,omitempty"`
	InstallLocation string `json:"installLocation,omitempty"`
	Scripts         string `json:"scripts,omitempty"`
	Version         string `json:"version,omitempty"`
	MinOS           string `json:"minOS,omitempty"`
}
type Distribution struct {
	Title        string   `json:"title,omitempty"`
	Organization string   `json:"organization,omitempty"`
	MinOS        string   `json:"minOS,omitempty"`
	Resources    string   `json:"resources,omitempty"`
	License      string   `json:"license,omitempty"`
	Choices      []Choice `json:"choices,omitempty"`
}
type Choice struct {
	ID          string   `json:"id"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Packages    []string `json:"packages"`
	Selected    bool     `json:"selected"`
	Visible     bool     `json:"visible"`
}

func Load(path string) (*Project, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	p, err := Parse(f, filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return p, nil
}
func Parse(r io.Reader, baseDir string) (*Project, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var keys map[string]any
	decoder := yaml.NewDecoder(bytes.NewReader(data), yaml.Strict())
	if err = decoder.Decode(&keys); err != nil {
		return nil, err
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("config must contain exactly one document")
	}

	if pkg, ok := keys["pkg"].(map[string]any); ok {
		_, components := pkg["components"]
		_, distribution := pkg["distribution"]
		if components || distribution {
			for _, key := range []string{"identifier", "version", "installLocation", "scripts", "minOS", "license"} {
				if _, ok := pkg[key]; ok {
					return nil, fmt.Errorf("pkg cannot mix short form field %s with components/distribution", key)
				}
			}
		}
	}
	// Zero is meaningful in coordinates but invalid in explicitly supplied sizes.
	layout := keys
	if v, ok := keys["dmg"].(map[string]any); ok {
		layout = v
	}
	for _, key := range []string{"iconSize", "labelSize"} {
		if v, ok := layout[key]; ok && fmt.Sprint(v) == "0" {
			return nil, fmt.Errorf("%s must be positive", key)
		}
	}
	if w, ok := layout["window"].(map[string]any); ok {
		for _, key := range []string{"width", "height"} {
			if v, ok := w[key]; ok && fmt.Sprint(v) == "0" {
				return nil, fmt.Errorf("window.%s must be positive", key)
			}
		}
	}
	p := new(Project)
	// Without section keys this is the old flat DMG schema.
	section := false
	for _, k := range []string{"dmg", "pkg", "dep", "sign", "notarize"} {
		if _, ok := keys[k]; ok {
			section = true
		}
	}
	if !section {
		var flat struct {
			Version   int    `json:"version"`
			App       string `json:"app"`
			DMGConfig `json:",inline"`
		}
		if err = yaml.UnmarshalWithOptions(data, &flat, yaml.Strict()); err != nil {
			return nil, err
		}
		p.Version, p.App, p.DMG, p.legacy = flat.Version, flat.App, &flat.DMGConfig, true
	} else if err = yaml.UnmarshalWithOptions(data, p, yaml.Strict()); err != nil {
		return nil, err
	}
	if p.Version != 1 {
		return nil, fmt.Errorf("config version must be 1")
	}
	p.dir, err = filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	if err = p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}
func (p *Project) validate() error {
	if p.Version != 0 && p.Version != 1 {
		return fmt.Errorf("config version must be 1")
	}
	if c := p.PKG; c != nil {
		full := c.HasFullForm()
		if full && c.HasShortForm() {
			return fmt.Errorf("pkg cannot mix short form fields with components/distribution")
		}
		if full && len(c.Components) == 0 {
			return fmt.Errorf("pkg full form requires components")
		}
		if c.Type != "" && c.Type != "product" && c.Type != "component" {
			return fmt.Errorf("pkg type must be product or component")
		}
		if c.Type == "component" && len(c.Components) > 1 {
			return fmt.Errorf("pkg component type requires exactly one component")
		}
		if c.Type == "component" && c.Distribution != nil {
			return fmt.Errorf("component package cannot have distribution")
		}
	}
	return nil
}
func (p *Project) Legacy() bool { return p.legacy }

var ErrNotFound = errors.New(".zapp.yaml not found")

func Discover(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		name := filepath.Join(dir, ".zapp.yaml")
		info, err := os.Stat(name)
		if err == nil {
			if info.IsDir() {
				return "", fmt.Errorf("%s is a directory", name)
			}
			return name, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotFound
		}
		dir = parent
	}
}

// YAML returns the project representation, excluding runtime secrets.
func (p *Project) YAML() ([]byte, error) { return yaml.Marshal(p) }

// Clone copies mutable schema data without serializing runtime credentials.
func (p *Project) Clone() *Project {
	q := *p
	if p.Sign != nil {
		x := *p.Sign
		q.Sign = &x
	}
	if p.Notarize != nil {
		x := *p.Notarize
		q.Notarize = &x
	}
	if p.Dep != nil {
		x := *p.Dep
		x.Libs = append([]string(nil), x.Libs...)
		q.Dep = &x
	}
	if p.DMG != nil {
		x := *p.DMG
		q.DMG = &x
		if x.Contents != nil {
			x.Contents = make(map[string]Content)
			maps.Copy(x.Contents, p.DMG.Contents)
		}
	}
	if p.PKG != nil {
		x := *p.PKG
		q.PKG = &x
		x.Components = slices.Clone(x.Components)
		if x.License != nil {
			x.License = License{}
			maps.Copy(x.License, p.PKG.License)
		}
		if x.Distribution != nil {
			d := *x.Distribution
			x.Distribution = &d
			d.Choices = append([]Choice(nil), d.Choices...)
			for i := range d.Choices {
				d.Choices[i].Packages = append([]string(nil), d.Choices[i].Packages...)
			}
		}
	}
	return &q
}
