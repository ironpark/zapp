package dmg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ironpark/zapp/mactools/dmg"

	_ "embed"

	"github.com/briandowns/spinner"
	"github.com/urfave/cli/v2"
)

//go:embed iconfile.icns
var defaultIconFile []byte

// flags
var (
	appDir                    string
	out                       string
	title                     string
	icon                      string
	background                string
	windowWidth, windowHeight int
	labelSize                 int
	contentsIconSize          int
)

var Command = &cli.Command{
	Name:        "dmg",
	Usage:       "Create a .dmg for macOS application deployment",
	UsageText:   "",
	Description: "",
	Args:        true,
	ArgsUsage:   " <path of app-bundle>",
	Action: func(c *cli.Context) error {
		// Create a temporary working directory
		tempDir, err := os.MkdirTemp("", "*-zapp-dmg")
		if err != nil {
			return fmt.Errorf("error creating temporary directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		fmt.Fprint(c.App.Writer, "Creating DMG file...")

		if icon == "" {
			tempDirForIcon, err := os.MkdirTemp("", "*-zapp-dmg-icon")
			if err != nil {
				return fmt.Errorf("error creating temporary directory: %v", err)
			}
			defer os.RemoveAll(tempDir)
			icon = filepath.Join(tempDirForIcon, "icon.icns")
			err = createIconSet(appDir, icon, !c.Bool("use-original-icon"))
			if err != nil {
				return err
			}
		}
		if out == "" {
			out = filepath.Base(appDir)
			out = strings.TrimSuffix(out, filepath.Ext(out))
			out = out + ".dmg"
		}
		if title == "" {
			title = filepath.Base(appDir)
			title = strings.TrimSuffix(title, filepath.Ext(title))
		}

		centerY := int(float64(windowHeight)/2-float64(contentsIconSize)/2) + labelSize
		defaultConfig := dmg.Config{
			FileName:         out,
			Title:            title,
			Icon:             icon,
			LabelSize:        labelSize,
			ContentsIconSize: contentsIconSize,
			WindowWidth:      windowWidth,
			WindowHeight:     windowHeight,
			Background:       background,
			Contents: []dmg.Item{
				{X: int(float64(windowWidth)/3*1 - float64(contentsIconSize)/2), Y: centerY, Type: dmg.Dir, Path: appDir},
				{X: int(float64(windowWidth)/3*2 + float64(contentsIconSize)/2), Y: centerY, Type: dmg.Link, Path: "/Applications"},
			},
		}

		s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
		s.Suffix = " Creating DMG file..."
		s.Start()
		err = dmg.CreateDMG(defaultConfig, tempDir)
		s.Stop()
		if err != nil {
			return err
		}
		fmt.Fprintf(c.App.Writer, "%s created at %s\n", out, time.Now().Format("2006-01-02 15:04:05"))
		return nil
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "background",
			Usage:       "Path to the background image file",
			Aliases:     []string{"bg"},
			Destination: &background,
		},
		&cli.StringFlag{
			Name:        "title",
			Usage:       "The title displayed when the DMG file is mounted",
			Aliases:     []string{"t"},
			Destination: &title,
		},
		&cli.StringFlag{
			Name:        "app",
			Usage:       "App bundle path",
			Destination: &appDir,
			Required:    true,
			Action: func(c *cli.Context, app string) error {
				if !strings.HasSuffix(app, ".app") {
					return fmt.Errorf("not valid app bundle extension")
				}
				// Check if the app bundle path is valid
				fileInfo, err := os.Stat(app)
				if err != nil {
					return fmt.Errorf("error accessing app-bundle path: %v", err)
				}
				if !fileInfo.IsDir() {
					return fmt.Errorf("app-bundle path must be a directory")
				}
				return nil
			},
		},
		&cli.StringFlag{
			Name:        "out",
			Usage:       "The output DMG file name",
			Aliases:     []string{"o"},
			Destination: &out,
		},
		&cli.StringFlag{
			Name:        "icon",
			Usage:       "Path to the icon file to display in the DMG file (icns, png)",
			Destination: &icon,
		},
		&cli.IntFlag{
			Name:        "window-width",
			Usage:       "Width of the Finder window when the DMG file is opened",
			Aliases:     []string{"ww"},
			Destination: &windowWidth,
			Value:       640,
		},
		&cli.IntFlag{
			Name:        "window-height",
			Usage:       "Height of the Finder window when the DMG file is opened",
			Aliases:     []string{"wh"},
			Destination: &windowHeight,
			Value:       480,
		},
		&cli.IntFlag{
			Name:        "label-size",
			Usage:       "Size of the label text in the Finder window (10-16)",
			Aliases:     []string{"ls"},
			Destination: &labelSize,
			Value:       14,
			Action: func(*cli.Context, int) error {
				if labelSize < 10 || labelSize > 16 {
					return fmt.Errorf("label-size must be between 10 and 16")
				}
				return nil
			},
		},
		&cli.IntFlag{
			Name:        "contents-icon-size",
			Usage:       "Size of the icons in the Finder window (16-512)",
			Aliases:     []string{"cis"},
			Destination: &contentsIconSize,
			Value:       128,
			Action: func(*cli.Context, int) error {
				if contentsIconSize < 16 || contentsIconSize > 512 {
					return fmt.Errorf("contents-icon-size must be between 16 and 512")
				}
				return nil
			},
		},
		&cli.BoolFlag{
			Name:    "use-original-icon ",
			Aliases: []string{"uoi"},
			Usage:   "Use the original icon file without modifications.",
		},
	},
	HelpName:           "",
	CustomHelpTemplate: "",
}
