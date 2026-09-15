package notarize

import (
	"context"
	"fmt"
	"github.com/ironpark/zapp/cmd"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/pkg/mactools/notarytool"
	"github.com/urfave/cli/v3"
)

var Command = &cli.Command{
	Name:  "notarize",
	Usage: "Notarization & Stapling for macOS app/dmg/pkg",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "target",
			Aliases:  []string{"app", "dmg", "pkg"},
			Usage:    "Path to the target(app,dmg,pkg) file",
			Required: true,
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
		},
		&cli.StringFlag{
			Name:    "profile",
			Aliases: []string{"p"},
			Usage:   "Keychain profile name",
		},
		&cli.StringFlag{
			Name:  "apple-id",
			Usage: "Apple ID email",
		},
		&cli.StringFlag{
			Name:  "password",
			Usage: "Apple ID password or app-specific password",
		},
		&cli.StringFlag{
			Name:  "team-id",
			Usage: "Developer Team ID",
		},
		&cli.BoolFlag{
			Name:  "staple",
			Usage: "Perform stapling after notarization",
		},
	},
	Action: action,
}

func action(ctx context.Context, c *cli.Command) error {
	return Run(ctx, cmd.NewAppLogger(c.Root()), Options{
		Target:   c.String("target"),
		Profile:  c.String("profile"),
		AppleID:  c.String("apple-id"),
		Password: c.String("password"),
		TeamID:   c.String("team-id"),
		Staple:   c.Bool("staple"),
	})
}

// Options are the notarization credentials and target.
type Options struct {
	Target   string
	Profile  string
	AppleID  string
	Password string
	TeamID   string
	Staple   bool
}

// Run notarizes opts.Target, optionally stapling the ticket afterwards. It is
// exported so other commands can notarize what they just produced without
// re-entering the CLI parser.
func Run(ctx context.Context, logger *cmd.AppLogger, opts Options) error {
	profile := opts.Profile
	appleID := opts.AppleID
	password := opts.Password
	teamID := opts.TeamID
	staple := opts.Staple
	filePath := opts.Target
	// Check if either profile or all of apple-id, password, and team-id are provided
	if profile == "" && (appleID == "" || password == "" || teamID == "") {
		return fmt.Errorf("either --profile or all of [--apple-id, --password, --team-id] must be provided")
	}
	logger.Println("Start notarization")

	if profile != "" {
		logger.PrintValue("Profile", profile)
	}
	logger.PrintValue("Target", filePath)

	err := notarize(ctx, logger, filePath, profile, appleID, password, teamID)
	if err != nil {
		return err
	}
	logger.Success("Notarization completed successfully!")
	if staple {
		logger.Println("Start stapling")
		err = performStapling(ctx, logger, filePath)
		if err != nil {
			return err
		}
	}
	return nil
}
func notarize(ctx context.Context, logger *cmd.AppLogger, filePath, profile, appleID, password, teamID string) error {
	// Step 1: Store credentials if profile is not provided
	if profile == "" {
		logger.Println("Storing credentials...")
		profile = "temp_profile"
		err := notarytool.StoreCredentials(ctx, appleID, password, teamID, profile)
		if err != nil {
			return fmt.Errorf("failed to store credentials: %w", err)
		}
	}

	// Step 2: Submit and wait for notarization
	ext := strings.ToLower(filepath.Ext(filePath))
	var fileToSubmit string
	var tempDir string
	var err error

	if ext == ".app" {
		tempDir, err = os.MkdirTemp("", "zapp-notary-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tempDir) // Clean up temp directory after notarization
		logger.Println("Zipping the app...")
		fileToSubmit, err = zipApp(filePath, tempDir)
		if err != nil {
			return err
		}
	} else if ext == ".dmg" || ext == ".pkg" {
		fileToSubmit = filePath
	} else {
		return fmt.Errorf("unsupported file type: %s", ext)
	}
	logger.Println("Submitting for notarization...")
	result, err := notarytool.Submit(ctx, fileToSubmit, profile)
	if err != nil {
		return err
	}

	logger.PrintValue("Submission ID", result.ID)
	logger.PrintValue("Status", result.Status)
	logger.PrintValue("Message", result.Message)

	if result.Status == "In Progress" {
		logger.Println("Waiting for notarization to complete...")
		result, err = notarytool.WaitForCompletion(ctx, result.ID, profile)
		if err != nil {
			return err
		}
		logger.PrintValue("Final Status", result.Status)
		logger.PrintValue("Message", result.Message)
	}

	if result.Status != "Accepted" {
		return fmt.Errorf("notarization failed: %s", result.Message)
	}
	return nil
}

func performStapling(ctx context.Context, logger *cmd.AppLogger, filePath string) error {
	logger.Println("Stapling the notarization ticket...")
	err := notarytool.Staple(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to staple: %w", err)
	}
	isStapled, err := notarytool.IsStapled(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to check stapling: %w", err)
	}
	if !isStapled {
		return fmt.Errorf("file is not stapled after notarization")
	}
	logger.Success("Stapling completed successfully!")
	return nil
}

func zipApp(appPath string, tempDir string) (string, error) {
	zipName := filepath.Base(appPath) + ".zip"
	zipPath := filepath.Join(tempDir, zipName)
	err := createZip(appPath, zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to create zip file: %w", err)
	}
	return zipPath, nil
}
