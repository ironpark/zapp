package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ironpark/zapp/mactools/dmg"

	"github.com/briandowns/spinner"
	"github.com/urfave/cli/v2"
)

func init() {
	commands = append(commands, &cli.Command{
		Name:        "dmg",
		Usage:       "",
		UsageText:   "",
		Description: "",
		Args:        true,
		ArgsUsage:   " <path of app-bundle>",
		Action: func(c *cli.Context) error {
			appFile := c.Args().First()
			if appFile == "" {
				return fmt.Errorf("target app-bundle is required")
			}
			if !strings.HasSuffix(appFile, ".app") {
				return fmt.Errorf("not valid app bundle extension")
			}
			fileInfo, err := os.Stat(appFile)
			if err != nil {
				return fmt.Errorf("error accessing app-bundle path: %v", err)
			}
			if !fileInfo.IsDir() {
				return fmt.Errorf("app-bundle path must be a directory")
			}

			fmt.Fprint(c.App.Writer, "Creating DMG file...")
			title := filepath.Base(appFile)
			if c.String("title") != "" {
				title = c.String("title")
			}
			fmt.Fprint(c.App.Writer, "Creating DMG file2...")
			defaultConfig := dmg.Config{
				FileName:         c.String("out"),
				Title:            title,
				Icon:             "",
				LabelSize:        30,
				ContentsIconSize: 100,
				WindowWidth:      640,
				WindowHeight:     480,
				Background:       "",
				Contents:         []dmg.Item{{X: int(float64(640) / 5 * 1), Y: 480 / 2, Type: dmg.Dir, Path: appFile}, {X: int(float64(640) / 5 * 3), Y: 480 / 2, Type: dmg.Link, Path: "/Applications"}},
			}
			tempDir, err := os.MkdirTemp("", "zapp-dmg")
			if err != nil {
				return fmt.Errorf("error creating temporary directory: %v", err)
			}
			defer os.RemoveAll(tempDir)
			s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
			s.Suffix = " Creating DMG file..."

			s.Start()
			err = dmg.CreateDMG(defaultConfig, tempDir)
			s.Stop()
			if err != nil {
				return err
			}
			fmt.Fprintf(c.App.Writer, "DMG file created at %s\n", defaultConfig.FileName)
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "title",
				Usage:   "The title name displayed when a DMG file is mounted",
				Aliases: []string{"t"},
			},
			&cli.StringFlag{
				Name:    "out",
				Usage:   "The output file name of the DMG file",
				Aliases: []string{"o"},
			},
		},
		HelpName:           "",
		CustomHelpTemplate: "",
	})
}
