package sign

import (
	"fmt"
	"github.com/ironpark/zapp/cmd"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/mactools/codesign"
	"github.com/ironpark/zapp/mactools/security"
	"github.com/urfave/cli/v2"
)

var (
	identity string
	target   string
)

func getIdentity(c *cli.Context, prioritys ...string) (security.Identity, error) {
	idt, err := security.FindIdentity(c.Context, "")
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
	Args:        true,
	ArgsUsage:   "",
	Action: func(c *cli.Context) error {
		logger := cmd.NewAppLogger(c.App)
		var idt security.Identity
		var err error
		targetExt := filepath.Ext(target)
		switch targetExt {
		case ".app":
			idt, err = getIdentity(c, "Developer ID Application")
		case ".dmg":
			idt, err = getIdentity(c, "Developer ID Application")
		case ".pkg":
			idt, err = getIdentity(c, "Developer ID Installer")
		default:
			return fmt.Errorf("not a valid target type please provide a valid target(app,dmg,pkg)")
		}
		logger.Println("Start signing")
		logger.PrintValue("Target", target)
		if identity != "" {
			idt, err = getIdentity(c, identity)
		}
		if err != nil {
			return err
		}
		logger.PrintValue("Selected Identity", idt.SecureString())

		if targetExt == ".pkg" {
			logger.Println("Product sign (pkg)..")
			err = signPKG(target, idt.String())
		} else {
			logger.Println("Codesign (app/dmg)..")
			err = codesign.CodeSign(c.Context, idt.Fingerprint, target)
		}
		if err != nil {
			return err
		}
		logger.Success("%s signed successfully!", target)
		return nil
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "target",
			Usage:       "Path to the target(app,dmg,pkg) file",
			Destination: &target,
			Required:    true,
			Action: func(c *cli.Context, target string) error {
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

func signPKG(path, identity string) error {
	tempDir, err := os.MkdirTemp("", "pkg-signing-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	signedPath := filepath.Join(tempDir, "signed.pkg")

	cmd := exec.Command("productsign", "--sign", identity, path, signedPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to sign pkg: %w, output: %s", err, string(output))
	}

	// Replace the original file with the signed one
	err = os.Rename(signedPath, path)
	if err != nil {
		return fmt.Errorf("failed to replace original pkg with signed one: %w", err)
	}

	return nil
}
