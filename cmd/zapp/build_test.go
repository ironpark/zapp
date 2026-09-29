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

// Artifact paths are appended as absolute key=value lines, so the file can be
// $GITHUB_OUTPUT itself, and a step that was not built is empty.
func TestWriteArtifacts(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(cwd, "output")
	if err := os.WriteFile(file, []byte("earlier=kept\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	uploads := []zapp.Uploaded{{Artifact: "dmg", URL: "https://example.com/MyApp.dmg"}, {Artifact: "dmg", URL: "https://mirror.example.com/MyApp.dmg"}}
	if err := writeArtifacts(file, zapp.Artifacts{App: "MyApp.app", DMG: filepath.Join("dist", "MyApp.dmg"), Uploads: uploads}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	want := "earlier=kept\n" +
		"app=" + filepath.Join(cwd, "MyApp.app") + "\n" +
		"zip=\n" +
		"dmg=" + filepath.Join(cwd, "dist", "MyApp.dmg") + "\n" +
		"pkg=\n" +
		"checksums=\n" +
		"appcast=\n" +
		"homebrew=\n" +
		"dmg-url=https://example.com/MyApp.dmg\n"
	if string(data) != want {
		t.Fatalf("output =\n%s\nwant\n%s", data, want)
	}
}

// The endpoint on the command line or in ZAPP_UPLOAD_* joins the project's,
// --zip turns archiving on, and --no-upload drops every endpoint.
func TestUploadOverlay(t *testing.T) {
	run := func(args ...string) *zapp.Project {
		t.Helper()
		var got *zapp.Project
		c := buildCommand()
		c.Writer, c.ErrWriter = io.Discard, io.Discard
		c.Action = func(_ context.Context, c *cli.Command) error {
			p, err := loadProject(c, "build")
			got = p
			return err
		}
		if err := c.Run(t.Context(), append([]string{"build", "--no-config"}, args...)); err != nil {
			t.Fatal(err)
		}
		return got
	}
	t.Setenv("ZAPP_UPLOAD_URL", "https://example.com/${file.name}")
	t.Setenv("ZAPP_UPLOAD_HEADER", "Authorization: Bearer literal-token\nX-Channel: beta")
	t.Setenv("ZAPP_UPLOAD_ARTIFACTS", "zip,dmg")
	p := run("--zip")
	if p.Zip == nil || len(p.Upload) != 1 {
		t.Fatalf("project = %+v", p)
	}
	u := p.Upload[0]
	if u.URL != "https://example.com/${file.name}" || u.Headers["Authorization"] != "Bearer literal-token" || u.Headers["X-Channel"] != "beta" || strings.Join(u.Artifacts, ",") != "zip,dmg" {
		t.Fatalf("upload = %+v", u)
	}
	if p := run("--github-release", "v1.0.0", "--github-repo", "me/app"); len(p.Upload) != 2 || p.Upload[1].GitHub == nil || *p.Upload[1].GitHub != (zapp.GitHubRelease{Repo: "me/app", Tag: "v1.0.0"}) || strings.Join(p.Upload[1].Artifacts, ",") != "zip,dmg" {
		t.Fatalf("--github-release = %+v", p.Upload)
	}
	if p := run("--checksums"); p.Checksums == nil {
		t.Fatal("--checksums did not turn checksums on")
	}
	if p := run("--no-upload"); p.Upload != nil {
		t.Fatal("--no-upload kept an endpoint")
	}
	if p := run("--upload-method", "POST", "--upload-header", "Authorization: Bearer flag"); p.Upload[0].Method != "POST" || p.Upload[0].Headers["Authorization"] != "Bearer flag" {
		t.Fatalf("flags lost to the environment: %+v", p.Upload[0])
	}
}

// Named steps gain zip and upload when the command line asks for them.
func TestBuildStepsFollowFlags(t *testing.T) {
	steps := func(args ...string) string {
		t.Helper()
		var got []zapp.Step
		c := buildCommand()
		c.Writer, c.ErrWriter = io.Discard, io.Discard
		c.Action = func(_ context.Context, c *cli.Command) error {
			p, err := loadProject(c, "build")
			got = buildSteps(c, p)
			return err
		}
		if err := c.Run(t.Context(), append([]string{"build", "--no-config"}, args...)); err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, s := range got {
			names = append(names, string(s))
		}
		return strings.Join(names, " ")
	}
	if got := steps("dmg"); got != "dmg" {
		t.Fatalf("plain = %q", got)
	}
	if got := steps("--zip", "--upload-url", "https://example.com/${file.name}", "dmg"); got != "dmg zip upload" {
		t.Fatalf("flags = %q", got)
	}
	if got := steps("--github-release", "v1", "dmg"); got != "dmg upload" {
		t.Fatalf("--github-release = %q", got)
	}
	if got := steps("--checksums", "dmg"); got != "dmg checksums" {
		t.Fatalf("--checksums = %q", got)
	}
	if got := steps("--upload-url", "https://example.com/", "--no-upload", "pkg"); got != "pkg" {
		t.Fatalf("--no-upload = %q", got)
	}
	t.Setenv("ZAPP_NO_UPLOAD", "true")
	if got := steps("--upload-url", "https://example.com/", "pkg"); got != "pkg" {
		t.Fatalf("ZAPP_NO_UPLOAD = %q", got)
	}
	t.Setenv("ZAPP_ZIP", "1")
	if got := steps(); got != "" {
		t.Fatalf("no steps = %q", got)
	}
	if got := steps("pkg"); got != "pkg zip" {
		t.Fatalf("env = %q", got)
	}
}
