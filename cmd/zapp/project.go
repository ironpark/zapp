package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/urfave/cli/v3"
)

func projectFileFlags() []cli.Flag {
	return []cli.Flag{&cli.StringFlag{Name: "config", Usage: "Project YAML/JSON file (default: discover .zapp.yaml)"}, &cli.BoolFlag{Name: "no-config", Usage: "Ignore project files"}, &cli.BoolFlag{Name: "no-sign", Usage: "Skip signing"}, &cli.BoolFlag{Name: "no-notarize", Usage: "Skip notarization"}}
}

// target names the field a flag, or its environment variable, writes to. These
// are slices rather than maps so that a project with two bad flags always
// reports the same one.
type target struct {
	flag string
	dst  *string
}

// envName is the environment variable a flag falls back to: --window-width
// reads ZAPP_WINDOW_WIDTH.
func envName(flag string) string {
	return "ZAPP_" + strings.ToUpper(strings.ReplaceAll(flag, "-", "_"))
}

// parsePosition reads an icon centre written as "x,y".
func parsePosition(value string) (*[2]int, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected x,y")
	}
	x, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	y, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil || x < 0 || y < 0 {
		return nil, fmt.Errorf("expected nonnegative integer coordinates x,y")
	}
	return &[2]int{x, y}, nil
}

// signAndNotarize finishes an artifact a command just produced. The build
// command runs the same steps through Plan.Build, which drives them for every
// artifact it makes rather than for one.
func signAndNotarize(ctx context.Context, pl *zapp.Plan, target string) error {
	if pl.SignCredentials != nil {
		if err := pl.Sign(ctx, target); err != nil {
			return err
		}
	}
	if pl.NotarizeCredentials != nil {
		return pl.Notarize(ctx, target)
	}
	return nil
}

func loadProject(c *cli.Command, kind string) (*zapp.Project, error) {
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
	// A section the command is about to write to needs somewhere to write, so
	// the overlay has a configuration to fill in even when the file omits it.
	// "build" and "show" name no section of their own and fall through.
	ensure := func(section string) {
		switch section {
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
	}
	if kind == "build" {
		for _, step := range c.Args().Slice() {
			if step != "dmg" && step != "pkg" && step != "dep" {
				return nil, fmt.Errorf("unknown build step %q", step)
			}
			ensure(step)
		}
	}
	ensure(kind)
	if err := overlayProject(c, p, kind); err != nil {
		return nil, err
	}
	return p, nil
}

// overlayProject copies explicitly supplied values only. Env is applied first, then
// command line flags, with both path sources interpreted relative to cwd.
func overlayProject(c *cli.Command, p *zapp.Project, kind string) error {
	value := func(flag string) (string, bool) {
		if c.IsSet(flag) {
			return c.String(flag), true
		}
		v, ok := os.LookupEnv(envName(flag))
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
		v, ok := os.LookupEnv(envName(flag))
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
		for _, t := range []target{{"title", &d.Title}, {"fs", &d.FS}, {"format", &d.Format}} {
			if err := assign(t.flag, t.dst, false); err != nil {
				return err
			}
		}
		paths := []target{{"icon", &d.Icon}, {"background", &d.Background}}
		if kind == "dmg" {
			paths = append(paths, target{"out", &d.Out})
		}
		for _, t := range paths {
			if err := assign(t.flag, t.dst, true); err != nil {
				return err
			}
		}
		for _, t := range []struct {
			flag string
			dst  *int
		}{{"window-width", &d.Window.Width}, {"window-height", &d.Window.Height}, {"contents-icon-size", &d.IconSize}, {"label-size", &d.LabelSize}} {
			var v string
			var ok bool
			if c.IsSet(t.flag) {
				v, ok = strconv.Itoa(c.Int(t.flag)), true
			} else {
				v, ok = os.LookupEnv(envName(t.flag))
			}
			if ok {
				n, err := strconv.Atoi(v)
				if err != nil || n <= 0 {
					return fmt.Errorf("%s must be a positive integer", t.flag)
				}
				*t.dst = n
			}
		}
		for _, t := range []struct {
			flag string
			dst  **[2]int
		}{{"app-position", &d.AppPosition}, {"applications-position", &d.ApplicationsPosition}} {
			v, ok := value(t.flag)
			if !ok {
				continue
			}
			pos, err := parsePosition(v)
			if err != nil {
				return fmt.Errorf("--%s: %w", t.flag, err)
			}
			*t.dst = pos
		}
		if b, ok, err := boolean("use-original-icon"); err != nil {
			return err
		} else if ok {
			d.UseOriginalIcon = b
		}
	}
	if k := p.PKG; k != nil {
		for _, t := range []target{{"identifier", &k.Identifier}, {"version", &k.Version}, {"install-location", &k.InstallLocation}, {"min-os", &k.MinOS}, {"type", &k.Type}} {
			if err := assign(t.flag, t.dst, false); err != nil {
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
		} else if v, ok := os.LookupEnv(envName("license")); ok {
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
		} else if v, ok := os.LookupEnv(envName("libs")); ok {
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
	// --sign and --notarize turn a step on by giving it an empty configuration
	// for the flags below to fill in, and off by taking the section away.
	if b, ok, err := boolean("sign"); err != nil {
		return err
	} else if ok {
		if !b {
			p.Sign = nil
		} else if p.Sign == nil {
			p.Sign = &zapp.SignConfig{}
		}
	}
	if b, ok, err := boolean("notarize"); err != nil {
		return err
	} else if ok {
		if !b {
			p.Notarize = nil
		} else if p.Notarize == nil {
			p.Notarize = &zapp.NotarizeConfig{}
		}
	}
	if s := p.Sign; s != nil {
		for _, t := range []target{{"identity", &s.Identity}, {"p12-password", &s.P12Password}} {
			if err := assign(t.flag, t.dst, false); err != nil {
				return err
			}
		}
		for _, t := range []target{{"p12-file", &s.P12File}, {"pem-file", &s.PEMFile}, {"p12-password-file", &s.P12PasswordFile}} {
			if err := assign(t.flag, t.dst, true); err != nil {
				return err
			}
		}
	}
	if n := p.Notarize; n != nil {
		for _, t := range []target{{"profile", &n.Profile}, {"apple-id", &n.AppleID}, {"team-id", &n.TeamID}, {"password", &n.Password}} {
			if err := assign(t.flag, t.dst, false); err != nil {
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
