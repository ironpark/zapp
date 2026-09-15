package app

import (
	"context"
	"log"
	"net/mail"
	"os"

	"github.com/ironpark/zapp/cmd/zapp/internal/cli/dep"
	"github.com/ironpark/zapp/cmd/zapp/internal/cli/dmg"
	"github.com/ironpark/zapp/cmd/zapp/internal/cli/info"
	"github.com/ironpark/zapp/cmd/zapp/internal/cli/notarize"
	"github.com/ironpark/zapp/cmd/zapp/internal/cli/pkg"
	"github.com/ironpark/zapp/cmd/zapp/internal/cli/plist"
	"github.com/ironpark/zapp/cmd/zapp/internal/cli/sign"

	"github.com/urfave/cli/v3"
)

func New() *cli.Command {
	app := &cli.Command{
		Name: "zapp",
		Commands: []*cli.Command{
			info.Command,
			dmg.Command,
			BuildCommand(), InitCommand(), ConfigCommand(),
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

	return app
}

func Run() {
	if err := New().Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
