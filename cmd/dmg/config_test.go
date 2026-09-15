package dmg

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/urfave/cli/v3"
)

func resolveForTest(t *testing.T, content string, args ...string) (dmg.Config, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(dir, "dmg.yaml")
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	c := testCommand()
	var cfg dmg.Config
	c.Action = func(_ context.Context, c *cli.Command) error {
		var err error
		cfg, _, err = resolveConfig(c)
		return err
	}
	err := c.Run(t.Context(), append([]string{"dmg", "--config", name}, args...))
	return cfg, err
}

func TestConfigPrecedenceAndPaths(t *testing.T) {
	cfg, err := resolveForTest(t, `version: 1
title: Files
out: archive
window: {width: 720, height: 460}
iconSize: 96
contents:
  readme.txt:
    name: 사용 안내.txt
    x: 0
    y: 120
  /Applications:
    link: true
    name: Install
    x: 500
    y: 120
`, "--ww", "800", "--out", "release", "--title", "Override")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WindowWidth != 800 || cfg.WindowHeight != 460 || cfg.ContentsIconSize != 96 || cfg.LabelSize != 14 || cfg.FileName != "release.dmg" || cfg.Title != "Override" {
		t.Fatalf("bad merged config: %+v", cfg)
	}
	if len(cfg.Contents) != 2 || cfg.Contents[1].X != 0 || cfg.Contents[1].Name != "사용 안내.txt" || cfg.Contents[0].Path != "/Applications" {
		t.Fatalf("bad contents: %+v", cfg.Contents)
	}
	if _, err := os.Stat(cfg.Contents[1].Path); err != nil {
		t.Fatal(err)
	}
	cfg.FileName = filepath.Join(t.TempDir(), "files.dmg")
	if err := dmg.CreateDMG(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
}

func TestJSONConfig(t *testing.T) {
	_, err := resolveForTest(t, `{"version":1,"title":"Files","contents":{"/Applications":{"link":true,"x":0,"y":0}}}`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInvalidConfig(t *testing.T) {
	base := "version: 1\ntitle: Files\ncontents:\n  readme.txt:\n    x: 100\n    y: 100\n"
	for name, input := range map[string]string{
		"unknown":        base + "typo: true\n",
		"version":        strings.Replace(base, "version: 1", "version: 2", 1),
		"duplicate key":  base + "title: Other\n",
		"duplicate path": base + "  readme.txt: {x: 0, y: 0}\n",
		"empty":          "version: 1\ntitle: Files\ncontents: {}\n",
		"size":           base + "iconSize: 0\n",
		"negative":       strings.Replace(base, "x: 100", "x: -1", 1),
		"missing x":      strings.Replace(base, "    x: 100\n", "", 1),
		"missing y":      strings.Replace(base, "    y: 100\n", "", 1),
		"reserved":       base + "    name: .DS_Store\n",
		"traversal":      base + "    name: ../escape\n",
		"missing source": strings.Replace(base, "readme.txt", "missing", 1),
		"old type":       base + "    type: file\n",
		"duplicate name": base + "  /README.txt: {link: true, x: 200, y: 100}\n",
		"invalid link":   base + "    link: invalid\n",
		"empty path":     strings.Replace(base, "readme.txt", `""`, 1),
		"null item":      "version: 1\ntitle: Files\ncontents: {readme.txt: null}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := resolveForTest(t, input); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	if _, err := resolveForTest(t, base, "--app-position", "1,2"); err == nil {
		t.Fatal("expected position conflict")
	}
}

func TestPositionFlags(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Demo.app")
	if err := os.Mkdir(app, 0755); err != nil {
		t.Fatal(err)
	}
	cfg, err := resolveForTest(t, "version: 1\n", "--app", app, "--app-position", "0,200", "--applications-position", "500,200")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Contents[0].X != 0 || cfg.Contents[0].Y != 200 || cfg.Contents[1].X != 500 {
		t.Fatal(cfg.Contents)
	}
	for _, value := range []string{"1", "a,b", "-1,2", "1,2,3"} {
		if _, err := parsePosition(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

func TestContentsMapTypesAndOrder(t *testing.T) {
	cfg, err := resolveForTest(t, `version: 1
title: Files
contents:
  readme.txt: {link: false, x: 10, y: 20}
  .: {name: Bundle, x: 30, y: 40}
  relative-target: {link: true, x: 0, y: 0}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Contents) != 3 || cfg.Contents[0].Type != dmg.Dir || cfg.Contents[1].Type != dmg.File || cfg.Contents[2].Type != dmg.Link {
		t.Fatalf("incorrect types/order: %+v", cfg.Contents)
	}
	if cfg.Contents[2].Path != "relative-target" || cfg.Contents[2].X != 0 || cfg.Contents[2].Y != 0 {
		t.Fatalf("link target or origin changed: %+v", cfg.Contents[2])
	}
}
