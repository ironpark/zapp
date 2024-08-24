package main

import (
	"log"
	"os"

	"github.com/ironpark/zapp/cmd"

	"github.com/urfave/cli/v2"
)

var commands []*cli.Command

func main() {
	app := &cli.App{
		Commands: cmd.GetCommands(),
		Usage:    "Simplify your macOS App deployment",
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
