package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ironpark/zapp/internal/hdiutil"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/ironpark/zapp/pkg/icns"
	"github.com/urfave/cli/v3"
)

func TestFSFlag(t *testing.T) {
	for _, value := range []string{"", "hfsplus", "apfs", "APFS", "apfs-case-sensitive"} {
		t.Run("value="+value, func(t *testing.T) {
			var filesystemFlag cli.Flag
			for _, flag := range dmgCommand.Flags {
				if flag.Names()[0] == "fs" {
					copyOf := *(flag.(*cli.StringFlag))
					filesystemFlag = &copyOf
				}
			}
			if filesystemFlag == nil {
				t.Fatal("missing fs flag")
			}
			called := false
			command := &cli.Command{Writer: io.Discard, ErrWriter: io.Discard, Flags: []cli.Flag{filesystemFlag}, Action: func(context.Context, *cli.Command) error { called = true; return nil }}
			args := []string{"dmg"}
			if value != "" {
				args = append(args, "--fs", value)
			}
			err := command.Run(context.Background(), args)
			if err != nil || !called {
				t.Fatalf("run: called=%v, %v", called, err)
			}
			want := value
			if want == "" {
				want = "hfsplus"
			}
			if strings.ToLower(command.String("fs")) != strings.ToLower(want) {
				t.Fatalf("filesystem %q != %q", command.String("fs"), want)
			}
		})
	}
}

// Clone flag definitions because cli flags keep parsing state between runs.
func testCommand() *cli.Command {
	c := *dmgCommand
	c.Writer, c.ErrWriter = io.Discard, io.Discard
	c.Flags = nil
	for _, flag := range dmgCommand.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			copy := *f
			c.Flags = append(c.Flags, &copy)
		case *cli.IntFlag:
			copy := *f
			c.Flags = append(c.Flags, &copy)
		case *cli.BoolFlag:
			copy := *f
			c.Flags = append(c.Flags, &copy)
		default:
			panic("unhandled test flag type")
		}
	}
	return &c
}

func TestPNGIconAndOutputExtension(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "Demo.app")
	if err := os.Mkdir(app, 0755); err != nil {
		t.Fatal(err)
	}
	src := image.NewNRGBA(image.Rect(0, 0, 32, 16))
	for y := range 16 {
		for x := range 32 {
			src.SetNRGBA(x, y, color.NRGBA{R: 200, A: 255})
		}
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, src); err != nil {
		t.Fatal(err)
	}
	for _, extension := range []string{".png", ".PNG"} {
		t.Run(extension, func(t *testing.T) {
			iconPath := filepath.Join(dir, "icon"+extension)
			if err := os.WriteFile(iconPath, pngData.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "output"+extension)
			// Exercise both omitted and supplied DMG extensions.
			if extension == ".PNG" {
				output += ".dmg"
			}
			var log bytes.Buffer
			c := testCommand()
			c.Writer = &log
			if err := c.Run(t.Context(), []string{"dmg", "--app", app, "--icon", iconPath, "--out", output}); err != nil {
				t.Fatal(err)
			}
			want := output
			if !strings.HasSuffix(want, ".dmg") {
				want += ".dmg"
			}
			if _, err := os.Stat(want); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(log.String(), want) {
				t.Fatalf("output path was not normalized in logs: %s", &log)
			}
			if c.String("icon") != iconPath || c.String("out") != output {
				t.Fatal("generation changed flag values")
			}
			if got, err := os.ReadFile(iconPath); err != nil || !bytes.Equal(got, pngData.Bytes()) {
				t.Fatal("PNG input was changed")
			}
			if runtime.GOOS != "darwin" {
				return
			}
			mount := filepath.Join(t.TempDir(), "mounted")
			// The claim covers the whole time the image stays attached, not
			// just the attach itself, so no other test binary has an image
			// mounted meanwhile.
			release := hdiutil.Lock()
			if out, err := hdiutil.Run("attach", "-readonly", "-nobrowse", "-mountpoint", mount, want); err != nil {
				release()
				t.Fatalf("mount: %v\n%s", err, out)
			}
			defer func() {
				defer release()
				if out, err := hdiutil.Run("detach", mount); err != nil {
					t.Errorf("detach: %v\n%s", err, out)
				}
			}()
			data, err := os.ReadFile(filepath.Join(mount, ".VolumeIcon.icns"))
			if err != nil {
				t.Fatal(err)
			}
			family, err := icns.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			img, err := family.HighestResolution()
			if err != nil {
				t.Fatalf("DMG icon is not valid ICNS: %v", err)
			}
			if _, _, _, a := img.At(256, 0).RGBA(); a != 0 {
				t.Fatal("icon was stretched instead of padded")
			}
			if r, _, _, a := img.At(256, 256).RGBA(); r == 0 || a != 65535 {
				t.Fatal("icon center lost its artwork")
			}
		})
	}
}

func TestInvalidPNGDoesNotCreateDMG(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "Demo.app")
	if err := os.Mkdir(app, 0755); err != nil {
		t.Fatal(err)
	}
	iconPath := filepath.Join(dir, "broken.png")
	if err := os.WriteFile(iconPath, []byte("not PNG"), 0644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "output.dmg")
	err := testCommand().Run(t.Context(), []string{"dmg", "--app", app, "--icon", iconPath, "--out", output})
	if err == nil || !strings.Contains(err.Error(), "PNG icon") {
		t.Fatalf("expected PNG error, got %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output created for invalid PNG: %v", err)
	}
}

func TestInvalidFileSystemRejected(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "Demo.app")
	if err := os.Mkdir(app, 0755); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "output.dmg")
	err := testCommand().Run(t.Context(), []string{"dmg", "--app", app, "--fs", "ntfs", "--out", output})
	if err == nil || !strings.Contains(err.Error(), "unknown filesystem") {
		t.Fatalf("expected filesystem error, got %v", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output created for invalid filesystem: %v", err)
	}
}

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
	if cfg.WindowWidth != 800 || cfg.WindowHeight != 460 || cfg.ContentsIconSize != 96 || cfg.LabelSize != 14 || cfg.FileName != filepath.Join(mustCwd(t), "release.dmg") || cfg.Title != "Override" {
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

func mustCwd(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
