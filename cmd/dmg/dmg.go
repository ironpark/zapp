package dmg

import (
	"context"
	"fmt"
	"github.com/ironpark/zapp/cmd"
	"github.com/ironpark/zapp/cmd/subtask"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/ironpark/zapp/pkg/udif"
	"os"
	"path/filepath"
	"strings"

	_ "embed"

	"github.com/urfave/cli/v3"
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
	format                    string
	filesystem                string
)

// imageFormats are the names the --format flag accepts.
var imageFormats = map[string]udif.Format{
	"udzo":  udif.UDZO,
	"zlib":  udif.UDZO,
	"ulfo":  udif.ULFO,
	"lzfse": udif.ULFO,
}

var Command = &cli.Command{
	Name:        "dmg",
	Usage:       "Create a .dmg for macOS application deployment",
	UsageText:   "",
	Description: "",
	ArgsUsage:   " <path of app-bundle>",
	Action: func(ctx context.Context, c *cli.Command) error {
		logger := cmd.NewAppLogger(c.Root())
		_, _ = logger.Printf("Start Creating DMG file for %s\n", filepath.Base(appDir))

		if icon == "" {
			_, _ = logger.Println("Icon file not provided")
			_, _ = logger.Println("Create dmg disk file icon using app icon")
			tempDirForIcon, err := os.MkdirTemp("", "*-zapp-dmg-icon")
			if err != nil {
				return fmt.Errorf("error creating temporary directory: %w", err)
			}
			defer func() { _ = os.RemoveAll(tempDirForIcon) }()
			icon = filepath.Join(tempDirForIcon, "icon.icns")
			if err = createIconSet(appDir, icon, !c.Bool("use-original-icon")); err != nil {
				return fmt.Errorf("could not derive a disk icon from %s: %w\n"+
					"       pass --icon with an .icns or .png to supply one",
					filepath.Base(appDir), err)
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
			Format:           imageFormats[strings.ToLower(format)],
			FileSystem:       dmg.FileSystem(strings.ToLower(filesystem)),
			Contents: []dmg.Item{
				{X: int(float64(windowWidth)/3*1 - float64(contentsIconSize)/2), Y: centerY, Type: dmg.Dir, Path: appDir},
				{X: int(float64(windowWidth)/3*2 + float64(contentsIconSize)/2), Y: centerY, Type: dmg.Link, Path: "/Applications"},
			},
		}
		logger.PrintValue("Title", title)
		logger.PrintValue("Icon", icon)
		logger.PrintValue("labelSize", labelSize)
		logger.PrintValue("AppPath", appDir)
		logger.PrintValue("OutputPath", out)
		logger.PrintValue("ContentsIconSize", contentsIconSize)
		logger.PrintValue("WindowWidth", windowWidth)
		logger.PrintValue("WindowHeight", windowHeight)
		logger.PrintValue("Background", background)
		logger.PrintValue("Format", imageFormats[strings.ToLower(format)].String())
		logger.PrintValue("FileSystem", defaultConfig.FileSystem.String())
		_, _ = logger.Println("Creating DMG file...")
		err := dmg.CreateDMG(ctx, defaultConfig)
		if err != nil {
			return err
		}
		_, _ = logger.Success("DMG file created successfully!")
		err = subtask.Sign(ctx, c, out)
		if err != nil {
			return fmt.Errorf("failed to sign DMG: %w", err)
		}

		err = subtask.Notarize(ctx, c, out)
		if err != nil {
			return fmt.Errorf("failed to notarize DMG: %w", err)
		}
		return nil
	},
	Flags: append([]cli.Flag{
		&cli.StringFlag{
			Name:        "filesystem",
			Usage:       "Volume filesystem: hfsplus, apfs, or apfs-case-sensitive (APFS needs macOS 10.13 or later)",
			Value:       "hfsplus",
			Destination: &filesystem,
			Action: func(ctx context.Context, c *cli.Command, v string) error {
				switch dmg.FileSystem(strings.ToLower(v)) {
				case dmg.HFSPlus, dmg.APFS, dmg.APFSCaseSensitive:
					return nil
				default:
					return fmt.Errorf("unknown filesystem %q: use hfsplus, apfs, or apfs-case-sensitive", v)
				}
			},
		},
		&cli.StringFlag{
			Name:  "format",
			Usage: "Compression of the disk image: udzo (zlib, read by every macOS) or ulfo (lzfse, smaller, needs macOS 10.11)",
			Value: "udzo",
			Action: func(ctx context.Context, c *cli.Command, v string) error {
				if _, ok := imageFormats[strings.ToLower(v)]; !ok {
					return fmt.Errorf("unknown image format %q: use udzo or ulfo", v)
				}
				return nil
			},
			Destination: &format,
		},
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
			Action: func(ctx context.Context, c *cli.Command, app string) error {
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
			Action: func(context.Context, *cli.Command, int) error {
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
			Action: func(context.Context, *cli.Command, int) error {
				if contentsIconSize < 16 || contentsIconSize > 512 {
					return fmt.Errorf("contents-icon-size must be between 16 and 512")
				}
				return nil
			},
		},
		&cli.BoolFlag{
			Name:    "use-original-icon",
			Aliases: []string{"uoi"},
			Usage:   "Use the original icon file without modifications.",
		},
	}, cmd.CreateSubTaskFlags()...),
}
