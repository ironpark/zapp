package dmg

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestFilesystemFlag(t *testing.T) {
	for _, value := range []string{"", "hfsplus", "apfs", "APFS", "apfs-case-sensitive", "ntfs"} {
		t.Run("value="+value, func(t *testing.T) {
			var filesystemFlag cli.Flag
			for _, flag := range Command.Flags {
				if flag.Names()[0] == "filesystem" {
					copyOf := *(flag.(*cli.StringFlag))
					filesystemFlag = &copyOf
				}
			}
			if filesystemFlag == nil {
				t.Fatal("missing filesystem flag")
			}
			called := false
			command := &cli.Command{Writer: io.Discard, ErrWriter: io.Discard, Flags: []cli.Flag{filesystemFlag}, Action: func(context.Context, *cli.Command) error { called = true; return nil }}
			args := []string{"dmg"}
			if value != "" {
				args = append(args, "--filesystem", value)
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
			if strings.ToLower(filesystem) != strings.ToLower(want) {
				t.Fatalf("filesystem %q != %q", filesystem, want)
			}
		})
	}
}
