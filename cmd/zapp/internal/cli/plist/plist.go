package plist

import (
	"context"
	"fmt"

	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/urfave/cli/v3"
)

var Command = &cli.Command{
	Name:        "plist",
	Usage:       "Manage plist files",
	UsageText:   "zapp plist [command] [arguments...]",
	Description: "Perform operations on plist files",
	ArgsUsage:   "<path of .app directory> or <path of .plist file>",
	Action: func(ctx context.Context, c *cli.Command) error {
		if c.NArg() < 1 {
			return fmt.Errorf("path is required")
		}

		path := c.Args().First()
		plistPath, err := appbundle.FindPlistPath(path)
		if err != nil {
			return err
		}

		fmt.Printf("Using plist file: %s\n", plistPath)
		return nil
	},
	Commands: []*cli.Command{
		getCommand,
		setCommand,
		deleteCommand,
		bumpCommand,
	},
}
