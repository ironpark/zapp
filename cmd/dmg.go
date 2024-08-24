package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"zapp/mactools/dmg"

	"github.com/urfave/cli/v2"
)

func init() {
	commands = append(commands, &cli.Command{
		Name:        "dmg",
		Usage:       "",
		UsageText:   "",
		Description: "",
		Args:        true,
		ArgsUsage:   " <path of app>",
		Action: func(c *cli.Context) error {
			appFile := c.Args().First()
			if appFile == "" {
				return fmt.Errorf("target app bundle is required")
			}
			if !strings.HasSuffix(appFile, ".app") {
				return fmt.Errorf("not valid app bundle extension")
			}
			fileInfo, err := os.Stat(appFile)
			if err != nil {
				return fmt.Errorf("error accessing app path: %v", err)
			}
			if !fileInfo.IsDir() {
				return fmt.Errorf("app bundle path must be a directory")
			}
			fmt.Fprint(c.App.Writer, "Creating DMG file...")
			defaultConfig := dmg.Config{
				Title:            filepath.Base(appFile),
				LabelSize:        30,
				ContentsIconSize: 100,
				WindowWidth:      640,
				WindowHeight:     480,
				Contents: []dmg.Item{
					{X: int(float64(640) / 5 * 1), Y: 480 / 2, Type: dmg.Dir, Path: appFile},
					{X: int(float64(640) / 5 * 3), Y: 480 / 2, Type: dmg.Link, Path: "/Applications"},
				},
			}
			tempDir, err := os.MkdirTemp("", "zapp-dmg")
			if err != nil {
				return fmt.Errorf("error creating temporary directory: %v", err)
			}
			defer os.RemoveAll(tempDir)
			return dmg.CreateDMG(defaultConfig, tempDir)
		},
		Flags:              nil,
		SkipFlagParsing:    true,
		HelpName:           "",
		CustomHelpTemplate: "",
	})
}
