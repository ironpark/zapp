package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/pkg/signing"
	"github.com/urfave/cli/v3"
)

var notarizeCommand = &cli.Command{
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
		notaryKeyFlag(),
	},
	Action: notarizeAction,
}

func notarizeAction(ctx context.Context, c *cli.Command) error {
	return runNotarize(ctx, newAppLogger(c.Root()), c.String("target"), notarizeCredentials(c), c.Bool("staple"))
}

// notarizeCredentials returns the notarization credentials a command's flags describe.
func notarizeCredentials(c *cli.Command) signing.Credentials {
	return signing.Credentials{
		Profile:    c.String("profile"),
		AppleID:    c.String("apple-id"),
		Password:   c.String("password"),
		TeamID:     c.String("team-id"),
		APIKeyFile: c.String("api-key-file"),
	}
}

// runNotarize notarizes target, optionally stapling the ticket afterwards. It is
// exported so other commands can notarize what they just produced without
// re-entering the CLI parser.
func runNotarize(ctx context.Context, logger *appLogger, target string, creds signing.Credentials, staple bool) error {
	pl, err := (&zapp.Project{Notarize: &zapp.NotarizeConfig{Profile: creds.Profile, AppleID: creds.AppleID, TeamID: creds.TeamID, Password: creds.Password, APIKeyFile: creds.APIKeyFile, Staple: staple}}).Resolve(zapp.WithLogger(logger))
	if err != nil {
		return err
	}
	return pl.Notarize(ctx, target)
}
