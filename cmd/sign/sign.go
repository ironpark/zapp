package sign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/cmd"
	"github.com/ironpark/zapp/pkg/mactools/codesign"
	"github.com/ironpark/zapp/pkg/mactools/productsign"
	"github.com/ironpark/zapp/pkg/mactools/security"
	"github.com/urfave/cli/v3"
)

var (
	identity string
	target   string
)

func getIdentity(ctx context.Context, prioritys ...string) (security.Identity, error) {
	idt, err := security.FindIdentity(ctx, "")
	if err != nil {
		return security.Identity{}, err
	}
	if len(idt) == 0 {
		return security.Identity{}, fmt.Errorf("no identity found")
	}
	for _, t := range prioritys {
		for _, identity := range idt {
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
	ArgsUsage:   "",
	Action: func(ctx context.Context, c *cli.Command) error {
		return Run(ctx, cmd.NewAppLogger(c.Root()), c.String("target"), c.String("identity"))
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "target",
			Usage:       "Path to the target(app,dmg,pkg) file",
			Destination: &target,
			Required:    true,
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
			Name:        "identity",
			Aliases:     []string{"i"},
			Usage:       "Identity to use for signing",
			Destination: &identity,
		},
	},
	SkipFlagParsing: false,
}

// Run signs target with identity, or with the best matching Developer ID when
// identity is empty. It is exported so other commands can sign what they just
// produced without re-entering the CLI parser.
func Run(ctx context.Context, logger *cmd.AppLogger, target, identity string) error {
	var idt security.Identity
	var err error
	targetExt := filepath.Ext(target)
	switch targetExt {
	case ".app", ".dmg":
		idt, err = getIdentity(ctx, "Developer ID Application")
	case ".pkg":
		idt, err = getIdentity(ctx, "Developer ID Installer")
	default:
		return fmt.Errorf("not a valid target type please provide a valid target(app,dmg,pkg)")
	}
	logger.Println("Start signing")
	logger.PrintValue("Target", target)
	if identity != "" {
		idt, err = getIdentity(ctx, identity)
	}
	if err != nil {
		return err
	}
	logger.PrintValue("Selected Identity", idt.SecureString())

	if targetExt == ".pkg" {
		logger.Println("Product sign (pkg)..")
		err = productsign.Sign(ctx, target, idt.String())
	} else {
		logger.Println("Codesign (app/dmg)..")
		err = codesign.CodeSign(ctx, idt.Fingerprint, target)
	}
	if err != nil {
		return err
	}
	logger.Success("%s signed successfully!", target)
	return nil
}
