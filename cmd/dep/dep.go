package dep

import (
	"fmt"
	"github.com/ironpark/zapp/cmd"
	"github.com/ironpark/zapp/pkg/fsutil"
	"github.com/ironpark/zapp/pkg/mactools/install_name_tool"
	"github.com/ironpark/zapp/pkg/mactools/otool"
	"github.com/ironpark/zapp/pkg/mactools/plist"
	"github.com/samber/lo"
	"github.com/urfave/cli/v2"
	"os"
	"path/filepath"
	"strings"
)

var (
	appDir string
)
var Command = &cli.Command{
	Name:      "dep",
	Usage:     "Find dependencies of the specified app-bundle and bundle them",
	UsageText: "zapp dep <path of app-bundle>",
	Args:      true,
	ArgsUsage: " <path of app-bundle>",
	Action: func(c *cli.Context) error {
		logger := cmd.NewAppLogger(c.App)
		if appDir == "" {
			return fmt.Errorf("[--app] target app-bundle is required")
		}
		if !strings.HasSuffix(appDir, ".app") {
			return fmt.Errorf("not valid app bundle extension")
		}
		fileInfo, err := os.Stat(appDir)
		if err != nil {
			return fmt.Errorf("error accessing app-bundle path: %v", err)
		}
		if !fileInfo.IsDir() {
			return fmt.Errorf("app-bundle path must be a directory")
		}
		info, err := plist.GetAppInfo(appDir)
		if err != nil {
			return fmt.Errorf("failed to get app info: %v", err)
		}
		bundleExecutable, err := info.BundleExecutable()
		if err != nil {
			return fmt.Errorf("failed to get BundleExecutable: %v", err)
		}
		targetBundle := filepath.Join(appDir, "Contents", "MacOS", bundleExecutable)
		frameworksPath := filepath.Join(appDir, "Contents", "Frameworks")

		logger.Printf("Start bundling dependencies for %s\n", bundleExecutable)
		logger.PrintValue("Target Bundle", targetBundle)
		logger.PrintValue("Frameworks Path", frameworksPath)
		logger.Println("Getting dependencies")

		dependencies, err := otool.GetDependencies(targetBundle)
		if err != nil {
			return fmt.Errorf("failed to get dependencies: %v", err)
		}
		// Filter out system libraries
		dependencies = lo.Filter(dependencies, func(s string, i int) bool {
			return !strings.HasPrefix(s, "/usr/lib")
		})
		if len(dependencies) == 0 {
			return fmt.Errorf("no dependencies found")
		}

		for i, dep := range dependencies {
			logger.PrintValue(fmt.Sprintf("%d", i), dep)
		}

		libPaths := c.StringSlice("libs")
		if len(libPaths) == 0 {
			logger.Println("No library path specified, using default paths")
		} else {
			logger.Println("Using specified library paths first")
			for i, path := range libPaths {
				logger.PrintValue(fmt.Sprintf("%d", i), path)
			}
		}
		// Copy dependencies to the specified directory
		foundedDeps := map[string]string{}
		for _, dependency := range dependencies {
			// Check if the library exists in the specified directory
			for _, libPath := range libPaths {
				depPath := filepath.Join(libPath, filepath.Base(dependency))
				if _, err := os.Stat(depPath); err == nil {
					foundedDeps[dependency] = depPath
					break
				}
			}
			if _, ok := foundedDeps[dependency]; ok {
				continue
			}
			// Check if the library exists in the system
			if _, err := os.Stat(dependency); err == nil {
				foundedDeps[dependency] = dependency
				continue
			} else {
				if !os.IsNotExist(err) {
					return fmt.Errorf("dependency not found: %s", dependency)
				}
				return fmt.Errorf("error accessing path: %v", err)
			}
		}
		_ = os.Mkdir(frameworksPath, os.ModePerm)
		// Copy dependencies to the specified directory
		logger.Println("Dependencies copied to the Frameworks directory")

		for dep, depPath := range foundedDeps {
			logger.PrintValue(dep, depPath)
			err = fsutil.CopyFileAnyway(depPath, filepath.Join(frameworksPath, filepath.Base(dep)))
			if err != nil {
				return fmt.Errorf("failed to copy dependency: %v", err)
			}
			err = install_name_tool.Change(dep, fmt.Sprintf("@executable_path/../Frameworks/%s", filepath.Base(dep)), targetBundle)
			if err != nil {
				return fmt.Errorf("failed to change install name: %v", err)
			}
		}
		logger.Printf("(%d) Dependencies bundled successfully\n", len(foundedDeps))
		err = cmd.RunSignCmd(c, appDir)
		if err != nil {
			return fmt.Errorf("failed to sign: %v", err)
		}

		err = cmd.RunNotarizeCmd(c, appDir)
		if err != nil {
			return fmt.Errorf("failed to notarize: %v", err)
		}
		return nil
	},
	Flags: append([]cli.Flag{
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
		&cli.StringSliceFlag{
			Name:    "libs",
			Usage:   "Path to the directory containing the libraries",
			Aliases: []string{"l"},
			//Destination: &libPaths,
		},
	}, cmd.CreateSubTaskFlags()...),
}
