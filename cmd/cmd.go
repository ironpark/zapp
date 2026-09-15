package cmd

import (
	"context"
	"fmt"
	"github.com/urfave/cli/v3"
)

func CreateSubTaskFlags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "notarize",
			Hidden:   true,
		},
		&cli.StringFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "profile",
			Aliases:  []string{"p"},
			Usage:    "Keychain profile name",
			Action:   requireFlag[string]("notarize", "profile"),
		},
		&cli.StringFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "apple-id",
			Usage:    "Apple ID email",
			Action:   requireFlag[string]("notarize", "apple-id"),
		},
		&cli.StringFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "password",
			Usage:    "Apple ID password or app-specific password",
			Action:   requireFlag[string]("notarize", "password"),
		},
		&cli.StringFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "team-id",
			Usage:    "Developer Team ID",
			Action:   requireFlag[string]("notarize", "team-id"),
		},
		&cli.BoolFlag{
			Category: "[with --notarize (default: false)]",
			Name:     "staple",
			Usage:    "Perform stapling after notarization",
			Action:   requireFlag[bool]("notarize", "staple"),
		},
		&cli.BoolFlag{
			Category: "[with --sign (default: false)]",
			Name:     "sign",
			Usage:    "Codesign after creating DMG",
			Hidden:   true,
		},
		&cli.StringFlag{
			Category: "[with --sign (default: false)]",
			Name:     "identity",
			Usage:    "Identity to use for signing",
			Action:   requireFlag[string]("sign", "identity"),
		},
	}
}

func requireFlag[T any](requiredFlag, flagName string) func(context.Context, *cli.Command, T) error {
	return func(ctx context.Context, c *cli.Command, value T) error {
		if !c.Bool(requiredFlag) {
			return fmt.Errorf("%s flag must be used with %s flag", flagName, requiredFlag)
		}
		return nil
	}
}
