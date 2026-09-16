package main

import (
	"context"

	"github.com/ironpark/zapp"
	"github.com/urfave/cli/v3"
)

var depCommand = &cli.Command{Name: "dep", Usage: "Bundle app dependencies", Action: func(ctx context.Context, c *cli.Command) error {
	p, err := loadProject(c, "dep")
	if err != nil {
		return err
	}
	pl, err := p.Resolve(zapp.WithLogger(newAppLogger(c.Root())))
	if err != nil {
		return err
	}
	target := pl.App
	err = pl.BundleDeps(ctx)
	if err != nil {
		return err
	}
	if pl.SignCredentials != nil {
		if err = pl.Sign(ctx, target); err != nil {
			return err
		}
	}
	if pl.NotarizeCredentials != nil {
		return pl.Notarize(ctx, target)
	}
	return nil
},
	Flags: append([]cli.Flag{
		&cli.StringFlag{Name: "app", Usage: "App bundle path"},
		&cli.StringSliceFlag{
			Name:    "libs",
			Usage:   "Path to the directory containing the libraries",
			Aliases: []string{"l"},
			//Destination: &libPaths,
		},
	}, append(subTaskFlags(), projectFileFlags()...)...),
}
