package zapp

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sparkleApp is an app whose Info.plist carries what an appcast reads.
func sparkleApp(t *testing.T, dir, publicKey string) string {
	t.Helper()
	app := syntheticApp(t, dir)
	plist := `<?xml version="1.0"?><plist version="1.0"><dict>` +
		`<key>CFBundleIdentifier</key><string>dev.zapp.demo</string>` +
		`<key>CFBundleShortVersionString</key><string>2.3</string>` +
		`<key>CFBundleVersion</key><string>230</string>` +
		`<key>LSMinimumSystemVersion</key><string>12.0</string>` +
		`<key>SUPublicEDKey</key><string>` + publicKey + `</string>` +
		`</dict></plist>`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
	return app
}

// The ZIP is signed with the app's key and joins the published feed, and the
// appcast is uploaded with the other artifacts.
func TestAppcast(t *testing.T) {
	dir := t.TempDir()
	seed := make([]byte, ed25519.SeedSize)
	seed[0] = 7
	private := ed25519.NewKeyFromSeed(seed)
	public := base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	t.Setenv("ZAPP_SPARKLE_KEY", base64.StdEncoding.EncodeToString(seed))

	published := `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle">
  <channel>
    <title>Demo</title>
    <item>
      <title>2.2</title>
      <sparkle:version>220</sparkle:version>
      <enclosure url="https://example.com/Demo-220.zip" length="1" type="application/octet-stream" sparkle:edSignature="b2xk"/>
    </item>
  </channel>
</rss>
`
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/appcast.xml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(published))
	}))
	t.Cleanup(feed.Close)
	srv, uploaded := uploadServer(t)
	p := &Project{
		App: sparkleApp(t, dir, public), Out: filepath.Join(dir, "out"), Zip: &ZipConfig{},
		Appcast: &AppcastConfig{URL: "https://dl.example.com/${app.version}/${file.name}", Feed: feed.URL + "/appcast.xml", ReleaseNotes: "https://example.com/notes/${app.version}"},
		Upload:  []UploadConfig{{URL: srv.URL + "/${file.name}", Headers: map[string]string{"Authorization": "Bearer secret"}}},
	}
	pl, err := p.Resolve(WithClock(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	a, err := pl.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(a.Appcast)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	zip, err := os.ReadFile(a.Zip)
	if err != nil {
		t.Fatal(err)
	}
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(private, zip))
	for _, want := range []string{
		`<sparkle:version>230</sparkle:version>`, `<sparkle:shortVersionString>2.3</sparkle:shortVersionString>`,
		`<sparkle:minimumSystemVersion>12.0</sparkle:minimumSystemVersion>`, `<sparkle:releaseNotesLink>https://example.com/notes/2.3</sparkle:releaseNotesLink>`,
		`<pubDate>Tue, 01 Sep 2026 00:00:00 +0000</pubDate>`,
		`url="https://dl.example.com/2.3/Demo.zip"`, `sparkle:edSignature="` + signature + `"`,
		"Demo-220.zip",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("appcast lacks %s:\n%s", want, s)
		}
	}
	if strings.Index(s, "230") > strings.Index(s, "220") {
		t.Errorf("the new release is not first:\n%s", s)
	}
	if got := uploaded(); strings.Join(got, ",") != "/Demo.zip,/appcast.xml" {
		t.Fatalf("uploaded %v", got)
	}
}

// A missing or mismatched key stops the build before anything is built.
func TestAppcastKey(t *testing.T) {
	dir := t.TempDir()
	other := base64.StdEncoding.EncodeToString(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey))
	p := &Project{App: sparkleApp(t, dir, other), Out: filepath.Join(dir, "out"), Zip: &ZipConfig{}, Appcast: &AppcastConfig{URL: "https://example.com/${file.name}"}}
	pl, err := p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	var se *StepError
	t.Setenv("ZAPP_SPARKLE_KEY", "")
	if _, err := pl.Build(t.Context()); !errors.As(err, &se) || se.Step != StepAppcast || !strings.Contains(err.Error(), "ZAPP_SPARKLE_KEY") {
		t.Fatalf("no key: %v", err)
	}
	seed := make([]byte, ed25519.SeedSize)
	seed[0] = 1
	t.Setenv("ZAPP_SPARKLE_KEY", base64.StdEncoding.EncodeToString(seed))
	if _, err := pl.Build(t.Context()); !errors.As(err, &se) || !strings.Contains(err.Error(), "SUPublicEDKey") {
		t.Fatalf("wrong key: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out")); !os.IsNotExist(err) {
		t.Fatal("built before checking the key")
	}
	// The appcast publishes a DMG only when one is built.
	p.Appcast.Artifact = "dmg"
	t.Setenv("ZAPP_SPARKLE_KEY", base64.StdEncoding.EncodeToString(make([]byte, ed25519.SeedSize)))
	if pl, err = p.Resolve(); err != nil {
		t.Fatal(err)
	}
	if _, err := pl.Build(t.Context(), StepZip, StepAppcast); err == nil || !strings.Contains(err.Error(), "dmg") {
		t.Fatalf("no dmg: %v", err)
	}
	for _, bad := range []string{"version: 1\nappcast: {}\n", "version: 1\nappcast: {url: https://x/, artifact: pkg}\n"} {
		if _, err := Parse(strings.NewReader(bad), dir); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
