package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/ironpark/zapp/pkg/verify"
	"github.com/urfave/cli/v3"
)

func verifyCommand() *cli.Command {
	return &cli.Command{
		Name:      "verify",
		Usage:     "Check apps, DMGs, PKGs and ZIPs are signed, notarized and ready to ship",
		ArgsUsage: "<path> ...",
		Description: "Reads each artifact's code signatures on any platform: a Developer ID certificate, a secure\n" +
			"timestamp, the hardened runtime, one team, a stapled ticket, the minimum macOS it runs on.\n" +
			"On macOS codesign, pkgutil and spctl then check integrity and ask Gatekeeper; elsewhere the\n" +
			"code digests of every binary are checked. Exits non-zero when a check fails.",
		Action: func(ctx context.Context, c *cli.Command) error {
			if c.NArg() == 0 {
				return errors.New("name an app, DMG, PKG or ZIP to verify")
			}
			failed, total := 0, 0
			for _, path := range c.Args().Slice() {
				reports, err := verify.Path(ctx, path)
				if err != nil {
					return err
				}
				for _, r := range reports {
					total++
					fmt.Fprint(c.Root().Writer, r)
					if r.Failed() {
						failed++
					}
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d artifacts are not ready to ship", failed, total)
			}
			return nil
		},
	}
}
