package plist

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/ironpark/zapp/pkg/plist"
)

var setCommand = &cli.Command{
	Name:      "set",
	Usage:     "Set a value in the plist file",
	ArgsUsage: "<path> <key> <value>",
	Description: "An existing key keeps the type it already has, so setting a boolean " +
		"to false writes <false/> rather than the string \"false\". A key that does " +
		"not exist yet becomes a boolean for true or false, an integer for a whole " +
		"number, and a string otherwise.",
	Action: func(ctx context.Context, c *cli.Command) error {
		if c.NArg() < 3 {
			return fmt.Errorf("path, key, and value are required")
		}
		if err := setValue(c.Args().First(), c.Args().Get(1), c.Args().Get(2)); err != nil {
			return err
		}
		fmt.Printf("Value set successfully\n")
		return nil
	},
}

func setValue(path, key, literal string) error {
	return edit(path, key, func(existing any) (any, error) {
		return plist.Coerce(existing, literal)
	})
}
