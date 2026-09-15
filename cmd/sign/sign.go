package sign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/cmd"
	"github.com/ironpark/zapp/pkg/mactools/rcodesign"
	"github.com/ironpark/zapp/pkg/signing"
	"github.com/urfave/cli/v3"
)

var (
	identity string
	target   string
)

var Command = &cli.Command{
	Name:        "sign",
	Usage:       "Sign the app/dmg/pkg file",
	UsageText:   "",
	Description: "",
	ArgsUsage:   "",
	Action: func(ctx context.Context, c *cli.Command) error {
		return Run(ctx, cmd.NewAppLogger(c.Root()), c.String("target"), Credentials(c))
	},
	Flags: append([]cli.Flag{
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
			Usage:       "Keychain identity to sign with (macOS)",
			Destination: &identity,
		},
	}, cmd.CertificateFlags()...),
	SkipFlagParsing: false,
}

// Credentials returns the signing credentials a command's flags describe.
// Both toolchains read from the same set; which fields matter depends on which
// backend ends up being selected.
func Credentials(c *cli.Command) signing.Credentials {
	return signing.Credentials{
		Identity: c.String("identity"),
		Certificate: rcodesign.Credentials{
			P12File:         c.String("p12-file"),
			P12Password:     c.String("p12-password"),
			P12PasswordFile: c.String("p12-password-file"),
			PEMFile:         c.String("pem-file"),
		},
	}
}

// Run signs target. It is exported so other commands can sign what they just
// produced without re-entering the CLI parser.
func Run(ctx context.Context, logger *cmd.AppLogger, target string, creds signing.Credentials) error {
	backend, err := signing.Select(creds)
	if err != nil {
		return err
	}

	logger.Println("Start signing")
	logger.PrintValue("Target", target)
	logger.PrintValue("Toolchain", backend.Name())

	// Apple's tools pick a certificate out of the keychain, so say which one
	// before using it; rcodesign was handed one by path.
	if apple, ok := backend.(interface {
		Identity(context.Context, string, signing.Credentials) (signing.Identity, error)
	}); ok {
		idt, err := apple.Identity(ctx, target, creds)
		if err != nil {
			return err
		}
		logger.PrintValue("Selected Identity", idt.SecureString())
	}

	if err := backend.Sign(ctx, target, creds); err != nil {
		return err
	}
	logger.Success("%s signed successfully!", target)
	return nil
}
