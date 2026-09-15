package dmg

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestFSFlag(t *testing.T) {
	for _, value := range []string{"", "hfsplus", "apfs", "APFS", "apfs-case-sensitive", "ntfs"} {
		t.Run("value="+value, func(t *testing.T) {
			var filesystemFlag cli.Flag
			for _, flag := range Command.Flags {
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
			if value == "ntfs" {
				if err == nil || called {
					t.Fatal("invalid filesystem reached the action")
				}
				return
			}
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
	c := *Command
	c.Writer, c.ErrWriter = io.Discard, io.Discard
	c.Flags = nil
	for _, flag := range Command.Flags {
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
			if out, err := exec.Command("hdiutil", "attach", "-readonly", "-nobrowse", "-mountpoint", mount, want).CombinedOutput(); err != nil {
				t.Fatalf("mount: %v\n%s", err, out)
			}
			defer func() {
				if out, err := exec.Command("hdiutil", "detach", mount).CombinedOutput(); err != nil {
					t.Errorf("detach: %v\n%s", err, out)
				}
			}()
			data, err := os.ReadFile(filepath.Join(mount, ".VolumeIcon.icns"))
			if err != nil {
				t.Fatal(err)
			}
			img, err := readIcnsFromBytes(data)
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
