package zapp

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The cask downloads the ZIP from where the upload put it, and is committed
// to the tap.
func TestHomebrew(t *testing.T) {
	dir := t.TempDir()
	srv, _ := uploadServer(t)
	committed := map[string]string{}
	tap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tap-token" {
			http.Error(w, "bad credentials", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet:
			http.NotFound(w, r)
		case http.MethodPut:
			var in struct{ Message, Content string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			data, _ := base64.StdEncoding.DecodeString(in.Content)
			committed[r.URL.Path] = in.Message + "\n" + string(data)
			_ = json.NewEncoder(w).Encode(map[string]any{"content": map[string]any{"html_url": "https://github.com/me/homebrew-tap/blob/main/Casks/demo.rb"}})
		}
	}))
	t.Cleanup(tap.Close)
	t.Setenv("GITHUB_API_URL", tap.URL)
	t.Setenv("ZAPP_HOMEBREW_TOKEN", "tap-token")
	p := &Project{App: syntheticApp(t, dir), Out: filepath.Join(dir, "out"), Zip: &ZipConfig{},
		Upload:   []UploadConfig{{URL: srv.URL + "/${file.name}", Headers: map[string]string{"Authorization": "Bearer secret"}}},
		Homebrew: &HomebrewConfig{Desc: "A demo", Homepage: "https://example.com", Tap: "me/homebrew-tap"}}
	pl, err := p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	a, err := pl.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cask, err := os.ReadFile(a.Homebrew)
	if err != nil {
		t.Fatal(err)
	}
	zip, _ := os.ReadFile(a.Zip)
	sum := sha256.Sum256(zip)
	for _, want := range []string{`cask "demo" do`, `version "2.3"`, `sha256 "` + hex.EncodeToString(sum[:]) + `"`, `url "` + srv.URL + `/Demo.zip"`, `desc "A demo"`, `app "Demo.app"`} {
		if !strings.Contains(string(cask), want) {
			t.Errorf("cask lacks %s:\n%s", want, cask)
		}
	}
	if filepath.Base(a.Homebrew) != "demo.rb" {
		t.Errorf("cask written to %s", a.Homebrew)
	}
	if got := committed["/repos/me/homebrew-tap/contents/Casks/demo.rb"]; got != "demo 2.3\n"+string(cask) {
		t.Fatalf("committed %q", got)
	}
}

// A cask that could not be written stops the build before it starts.
func TestHomebrewChecks(t *testing.T) {
	dir := t.TempDir()
	p := &Project{App: syntheticApp(t, dir), Out: filepath.Join(dir, "out"), Zip: &ZipConfig{}, Homebrew: &HomebrewConfig{Homepage: "https://example.com"}}
	pl, err := p.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	var se *StepError
	if _, err := pl.DryRun(); !errors.As(err, &se) || se.Step != StepHomebrew || !strings.Contains(err.Error(), "download URL") {
		t.Fatalf("no URL: %v", err)
	}
	p.Homebrew.URL = "https://example.com/${app.version}/${file.name}"
	p.Homebrew.Tap = "me/homebrew-tap"
	t.Setenv("ZAPP_HOMEBREW_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	if pl, err = p.Resolve(); err != nil {
		t.Fatal(err)
	}
	if _, err := pl.DryRun(); err == nil || !strings.Contains(err.Error(), "ZAPP_HOMEBREW_TOKEN") {
		t.Fatalf("no token: %v", err)
	}
	p.Homebrew.Tap = ""
	if pl, err = p.Resolve(); err != nil {
		t.Fatal(err)
	}
	a, err := pl.Build(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cask, _ := os.ReadFile(a.Homebrew); !strings.Contains(string(cask), `url "https://example.com/2.3/Demo.zip"`) {
		t.Fatalf("cask:\n%s", cask)
	}
	for _, bad := range []string{
		"version: 1\nhomebrew: {}\n",
		"version: 1\nhomebrew: {homepage: https://x, artifact: tar}\n",
		"version: 1\nhomebrew: {homepage: https://x, tap: tap}\n",
	} {
		if _, err := Parse(strings.NewReader(bad), dir); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
