package main

import (
	"context"
	"log"
	"net/mail"
	"os"

	"github.com/ironpark/zapp/cmd/dep"
	"github.com/ironpark/zapp/cmd/dmg"
	"github.com/ironpark/zapp/cmd/info"
	"github.com/ironpark/zapp/cmd/notarize"
	"github.com/ironpark/zapp/cmd/pkg"
	"github.com/ironpark/zapp/cmd/plist"
	"github.com/ironpark/zapp/cmd/sign"

	"github.com/urfave/cli/v3"
)

func main() {
	app := &cli.Command{
		Name: "zapp",
		Commands: []*cli.Command{
			info.Command,
			dmg.Command,
			pkg.Command,
			sign.Command,
			plist.Command,
			notarize.Command,
			dep.Command,
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

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
