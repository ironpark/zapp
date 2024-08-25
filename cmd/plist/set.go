package plist

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"howett.net/plist"
)

var setCommand = &cli.Command{
	Name:      "set",
	Usage:     "Set a value in the plist file",
	ArgsUsage: "<path> <key> <value>",
	Action: func(c *cli.Context) error {
		if c.NArg() < 3 {
			return fmt.Errorf("path, key, and value are required")
		}

		path := c.Args().First()
		key := c.Args().Get(1)
		value := c.Args().Get(2)

		plistPath, err := findPlistPath(path)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(plistPath)
		if err != nil {
			return fmt.Errorf("failed to read plist file: %v", err)
		}

		var plistData map[string]interface{}
		_, err = plist.Unmarshal(data, &plistData)
		if err != nil {
			return fmt.Errorf("failed to parse plist: %v", err)
		}

		plistData[key] = value

		newData, err := plist.Marshal(plistData, plist.XMLFormat)
		if err != nil {
			return fmt.Errorf("failed to marshal plist: %v", err)
		}

		err = os.WriteFile(plistPath, newData, 0644)
		if err != nil {
			return fmt.Errorf("failed to write plist file: %v", err)
		}

		fmt.Printf("Value set successfully\n")
		return nil
	},
}
