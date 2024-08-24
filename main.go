package main

import (
	"fmt"
	"log"
	"os"

	"zapp/cmd"

	"github.com/urfave/cli/v2"
)

var commands []*cli.Command

func main() {
	app := &cli.App{
		Commands: cmd.GetCommands(),
		Usage:    "Simplify your macOS App deployment",
		Action: func(ctx *cli.Context) error {
			fmt.Println("boom! I say!")
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
