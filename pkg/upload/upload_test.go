package upload

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func artifact(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// PUT sends the file as the body, under its escaped name, with the headers
// given; the returned location leaves out the query's credentials.
func TestPut(t *testing.T) {
	var got struct{ path, query, auth, contentType, body string }
	var length int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		got.path, got.query, got.auth, got.contentType, got.body = r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Authorization"), r.Header.Get("Content-Type"), string(body)
		length = r.ContentLength
	}))
	defer srv.Close()
	path := artifact(t, "My App.zip", "payload")
	where, err := File(t.Context(), nil, Target{URL: srv.URL + "/releases/${file.name}?X-Signature=secret", Headers: map[string]string{"Authorization": "Bearer token"}}, path)
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "/releases/My%20App.zip" || got.query != "X-Signature=secret" || got.auth != "Bearer token" || got.body != "payload" || length != 7 || got.contentType != "application/zip" {
		t.Fatalf("request = %+v, length %d", got, length)
	}
	if where != srv.URL+"/releases/My%20App.zip" {
		t.Fatalf("location = %q", where)
	}
}

// POST sends the file as one multipart field, with a known length.
func TestPostMultipart(t *testing.T) {
	var field, name, body string
	var length int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		length = r.ContentLength
		f, h, err := r.FormFile("asset")
		if err != nil {
			t.Error(err)
			return
		}
		b, _ := io.ReadAll(f)
		field, name, body = "asset", h.Filename, string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	path := artifact(t, "Demo.dmg", "disk image")
	if _, err := File(t.Context(), nil, Target{URL: srv.URL + "/upload", Method: "post", Field: "asset"}, path); err != nil {
		t.Fatal(err)
	}
	if field != "asset" || name != "Demo.dmg" || body != "disk image" || length <= int64(len(body)) {
		t.Fatalf("form = %s %s %q, length %d", field, name, body, length)
	}
}

// Server errors are retried and client errors are not; a failure quotes the
// server without the URL's credentials.
func TestRetries(t *testing.T) {
	defer func(d time.Duration) { RetryDelay = d }(RetryDelay)
	RetryDelay = time.Millisecond
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()
	path := artifact(t, "Demo.pkg", "installer")
	if _, err := File(t.Context(), nil, Target{URL: srv.URL + "/${file.name}"}, path); err != nil || calls.Load() != 3 {
		t.Fatalf("after %d calls: %v", calls.Load(), err)
	}

	calls.Store(0)
	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "token expired", http.StatusForbidden)
	}))
	defer denied.Close()
	_, err := File(t.Context(), nil, Target{URL: denied.URL + "/x?token=secret"}, path)
	if err == nil || calls.Load() != 1 || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "token expired") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("after %d calls: %v", calls.Load(), err)
	}
}

func TestCheck(t *testing.T) {
	for _, target := range []Target{
		{URL: "ftp://example.com/${file.name}"},
		{URL: "https:///${file.name}"},
		{URL: "https://example.com/", Method: "PATCH"},
		{URL: "https://example.com/", Headers: map[string]string{"Bad Name": "x"}},
	} {
		if err := target.Check(); err == nil {
			t.Errorf("accepted %+v", target)
		}
	}
	if err := (Target{URL: "https://example.com/${file.name}", Method: "POST"}).Check(); err != nil {
		t.Fatal(err)
	}
	for raw, want := range map[string]string{
		"https://user:pw@example.com/a.zip?sig=1#x":   "https://example.com/a.zip",
		"https://example.com/v1/${file.name}?token=x": "https://example.com/v1/${file.name}",
		"https://example.com":                         "https://example.com",
	} {
		if got := Redact(raw); got != want {
			t.Errorf("Redact(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := (Target{URL: "https://example.com/${file.name}?sig=1"}).Location("My App.zip"); got != "https://example.com/My%20App.zip" {
		t.Fatalf("location = %q", got)
	}
}
