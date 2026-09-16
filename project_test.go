package zapp

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ironpark/zapp/pkg/macpkg"
)

func syntheticApp(t *testing.T, dir string) string {
	t.Helper()
	app := filepath.Join(dir, "Demo.app")
	if err := os.MkdirAll(filepath.Join(app, "Contents", "Resources"), 0755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>dev.zapp.demo</string><key>CFBundleShortVersionString</key><string>2.3</string><key>CFBundleExecutable</key><string>Demo</string><key>CFBundleIconFile</key><string>AppIcon</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0644); err != nil {
		t.Fatal(err)
	}
	icon, err := os.ReadFile("iconfile.icns")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(app, "Contents", "Resources", "AppIcon.icns"), icon, 0644); err != nil {
		t.Fatal(err)
	}
	return app
}
func TestParseStrict(t *testing.T) {
	for _, body := range []string{
		"version: 1\n---\nversion: 1", "version: 2", "version: 1\nwat: true", "version: 1\nversion: 1", "version: 1\nsign: {p12Password: secret}", "version: 1\nnotarize: {password: secret}",
		"version: 1\npkg: {components: [{id: a, typo: b}]}", "version: 1\npkg: {license: {en: a, en: b}}", "version: 1\ndmg: {contents: {a: {x: 0, y: 1}, a: {x: 1, y: 2}}}",
		"version: 1\npkg: {identifier: a, components: [{id: b}]}", "version: 1\npkg: {distribution: {title: hi}}",
		`{"version":1,"notarize":{"password":"secret"}}`, "version: 1\ndmg: {window: {width: 0}}",
	} {
		t.Run(body, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(body), t.TempDir()); err == nil {
				t.Fatalf("accepted %s", body)
			}
		})
	}
	for _, body := range []string{"version: 1\npkg: {license: license.txt}", `{"version":1,"pkg":{"license":{"default":"license.txt","ko":"ko.txt"}}}`, "version: 1\nsign: {}\nnotarize: {}"} {
		if _, err := Parse(strings.NewReader(body), t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
}
func TestResolvePathsSubstitutionAndClock(t *testing.T) {
	dir := t.TempDir()
	app := syntheticApp(t, dir)
	t.Setenv("ZAPP_TEST_ICON", "Demo.app/Contents/Resources/AppIcon.icns")
	p, err := Parse(strings.NewReader(`version: 1
app: Demo.app
out: artifacts
sign: {p12File: cert.p12}
notarize: {apiKeyFile: key.json, staple: true}
dep: {libs: [lib]}
dmg:
  icon: ${env:ZAPP_TEST_ICON}
  out: ${app.name}-${app.version}.dmg
  contents:
    ${app}: {x: 10, y: 20}
    /Applications: {link: true, x: 300, y: 20}
pkg:
  scripts: scripts
  license: {default: eula.txt, ko: ko.txt}
`), dir)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Unix(1234567890, 0)
	pl, err := p.Resolve(WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	if pl.App != app || pl.DMG.FileName != filepath.Join(dir, "Demo-2.3.dmg") || pl.DMG.Created != clock || pl.PKG.App.Created != clock {
		t.Fatalf("bad resolved plan: %+v", pl)
	}
	if pl.PKG.App.OutputPath != filepath.Join(dir, "artifacts", "Demo.pkg") || pl.PKG.App.Identifier != "dev.zapp.demo" || pl.PKG.App.Version != "2.3" || pl.PKG.App.InstallLocation != "/Applications" {
		t.Fatal(pl.PKG.App)
	}
	if pl.SignCredentials.P12File != filepath.Join(dir, "cert.p12") || pl.NotarizeCredentials.APIKeyFile != filepath.Join(dir, "key.json") || pl.Dep.Libs[0] != filepath.Join(dir, "lib") {
		t.Fatal(pl)
	}
	if p.App != "Demo.app" || p.PKG.Scripts != "scripts" {
		t.Fatal("Resolve mutated source project")
	}
	p.Sign.P12Password = "secret"
	p.Notarize.Password = "private"
	pl, err = p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	b, err := pl.YAML()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("secret")) || bytes.Contains(b, []byte("private")) {
		t.Fatal("secret leaked")
	}
	p.DMG.Title = "${env:ZAPP_UNSET_TEST}"
	t.Setenv("ZAPP_UNSET_TEST", "unused")
	os.Unsetenv("ZAPP_UNSET_TEST")
	if _, err = p.Resolve(); err == nil || !strings.Contains(err.Error(), "ZAPP_UNSET_TEST") {
		t.Fatal(err)
	}
}
func TestDiscoverAndLoad(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Discover(nested); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	file := filepath.Join(dir, ".zapp.yaml")
	if err := os.WriteFile(file, []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(nested)
	if err != nil || got != file {
		t.Fatalf("%s %v", got, err)
	}
	if _, err = Load(file); err != nil {
		t.Fatal(err)
	}
}
func TestLegacy(t *testing.T) {
	dir := t.TempDir()
	p, err := Parse(strings.NewReader("version: 1\ntitle: Legacy\nout: legacy\ncontents: {relative: {link: true, x: 0, y: 0}}"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Legacy() {
		t.Fatal("not detected")
	}
	pl, err := p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	out, _ := filepath.Abs("legacy.dmg")
	if pl.DMG.FileName != out || pl.DMG.Contents[0].Path != "relative" {
		t.Fatal(pl.DMG)
	}
	p, err = Parse(strings.NewReader("version: 1\napp: foo.app"), dir)
	if err != nil || !p.Legacy() {
		t.Fatalf("shared-only project: %v", err)
	}
}
func TestFullPKG(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "payload"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "payload", "hello"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := Parse(strings.NewReader(`version: 1
out: dist
pkg:
  components:
    - {id: dev.demo.a, root: payload, installLocation: /opt/demo}
    - {id: dev.demo.b, root: payload, installLocation: /opt/other}
  distribution:
    title: Demo
    choices:
      - {id: main, title: Main, packages: [dev.demo.a], selected: true, visible: true}
      - {id: optional, packages: [dev.demo.b], selected: false, visible: true}
`), dir)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if pl.PKG.App != nil || len(pl.PKG.Components) != 2 || pl.PKG.Product.Distribution.Choices[1].Selected {
		t.Fatal(pl.PKG)
	}
	out, err := pl.BuildPKG(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(out); err != nil || info.Size() == 0 {
		t.Fatalf("artifact: %v", err)
	}
	p.PKG.Type = "component"
	if _, err = p.Resolve(); err == nil {
		t.Fatal("accepted multiple components for component type")
	}
}
func TestBuildArtifactsAndShortFormEquivalence(t *testing.T) {
	dir := t.TempDir()
	syntheticApp(t, dir)
	file := filepath.Join(dir, ".zapp.yaml")
	if err := os.WriteFile(file, []byte("version: 1\napp: Demo.app\nout: artifacts\ndmg: {}\npkg: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := p.Resolve(WithClock(time.Unix(1234567890, 0)))
	if err != nil {
		t.Fatal(err)
	}
	a, err := pl.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{a.DMG, a.PKG} {
		if info, err := os.Stat(name); err != nil || info.Size() == 0 {
			t.Fatalf("missing artifact %s: %v", name, err)
		}
	}
	old := macpkg.AppConfig{AppPath: pl.App, OutputPath: filepath.Join(dir, "direct.pkg"), Identifier: "dev.zapp.demo", Version: "2.3", Created: time.Unix(1234567890, 0)}
	if err = macpkg.BuildApp(t.Context(), old); err != nil {
		t.Fatal(err)
	}
	expected, _ := os.ReadFile(old.OutputPath)
	actual, _ := os.ReadFile(a.PKG)
	if !bytes.Equal(expected, actual) {
		t.Fatal("short form differs from engine AppConfig output")
	}
}
func TestStepError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pl := &Plan{}
	_, err := pl.BuildDMG(ctx)
	var se *StepError
	if !errors.As(err, &se) || se.Step != StepDMG {
		t.Fatal(err)
	}
	p := &Project{App: syntheticApp(t, t.TempDir()), DMG: &DMGConfig{}}
	pl, err = p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	_, err = pl.BuildDMG(ctx)
	if !errors.Is(err, context.Canceled) || !errors.As(err, &se) {
		t.Fatal(err)
	}
	_, err = pl.Build(t.Context(), Step("invalid"))
	if !errors.As(err, &se) || se.Step != "invalid" {
		t.Fatal(err)
	}
}

type recordingBackend struct {
	events []string
	fail   string
	err    error
}

func (b *recordingBackend) Name() string                                     { return "test" }
func (b *recordingBackend) Describe(context.Context, string) (string, error) { return "test", nil }
func (b *recordingBackend) record(op, target string) error {
	b.events = append(b.events, op+":"+filepath.Ext(target))
	if op == b.fail {
		return b.err
	}
	return nil
}
func (b *recordingBackend) Sign(_ context.Context, target string) error {
	return b.record("sign", target)
}
func (b *recordingBackend) Submit(_ context.Context, target string) error {
	return b.record("submit", target)
}
func (b *recordingBackend) Staple(_ context.Context, target string) error {
	return b.record("staple", target)
}
func TestPipelineOrderAndFailures(t *testing.T) {
	dir := t.TempDir()
	p := &Project{App: syntheticApp(t, dir), Out: filepath.Join(dir, "out"), DMG: &DMGConfig{}, PKG: &PKGConfig{}, Sign: &SignConfig{}, Notarize: &NotarizeConfig{Staple: true}}
	b := &recordingBackend{}
	pl, err := p.Resolve(WithSigningBackend(b), WithNotarizationBackend(b))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pl.Build(t.Context(), StepPKG, StepDMG); err != nil {
		t.Fatal(err)
	}
	want := "sign:.app,sign:.dmg,sign:.pkg,submit:.dmg,staple:.dmg,submit:.pkg,staple:.pkg"
	if got := strings.Join(b.events, ","); got != want {
		t.Fatalf("pipeline = %s; want %s", got, want)
	}
	sentinel := errors.New("backend failure")
	b.fail, b.err = "staple", sentinel
	err = pl.Notarize(t.Context(), pl.DMG.FileName)
	var se *StepError
	if !errors.Is(err, sentinel) || !errors.As(err, &se) || se.Step != StepStaple {
		t.Fatal(err)
	}
	b.fail = "sign"
	b.events = nil
	if _, err = pl.Build(t.Context()); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if len(b.events) != 1 {
		t.Fatal("pipeline continued after failed app signing")
	}
}
func TestFullPKGLicensePaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "payload"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "license.txt"), []byte("terms"), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := Parse(strings.NewReader("version: 1\npkg:\n  components: [{id: dev.demo, root: payload}]\n  distribution: {license: license.txt}\n"), dir)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	if pl.PKG.License != filepath.Join(dir, "license.txt") {
		t.Fatal(pl.PKG)
	}
	if _, err = pl.BuildPKG(t.Context()); err != nil {
		t.Fatal(err)
	}
}

// Resolve and DMGConfig.DefaultPositions must agree: the GUI preview places
// automatic contents with DefaultPositions, so any drift would show the user a
// layout different from the one a build produces.
func TestResolveMatchesDefaultPositions(t *testing.T) {
	for _, c := range []*DMGConfig{
		{},
		{Window: Window{Width: 900, Height: 700}},
		{IconSize: 96, LabelSize: 12},
		{Window: Window{Width: 1024}, IconSize: 64},
	} {
		dir := t.TempDir()
		p := &Project{App: syntheticApp(t, dir), Out: filepath.Join(dir, "out"), DMG: c}
		plan, err := p.Resolve()
		if err != nil {
			t.Fatal(err)
		}
		appX, linkX, y := c.DefaultPositions()
		if len(plan.DMG.Contents) != 2 {
			t.Fatalf("expected the automatic app + Applications layout, got %d items", len(plan.DMG.Contents))
		}
		app, link := plan.DMG.Contents[0], plan.DMG.Contents[1]
		if app.X != appX || app.Y != y {
			t.Errorf("app placed at (%d,%d), DefaultPositions says (%d,%d)", app.X, app.Y, appX, y)
		}
		if link.X != linkX || link.Y != y {
			t.Errorf("Applications placed at (%d,%d), DefaultPositions says (%d,%d)", link.X, link.Y, linkX, y)
		}
	}
}

func TestMetricsAppliesDocumentedDefaults(t *testing.T) {
	w, h, icon, label := (&DMGConfig{}).Metrics()
	if w != DefaultWindowWidth || h != DefaultWindowHeight || icon != DefaultIconSize || label != DefaultLabelSize {
		t.Errorf("empty config metrics = (%d,%d,%d,%d), want the documented defaults", w, h, icon, label)
	}
	set := &DMGConfig{Window: Window{Width: 800, Height: 600}, IconSize: 64, LabelSize: 11}
	if w, h, icon, label = set.Metrics(); w != 800 || h != 600 || icon != 64 || label != 11 {
		t.Errorf("explicit metrics = (%d,%d,%d,%d), want them preserved", w, h, icon, label)
	}
}
