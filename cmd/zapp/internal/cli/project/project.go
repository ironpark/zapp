package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/urfave/cli/v3"
)

func Flags() []cli.Flag {
	return []cli.Flag{&cli.StringFlag{Name: "config", Usage: "Project YAML/JSON file (default: discover .zapp.yaml)"}, &cli.BoolFlag{Name: "no-config", Usage: "Ignore project files"}, &cli.BoolFlag{Name: "no-sign", Usage: "Skip signing"}, &cli.BoolFlag{Name: "no-notarize", Usage: "Skip notarization"}}
}
func Load(c *cli.Command, kind string) (*zapp.Project, error) {
	if c.Bool("no-config") && c.IsSet("config") {
		return nil, fmt.Errorf("--config and --no-config are mutually exclusive")
	}
	p := &zapp.Project{Version: 1}
	name := c.String("config")
	if !c.IsSet("config") {
		name = os.Getenv("ZAPP_CONFIG")
	}
	if !c.Bool("no-config") {
		if name == "" {
			var err error
			name, err = zapp.Discover(".")
			if err != nil && !errors.Is(err, zapp.ErrNotFound) {
				return nil, err
			}
		}
		if name != "" {
			var err error
			p, err = zapp.Load(name)
			if err != nil {
				return nil, err
			}
			if p.Legacy() {
				if kind != "dmg" {
					return nil, fmt.Errorf("legacy flat configuration is supported only by dmg")
				}
				fmt.Fprintln(c.Root().ErrWriter, "Warning: legacy flat DMG configuration is deprecated; migrate to .zapp.yaml with a dmg section")
			}
		}
	}
	if kind == "build" {
		for _, step := range c.Args().Slice() {
			switch step {
			case "dmg":
				if p.DMG == nil {
					p.DMG = &zapp.DMGConfig{}
				}
			case "pkg":
				if p.PKG == nil {
					p.PKG = &zapp.PKGConfig{}
				}
			case "dep":
				if p.Dep == nil {
					p.Dep = &zapp.DepConfig{}
				}
			default:
				return nil, fmt.Errorf("unknown build step %q", step)
			}
		}
	}
	switch kind {
	case "dmg":
		if p.DMG == nil {
			p.DMG = &zapp.DMGConfig{}
		}
	case "pkg":
		if p.PKG == nil {
			p.PKG = &zapp.PKGConfig{}
		}
	case "dep":
		if p.Dep == nil {
			p.Dep = &zapp.DepConfig{}
		}
	}
	if err := Overlay(c, p, kind); err != nil {
		return nil, err
	}
	return p, nil
}

// Overlay copies explicitly supplied values only. Env is applied first, then
// command line flags, with both path sources interpreted relative to cwd.
func Overlay(c *cli.Command, p *zapp.Project, kind string) error {
	value := func(flag string) (string, bool) {
		if c.IsSet(flag) {
			return c.String(flag), true
		}
		v, ok := os.LookupEnv("ZAPP_" + strings.ToUpper(strings.ReplaceAll(flag, "-", "_")))
		return v, ok
	}
	absolute := func(s string) (string, error) {
		if s == "" || filepath.IsAbs(s) {
			return s, nil
		}
		return filepath.Abs(s)
	}
	assign := func(flag string, dst *string, isPath bool) error {
		v, ok := value(flag)
		if !ok {
			return nil
		}
		if isPath {
			var err error
			v, err = absolute(v)
			if err != nil {
				return err
			}
		}
		*dst = v
		return nil
	}
	boolean := func(flag string) (bool, bool, error) {
		if c.IsSet(flag) {
			return c.Bool(flag), true, nil
		}
		v, ok := os.LookupEnv("ZAPP_" + strings.ToUpper(strings.ReplaceAll(flag, "-", "_")))
		if !ok {
			return false, false, nil
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, true, fmt.Errorf("%s: %w", flag, err)
		}
		return b, true, nil
	}
	if err := assign("app", &p.App, true); err != nil {
		return err
	}
	if kind == "build" || kind == "show" {
		if err := assign("out", &p.Out, true); err != nil {
			return err
		}
	}
	if d := p.DMG; d != nil {
		for flag, dst := range map[string]*string{"title": &d.Title, "fs": &d.FS, "format": &d.Format} {
			if err := assign(flag, dst, false); err != nil {
				return err
			}
		}
		paths := map[string]*string{"icon": &d.Icon, "background": &d.Background}
		if kind == "dmg" {
			paths["out"] = &d.Out
		}
		for flag, dst := range paths {
			if err := assign(flag, dst, true); err != nil {
				return err
			}
		}
		for flag, dst := range map[string]*int{"window-width": &d.Window.Width, "window-height": &d.Window.Height, "contents-icon-size": &d.IconSize, "label-size": &d.LabelSize} {
			var v string
			var ok bool
			if c.IsSet(flag) {
				v, ok = strconv.Itoa(c.Int(flag)), true
			} else {
				v, ok = os.LookupEnv("ZAPP_" + strings.ToUpper(strings.ReplaceAll(flag, "-", "_")))
			}
			if ok {
				n, err := strconv.Atoi(v)
				if err != nil || n <= 0 {
					return fmt.Errorf("%s must be a positive integer", flag)
				}
				*dst = n
			}
		}
		for flag, dst := range map[string]**[2]int{"app-position": &d.AppPosition, "applications-position": &d.ApplicationsPosition} {
			if v, ok := value(flag); ok {
				parts := strings.Split(v, ",")
				if len(parts) != 2 {
					return fmt.Errorf("--%s: expected x,y", flag)
				}
				x, e := strconv.Atoi(strings.TrimSpace(parts[0]))
				y, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
				if e != nil || e2 != nil || x < 0 || y < 0 {
					return fmt.Errorf("--%s: expected nonnegative integer coordinates x,y", flag)
				}
				*dst = &[2]int{x, y}
			}
		}
		if b, ok, err := boolean("use-original-icon"); err != nil {
			return err
		} else if ok {
			d.UseOriginalIcon = b
		}
	}
	if k := p.PKG; k != nil {
		for flag, dst := range map[string]*string{"identifier": &k.Identifier, "version": &k.Version, "install-location": &k.InstallLocation, "min-os": &k.MinOS, "type": &k.Type} {
			if err := assign(flag, dst, false); err != nil {
				return err
			}
		}
		if err := assign("scripts", &k.Scripts, true); err != nil {
			return err
		}
		if kind == "pkg" {
			if err := assign("out", &k.Out, true); err != nil {
				return err
			}
		}
		var vals []string
		set := false
		if c.IsSet("license") {
			vals, set = c.StringSlice("license"), true
		} else if v, ok := os.LookupEnv("ZAPP_LICENSE"); ok {
			vals, set = strings.Split(v, ","), true
		}
		if set {
			k.License = zapp.License{}
			for _, v := range vals {
				lang, path, localized := strings.Cut(v, ":")
				if !localized {
					path, lang = lang, "default"
				}
				if path == "" {
					return fmt.Errorf("invalid eula arg format: %s", v)
				}
				a, err := absolute(path)
				if err != nil {
					return err
				}
				k.License[lang] = a
			}
		}
	}
	if d := p.Dep; d != nil {
		var libs []string
		set := false
		if c.IsSet("libs") {
			libs, set = c.StringSlice("libs"), true
		} else if v, ok := os.LookupEnv("ZAPP_LIBS"); ok {
			libs, set = strings.Split(v, ","), true
		}
		if set {
			d.Libs = nil
			for _, lib := range libs {
				v, err := absolute(lib)
				if err != nil {
					return err
				}
				d.Libs = append(d.Libs, v)
			}
		}
	}
	for _, section := range []string{"sign", "notarize"} {
		b, ok, err := boolean(section)
		if err != nil {
			return err
		}
		if ok {
			if section == "sign" {
				if b {
					if p.Sign == nil {
						p.Sign = &zapp.SignConfig{}
					}
				} else {
					p.Sign = nil
				}
			} else {
				if b {
					if p.Notarize == nil {
						p.Notarize = &zapp.NotarizeConfig{}
					}
				} else {
					p.Notarize = nil
				}
			}
		}
	}
	if s := p.Sign; s != nil {
		for flag, dst := range map[string]*string{"identity": &s.Identity, "p12-password": &s.P12Password} {
			if err := assign(flag, dst, false); err != nil {
				return err
			}
		}
		for flag, dst := range map[string]*string{"p12-file": &s.P12File, "pem-file": &s.PEMFile, "p12-password-file": &s.P12PasswordFile} {
			if err := assign(flag, dst, true); err != nil {
				return err
			}
		}
	}
	if n := p.Notarize; n != nil {
		for flag, dst := range map[string]*string{"profile": &n.Profile, "apple-id": &n.AppleID, "team-id": &n.TeamID, "password": &n.Password} {
			if err := assign(flag, dst, false); err != nil {
				return err
			}
		}
		if err := assign("api-key-file", &n.APIKeyFile, true); err != nil {
			return err
		}
		if b, ok, err := boolean("staple"); err != nil {
			return err
		} else if ok {
			n.Staple = b
		}
	}
	if b, _, err := boolean("no-sign"); err != nil {
		return err
	} else if b {
		p.Sign = nil
	}
	if b, _, err := boolean("no-notarize"); err != nil {
		return err
	} else if b {
		p.Notarize = nil
	}
	return nil
}
