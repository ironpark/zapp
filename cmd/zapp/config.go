package main

import (
	"context"

	"github.com/urfave/cli/v3"
)

func configCommand() *cli.Command {
	return &cli.Command{Name: "config", Usage: "Inspect project configuration", Commands: []*cli.Command{{Name: "show", Usage: "Print resolved configuration with secrets redacted", Flags: buildFlags(), Action: func(ctx context.Context, c *cli.Command) error {
		p, err := loadProject(c, "show")
		if err != nil {
			return err
		}
		pl, err := p.Resolve()
		if err != nil {
			return err
		}
		data, err := pl.YAML()
		if err != nil {
			return err
		}
		_, err = c.Root().Writer.Write(data)
		return err
	}}}}
}
