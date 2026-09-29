package upload

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRepoFileCommit(t *testing.T) {
	var mu sync.Mutex
	files := map[string]string{}
	var puts []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer token" {
			http.Error(w, "bad credentials", http.StatusUnauthorized)
			return
		}
		const prefix = "/repos/me/homebrew-tap/contents/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, prefix)
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("ref") != "main" {
				http.Error(w, "wrong branch", http.StatusBadRequest)
				return
			}
			content, ok := files[name]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"sha": "sha-" + content, "content": base64.StdEncoding.EncodeToString([]byte(content)), "html_url": "https://github.com/me/homebrew-tap/blob/main/" + name})
		case http.MethodPut:
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			if old, ok := files[name]; ok && in["sha"] != "sha-"+old {
				http.Error(w, "sha does not match", http.StatusConflict)
				return
			}
			data, _ := base64.StdEncoding.DecodeString(in["content"].(string))
			files[name] = string(data)
			puts = append(puts, in)
			_ = json.NewEncoder(w).Encode(map[string]any{"content": map[string]any{"html_url": "https://github.com/me/homebrew-tap/blob/main/" + name}})
		}
	}))
	t.Cleanup(srv.Close)
	f := RepoFile{Repo: "me/homebrew-tap", Path: "Casks/demo.rb", Branch: "main", Token: "token", API: srv.URL}
	for i, content := range []string{"v1", "v2", "v2"} {
		page, err := f.Commit(t.Context(), nil, []byte(content), "Update demo")
		if err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
		if page != "https://github.com/me/homebrew-tap/blob/main/Casks/demo.rb" {
			t.Fatalf("page = %s", page)
		}
	}
	if files["Casks/demo.rb"] != "v2" || len(puts) != 2 || puts[0]["sha"] != nil || puts[1]["sha"] != "sha-v1" || puts[1]["branch"] != "main" {
		t.Fatalf("files %v, puts %v", files, puts)
	}
	f.Token = "wrong"
	if _, err := f.Commit(t.Context(), nil, []byte("v3"), "x"); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("bad token: %v", err)
	}
	f.Repo = "tap"
	if _, err := f.Commit(t.Context(), nil, []byte("v3"), "x"); err == nil {
		t.Fatal("accepted a repo without an owner")
	}
}
