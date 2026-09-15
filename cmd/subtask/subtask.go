// Package subtask runs the sign and notarize steps that dmg, pkg and dep can
// chain onto their own output via --sign and --notarize.
//
// These used to be dispatched by re-running the root command with a synthesised
// argument list. urfave/cli v3 does not support re-entering a command that is
// already running — it loops forever — so the steps are invoked as ordinary
// function calls instead. That is also what they always were: the argument
// round trip only served to look them up by name.
package subtask

import (
	"context"

	"github.com/ironpark/zapp/cmd"
	"github.com/ironpark/zapp/cmd/notarize"
	"github.com/ironpark/zapp/cmd/sign"
	"github.com/urfave/cli/v3"
)

// Sign signs target when --sign was given, and is a no-op otherwise.
func Sign(ctx context.Context, c *cli.Command, target string) error {
	if !c.Bool("sign") {
		return nil
	}
	return sign.Run(ctx, cmd.NewAppLogger(c.Root()), target, sign.Credentials(c))
}

// Notarize notarizes target when --notarize was given, and is a no-op otherwise.
func Notarize(ctx context.Context, c *cli.Command, target string) error {
	if !c.Bool("notarize") {
		return nil
	}
	return notarize.Run(ctx, cmd.NewAppLogger(c.Root()), target, notarize.Credentials(c), c.Bool("staple"))
}
