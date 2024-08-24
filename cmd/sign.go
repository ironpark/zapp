package cmd

import (
	"fmt"
	"zapp/mactools/security"

	"github.com/urfave/cli/v2"
)

func init() {
	commands = append(commands, &cli.Command{
		Name:        "sign",
		Usage:       "",
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
	})
}
