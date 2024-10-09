package main

import (
	"github.com/ironpark/zapp/cmd/dep"
	"github.com/ironpark/zapp/cmd/dmg"
	"github.com/ironpark/zapp/cmd/notarize"
	"github.com/ironpark/zapp/cmd/pkg"
	"github.com/ironpark/zapp/cmd/plist"
	"github.com/ironpark/zapp/cmd/sign"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Commands: []*cli.Command{
			dmg.Command,
			pkg.Command,
			sign.Command,
			plist.Command,
			notarize.Command,
			dep.Command,
		},
		Usage: "Simplify your macOS App deployment",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() == 0 {
				return cli.ShowAppHelp(ctx)
			}
			return nil
		},
		Authors: []*cli.Author{
			{
				Name:  "Cheolwan. Park",
				Email: "cjfdhksaos@gmail.com",
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
