package plist

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/ironpark/zapp/pkg/plist"
)

var getCommand = &cli.Command{
	Name:      "get",
	Usage:     "Get a value from the plist file",
	ArgsUsage: "<path> <key>",
	Action: func(ctx context.Context, c *cli.Command) error {
		if c.NArg() < 2 {
			return fmt.Errorf("path and key are required")
		}
		key := c.Args().Get(1)
		doc, err := open(c.Args().First())
		if err != nil {
			return err
		}
		_, _, value, found := plist.Resolve(doc.Root, key)
		if !found {
			return fmt.Errorf("key not found: %s", key)
		}
		text, err := render(value)
		if err != nil {
			return err
		}
		fmt.Println(text)
		return nil
	},
}

// render prints a scalar bare, so it can be captured by a shell, and a
// dictionary or array as a property list rather than as Go's map syntax, which
// nothing can read back.
func render(value any) (string, error) {
	if text, ok := plist.Format(value); ok {
		return text, nil
	}
	xml, err := plist.MarshalXML(value)
	if err != nil {
		return "", err
	}
	return string(xml), nil
}
