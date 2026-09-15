package pkg

import (
	"context"
	"fmt"
	"github.com/ironpark/zapp/cmd"
	"github.com/ironpark/zapp/cmd/subtask"
	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/macpkg"
	"github.com/urfave/cli/v3"
	"os"
	"path/filepath"
	"strings"
)

var (
	appDir string
)
var Command = &cli.Command{
	Name:        "pkg",
	Usage:       "Create a .pkg installer for macOS",
	UsageText:   "zapp pkg --app=<path of app-bundle>",
	Description: "Creates a .pkg installer from the specified .app bundle",
	Action: func(ctx context.Context, c *cli.Command) error {
		info, err := appbundle.Open(appDir)
		if err != nil {
			return fmt.Errorf("failed to get app info: %v", err)
		}
		logger := cmd.NewAppLogger(c.Root())
		appName := filepath.Base(appDir)
		_, _ = logger.Printf("Start Creating PKG file for %s\n", appName)
		appName = strings.TrimSuffix(appName, ".app")

		config := macpkg.AppConfig{
			AppPath:    appDir,
			OutputPath: c.String("out"),
			Version:    c.String("version"),
			Identifier: c.String("identifier"),
			Licenses:   make(map[string]string),
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

		logger.PrintValue("AppPath", config.AppPath)
		logger.PrintValue("OutputPath", config.OutputPath)
		logger.PrintValue("Version", config.Version)
		logger.PrintValue("Identifier", config.Identifier)

		for _, eula := range c.StringSlice("eula") {
			lang, path, localized := strings.Cut(eula, ":")
			if !localized {
				// A bare path is the license used for every locale.
				config.License = lang
				continue
			}
			if path == "" {
				return fmt.Errorf("invalid eula arg format: %s", eula)
			}
			config.Licenses[lang] = path
		}
		if config.License == "" && len(config.Licenses) == 0 {
			_, _ = logger.Println("EULA files not found.")
		}
		err = macpkg.BuildApp(ctx, config)
		if err != nil {
			return fmt.Errorf("failed to create PKG: %v", err)
		}
		_, _ = logger.Success("PKG file created successfully!")
		logger.PrintValue("OutputPath", config.OutputPath)
		err = subtask.Sign(ctx, c, config.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to sign PKG: %v", err)
		}

		err = subtask.Notarize(ctx, c, config.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to notarize PKG: %v", err)
		}
		return nil
	},
	Flags: append([]cli.Flag{
		&cli.StringFlag{
			Name:        "app",
			Usage:       "App bundle path",
			Destination: &appDir,
			Required:    true,
			Action: func(ctx context.Context, c *cli.Command, app string) error {
				if !strings.HasSuffix(app, ".app") {
					return fmt.Errorf("not valid app bundle extension")
				}
				// Check if the app bundle path is valid
				fileInfo, err := os.Stat(app)
				if err != nil {
					return fmt.Errorf("error accessing app-bundle path: %v", err)
				}
				if !fileInfo.IsDir() {
					return fmt.Errorf("app-bundle path must be a directory")
				}
				return nil
			},
		},
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
			Usage:   "Path to the license (EULA) file, optionally per language (e.g., eula.txt or en:en_eula.txt,ko:ko_eula.txt)",
			Aliases: []string{"eula"},
		},
	}, cmd.CreateSubTaskFlags()...),
}
