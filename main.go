package main

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"log"
	"os"
)

var commands []*cli.Command

func main() {
	app := &cli.App{
		Commands: commands,
		Name:     "boom",
		Usage:    "make an explosive entrance",
		Action: func(*cli.Context) error {
			fmt.Println("boom! I say!")
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
