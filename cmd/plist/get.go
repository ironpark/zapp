package plist

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/ironpark/zapp/pkg/plist"
)

var getCommand = &cli.Command{
	Name:      "get",
	Usage:     "Get a value from the plist file",
	ArgsUsage: "<key>",
	Action: func(c *cli.Context) error {
		if c.NArg() < 1 {
			return fmt.Errorf("key is required")
		}

		if c.NArg() < 1 {
			return fmt.Errorf("path is required")
		}

		path := c.Args().First()
		plistPath, err := findPlistPath(path)
		if err != nil {
			return err
		}

		key := c.Args().Get(1)

		data, err := os.ReadFile(plistPath)
		if err != nil {
			return fmt.Errorf("failed to read plist file: %v", err)
		}

		plistData, err := plist.ParseDict(data)
		if err != nil {
			return fmt.Errorf("failed to parse plist: %v", err)
		}

		value, ok := plistData[key]
		if !ok {
			return fmt.Errorf("key not found: %s", key)
		}

		fmt.Printf("%v\n", value)
		return nil
	},
}
