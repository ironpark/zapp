package main

import (
	"context"
	"fmt"

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
	return &cli.Command{Name: "build", Usage: "Build project sections in deployment order", ArgsUsage: "[dep|dmg|pkg ...]", Flags: buildFlags(), Action: func(ctx context.Context, c *cli.Command) error {
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
		_, err = pl.Build(ctx, steps...)
		return err
	}}
}
