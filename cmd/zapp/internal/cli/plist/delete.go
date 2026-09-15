package plist

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/ironpark/zapp/pkg/plist"
)

var deleteCommand = &cli.Command{
	Name:      "delete",
	Aliases:   []string{"remove"},
	Usage:     "Remove a key from the plist file",
	ArgsUsage: "<path> <key>",
	Action: func(ctx context.Context, c *cli.Command) error {
		if c.NArg() < 2 {
			return fmt.Errorf("path and key are required")
		}
		key := c.Args().Get(1)
		if err := deleteValue(c.Args().First(), key); err != nil {
			return err
		}
		fmt.Printf("Removed %s\n", key)
		return nil
	},
}

func deleteValue(path, key string) error {
	doc, err := open(path)
	if err != nil {
		return err
	}
	container, name, existing, _ := plist.Resolve(doc.Root, key)
	if err := mustExist(key, existing); err != nil {
		return err
	}
	if err := plist.Delete(container, name); err != nil {
		return err
	}
	return doc.Save()
}
