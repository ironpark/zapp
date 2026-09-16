package zapp

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/dep"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/ironpark/zapp/pkg/macpkg"
	"github.com/ironpark/zapp/pkg/signing"
	"github.com/ironpark/zapp/pkg/udif"
)

var variable = regexp.MustCompile(`\$\{([^}]+)\}`)

func expand(s string, values map[string]string) (string, error) {
	var failure error
	result := variable.ReplaceAllStringFunc(s, func(token string) string {
		key := token[2 : len(token)-1]
		if after, ok := strings.CutPrefix(key, "env:"); ok {
			v, ok := os.LookupEnv(after)
			if !ok {
				failure = fmt.Errorf("environment variable %s is not set", strings.TrimPrefix(key, "env:"))
			}
			return v
		}
		v, ok := values[key]
		if !ok || v == "" {
			failure = fmt.Errorf("cannot resolve %s", token)
		}
		return v
	})
	return result, failure
}
func expandValue(v reflect.Value, values map[string]string) error {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			return expandValue(v.Elem(), values)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() && v.Type().Field(i).Tag.Get("json") != "-" {
				if err := expandValue(v.Field(i), values); err != nil {
					return err
				}
			}
		}
	case reflect.String:
		s, err := expand(v.String(), values)
		if err != nil {
			return err
		}
		v.SetString(s)
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := expandValue(v.Index(i), values); err != nil {
				return err
			}
		}
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		iter := v.MapRange()
		for iter.Next() {
			key := reflect.New(v.Type().Key()).Elem()
			key.Set(iter.Key())
			val := reflect.New(v.Type().Elem()).Elem()
			val.Set(iter.Value())
			if err := expandValue(key, values); err != nil {
				return err
			}
			if err := expandValue(val, values); err != nil {
				return err
			}
			if m.MapIndex(key).IsValid() {
				return fmt.Errorf("duplicate key after substitution: %s", key)
			}
			m.SetMapIndex(key, val)
		}
		if !v.IsNil() {
			v.Set(m)
		}
	}
	return nil
}
func (p *Project) Resolve(opts ...Option) (*Plan, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	q := p.Clone()
	q.Version = 1
	base := q.dir
	if base == "" {
		var err error
		base, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	path := func(s string) string {
		if s == "" || filepath.IsAbs(s) {
			return s
		}
		return filepath.Join(base, s)
	}
	app, err := expand(q.App, nil)
	if err != nil {
		return nil, err
	}
	q.App = path(app)
	name, version, identifier := "", "", ""
	if q.App != "" {
		info, err := os.Stat(q.App)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || !strings.HasSuffix(q.App, ".app") {
			return nil, fmt.Errorf("app must be an .app directory")
		}
		name = strings.TrimSuffix(filepath.Base(q.App), ".app")
		infoPlist, err := appbundle.Open(q.App)
		if err != nil {
			if q.PKG != nil && q.PKG.Components == nil {
				return nil, fmt.Errorf("app Info.plist: %w", err)
			}
		} else {
			version, _ = infoPlist.Version()
			identifier, _ = infoPlist.BundleID()
		}
	}
	q.App = ""
	if err := expandValue(reflect.ValueOf(q).Elem(), map[string]string{"app": path(app), "app.name": name, "app.version": version}); err != nil {
		return nil, err
	}
	q.App = path(app)
	q.Out = path(q.Out)
	output := func(out, stem, ext string) string {
		if out == "" {
			if q.Out != "" {
				return filepath.Join(q.Out, stem+ext)
			}
			return path(stem + ext)
		}
		return path(out)
	}
	pl := &Plan{App: q.App, logger: o.logger, project: q, signBackend: o.signBackend, notaryBackend: o.notaryBackend}
	if q.Dep != nil {
		for i := range q.Dep.Libs {
			q.Dep.Libs[i] = path(q.Dep.Libs[i])
		}
		pl.Dep = &dep.Config{App: q.App, Libs: q.Dep.Libs}
		if q.App == "" {
			return nil, fmt.Errorf("dep requires app")
		}
	}
	if s := q.Sign; s != nil {
		s.P12File = path(s.P12File)
		s.PEMFile = path(s.PEMFile)
		s.P12PasswordFile = path(s.P12PasswordFile)
		pl.SignCredentials = &signing.Credentials{Identity: s.Identity, P12File: s.P12File, PEMFile: s.PEMFile, P12Password: s.P12Password, P12PasswordFile: s.P12PasswordFile}
	}
	if n := q.Notarize; n != nil {
		n.APIKeyFile = path(n.APIKeyFile)
		pl.NotarizeCredentials = &signing.Credentials{Profile: n.Profile, AppleID: n.AppleID, TeamID: n.TeamID, Password: n.Password, APIKeyFile: n.APIKeyFile}
		pl.Staple = n.Staple
	}
	if c := q.DMG; c != nil {
		if c.Title == "" {
			c.Title = name
		}
		if c.Window.Width == 0 {
			c.Window.Width = 640
		}
		if c.Window.Height == 0 {
			c.Window.Height = 480
		}
		if c.IconSize == 0 {
			c.IconSize = 128
		}
		if c.LabelSize == 0 {
			c.LabelSize = 14
		}
		if c.FS == "" {
			c.FS = "hfsplus"
		}
		if c.Format == "" {
			c.Format = "udzo"
		}
		c.Icon, c.Background = path(c.Icon), path(c.Background)
		if q.legacy {
			if c.Out == "" {
				c.Out = c.Title
			}
			c.Out, err = filepath.Abs(c.Out)
			if err != nil {
				return nil, err
			}
		} else {
			c.Out = output(c.Out, c.Title, ".dmg")
		}
		if !strings.HasSuffix(c.Out, ".dmg") {
			c.Out += ".dmg"
		}
		fs, err := dmg.ParseFileSystem(c.FS)
		if err != nil {
			return nil, err
		}
		format, ok := map[string]udif.Format{"udzo": udif.UDZO, "zlib": udif.UDZO, "ulfo": udif.ULFO, "lzfse": udif.ULFO}[strings.ToLower(c.Format)]
		if !ok {
			return nil, fmt.Errorf("unknown image format %q: use udzo or ulfo", c.Format)
		}
		d := &dmg.Config{FileName: c.Out, Title: c.Title, Icon: c.Icon, Background: c.Background, WindowWidth: c.Window.Width, WindowHeight: c.Window.Height, ContentsIconSize: c.IconSize, LabelSize: c.LabelSize, FileSystem: fs, Format: format, Created: o.clock}
		if c.Contents != nil {
			if c.AppPosition != nil || c.ApplicationsPosition != nil {
				return nil, fmt.Errorf("position flags cannot be used with explicit contents; set contents[path].x and contents[path].y")
			}
			keys := make([]string, 0, len(c.Contents))
			for k := range c.Contents {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			normalized := make(map[string]Content, len(keys))
			for _, k := range keys {
				item := c.Contents[k]
				if k == "" {
					return nil, fmt.Errorf("contents path must not be empty")
				}
				if item.X == nil || item.Y == nil {
					return nil, fmt.Errorf("contents[%q] requires x and y", k)
				}
				source := path(k)
				if q.legacy && item.Link {
					source = k
				}
				kind := dmg.Link
				if !item.Link {
					info, err := os.Lstat(source)
					if err != nil {
						return nil, fmt.Errorf("contents[%q]: %w", k, err)
					}
					kind = dmg.File
					if info.IsDir() {
						kind = dmg.Dir
					}
				}
				normalized[source] = item
				d.Contents = append(d.Contents, dmg.Item{Path: source, Type: kind, Name: item.Name, X: *item.X, Y: *item.Y})
			}
			c.Contents = normalized
		} else {
			if q.App == "" {
				return nil, fmt.Errorf("provide --app or config contents")
			}
			y := int(float64(c.Window.Height)/2-float64(c.IconSize)/2) + c.LabelSize
			d.Contents = []dmg.Item{{Type: dmg.Dir, Path: q.App, X: int(float64(c.Window.Width)/3 - float64(c.IconSize)/2), Y: y}, {Type: dmg.Link, Path: "/Applications", X: int(float64(c.Window.Width)/3*2 + float64(c.IconSize)/2), Y: y}}
			for i, pos := range []*[2]int{c.AppPosition, c.ApplicationsPosition} {
				if pos != nil {
					d.Contents[i].X, d.Contents[i].Y = pos[0], pos[1]
				}
			}
		}
		if c.Contents == nil {
			c.Contents = make(map[string]Content, len(d.Contents))
			for _, item := range d.Contents {
				x, y := item.X, item.Y
				c.Contents[item.Path] = Content{X: &x, Y: &y, Link: item.Type == dmg.Link, Name: item.Name}
			}
		}
		if err := d.Validate(); err != nil {
			return nil, err
		}
		pl.DMG = d
		pl.originalIcon = c.UseOriginalIcon
	}
	if c := q.PKG; c != nil {
		stem := name
		if stem == "" {
			stem = "installer"
		}
		c.Out = output(c.Out, stem, ".pkg")
		if c.Type == "" {
			c.Type = "product"
		}
		spec := &PKGSpec{Output: c.Out}
		pl.PKG = spec
		if c.Components == nil {
			if q.App == "" {
				return nil, fmt.Errorf("pkg short form requires app")
			}
			if c.Identifier == "" {
				c.Identifier = identifier
				if c.Identifier == "" {
					c.Identifier = "com.example." + name
				}
			}
			if c.Version == "" {
				c.Version = version
				if c.Version == "" {
					c.Version = "1.0"
				}
			}
			if c.InstallLocation == "" {
				c.InstallLocation = "/Applications"
			}
			c.Scripts = path(c.Scripts)
			licenses := map[string]string{}
			fallback := ""
			for k, v := range c.License {
				v = path(v)
				c.License[k] = v
				if k == "default" {
					fallback = v
				} else {
					licenses[k] = v
				}
			}
			a := &macpkg.AppConfig{Created: o.clock, AppPath: q.App, OutputPath: c.Out, Identifier: c.Identifier, Version: c.Version, InstallLocation: c.InstallLocation, ScriptsDir: c.Scripts, MinOSVersion: c.MinOS, License: fallback, Licenses: licenses}
			if c.Type == "component" {
				if len(c.License) > 0 {
					return nil, fmt.Errorf("component package cannot have license")
				}
				spec.Components = []macpkg.ComponentConfig{{Created: o.clock, Root: filepath.Dir(q.App), RootEntry: filepath.Base(q.App), OutputPath: c.Out, Identifier: c.Identifier, Version: c.Version, InstallLocation: c.InstallLocation, ScriptsDir: c.Scripts, MinOSVersion: c.MinOS}}
			} else {
				spec.App = a
			}
		} else {
			ids := map[string]bool{}
			for i := range c.Components {
				v := &c.Components[i]
				if v.ID == "" || ids[v.ID] {
					return nil, fmt.Errorf("component id must be nonempty and unique: %q", v.ID)
				}
				ids[v.ID] = true
				v.Root, v.Scripts = path(v.Root), path(v.Scripts)
				if v.Version == "" {
					v.Version = "1.0"
				}
				if v.InstallLocation == "" {
					v.InstallLocation = "/"
				}
				spec.Components = append(spec.Components, macpkg.ComponentConfig{Created: o.clock, Root: v.Root, RootEntry: v.Entry, Identifier: v.ID, Version: v.Version, InstallLocation: v.InstallLocation, ScriptsDir: v.Scripts, MinOSVersion: v.MinOS})
			}
			if c.Type == "product" {
				product := &macpkg.ProductConfig{Created: o.clock, OutputPath: c.Out}
				spec.Product = product
				if d := c.Distribution; d != nil {
					d.Resources = path(d.Resources)
					d.License = path(d.License)
					if d.License != "" {
						spec.License = d.License
					}
					product.ResourcesDir = d.Resources
					product.Distribution = &macpkg.Distribution{Title: d.Title, Organization: d.Organization, MinOSVersion: d.MinOS, LicenseFile: filepath.Base(d.License)}
					if d.License == "" {
						product.Distribution.LicenseFile = ""
					}
					for _, ch := range d.Choices {
						for _, id := range ch.Packages {
							if !ids[id] {
								return nil, fmt.Errorf("choice %s references unknown package %s", ch.ID, id)
							}
						}
						product.Distribution.Choices = append(product.Distribution.Choices, macpkg.Choice{ID: ch.ID, Title: ch.Title, Description: ch.Description, PackageIDs: ch.Packages, Selected: ch.Selected, Visible: ch.Visible})
					}
				}
			}
		}
	}
	return pl, nil
}
