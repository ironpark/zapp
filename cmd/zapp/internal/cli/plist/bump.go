package plist

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/ironpark/zapp/pkg/plist"
)

var bumpCommand = &cli.Command{
	Name:      "bump",
	Usage:     "Raise a version number in the plist file",
	ArgsUsage: "<path>",
	Description: "Without a flag the last component is raised, which is what a build " +
		"number wants. --major, --minor and --patch raise that component of a " +
		"dotted version instead and reset the ones after it. The key keeps its " +
		"type, so an integer build number stays an integer.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "key",
			Usage: "Version key to raise",
			Value: "CFBundleVersion",
		},
		&cli.BoolFlag{Name: "major", Usage: "Raise the first component"},
		&cli.BoolFlag{Name: "minor", Usage: "Raise the second component"},
		&cli.BoolFlag{Name: "patch", Usage: "Raise the third component"},
	},
	Action: func(ctx context.Context, c *cli.Command) error {
		if c.NArg() < 1 {
			return fmt.Errorf("path is required")
		}
		component, err := selectedComponent(c)
		if err != nil {
			return err
		}
		key := c.String("key")
		before, after, err := bumpValue(c.Args().First(), key, component)
		if err != nil {
			return err
		}
		fmt.Printf("%s: %s -> %s\n", key, before, after)
		return nil
	},
}

// bumpValue raises one component of the version held at key, reporting what it
// was and what it became.
func bumpValue(path, key string, component int) (before, after string, err error) {
	err = edit(path, key, func(existing any) (any, error) {
		if err := mustExist(key, existing); err != nil {
			return nil, err
		}
		text, ok := plist.Format(existing)
		if !ok {
			return nil, fmt.Errorf("a %s is not a version", plist.Kind(existing))
		}
		raised, err := raise(text, component)
		if err != nil {
			return nil, err
		}
		before, after = text, raised
		return plist.Coerce(existing, raised)
	})
	return before, after, err
}

// selectedComponent reports which dotted component to raise, counting from
// zero, or -1 for the last one.
func selectedComponent(c *cli.Command) (int, error) {
	chosen := -1
	for i, name := range []string{"major", "minor", "patch"} {
		if !c.Bool(name) {
			continue
		}
		if chosen >= 0 {
			return 0, fmt.Errorf("choose only one of --major, --minor and --patch")
		}
		chosen = i
	}
	return chosen, nil
}

// raise increments one component of a dotted version, resetting the components
// after it so 1.4.2 raised at the minor becomes 1.5.0.
func raise(version string, component int) (string, error) {
	if strings.TrimSpace(version) == "" {
		return "", fmt.Errorf("version is empty")
	}
	parts := strings.Split(version, ".")
	if component < 0 {
		component = len(parts) - 1
	}
	if component >= len(parts) {
		return "", fmt.Errorf("%q has no component %d", version, component+1)
	}
	n, err := strconv.ParseUint(parts[component], 10, 64)
	if err != nil {
		return "", fmt.Errorf("component %q of %q is not a number", parts[component], version)
	}
	parts[component] = strconv.FormatUint(n+1, 10)
	for i := component + 1; i < len(parts); i++ {
		parts[i] = "0"
	}
	return strings.Join(parts, "."), nil
}
