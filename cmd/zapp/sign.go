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

var signCommand = &cli.Command{
	Name:        "sign",
	Usage:       "Sign the app/dmg/pkg file",
	UsageText:   "",
	Description: "",
	ArgsUsage:   "",
	Action: func(ctx context.Context, c *cli.Command) error {
		return runSign(ctx, newAppLogger(c.Root()), c.String("target"), signCredentials(c))
	},
	Flags: append([]cli.Flag{
		&cli.StringFlag{
			Name:     "target",
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
			Aliases: []string{"app", "dmg", "pkg"},
		},
		&cli.StringFlag{
			Name:    "identity",
			Aliases: []string{"i"},
			Usage:   "Keychain identity to sign with (macOS)",
		},
	}, certificateFlags()...),
	SkipFlagParsing: false,
}

// signCredentials returns the signing credentials a command's flags describe.
// Both toolchains read from the same set; which fields matter depends on which
// backend ends up being selected.
func signCredentials(c *cli.Command) signing.Credentials {
	return signing.Credentials{
		Identity:        c.String("identity"),
		P12File:         c.String("p12-file"),
		P12Password:     c.String("p12-password"),
		P12PasswordFile: c.String("p12-password-file"),
		PEMFile:         c.String("pem-file"),
	}
}

// runSign signs target through the library, so that the standalone command and
// a project build reach the signing backends the same way.
func runSign(ctx context.Context, logger *appLogger, target string, creds signing.Credentials) error {
	pl, err := (&zapp.Project{Sign: &zapp.SignConfig{Identity: creds.Identity, P12File: creds.P12File, PEMFile: creds.PEMFile, P12Password: creds.P12Password, P12PasswordFile: creds.P12PasswordFile}}).Resolve(zapp.WithLogger(logger))
	if err != nil {
		return err
	}
	return pl.Sign(ctx, target)
}
