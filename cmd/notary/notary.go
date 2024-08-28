package notary

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/mactools/notarytool" // 이 패키지를 새로 만들어야 합니다
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "notary",
	Usage: "Notarization & Stapling for macOS app/dmg/pkg",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "file",
			Aliases:  []string{"f"},
			Usage:    "Path to the file to be notarized and stapled (app/dmg/pkg)",
			Required: true,
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
	},
	Action: func(c *cli.Context) error {
		filePath := c.String("file")
		profile := c.String("profile")
		appleID := c.String("apple-id")
		password := c.String("password")
		teamID := c.String("team-id")

		// Check if either profile or all of apple-id, password, and team-id are provided
		if profile == "" && (appleID == "" || password == "" || teamID == "") {
			return fmt.Errorf("either --profile or all of [--apple-id, --password, --team-id] must be provided")
		}

		return notarizeAndStaple(c, filePath, profile, appleID, password, teamID)
	},
}

func notarizeAndStaple(c *cli.Context, filePath, profile, appleID, password, teamID string) error {
	// Step 1: Store credentials if profile is not provided
	if profile == "" {
		fmt.Println("Storing credentials...")
		profile = "temp_profile"
		err := notarytool.StoreCredentials(c.Context, appleID, password, teamID, profile)
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
		fmt.Println("Zipping the app...")
		fileToSubmit, err = zipApp(filePath, tempDir)
		if err != nil {
			return err
		}
	} else if ext == ".dmg" || ext == ".pkg" {
		fileToSubmit = filePath
	} else {
		return fmt.Errorf("unsupported file type: %s", ext)
	}
	fmt.Println("Submitting for notarization...")
	result, err := notarytool.Submit(c.Context, fileToSubmit, profile)
	if err != nil {
		return err
	}
	fmt.Printf("Submission ID: %s\nStatus: %s\nMessage: %s\n", result.ID, result.Status, result.Message)

	if result.Status == "In Progress" {
		fmt.Println("Waiting for notarization to complete...")
		result, err = notarytool.WaitForCompletion(c.Context, result.ID, profile)
		if err != nil {
			return err
		}
		fmt.Printf("Final Status: %s\nMessage: %s\n", result.Status, result.Message)
	}

	if result.Status != "Accepted" {
		return fmt.Errorf("notarization failed: %s", result.Message)
	}

	// Step 3: Staple
	fmt.Println("Stapling the notarization ticket...")
	err = notarytool.Staple(c.Context, filePath)
	if err != nil {
		return fmt.Errorf("failed to staple: %w", err)
	}
	isStapled, err := notarytool.IsStapled(c.Context, filePath)
	if err != nil {
		return fmt.Errorf("failed to check notarization: %w", err)
	}
	if !isStapled {
		return fmt.Errorf("file is not notarized after submission")
	}
	fmt.Printf("%s is now notarized & stapled\n", filepath.Base(filePath))
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
