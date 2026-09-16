package main

import (
	"context"

	"github.com/ironpark/zapp"
	"github.com/urfave/cli/v3"
)

var pkgCommand = &cli.Command{Name: "pkg", Usage: "Create a PKG installer", Action: func(ctx context.Context, c *cli.Command) error {
	p, err := loadProject(c, "pkg")
	if err != nil {
		return err
	}
	pl, err := p.Resolve(zapp.WithLogger(newAppLogger(c.Root())))
	if err != nil {
		return err
	}
	target, err := pl.BuildPKG(ctx)
	if err != nil {
		return err
	}
	return signAndNotarize(ctx, pl, target)
},
	Flags: append([]cli.Flag{
		&cli.StringFlag{Name: "install-location"}, &cli.StringFlag{Name: "scripts"}, &cli.StringFlag{Name: "min-os"}, &cli.StringFlag{Name: "type"},
		&cli.StringFlag{Name: "app", Usage: "App bundle path"},
		&cli.StringFlag{
			Name:    "out",
			Usage:   "The output file name of the PKG file",
			Aliases: []string{"o"},
		},
		&cli.StringFlag{
			Name:    "version",
			Usage:   "The version of the package",
			Aliases: []string{"v"},
		},
		&cli.StringFlag{
			Name:    "identifier",
			Usage:   "The bundle identifier for the package",
			Aliases: []string{"id"},
		},
		&cli.StringSliceFlag{
			Name:    "license",
			Usage:   "Path to the license (EULA) file, optionally per language (e.g., eula.txt or en:en_eula.txt,ko:ko_eula.txt)",
			Aliases: []string{"eula"},
		},
	}, append(subTaskFlags(), projectFileFlags()...)...),
}
