package dmg

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/urfave/cli/v3"
)

type position struct {
	X int
	Y int
}
type configItem struct {
	Link bool   `json:"link"`
	Name string `json:"name"`
	X    *int   `json:"x"`
	Y    *int   `json:"y"`
}

// fileConfig is a versioned user-facing schema, separate from the build API.
type fileConfig struct {
	Version    int    `json:"version"`
	App        string `json:"app"`
	Out        string `json:"out"`
	Title      string `json:"title"`
	Icon       string `json:"icon"`
	Background string `json:"background"`
	Window     struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"window"`
	IconSize  int                   `json:"iconSize"`
	LabelSize int                   `json:"labelSize"`
	FS        string                `json:"fs"`
	Format    string                `json:"format"`
	Contents  map[string]configItem `json:"contents"`
}

func resolveConfig(c *cli.Command) (dmg.Config, string, error) {
	fail := func(err error) (dmg.Config, string, error) { return dmg.Config{}, "", err }
	f := fileConfig{IconSize: 128, LabelSize: 14, FS: "hfsplus", Format: "udzo"}
	f.Window.Width, f.Window.Height = 640, 480
	base := "."
	if name := c.String("config"); name != "" {
		data, err := os.ReadFile(name)
		if err != nil {
			return fail(err)
		}
		if err := yaml.UnmarshalWithOptions(data, &f, yaml.Strict()); err != nil {
			return fail(fmt.Errorf("config %s: %w", name, err))
		}
		if f.Version != 1 {
			return fail(fmt.Errorf("config version must be 1"))
		}
		base = filepath.Dir(name)
		resolve := func(p string) string {
			if p != "" && !filepath.IsAbs(p) {
				return filepath.Join(base, p)
			}
			return p
		}
		f.App, f.Icon, f.Background = resolve(f.App), resolve(f.Icon), resolve(f.Background)

	}
	for flag, target := range map[string]*string{"app": &f.App, "out": &f.Out, "title": &f.Title, "icon": &f.Icon, "background": &f.Background, "fs": &f.FS, "format": &f.Format} {
		if c.IsSet(flag) {
			*target = c.String(flag)
		}
	}
	for flag, target := range map[string]*int{"window-width": &f.Window.Width, "window-height": &f.Window.Height, "contents-icon-size": &f.IconSize, "label-size": &f.LabelSize} {
		if c.IsSet(flag) {
			*target = c.Int(flag)
		}
	}
	if f.App != "" {
		info, err := os.Stat(f.App)
		if err != nil {
			return fail(err)
		}
		if !strings.HasSuffix(f.App, ".app") || !info.IsDir() {
			return fail(fmt.Errorf("app must be an .app directory"))
		}
	}
	if f.Title == "" && f.App != "" {
		f.Title = strings.TrimSuffix(filepath.Base(f.App), ".app")
	}
	if f.Out == "" {
		f.Out = f.Title
	}
	if !strings.HasSuffix(f.Out, ".dmg") {
		f.Out += ".dmg"
	}
	fs, err := dmg.ParseFileSystem(f.FS)
	if err != nil {
		return fail(err)
	}
	format, ok := imageFormats[strings.ToLower(f.Format)]
	if !ok {
		return fail(fmt.Errorf("unknown image format %q: use udzo or ulfo", f.Format))
	}
	cfg := dmg.Config{FileName: f.Out, Title: f.Title, Icon: f.Icon, Background: f.Background, WindowWidth: f.Window.Width, WindowHeight: f.Window.Height, ContentsIconSize: f.IconSize, LabelSize: f.LabelSize, FileSystem: fs, Format: format}
	if f.Contents != nil {
		if c.IsSet("app-position") || c.IsSet("applications-position") {
			return fail(fmt.Errorf("position flags cannot be used with explicit contents; set contents[path].x and contents[path].y"))
		}
		// Sort source keys so map iteration cannot change image ordering.
		paths := make([]string, 0, len(f.Contents))
		for path := range f.Contents {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, source := range paths {
			item := f.Contents[source]
			if source == "" {
				return fail(fmt.Errorf("contents path must not be empty"))
			}
			if item.X == nil || item.Y == nil {
				return fail(fmt.Errorf("contents[%q] requires x and y", source))
			}
			path := source
			kind := dmg.Link
			if !item.Link {
				if !filepath.IsAbs(path) {
					path = filepath.Join(base, path)
				}
				info, err := os.Lstat(path)
				if err != nil {
					return fail(fmt.Errorf("contents[%q]: %w", source, err))
				}
				kind = dmg.File
				if info.IsDir() {
					kind = dmg.Dir
				}
			}
			cfg.Contents = append(cfg.Contents, dmg.Item{Type: kind, Path: path, Name: item.Name, X: *item.X, Y: *item.Y})
		}
	} else {
		if f.App == "" {
			return fail(fmt.Errorf("provide --app or config contents"))
		}
		y := int(float64(f.Window.Height)/2-float64(f.IconSize)/2) + f.LabelSize
		cfg.Contents = []dmg.Item{{Type: dmg.Dir, Path: f.App, X: int(float64(f.Window.Width)/3 - float64(f.IconSize)/2), Y: y}, {Type: dmg.Link, Path: "/Applications", X: int(float64(f.Window.Width)/3*2 + float64(f.IconSize)/2), Y: y}}
		for i, flag := range []string{"app-position", "applications-position"} {
			if c.IsSet(flag) {
				p, err := parsePosition(c.String(flag))
				if err != nil {
					return fail(fmt.Errorf("--%s: %w", flag, err))
				}
				cfg.Contents[i].X, cfg.Contents[i].Y = p.X, p.Y
			}
		}
	}
	if err := cfg.Validate(); err != nil {
		return fail(err)
	}
	return cfg, f.App, nil
}
func parsePosition(value string) (position, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return position{}, fmt.Errorf("expected x,y")
	}
	x, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	y, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil || x < 0 || y < 0 {
		return position{}, fmt.Errorf("expected nonnegative integer coordinates x,y")
	}
	return position{x, y}, nil
}
