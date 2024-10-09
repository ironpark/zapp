package cmd

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

func CreateSubTaskFlags() []cli.Flag {
	return []cli.Flag{
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
	}
}

func requireFlag[T any](requiredFlag, flagName string) func(*cli.Context, T) error {
	return func(c *cli.Context, value T) error {
		if !c.Bool(requiredFlag) {
			return fmt.Errorf("%s flag must be used with %s flag", flagName, requiredFlag)
		}
		return nil
	}
}

func runner(c *cli.Context, command string, req string, flags ...string) error {
	var args []string
	for _, flag := range flags {
		if c.String(flag) != "" {
			args = append(args, "--"+flag+"="+c.String(flag))
		} else if c.Bool(flag) {
			args = append(args, "--"+flag)
		}
	}
	return c.App.Run(append([]string{c.App.Name, command, req}, args...))
}

func RunSignCmd(c *cli.Context, target string) error {
	if c.Bool("sign") {
		if err := runner(c, "sign", "--target="+target, "identity"); err != nil {
			return err
		}
	}
	return nil
}

func RunNotarizeCmd(c *cli.Context, target string) error {
	if c.Bool("notarize") {
		if err := runner(c, "notarize", "--target="+target, "profile", "apple-id", "password", "team-id", "staple"); err != nil {
			return err
		}
	}
	return nil
}
