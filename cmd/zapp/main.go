package main

import (
	"context"
	"log"
	"net/mail"
	"os"

	"github.com/urfave/cli/v3"
)

func newApp() *cli.Command {
	app := &cli.Command{
		Name: "zapp",
		Commands: []*cli.Command{
			infoCommand,
			dmgCommand,
			buildCommand(), initCommand(), configCommand(),
			pkgCommand,
			signCommand,
			plistCommand,
			notarizeCommand,
			depCommand,
		},
		Usage: "Simplify your macOS App deployment",
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return cli.ShowAppHelp(c)
			}
			return nil
		},
		Authors: []any{
			&mail.Address{Name: "Cheolwan. Park", Address: "cjfdhksaos@gmail.com"},
		},
	}

	return app
}

func main() {
	if err := newApp().Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
