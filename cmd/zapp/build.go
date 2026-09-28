package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironpark/zapp"
	"github.com/urfave/cli/v3"
)

func buildFlags() []cli.Flag {
	var out []cli.Flag
	seen := map[string]bool{}
	for _, flags := range [][]cli.Flag{dmgCommand.Flags, pkgCommand.Flags, {&cli.StringSliceFlag{Name: "libs", Aliases: []string{"l"}}}} {
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
func buildCommand() *cli.Command {
	flags := append(buildFlags(), &cli.StringFlag{Name: "artifacts", Usage: "Append the built app, DMG and PKG paths to this file as app=, dmg= and pkg= lines, e.g. $GITHUB_OUTPUT"})
	return &cli.Command{Name: "build", Usage: "Build project sections in deployment order", ArgsUsage: "[dep|dmg|pkg ...]", Flags: flags, Action: func(ctx context.Context, c *cli.Command) error {
		p, err := loadProject(c, "build")
		if err != nil {
			return err
		}
		pl, err := p.Resolve(zapp.WithLogger(newAppLogger(c.Root())))
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
		artifacts, err := pl.Build(ctx, steps...)
		if err != nil {
			return err
		}
		if file := c.String("artifacts"); file != "" {
			return writeArtifacts(file, artifacts)
		}
		return nil
	}}
}

// writeArtifacts appends the artifact paths as key=value lines, the form
// GitHub Actions reads from $GITHUB_OUTPUT. Paths are absolute so a later step
// in another directory can use them; a step that was not built is empty.
func writeArtifacts(file string, a zapp.Artifacts) error {
	var lines string
	for _, artifact := range [][2]string{{"app", a.App}, {"dmg", a.DMG}, {"pkg", a.PKG}} {
		path := artifact[1]
		if path != "" {
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			path = abs
		}
		lines += artifact[0] + "=" + path + "\n"
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(lines); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
