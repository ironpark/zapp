package sign

import (
	"fmt"

	"github.com/ironpark/zapp/mactools/security"

	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:        "sign",
	Usage:       "Sign the app/dmg/pkg file",
	UsageText:   "",
	Description: "",
	Args:        true,
	ArgsUsage:   " <path of app>",
	Action: func(c *cli.Context) error {
		idt, err := security.FindIdentity(c.Context, "")
		if err != nil {
			return err
		}
		if len(idt) == 0 {
			return fmt.Errorf("no identity found")
		}
		for d, i := range idt {
			fmt.Println(i, d)
		}
		return nil
	},
	Flags:              nil,
	SkipFlagParsing:    true,
	HelpName:           "",
	CustomHelpTemplate: "",
}
