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
	ArgsUsage:   "",
	Action: func(ctx context.Context, c *cli.Command) error {
		config, appDir, err := resolveConfig(c)
		if err != nil {
			return err
		}
		icon := config.Icon
		out := config.FileName
		logger := cmd.NewAppLogger(c.Root())

		if (icon == "" && appDir != "") || strings.EqualFold(filepath.Ext(icon), ".png") {
			source := icon
			withDiskBackground := false
			if source == "" {
				source = appDir
				withDiskBackground = !c.Bool("use-original-icon")
				_, _ = logger.Println("Create dmg disk file icon using app icon")
			}
			tempDirForIcon, err := os.MkdirTemp("", "*-zapp-dmg-icon")
			if err != nil {
				return fmt.Errorf("error creating temporary directory: %w", err)
			}
			defer func() { _ = os.RemoveAll(tempDirForIcon) }()
			icon = filepath.Join(tempDirForIcon, "icon.icns")
			if err = createIconSet(source, icon, withDiskBackground); err != nil {
				if config.Icon != "" {
					return fmt.Errorf("could not convert PNG icon %s: %w", source, err)
				}
				return fmt.Errorf("could not derive a disk icon from %s: %w\n"+
					"       pass --icon with an .icns or .png to supply one",
					filepath.Base(source), err)
			}
		}
		config.Icon = icon
		logger.PrintValue("Title", config.Title)
		logger.PrintValue("OutputPath", out)
		logger.PrintValue("Icon", config.Icon)
		logger.PrintValue("WindowWidth", config.WindowWidth)
		logger.PrintValue("WindowHeight", config.WindowHeight)
		logger.PrintValue("ContentsIconSize", config.ContentsIconSize)
		logger.PrintValue("LabelSize", config.LabelSize)
		logger.PrintValue("Background", config.Background)
		logger.PrintValue("Format", config.Format.String())
		logger.PrintValue("FileSystem", config.FileSystem.String())

		_, _ = logger.Println("Creating DMG file...")
		err = dmg.CreateDMG(ctx, config)
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
		&cli.StringFlag{Name: "config", Usage: "Path to a YAML or JSON DMG configuration"},
		&cli.StringFlag{Name: "app-position", Usage: "App icon center in content coordinates: x,y"},
		&cli.StringFlag{Name: "applications-position", Usage: "Applications icon center in content coordinates: x,y"},
		&cli.StringFlag{
			Name:  "fs",
			Usage: "Volume filesystem: hfsplus, apfs, or apfs-case-sensitive (APFS needs macOS 10.13 or later)",
			Value: "hfsplus",
			Action: func(ctx context.Context, c *cli.Command, v string) error {
				_, err := dmg.ParseFileSystem(v)
				return err
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
		},
		&cli.StringFlag{
			Name:    "background",
			Usage:   "Path to the background image file",
			Aliases: []string{"bg"},
		},
		&cli.StringFlag{
			Name:    "title",
			Usage:   "The title displayed when the DMG file is mounted",
			Aliases: []string{"t"},
		},
		&cli.StringFlag{
			Name:  "app",
			Usage: "App bundle path",

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
			Name:    "out",
			Usage:   "The output DMG file name",
			Aliases: []string{"o"},
		},
		&cli.StringFlag{
			Name:  "icon",
			Usage: "Path to the icon file to display in the DMG file (icns, png)",
		},
		&cli.IntFlag{
			Name:    "window-width",
			Usage:   "Width of the Finder window when the DMG file is opened",
			Aliases: []string{"ww"},
			Value:   640,
		},
		&cli.IntFlag{
			Name:    "window-height",
			Usage:   "Height of the Finder window when the DMG file is opened",
			Aliases: []string{"wh"},
			Value:   480,
		},
		&cli.IntFlag{
			Name:    "label-size",
			Usage:   "Size of the label text in the Finder window (10-16)",
			Aliases: []string{"ls"},
			Value:   14,
			Action: func(_ context.Context, _ *cli.Command, labelSize int) error {
				if labelSize < 10 || labelSize > 16 {
					return fmt.Errorf("label-size must be between 10 and 16")
				}
				return nil
			},
		},
		&cli.IntFlag{
			Name:    "contents-icon-size",
			Usage:   "Size of the icons in the Finder window (16-512)",
			Aliases: []string{"cis"},
			Value:   128,
			Action: func(_ context.Context, _ *cli.Command, contentsIconSize int) error {
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
