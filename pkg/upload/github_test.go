package upload

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeGitHub serves the part of the releases API OpenRelease and Send use.
type fakeGitHub struct {
	mu       sync.Mutex
	srv      *httptest.Server
	releases map[string]*fakeRelease // by tag
	nextID   int64
	log      []string
}

type fakeRelease struct {
	id     int64
	draft  bool
	assets map[string][]byte
	ids    map[string]int64
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	g := &fakeGitHub{releases: map[string]*fakeRelease{}, nextID: 100}
	g.srv = httptest.NewServer(http.HandlerFunc(g.serve))
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGitHub) json(r *fakeRelease, tag string) map[string]any {
	var assets []map[string]any
	for name, id := range r.ids {
		assets = append(assets, map[string]any{"id": id, "name": name})
	}
	return map[string]any{"tag_name": tag, "upload_url": fmt.Sprintf("%s/uploads/%d/assets{?name,label}", g.srv.URL, r.id), "assets": assets}
}

func (g *fakeGitHub) serve(w http.ResponseWriter, req *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.log = append(g.log, req.Method+" "+req.URL.Path)
	if req.Header.Get("Authorization") != "Bearer token" {
		http.Error(w, "bad credentials", http.StatusUnauthorized)
		return
	}
	const repo = "/repos/me/app/"
	switch {
	case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, repo+"releases/tags/"):
		tag := strings.TrimPrefix(req.URL.Path, repo+"releases/tags/")
		if r, ok := g.releases[tag]; ok && !r.draft {
			_ = json.NewEncoder(w).Encode(g.json(r, tag))
			return
		}
		http.NotFound(w, req)
	case req.Method == http.MethodGet && req.URL.Path == repo+"releases":
		var list []map[string]any
		for tag, r := range g.releases {
			list = append(list, g.json(r, tag))
		}
		_ = json.NewEncoder(w).Encode(list)
	case req.Method == http.MethodPost && req.URL.Path == repo+"releases":
		var in struct {
			Tag   string `json:"tag_name"`
			Draft bool   `json:"draft"`
		}
		_ = json.NewDecoder(req.Body).Decode(&in)
		g.nextID++
		r := &fakeRelease{id: g.nextID, draft: in.Draft, assets: map[string][]byte{}, ids: map[string]int64{}}
		g.releases[in.Tag] = r
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(g.json(r, in.Tag))
	case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, repo+"releases/assets/"):
		for _, r := range g.releases {
			for name, id := range r.ids {
				if req.URL.Path == fmt.Sprintf("%sreleases/assets/%d", repo, id) {
					delete(r.ids, name)
					delete(r.assets, name)
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
		}
		http.NotFound(w, req)
	case req.Method == http.MethodPost && strings.HasPrefix(req.URL.Path, "/uploads/"):
		name := strings.ReplaceAll(req.URL.Query().Get("name"), " ", ".")
		for tag, r := range g.releases {
			if req.URL.Path != fmt.Sprintf("/uploads/%d/assets", r.id) {
				continue
			}
			if _, ok := r.ids[name]; ok {
				http.Error(w, `{"errors":[{"code":"already_exists"}]}`, http.StatusUnprocessableEntity)
				return
			}
			data, _ := io.ReadAll(req.Body)
			g.nextID++
			r.ids[name], r.assets[name] = g.nextID, data
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": g.nextID, "name": name, "browser_download_url": "https://github.com/me/app/releases/download/" + tag + "/" + name})
			return
		}
		http.NotFound(w, req)
	default:
		http.Error(w, "unexpected "+req.Method+" "+req.URL.Path, http.StatusBadRequest)
	}
}

func TestReleaseUpload(t *testing.T) {
	g := newFakeGitHub(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "My App.dmg")
	if err := os.WriteFile(file, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Release{Repo: "me/app", Tag: "v1.0.0", Draft: true, Token: "token", API: g.srv.URL}

	// No release yet: it is created, as a draft, and the file is sent as
	// its body rather than in a form.
	a, err := OpenRelease(t.Context(), nil, r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Send(t.Context(), file)
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://github.com/me/app/releases/download/v1.0.0/My.App.dmg" {
		t.Fatalf("download URL = %s", got)
	}
	rel := g.releases["v1.0.0"]
	if rel == nil || !rel.draft || string(rel.assets["My.App.dmg"]) != "first" {
		t.Fatalf("release = %+v", rel)
	}

	// Running again finds the draft and replaces the asset.
	if err := os.WriteFile(file, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	if a, err = OpenRelease(t.Context(), nil, r); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Send(t.Context(), file); err != nil {
		t.Fatal(err)
	}
	if len(g.releases) != 1 || len(rel.assets) != 1 || string(rel.assets["My.App.dmg"]) != "second" {
		t.Fatalf("rerun left %d releases, assets %v", len(g.releases), rel.assets)
	}

	r.Token = "wrong"
	if _, err := OpenRelease(t.Context(), nil, r); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("bad token: %v", err)
	}
	r.Token = ""
	if _, err := OpenRelease(t.Context(), nil, r); err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("no token: %v", err)
	}
}

func TestReleaseCheck(t *testing.T) {
	for _, r := range []Release{{Repo: "app", Tag: "v1"}, {Repo: "me/app/x", Tag: "v1"}, {Repo: "me/app"}} {
		if r.Check() == nil {
			t.Errorf("%+v passed", r)
		}
	}
	if err := (Release{Repo: "me/app", Tag: "v1"}).Check(); err != nil {
		t.Error(err)
	}
}
