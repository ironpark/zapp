package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironpark/zapp"
	cmd "github.com/ironpark/zapp/cmd/zapp/internal/cli"
	"github.com/ironpark/zapp/cmd/zapp/internal/cli/dmg"
	pkgcmd "github.com/ironpark/zapp/cmd/zapp/internal/cli/pkg"
	"github.com/ironpark/zapp/cmd/zapp/internal/cli/project"
	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/urfave/cli/v3"
)

func projectFlags() []cli.Flag {
	var out []cli.Flag
	seen := map[string]bool{}
	for _, flags := range [][]cli.Flag{dmg.Command.Flags, pkgcmd.Command.Flags, {&cli.StringSliceFlag{Name: "libs", Aliases: []string{"l"}}}} {
		for _, f := range flags {
			if !seen[f.Names()[0]] {
				seen[f.Names()[0]] = true
				// Each command owns independent parsing state.
				switch v := f.(type) {
				case *cli.StringFlag:
					x := *v
					out = append(out, &x)
				case *cli.IntFlag:
					x := *v
					out = append(out, &x)
				case *cli.BoolFlag:
					x := *v
					out = append(out, &x)
				case *cli.StringSliceFlag:
					x := *v
					out = append(out, &x)
				}
			}
		}
	}
	return out
}
func BuildCommand() *cli.Command {
	return &cli.Command{Name: "build", Usage: "Build project sections in deployment order", ArgsUsage: "[dep|dmg|pkg ...]", Flags: projectFlags(), Action: func(ctx context.Context, c *cli.Command) error {
		p, err := project.Load(c, "build")
		if err != nil {
			return err
		}
		pl, err := p.Resolve(zapp.WithLogger(cmd.NewAppLogger(c.Root())))
		if err != nil {
			return err
		}
		if pl.Dep == nil && pl.DMG == nil && pl.PKG == nil {
			return fmt.Errorf("build requires a project with dep, dmg, or pkg sections")
		}
		var steps []zapp.Step
		for _, arg := range c.Args().Slice() {
			steps = append(steps, zapp.Step(arg))
		}
		_, err = pl.Build(ctx, steps...)
		return err
	}}
}
func ConfigCommand() *cli.Command {
	return &cli.Command{Name: "config", Usage: "Inspect project configuration", Commands: []*cli.Command{{Name: "show", Usage: "Print resolved configuration with secrets redacted", Flags: projectFlags(), Action: func(ctx context.Context, c *cli.Command) error {
		p, err := project.Load(c, "show")
		if err != nil {
			return err
		}
		pl, err := p.Resolve()
		if err != nil {
			return err
		}
		data, err := pl.YAML()
		if err != nil {
			return err
		}
		_, err = c.Root().Writer.Write(data)
		return err
	}}}}
}
func InitCommand() *cli.Command {
	return &cli.Command{Name: "init", Usage: "Create a starter .zapp.yaml from an app", Flags: []cli.Flag{&cli.StringFlag{Name: "app", Required: true}, &cli.BoolFlag{Name: "force"}}, Action: func(ctx context.Context, c *cli.Command) error {
		app := c.String("app")
		info, err := appbundle.Open(app)
		if err != nil {
			return err
		}
		version, _ := info.Version()
		id, _ := info.BundleID()
		if version == "" {
			version = "1.0"
		}
		p := &zapp.Project{Version: 1, App: filepath.Clean(app), Out: "dist", DMG: &zapp.DMGConfig{}, PKG: &zapp.PKGConfig{Identifier: id, Version: version}}
		pl, err := p.Resolve()
		if err != nil {
			return err
		}
		p.DMG.Title = pl.DMG.Title
		data, err := p.YAML()
		if err != nil {
			return err
		}
		flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
		if c.Bool("force") {
			flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		}
		f, err := os.OpenFile(".zapp.yaml", flags, 0644)
		if err != nil {
			return fmt.Errorf("create .zapp.yaml (use --force to overwrite): %w", err)
		}
		_, err = f.Write(data)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		return closeErr
	}}
}
