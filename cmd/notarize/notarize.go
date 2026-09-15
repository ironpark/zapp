package notarize

import (
	"context"
	"fmt"
	"github.com/ironpark/zapp/cmd"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/pkg/signing"
	"github.com/urfave/cli/v3"
)

var Command = &cli.Command{
	Name:  "notarize",
	Usage: "Notarization & Stapling for macOS app/dmg/pkg",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "target",
			Aliases:  []string{"app", "dmg", "pkg"},
			Usage:    "Path to the target(app,dmg,pkg) file",
			Required: true,
			Action: func(ctx context.Context, c *cli.Command, target string) error {
				ext := strings.ToLower(filepath.Ext(target))
				switch ext {
				case ".app", ".dmg", ".pkg":
				default:
					return fmt.Errorf("unsupported file type")
				}
				// Check if the app bundle path is valid
				fileInfo, err := os.Stat(target)
				if err != nil {
					return fmt.Errorf("error accessing target: %v", err)
				}
				if ext == ".app" {
					if !fileInfo.IsDir() {
						return fmt.Errorf("app-bundle path must be a directory")
					}
				} else {
					if fileInfo.IsDir() {
						return fmt.Errorf("dmg/pkg is must be a file")
					}
				}
				return nil
			},
		},
		&cli.StringFlag{
			Name:    "profile",
			Aliases: []string{"p"},
			Usage:   "Keychain profile name",
		},
		&cli.StringFlag{
			Name:  "apple-id",
			Usage: "Apple ID email",
		},
		&cli.StringFlag{
			Name:  "password",
			Usage: "Apple ID password or app-specific password",
		},
		&cli.StringFlag{
			Name:  "team-id",
			Usage: "Developer Team ID",
		},
		&cli.BoolFlag{
			Name:  "staple",
			Usage: "Perform stapling after notarization",
		},
		cmd.NotaryKeyFlag(),
	},
	Action: action,
}

func action(ctx context.Context, c *cli.Command) error {
	return Run(ctx, cmd.NewAppLogger(c.Root()), c.String("target"), Credentials(c), c.Bool("staple"))
}

// Credentials returns the notarization credentials a command's flags describe.
func Credentials(c *cli.Command) signing.Credentials {
	return signing.Credentials{
		Profile:    c.String("profile"),
		AppleID:    c.String("apple-id"),
		Password:   c.String("password"),
		TeamID:     c.String("team-id"),
		APIKeyFile: c.String("api-key-file"),
	}
}

// Run notarizes target, optionally stapling the ticket afterwards. It is
// exported so other commands can notarize what they just produced without
// re-entering the CLI parser.
func Run(ctx context.Context, logger *cmd.AppLogger, target string, creds signing.Credentials, staple bool) error {
	backend, err := signing.Select(creds)
	if err != nil {
		return err
	}

	_, _ = logger.Println("Start notarization")
	logger.PrintValue("Target", target)
	logger.PrintValue("Toolchain", backend.Name())
	if creds.Profile != "" {
		logger.PrintValue("Profile", creds.Profile)
	}
	if creds.APIKeyFile != "" {
		logger.PrintValue("API key", creds.APIKeyFile)
	}

	if err := signing.Notarize(ctx, backend, target, staple); err != nil {
		return err
	}
	_, _ = logger.Success("Notarization completed successfully!")
	return nil
}
