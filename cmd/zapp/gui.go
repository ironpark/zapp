package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ironpark/zapp/internal/gui"
	"github.com/urfave/cli/v3"
)

func guiCommand() *cli.Command {
	return &cli.Command{
		Name: "gui", Usage: "Open the project settings and DMG layout GUI",
		Flags: []cli.Flag{&cli.StringFlag{Name: "config", Usage: "Project file (default: discover .zapp.yaml)", Sources: cli.EnvVars("ZAPP_CONFIG")}},
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() != 0 {
				return fmt.Errorf("gui accepts no positional arguments; use --config")
			}
			s, err := gui.Open(c.String("config"))
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Opening Zapp GUI:", s.Path)
			return gui.Run(ctx, s)
		},
	}
}
