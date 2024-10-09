package dep

import (
	"fmt"
	"github.com/ironpark/zapp/cmd"
	"github.com/ironpark/zapp/mactools/install_name_tool"
	"github.com/ironpark/zapp/mactools/otool"
	"github.com/ironpark/zapp/mactools/plist"
	"github.com/samber/lo"
	"github.com/urfave/cli/v2"
	"io"
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
		bundleName, err := info.BundleName()
		if err != nil {
			return fmt.Errorf("failed to get bundle name: %v", err)
		}
		targetBundle := filepath.Join(appDir, "Contents", "MacOS", bundleName)
		frameworksPath := filepath.Join(appDir, "Contents", "Frameworks")

		fmt.Printf("Target bundle: %s\n", targetBundle)
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
		fmt.Println("Dependencies:")
		for _, dep := range dependencies {
			fmt.Println("\t", dep)
		}

		libPaths := c.StringSlice("libs")
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
		fmt.Println("Dependencies copied to the Frameworks directory")

		for dep, depPath := range foundedDeps {
			_, err = CopyFile(depPath, filepath.Join(frameworksPath, filepath.Base(dep)))
			if err != nil {
				return fmt.Errorf("failed to copy dependency: %v", err)
			}
			err = install_name_tool.Change(dep, fmt.Sprintf("@executable_path/../Frameworks/%s", filepath.Base(dep)), targetBundle)
			if err != nil {
				return fmt.Errorf("failed to change install name: %v", err)
			}
		}
		fmt.Printf("(%d) Dependencies bundled successfully\n", len(foundedDeps))
		err = cmd.RunSignCmd(c, appDir)
		if err != nil {
			return fmt.Errorf("failed to sign PKG: %v", err)
		}

		err = cmd.RunNotarizeCmd(c, appDir)
		if err != nil {
			return fmt.Errorf("failed to notarize PKG: %v", err)
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

func CopyFile(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceFileStat.Mode())
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}
