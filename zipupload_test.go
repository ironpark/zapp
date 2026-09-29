package zapp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// uploadServer records the path of every file PUT to it.
func uploadServer(t *testing.T) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(paths)
	}
}

// The app is notarized and stapled before it is archived and packaged, and
// the artifacts are uploaded last.
func TestZipAndUploadPipeline(t *testing.T) {
	dir := t.TempDir()
	srv, uploaded := uploadServer(t)
	t.Setenv("ZAPP_TEST_TOKEN", "secret")
	config := "version: 1\napp: Demo.app\nout: out\nsign: {}\nnotarize:\n  staple: true\ndmg: {}\nzip:\nchecksums:\nupload:\n  - url: " + srv.URL + "/${app.version}/${file.name}?sig=hidden\n    headers:\n      Authorization: Bearer ${env:ZAPP_TEST_TOKEN}\n"
	syntheticApp(t, dir)
	file := filepath.Join(dir, ".zapp.yaml")
	if err := os.WriteFile(file, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	b := &recordingBackend{}
	pl, err := p.Resolve(WithSigningBackend(b), WithNotarizationBackend(b))
	if err != nil {
		t.Fatal(err)
	}
	a, err := pl.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(b.events, ","), "sign:.app,submit:.zip,staple:.app,sign:.dmg,submit:.dmg,staple:.dmg"; got != want {
		t.Fatalf("pipeline = %s; want %s", got, want)
	}
	if a.Zip != filepath.Join(pl.project.Out, "Demo.zip") {
		t.Fatalf("zip = %s", a.Zip)
	}
	if _, err := os.Stat(a.Zip); err != nil {
		t.Fatal(err)
	}
	if got := uploaded(); !slices.Equal(got, []string{"/2.3/Demo.zip", "/2.3/Demo.dmg", "/2.3/SHA256SUMS"}) {
		t.Fatalf("uploaded %v", got)
	}
	// The checksums are of the final, stapled artifacts.
	sums, err := os.ReadFile(a.Checksums)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, path := range []string{a.Zip, a.DMG} {
		sum, err := sha256File(path)
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, fmt.Sprintf("%x  %s\n", sum, filepath.Base(path)))
	}
	if string(sums) != strings.Join(want, "") {
		t.Fatalf("checksums =\n%s", sums)
	}
	if len(a.Uploads) != 3 || a.Uploads[0].Artifact != "zip" || a.Uploads[0].URL != srv.URL+"/2.3/Demo.zip" {
		t.Fatalf("uploads = %+v", a.Uploads)
	}

	// The resolved configuration shows where credentials come from, never
	// the credentials.
	shown, err := pl.YAML()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(shown), "secret") || strings.Contains(string(shown), "hidden") || !strings.Contains(string(shown), "Authorization: <redacted>") || !strings.Contains(string(shown), "/2.3/${file.name}") {
		t.Fatalf("config show:\n%s", shown)
	}

	// Without stapling the app does not change, so its distribution archive
	// is what gets submitted, rather than archiving it twice.
	p.Notarize.Staple = false
	b.events = nil
	if pl, err = p.Resolve(WithSigningBackend(b), WithNotarizationBackend(b)); err != nil {
		t.Fatal(err)
	}
	if _, err = pl.Build(t.Context(), StepZip); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(b.events, ","); got != "sign:.app,submit:.zip" {
		t.Fatalf("unstapled pipeline = %s", got)
	}
}

// Only the named artifacts go to an endpoint; one that gets nothing fails
// rather than silently doing nothing.
func TestUploadSelection(t *testing.T) {
	dir := t.TempDir()
	srv, uploaded := uploadServer(t)
	headers := map[string]string{"Authorization": "Bearer secret"}
	p := &Project{App: syntheticApp(t, dir), Out: filepath.Join(dir, "out"), Zip: &ZipConfig{}, Upload: []UploadConfig{{URL: srv.URL + "/${file.name}", Headers: headers, Artifacts: []string{"zip", "pkg"}}}}
	pl, err := p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pl.Build(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := uploaded(); !slices.Equal(got, []string{"/Demo.zip"}) {
		t.Fatalf("uploaded %v", got)
	}
	p.Upload[0].Artifacts = []string{"dmg"}
	if pl, err = p.Resolve(); err != nil {
		t.Fatal(err)
	}
	var se *StepError
	if _, err := pl.Build(t.Context()); !errors.As(err, &se) || se.Step != StepUpload || !strings.Contains(err.Error(), "nothing to send") {
		t.Fatalf("empty upload: %v", err)
	}
	if _, err := pl.Build(t.Context(), StepUpload); !errors.As(err, &se) || se.Step != StepUpload {
		t.Fatalf("upload without artifacts: %v", err)
	}
	p.Checksums = &ChecksumsConfig{}
	if pl, err = p.Resolve(); err != nil {
		t.Fatal(err)
	}
	if _, err := pl.Build(t.Context(), StepChecksums); !errors.As(err, &se) || se.Step != StepChecksums {
		t.Fatalf("checksums without artifacts: %v", err)
	}
}

func TestUploadConfigValidation(t *testing.T) {
	for config, want := range map[string]string{
		"upload:\n  - url: https://example.com\n    headers:\n      Authorization: Bearer literal\n": "from the environment",
		"upload:\n  - url: https://example.com\n    headers:\n      X-Api-Key: abc\n":                "from the environment",
		"upload:\n  - url: https://example.com\n    artifacts: [app]\n":                              "cannot send",
		"upload:\n  - method: PUT\n": "requires url",
	} {
		if _, err := Parse(strings.NewReader("version: 1\n"+config), "."); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", config, err, want)
		}
	}
	p, err := Parse(strings.NewReader("version: 1\nzip:\nupload:\n  - url: https://example.com/${file.name}\n    headers:\n      Content-Type: application/zip\n      Authorization: Bearer ${env:TOKEN}\n"), ".")
	if err != nil {
		t.Fatal(err)
	}
	if p.Zip == nil || len(p.Upload) != 1 {
		t.Fatalf("parsed %+v", p)
	}
	if _, err := (&Project{Version: 1, Upload: []UploadConfig{{URL: "ftp://example.com"}}}).Resolve(); err == nil {
		t.Fatal("accepted an ftp upload")
	}
	if _, err := (&Project{Version: 1, Zip: &ZipConfig{}}).Resolve(); err == nil {
		t.Fatal("zip without an app")
	}
	// ${file.name} means something only in an upload URL.
	if _, err := (&Project{Version: 1, App: syntheticApp(t, t.TempDir()), DMG: &DMGConfig{Title: "${file.name}"}}).Resolve(); err == nil || !strings.Contains(err.Error(), "${file.name}") {
		t.Fatalf("file.name outside upload: %v", err)
	}
}

// A GitHub release defaults to the repository and tag of the Actions run,
// and a missing token stops the build before anything is built.
func TestGitHubReleaseUpload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHUB_REPOSITORY", "me/app")
	t.Setenv("GITHUB_REF_TYPE", "tag")
	t.Setenv("GITHUB_REF_NAME", "v2.3")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	p := &Project{App: syntheticApp(t, dir), Out: filepath.Join(dir, "out"), Zip: &ZipConfig{}, Upload: []UploadConfig{{GitHub: &GitHubRelease{}}}}
	pl, err := p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if g := pl.Uploads[0].GitHub; g.Repo != "me/app" || g.Tag != "v2.3" || pl.Uploads[0].Method != "" {
		t.Fatalf("github = %+v, method %q", g, pl.Uploads[0].Method)
	}
	var se *StepError
	if _, err := pl.Build(t.Context()); !errors.As(err, &se) || se.Step != StepUpload || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("no token: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "Demo.zip")); !os.IsNotExist(err) {
		t.Fatal("built before finding the token missing")
	}

	// Outside a tag build the tag has to be named.
	t.Setenv("GITHUB_REF_TYPE", "branch")
	if _, err := p.Resolve(); err == nil || !strings.Contains(err.Error(), "tag") {
		t.Fatalf("no tag: %v", err)
	}
	p.Upload[0].GitHub.Tag = "v${app.version}"
	if pl, err = p.Resolve(); err != nil || pl.Uploads[0].GitHub.Tag != "v2.3" {
		t.Fatalf("tag = %v, %v", pl, err)
	}
}

func TestGitHubUploadValidation(t *testing.T) {
	for _, config := range []string{
		"upload:\n  - github: {}\n    url: https://example.com/\n",
		"upload:\n  - github: {}\n    method: POST\n",
		"upload:\n  - artifacts: [zip]\n",
	} {
		if _, err := Parse(strings.NewReader("version: 1\n"+config), t.TempDir()); err == nil {
			t.Errorf("accepted:\n%s", config)
		}
	}
	if _, err := Parse(strings.NewReader("version: 1\nupload:\n  - github: {repo: me/app, tag: v1, draft: true}\n"), t.TempDir()); err != nil {
		t.Error(err)
	}
}

// A dry run lists what Build does, in Build's order, and does none of it.
func TestDryRun(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ZAPP_TEST_TOKEN", "secret")
	p := &Project{
		App: syntheticApp(t, dir), Out: filepath.Join(dir, "out"),
		Sign: &SignConfig{P12File: "cert.p12", P12Password: "hunter2", Entitlements: "app.entitlements"}, Notarize: &NotarizeConfig{APIKeyFile: "key.json", Staple: true},
		Zip: &ZipConfig{}, DMG: &DMGConfig{}, Checksums: &ChecksumsConfig{},
		Upload: []UploadConfig{{URL: "https://user:pw@example.com/${file.name}?sig=hidden", Headers: map[string]string{"Authorization": "Bearer ${env:ZAPP_TEST_TOKEN}"}}},
	}
	b := &recordingBackend{}
	pl, err := p.Resolve(WithSigningBackend(b), WithNotarizationBackend(b))
	if err != nil {
		t.Fatal(err)
	}
	actions, err := pl.DryRun()
	if err != nil {
		t.Fatal(err)
	}
	var steps, text []string
	for _, a := range actions {
		steps = append(steps, string(a.Step))
		text = append(text, a.What)
	}
	if got := strings.Join(steps, " "); got != "sign notarize zip dmg sign notarize checksums upload" {
		t.Fatalf("steps = %s", got)
	}
	all := strings.Join(text, "\n")
	for _, secret := range []string{"hunter2", "secret", "hidden", "pw@"} {
		if strings.Contains(all, secret) {
			t.Errorf("dry run shows %q:\n%s", secret, all)
		}
	}
	if !strings.Contains(text[0], "app.entitlements") || strings.Contains(text[4], "entitlements") || !strings.Contains(text[1], "then staple") {
		t.Errorf("descriptions:\n%s", all)
	}
	if len(b.events) != 0 {
		t.Fatalf("dry run signed or notarized: %v", b.events)
	}
	if _, err := os.Stat(filepath.Join(dir, "out")); !os.IsNotExist(err) {
		t.Fatal("dry run built something")
	}
	if _, err := pl.DryRun(StepPKG); err == nil {
		t.Fatal("dry run accepted an unconfigured step")
	}
}

// A framework checked out without its links stops the build, and the dry
// run, before anything is built.
func TestBrokenFrameworkLinks(t *testing.T) {
	dir := t.TempDir()
	app := syntheticApp(t, dir)
	fw := filepath.Join(app, "Contents", "Frameworks", "Lib.framework")
	if err := os.MkdirAll(filepath.Join(fw, "Versions", "A"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fw, "Versions", "Current"), []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Project{App: app, Out: filepath.Join(dir, "out"), Zip: &ZipConfig{}}
	pl, err := p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pl.DryRun(); err == nil || !strings.Contains(err.Error(), "Versions/Current is a text file") {
		t.Fatalf("dry run: %v", err)
	}
	if _, err := pl.Build(t.Context()); err == nil || !strings.Contains(err.Error(), "symbolic links") {
		t.Fatalf("build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out")); !os.IsNotExist(err) {
		t.Fatal("built from a broken app")
	}
}

// Verification runs once the artifacts are final and stops the build before
// anything is listed or published.
func TestVerifyStep(t *testing.T) {
	dir := t.TempDir()
	srv, uploaded := uploadServer(t)
	p := &Project{App: syntheticApp(t, dir), Out: filepath.Join(dir, "out"), Zip: &ZipConfig{}, Checksums: &ChecksumsConfig{}, Verify: true,
		Upload: []UploadConfig{{URL: srv.URL + "/${file.name}"}}}
	pl, err := p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	actions, err := pl.DryRun()
	if err != nil {
		t.Fatal(err)
	}
	var steps []Step
	for _, a := range actions {
		steps = append(steps, a.Step)
	}
	if !slices.Equal(steps, []Step{StepZip, StepVerify, StepChecksums, StepUpload}) {
		t.Fatalf("steps %v", steps)
	}
	var se *StepError
	if _, err := pl.Build(t.Context()); !errors.As(err, &se) || se.Step != StepVerify || !strings.Contains(err.Error(), "Demo.app: bundle") {
		t.Fatalf("build: %v", err)
	}
	if got := uploaded(); len(got) != 0 {
		t.Fatalf("uploaded %v from an app that failed verification", got)
	}
}
