package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironpark/zapp"
	"github.com/urfave/cli/v3"
)

func TestOverlayPrecedence(t *testing.T) {
	t.Setenv("ZAPP_TITLE", "Environment")
	t.Setenv("ZAPP_WINDOW_WIDTH", "800")
	t.Setenv("ZAPP_ICON", "env.icns")
	t.Setenv("ZAPP_PASSWORD", "secret")
	t.Setenv("ZAPP_P12_PASSWORD", "private")
	for _, args := range [][]string{{}, {"--title", "Flag", "--window-width", "900", "--icon", "flag.icns"}} {
		p := &zapp.Project{Version: 1, DMG: &zapp.DMGConfig{Title: "File", Window: zapp.Window{Width: 700}}, Sign: &zapp.SignConfig{Identity: "File identity"}, Notarize: &zapp.NotarizeConfig{Profile: "File profile"}}
		c := buildCommand()
		c.Writer, c.ErrWriter = io.Discard, io.Discard
		c.Action = func(_ context.Context, c *cli.Command) error { return overlayProject(c, p, "dmg") }
		if err := c.Run(t.Context(), append([]string{"build"}, args...)); err != nil {
			t.Fatal(err)
		}
		title, width, icon := "Environment", 800, "env.icns"
		if len(args) > 0 {
			title, width, icon = "Flag", 900, "flag.icns"
		}
		absolute, _ := filepath.Abs(icon)
		if p.DMG.Title != title || p.DMG.Window.Width != width || p.DMG.Icon != absolute {
			t.Fatal(p.DMG)
		}
		if p.Sign.Identity != "File identity" || p.Sign.P12Password != "private" || p.Notarize.Password != "secret" {
			t.Fatal("lost file credentials or env passwords")
		}
	}
}
func TestDiscoveryNoConfigAndSkip(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(".zapp.yaml", []byte("version: 1\nsign: {}\nnotarize: {}\ndmg: {title: Project}"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args         []string
		sign, notary bool
		title        string
	}{
		{nil, true, true, "Project"}, {[]string{"--no-sign", "--no-notarize"}, false, false, "Project"}, {[]string{"--no-config"}, false, false, ""}, {[]string{"--no-config", "--sign", "--notarize", "--profile", "test", "--staple"}, true, true, ""},
	} {
		c := buildCommand()
		c.Writer, c.ErrWriter = io.Discard, io.Discard
		c.Action = func(_ context.Context, c *cli.Command) error {
			p, err := loadProject(c, "dmg")
			if err != nil {
				return err
			}
			if (p.Sign != nil) != tc.sign || (p.Notarize != nil) != tc.notary || p.DMG.Title != tc.title {
				t.Fatalf("wrong overlay: %+v", p)
			}
			return nil
		}
		if err := c.Run(t.Context(), append([]string{"build"}, tc.args...)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCLIPathsRelativeToCwd(t *testing.T) {
	dir := t.TempDir()
	fileDir := filepath.Join(dir, "project")
	if err := os.Mkdir(fileDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	file := filepath.Join(fileDir, ".zapp.yaml")
	if err := os.WriteFile(file, []byte("version: 1\ndmg: {title: Files, out: file.dmg, contents: {/Applications: {link: true, pos: [0, 0]}}}"), 0644); err != nil {
		t.Fatal(err)
	}
	c := buildCommand()
	c.Writer, c.ErrWriter = io.Discard, io.Discard
	c.Action = func(_ context.Context, c *cli.Command) error {
		p, err := loadProject(c, "dmg")
		if err != nil {
			return err
		}
		pl, err := p.Resolve()
		if err != nil {
			return err
		}
		if pl.DMG.FileName != filepath.Join(dir, "flag.dmg") {
			t.Fatal(pl.DMG.FileName)
		}
		return nil
	}
	if err := c.Run(t.Context(), []string{"build", "--config", file, "--out", "flag.dmg"}); err != nil {
		t.Fatal(err)
	}
}
func TestInitAndConfigShow(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("Demo.app/Contents", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("Demo.app/Contents/Info.plist", []byte(`<plist><dict><key>CFBundleIdentifier</key><string>dev.demo</string><key>CFBundleShortVersionString</key><string>1.2</string></dict></plist>`), 0644); err != nil {
		t.Fatal(err)
	}
	runInit := func(args ...string) error {
		c := initCommand()
		c.Writer, c.ErrWriter = io.Discard, io.Discard
		return c.Run(t.Context(), append([]string{"init", "--app", "Demo.app"}, args...))
	}
	if err := runInit(); err != nil {
		t.Fatal(err)
	}
	if err := runInit(); err == nil {
		t.Fatal("overwrote existing config")
	}
	if err := runInit("--force"); err != nil {
		t.Fatal(err)
	}
	p, err := zapp.Load(".zapp.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if p.PKG.Identifier != "dev.demo" || p.PKG.Version != "1.2" || p.DMG.Title != "Demo" {
		t.Fatal(p)
	}
	var out strings.Builder
	c := configCommand()
	c.Writer = &out
	c.ErrWriter = io.Discard
	if err = c.Run(t.Context(), []string{"config", "show"}); err != nil {
		t.Fatal(err)
	}
	resolved, err := zapp.Parse(strings.NewReader(out.String()), dir)
	if err != nil {
		t.Fatalf("invalid config show: %s: %v", out.String(), err)
	}
	if !filepath.IsAbs(resolved.App) || resolved.PKG.InstallLocation != "/Applications" {
		t.Fatal(out.String())
	}
}

func TestNamedStepsWithoutFile(t *testing.T) {
	c := buildCommand()
	c.Writer, c.ErrWriter = io.Discard, io.Discard
	c.Action = func(_ context.Context, c *cli.Command) error {
		p, err := loadProject(c, "build")
		if err != nil {
			return err
		}
		if p.DMG == nil || p.PKG == nil || p.Dep != nil || p.DMG.Title != "Named" {
			t.Fatalf("named step defaults/overlay: %+v", p)
		}
		return nil
	}
	if err := c.Run(t.Context(), []string{"build", "--no-config", "--title", "Named", "dmg", "pkg"}); err != nil {
		t.Fatal(err)
	}
}
