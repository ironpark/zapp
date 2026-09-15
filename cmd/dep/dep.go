package dep

import (
	"context"
	"fmt"
	"github.com/ironpark/zapp/cmd"
	"github.com/ironpark/zapp/cmd/subtask"
	"github.com/ironpark/zapp/internal/fsutil"
	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/macho"
	"github.com/urfave/cli/v3"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// frameworksRPath is where the main executable looks for bundled libraries.
// Libraries reference their siblings through @loader_path instead, so they
// resolve without depending on any runpath at all.
const frameworksRPath = "@executable_path/../Frameworks"

var (
	appDir string
)

// pendingDep is a not-yet-bundled library, along with enough context about the
// binary that referenced it to resolve @rpath/@loader_path install names.
type pendingDep struct {
	name      string   // install name exactly as recorded in the load command
	loaderDir string   // directory of the referencing binary, in its original location
	rpaths    []string // LC_RPATH entries of the referencing binary
}

var Command = &cli.Command{
	Name:      "dep",
	Usage:     "Find dependencies of the specified app-bundle and bundle them",
	UsageText: "zapp dep <path of app-bundle>",
	ArgsUsage: " <path of app-bundle>",
	Action: func(ctx context.Context, c *cli.Command) error {
		logger := cmd.NewAppLogger(c.Root())
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
		info, err := appbundle.Open(appDir)
		if err != nil {
			return fmt.Errorf("failed to get app info: %v", err)
		}
		bundleExecutable, err := info.BundleExecutable()
		if err != nil {
			return fmt.Errorf("failed to get BundleExecutable: %v", err)
		}
		targetBundle := filepath.Join(appDir, "Contents", "MacOS", bundleExecutable)
		frameworksPath := filepath.Join(appDir, "Contents", "Frameworks")

		_, _ = logger.Printf("Start bundling dependencies for %s\n", bundleExecutable)
		logger.PrintValue("Target Bundle", targetBundle)
		logger.PrintValue("Frameworks Path", frameworksPath)
		_, _ = logger.Println("Getting dependencies")

		dependencies, err := directDependencies(targetBundle)
		if err != nil {
			return fmt.Errorf("failed to get dependencies: %v", err)
		}
		if len(dependencies) == 0 {
			return fmt.Errorf("no dependencies found")
		}

		for i, dep := range dependencies {
			logger.PrintValue(fmt.Sprintf("%d", i), dep)
		}

		libPaths := c.StringSlice("libs")
		if len(libPaths) == 0 {
			_, _ = logger.Println("No library path specified, using default paths")
		} else {
			_, _ = logger.Println("Using specified library paths first")
			for i, path := range libPaths {
				logger.PrintValue(fmt.Sprintf("%d", i), path)
			}
		}

		if err = os.MkdirAll(frameworksPath, 0755); err != nil {
			return fmt.Errorf("failed to create Frameworks directory: %v", err)
		}

		// Walk the whole dependency graph, not just the executable's direct
		// dependencies: a bundled library referencing an unbundled one outside
		// the app fails to load on any machine that lacks it, and under the
		// hardened runtime fails even on the build machine because the outside
		// library carries a different Team ID.
		bundled, err := bundleDependencies(targetBundle, frameworksPath, libPaths, dependencies)
		if err != nil {
			return err
		}
		for _, dep := range bundled {
			logger.PrintValue(dep.name, dep.source)
		}

		// Point the executable at the copies and make @rpath install names
		// resolve inside the bundle.
		for _, dependency := range dependencies {
			target := fmt.Sprintf("%s/%s", frameworksRPath, filepath.Base(dependency))
			if err = macho.ChangeDependency(targetBundle, dependency, target); err != nil {
				return fmt.Errorf("failed to change install name: %v", err)
			}
		}
		if err = macho.AddRPath(targetBundle, frameworksRPath); err != nil {
			return fmt.Errorf("failed to add rpath: %v", err)
		}

		_, _ = logger.Printf("(%d) Dependencies bundled successfully\n", len(bundled))
		err = subtask.Sign(ctx, c, appDir)
		if err != nil {
			return fmt.Errorf("failed to sign: %v", err)
		}

		err = subtask.Notarize(ctx, c, appDir)
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
		&cli.StringSliceFlag{
			Name:    "libs",
			Usage:   "Path to the directory containing the libraries",
			Aliases: []string{"l"},
			//Destination: &libPaths,
		},
	}, cmd.CreateSubTaskFlags()...),
}

type bundledDep struct {
	name   string // install name it was reached by
	source string // file it was copied from
}

// directDependencies returns the non-system libraries a binary links against.
func directDependencies(file string) ([]string, error) {
	info, err := macho.Read(file)
	if err != nil {
		return nil, err
	}
	return nonSystem(info), nil
}

// nonSystem drops the libraries macOS itself provides.
func nonSystem(info *macho.Info) []string {
	return slices.DeleteFunc(info.Dependencies, macho.IsSystemLibrary)
}

// bundleDependencies copies every non-system library transitively reachable
// from targetBundle into frameworksPath, rewriting each copy so it refers to
// its siblings inside the bundle.
func bundleDependencies(targetBundle, frameworksPath string, libPaths, roots []string) ([]bundledDep, error) {
	execDir := filepath.Dir(targetBundle)
	rootInfo, err := macho.Read(targetBundle)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", targetBundle, err)
	}

	queue := make([]pendingDep, 0, len(roots))
	for _, name := range roots {
		queue = append(queue, pendingDep{name: name, loaderDir: execDir, rpaths: rootInfo.RPaths})
	}

	var bundled []bundledDep
	done := map[string]bool{}
	for len(queue) > 0 {
		dep := queue[0]
		queue = queue[1:]

		base := filepath.Base(dep.name)
		if done[base] {
			continue
		}
		done[base] = true

		source, err := resolveDep(dep, execDir, libPaths)
		if err != nil {
			return nil, err
		}
		dst := filepath.Join(frameworksPath, base)
		// Re-running on an already bundled app resolves a dependency to the
		// copy itself; copying it over itself would truncate it.
		if same, err := fsutil.SameFile(source, dst); err != nil {
			return nil, err
		} else if !same {
			if err = fsutil.CopyFile(source, dst); err != nil {
				return nil, fmt.Errorf("failed to copy dependency: %v", err)
			}
		}
		// Homebrew installs libraries read-only, and install_name_tool rewrites
		// the file in place.
		if err = os.Chmod(dst, 0755); err != nil {
			return nil, fmt.Errorf("failed to make %s writable: %v", dst, err)
		}
		if err = macho.SetID(dst, "@rpath/"+base); err != nil {
			return nil, fmt.Errorf("failed to change install name id: %v", err)
		}
		bundled = append(bundled, bundledDep{name: dep.name, source: source})

		subInfo, err := macho.Read(dst)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", base, err)
		}
		subDeps := nonSystem(subInfo)
		sourceDir := filepath.Dir(source)
		for _, sub := range subDeps {
			subBase := filepath.Base(sub)
			if err = macho.ChangeDependency(dst, sub, "@loader_path/"+subBase); err != nil {
				return nil, fmt.Errorf("failed to change install name in %s: %v", base, err)
			}
			// Resolve relative to where this library actually came from, not
			// where its copy now lives.
			queue = append(queue, pendingDep{name: sub, loaderDir: sourceDir, rpaths: subInfo.RPaths})
		}
	}
	return bundled, nil
}

// resolveDep locates the file an install name points at, preferring the
// directories given with --libs.
func resolveDep(dep pendingDep, execDir string, libPaths []string) (string, error) {
	base := filepath.Base(dep.name)
	candidates := make([]string, 0, len(libPaths)+len(dep.rpaths)+1)
	for _, libPath := range libPaths {
		candidates = append(candidates, filepath.Join(libPath, base))
	}

	switch {
	case strings.HasPrefix(dep.name, "@rpath/"):
		suffix := strings.TrimPrefix(dep.name, "@rpath/")
		for _, rpath := range dep.rpaths {
			candidates = append(candidates, filepath.Join(expandPath(rpath, dep.loaderDir, execDir), suffix))
		}
	default:
		candidates = append(candidates, expandPath(dep.name, dep.loaderDir, execDir))
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("dependency not found: %s (tried: %s)", dep.name, strings.Join(candidates, ", "))
}

func expandPath(path, loaderDir, execDir string) string {
	for prefix, dir := range map[string]string{
		"@loader_path":     loaderDir,
		"@executable_path": execDir,
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return filepath.Join(dir, strings.TrimPrefix(path, prefix))
		}
	}
	return path
}
