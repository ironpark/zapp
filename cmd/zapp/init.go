package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/urfave/cli/v3"
)

func initCommand() *cli.Command {
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
