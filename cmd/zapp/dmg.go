package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/urfave/cli/v3"
)

var dmgCommand = &cli.Command{Name: "dmg", Usage: "Create a DMG disk image", Action: func(ctx context.Context, c *cli.Command) error {
	p, err := loadProject(c, "dmg")
	if err != nil {
		return err
	}
	pl, err := p.Resolve(zapp.WithLogger(newAppLogger(c.Root())))
	if err != nil {
		return err
	}
	target, err := pl.BuildDMG(ctx)
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

		&cli.StringFlag{Name: "app-position", Usage: "App icon center in content coordinates: x,y"},
		&cli.StringFlag{Name: "applications-position", Usage: "Applications icon center in content coordinates: x,y"},
		&cli.StringFlag{
			Name:  "fs",
			Usage: "Volume filesystem: hfsplus, apfs, or apfs-case-sensitive (APFS needs macOS 10.13 or later)",
			Value: "hfsplus",
		},
		&cli.StringFlag{
			Name:  "format",
			Usage: "Compression of the disk image: udzo (zlib, read by every macOS) or ulfo (lzfse, smaller, needs macOS 10.11)",
			Value: "udzo",
		},
		&cli.StringFlag{
			Name:    "background",
			Usage:   "Path to the background image file",
			Aliases: []string{"bg"},
		},
		&cli.StringFlag{
			Name:    "title",
			Usage:   "The title displayed when the DMG file is mounted",
			Aliases: []string{"t"},
		},
		&cli.StringFlag{
			Name:  "app",
			Usage: "App bundle path",
		},
		&cli.StringFlag{
			Name:    "out",
			Usage:   "The output DMG file name",
			Aliases: []string{"o"},
		},
		&cli.StringFlag{
			Name:  "icon",
			Usage: "Path to the icon file to display in the DMG file (icns, png)",
		},
		&cli.IntFlag{
			Name:    "window-width",
			Usage:   "Width of the Finder window when the DMG file is opened",
			Aliases: []string{"ww"},
			Value:   640,
		},
		&cli.IntFlag{
			Name:    "window-height",
			Usage:   "Height of the Finder window when the DMG file is opened",
			Aliases: []string{"wh"},
			Value:   480,
		},
		&cli.IntFlag{
			Name:    "label-size",
			Usage:   "Size of the label text in the Finder window (10-16)",
			Aliases: []string{"ls"},
			Value:   14,
		},
		&cli.IntFlag{
			Name:    "contents-icon-size",
			Usage:   "Size of the icons in the Finder window (16-512)",
			Aliases: []string{"cis"},
			Value:   128,
		},
		&cli.BoolFlag{
			Name:    "use-original-icon",
			Aliases: []string{"uoi"},
			Usage:   "Use the original icon file without modifications.",
		},
	}, append(subTaskFlags(), projectFileFlags()...)...),
}

type position struct{ X, Y int }

func resolveConfig(c *cli.Command) (dmg.Config, string, error) {
	p, err := loadProject(c, "dmg")
	if err != nil {
		return dmg.Config{}, "", err
	}
	pl, err := p.Resolve()
	if err != nil {
		return dmg.Config{}, "", err
	}
	return *pl.DMG, pl.App, nil
}
func parsePosition(value string) (position, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return position{}, fmt.Errorf("expected x,y")
	}
	x, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	y, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil || x < 0 || y < 0 {
		return position{}, fmt.Errorf("expected nonnegative integer coordinates x,y")
	}
	return position{x, y}, nil
}
