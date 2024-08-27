package pkg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/ironpark/zapp/mactools/pkg"
	"github.com/ironpark/zapp/mactools/plist"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:        "pkg",
	Usage:       "Create a .pkg installer for macOS",
	UsageText:   "zapp pkg <path of app-bundle>",
	Description: "Creates a .pkg installer from the specified .app bundle",
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
		info, err := plist.GetAppInfo(appFile)
		if err != nil {
			return fmt.Errorf("failed to get app info: %v", err)
		}

		s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
		s.Suffix = " Creating PKG file..."
		s.Start()

		appName := filepath.Base(appFile)
		appName = strings.TrimSuffix(appName, ".app")

		config := pkg.Config{
			AppPath:         appFile,
			OutputPath:      c.String("out"),
			Version:         c.String("version"),
			Identifier:      c.String("identifier"),
			InstallLocation: "/Applications",
			LicensePaths:    make(map[string]string),
		}

		if config.OutputPath == "" {
			config.OutputPath = appName + ".pkg"
		}
		if config.Version == "" {
			config.Version, _ = info.Version()
			if config.Version == "" {
				config.Version = "1.0"
			}
		}
		if config.Identifier == "" {
			config.Identifier, _ = info.BundleID()
			if config.Identifier == "" {
				config.Identifier = "com.example." + appName
			}
		}

		for _, eula := range c.StringSlice("eula") {
			parts := strings.SplitN(eula, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid eula format: %s", eula)
			}
			config.LicensePaths[parts[0]] = parts[1]
		}

		err = pkg.CreatePKG(config)
		s.Stop()
		if err != nil {
			return fmt.Errorf("failed to create PKG: %v", err)
		}

		fmt.Fprintln(c.App.Writer, "PKG file created successfully!")
		return nil
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "out",
			Usage:   "The output file name of the PKG file",
			Aliases: []string{"o"},
		},
		&cli.StringFlag{
			Name:    "version",
			Usage:   "The version of the package",
			Aliases: []string{"v"},
		},
		&cli.StringFlag{
			Name:    "identifier",
			Usage:   "The bundle identifier for the package",
			Aliases: []string{"id"},
		},
		&cli.StringSliceFlag{
			Name:    "license",
			Usage:   "Path to the license (EULA) file (format: lang:path, e.g., en:en_eula.txt,ko:ko_eula.txt)",
			Aliases: []string{"eula"},
		},
	},
}
