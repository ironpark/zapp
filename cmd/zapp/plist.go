package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/plist"
	"github.com/urfave/cli/v3"
)

var plistCommand = &cli.Command{
	Name:        "plist",
	Usage:       "Manage plist files",
	UsageText:   "zapp plist [command] [arguments...]",
	Description: "Perform operations on plist files",
	ArgsUsage:   "<path of .app directory> or <path of .plist file>",
	Action: func(ctx context.Context, c *cli.Command) error {
		if c.NArg() < 1 {
			return fmt.Errorf("path is required")
		}

		path := c.Args().First()
		plistPath, err := appbundle.FindPlistPath(path)
		if err != nil {
			return err
		}

		fmt.Printf("Using plist file: %s\n", plistPath)
		return nil
	},
	Commands: []*cli.Command{
		getCommand,
		setCommand,
		deleteCommand,
		bumpCommand,
	},
}

var getCommand = &cli.Command{
	Name:      "get",
	Usage:     "Get a value from the plist file",
	ArgsUsage: "<path> <key>",
	Action: func(ctx context.Context, c *cli.Command) error {
		if c.NArg() < 2 {
			return fmt.Errorf("path and key are required")
		}
		key := c.Args().Get(1)
		doc, err := open(c.Args().First())
		if err != nil {
			return err
		}
		_, _, value, found := plist.Resolve(doc.Root, key)
		if !found {
			return fmt.Errorf("key not found: %s", key)
		}
		text, err := render(value)
		if err != nil {
			return err
		}
		fmt.Println(text)
		return nil
	},
}

// render prints a scalar bare, so it can be captured by a shell, and a
// dictionary or array as a property list rather than as Go's map syntax, which
// nothing can read back.
func render(value any) (string, error) {
	if text, ok := plist.Format(value); ok {
		return text, nil
	}
	xml, err := plist.MarshalXML(value)
	if err != nil {
		return "", err
	}
	return string(xml), nil
}

var setCommand = &cli.Command{
	Name:      "set",
	Usage:     "Set a value in the plist file",
	ArgsUsage: "<path> <key> <value>",
	Description: "An existing key keeps the type it already has, so setting a boolean " +
		"to false writes <false/> rather than the string \"false\". A key that does " +
		"not exist yet becomes a boolean for true or false, an integer for a whole " +
		"number, and a string otherwise.",
	Action: func(ctx context.Context, c *cli.Command) error {
		if c.NArg() < 3 {
			return fmt.Errorf("path, key, and value are required")
		}
		if err := setValue(c.Args().First(), c.Args().Get(1), c.Args().Get(2)); err != nil {
			return err
		}
		fmt.Printf("Value set successfully\n")
		return nil
	},
}

func setValue(path, key, literal string) error {
	return edit(path, key, func(existing any) (any, error) {
		return plist.Coerce(existing, literal)
	})
}

var deleteCommand = &cli.Command{
	Name:      "delete",
	Aliases:   []string{"remove"},
	Usage:     "Remove a key from the plist file",
	ArgsUsage: "<path> <key>",
	Action: func(ctx context.Context, c *cli.Command) error {
		if c.NArg() < 2 {
			return fmt.Errorf("path and key are required")
		}
		key := c.Args().Get(1)
		if err := deleteValue(c.Args().First(), key); err != nil {
			return err
		}
		fmt.Printf("Removed %s\n", key)
		return nil
	},
}

func deleteValue(path, key string) error {
	doc, err := open(path)
	if err != nil {
		return err
	}
	container, name, existing, _ := plist.Resolve(doc.Root, key)
	if err := mustExist(key, existing); err != nil {
		return err
	}
	if err := plist.Delete(container, name); err != nil {
		return err
	}
	return doc.Save()
}

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

// open resolves what the user named -- a .plist file, or a bundle whose
// Info.plist is meant -- and reads it.
func open(path string) (*plist.Document, error) {
	plistPath, err := appbundle.FindPlistPath(path)
	if err != nil {
		return nil, err
	}
	return plist.Open(plistPath)
}

// edit reads the document at path, replaces the value at key with whatever
// change returns for the current one, and writes it back. Every command that
// modifies a property list goes through here, so they agree on how a key is
// found, how a container is written to, and how the file is replaced.
//
// change receives nil when the key does not exist yet.
func edit(path, key string, change func(existing any) (any, error)) error {
	doc, err := open(path)
	if err != nil {
		return err
	}
	container, name, existing, _ := plist.Resolve(doc.Root, key)
	value, err := change(existing)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if err := plist.Put(container, name, value); err != nil {
		return err
	}
	return doc.Save()
}

// mustExist is for commands that change a value rather than assign one, and so
// have nothing to work from when the key is absent.
func mustExist(key string, existing any) error {
	if existing == nil {
		return fmt.Errorf("key not found: %s", key)
	}
	return nil
}
