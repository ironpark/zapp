package sign

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/mactools/codesign"
	"github.com/ironpark/zapp/mactools/security"
	"github.com/urfave/cli/v2"
)

var (
	identity string
)

func getIdentity(c *cli.Context, prioritys ...string) (security.Identity, error) {
	idt, err := security.FindIdentity(c.Context, "")
	if err != nil {
		return security.Identity{}, err
	}
	if len(idt) == 0 {
		return security.Identity{}, fmt.Errorf("no identity found")
	}
	for _, identity := range idt {
		for _, t := range prioritys {
			if strings.Contains(identity.String(), t) {
				return identity, nil
			}
		}
	}
	return security.Identity{}, fmt.Errorf("no identity found")
}

var Command = &cli.Command{
	Name:        "sign",
	Usage:       "Sign the app/dmg/pkg file",
	UsageText:   "",
	Description: "",
	Args:        true,
	ArgsUsage:   " <path of target>",
	Action: func(c *cli.Context) error {
		path := c.Args().First()
		if path == "" {
			return fmt.Errorf("please provide a path for the target(app,dmg,pkg) file")
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("error accessing path: %v", err)
		}
		var idt security.Identity
		var err error
		switch filepath.Ext(path) {
		case ".app":
			idt, err = getIdentity(c, "Developer ID Application")
		case ".dmg":
			idt, err = getIdentity(c, "Apple Distribution", "Developer ID Application")
		case ".pkg":
			idt, err = getIdentity(c, "Apple Distribution", "Developer ID Installer", "Developer ID Application")
		default:
			return fmt.Errorf("not a valid target type please provide a valid target(app,dmg,pkg)")
		}
		if identity != "" {
			idt, err = getIdentity(c, identity)
		}
		if err != nil {
			return err
		}
		err = codesign.CodeSign(c.Context, idt.Fingerprint, path)
		if err != nil {
			return err
		}
		return nil
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "identity",
			Aliases:     []string{"i"},
			Usage:       "Identity to use for signing",
			Destination: &identity,
		},
	},
	SkipFlagParsing:    true,
	HelpName:           "",
	CustomHelpTemplate: "",
}
