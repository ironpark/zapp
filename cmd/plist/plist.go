package plist

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
)

func findPlistPath(path string) (string, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("error accessing path: %v", err)
	}

	if fileInfo.IsDir() {
		if filepath.Ext(path) == ".app" {
			plistPath := filepath.Join(path, "Contents", "Info.plist")
			if _, err := os.Stat(plistPath); err == nil {
				return plistPath, nil
			}
		}
		return "", fmt.Errorf("not a valid .app directory or Info.plist not found")
	} else {
		if filepath.Ext(path) == ".plist" {
			return path, nil
		}
		return "", fmt.Errorf("not a .plist file")
	}
}

var Command = &cli.Command{
	Name:        "plist",
	Usage:       "Manage plist files",
	UsageText:   "zapp plist [command] [arguments...]",
	Description: "Perform operations on plist files",
	ArgsUsage:   "<path of .app directory> or <path of .plist file>",
	Action: func(c *cli.Context) error {
		if c.NArg() < 1 {
			return fmt.Errorf("path is required")
		}

		path := c.Args().First()
		plistPath, err := findPlistPath(path)
		if err != nil {
			return err
		}

		fmt.Printf("Using plist file: %s\n", plistPath)
		return nil
	},
	Subcommands: []*cli.Command{
		getCommand,
		setCommand,
	},
}
