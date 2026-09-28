// Package upload sends build artifacts to an HTTP endpoint.
package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileName is the placeholder a Target URL uses for the uploaded file's
// name, which is inserted URL-encoded.
const FileName = "${file.name}"

// Target is an HTTP endpoint that accepts a file.
type Target struct {
	// URL receives the file. FileName in it is replaced by the file's name.
	URL string
	// Method is PUT, which sends the file as the request body, or POST,
	// which sends it as a multipart form field. Empty means PUT.
	Method string
	// Field names the form field a POST carries the file in. Empty means
	// "file".
	Field string
	// Headers are sent with every request, such as Authorization.
	Headers map[string]string
}

// Check reports a target that cannot work, before anything is built.
func (t Target) Check() error {
	u, err := url.Parse(strings.ReplaceAll(t.URL, FileName, "file"))
	if err != nil {
		return fmt.Errorf("upload url: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" || u.Host == "" {
		return fmt.Errorf("upload url must be an http or https URL")
	}
	switch strings.ToUpper(t.Method) {
	case "", http.MethodPut, http.MethodPost:
	default:
		return fmt.Errorf("upload method must be PUT or POST, not %s", t.Method)
	}
	for name := range t.Headers {
		if name == "" || strings.ContainsAny(name, " :\r\n") {
			return fmt.Errorf("invalid upload header name %q", name)
		}
	}
	return nil
}

// Attempts is how often a file is sent before an upload fails; RetryDelay
// is the wait before the second attempt, doubling after each failure.
var (
	Attempts   = 3
	RetryDelay = 2 * time.Second
)

// File sends the file at path to t with client, or http.DefaultClient when
// client is nil. Network errors and 408, 429 and 5xx responses are retried.
// It returns where the file was sent, without the URL's query string, which
// may hold a presigned credential.
func File(ctx context.Context, client *http.Client, t Target, path string) (string, error) {
	if err := t.Check(); err != nil {
		return "", err
	}
	if client == nil {
		client = http.DefaultClient
	}
	name := filepath.Base(path)
	target := strings.ReplaceAll(t.URL, FileName, url.PathEscape(name))
	shown := t.Location(name)
	delay := RetryDelay
	var err error
	for attempt := 1; attempt <= Attempts; attempt++ {
		var retry bool
		retry, err = send(ctx, client, t, target, path)
		if err == nil {
			return shown, nil
		}
		if !retry || attempt == Attempts {
			break
		}
		select {
		case <-time.After(delay):
			delay *= 2
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "", fmt.Errorf("sending %s to %s: %w", name, shown, err)
}

// send makes one attempt, reporting whether a failure is worth retrying.
func send(ctx context.Context, client *http.Client, t Target, target, path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		return false, fmt.Errorf("%s is a directory; upload an archive of it", path)
	}
	body, length, contentType := io.Reader(f), info.Size(), contentTypeOf(path)
	method := strings.ToUpper(t.Method)
	if method == "" {
		method = http.MethodPut
	}
	if method == http.MethodPost {
		body, length, contentType, err = multipartBody(f, info.Size(), t.Field, filepath.Base(path), contentType)
		if err != nil {
			return false, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return false, err
	}
	req.ContentLength = length
	req.Header.Set("Content-Type", contentType)
	for name, value := range t.Headers {
		req.Header.Set(name, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		// The error names the URL, which may carry a presigned credential.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return ctx.Err() == nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return false, nil
	}
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	err = fmt.Errorf("server answered %s", resp.Status)
	if text := strings.TrimSpace(string(detail)); text != "" {
		err = fmt.Errorf("%w: %s", err, text)
	}
	retry := resp.StatusCode == http.StatusRequestTimeout || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
	return retry, err
}

// multipartBody wraps the file in a form with one field, with its length
// known up front so the request is not sent chunked, which some servers
// refuse.
func multipartBody(f io.Reader, size int64, field, name, contentType string) (io.Reader, int64, string, error) {
	if field == "" {
		field = "file"
	}
	var head bytes.Buffer
	w := multipart.NewWriter(&head)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, name))
	h.Set("Content-Type", contentType)
	if _, err := w.CreatePart(h); err != nil {
		return nil, 0, "", err
	}
	prefix := bytes.Clone(head.Bytes())
	head.Reset()
	if err := w.Close(); err != nil {
		return nil, 0, "", err
	}
	suffix := head.Bytes()
	return io.MultiReader(bytes.NewReader(prefix), f, bytes.NewReader(suffix)), int64(len(prefix)) + size + int64(len(suffix)), w.FormDataContentType(), nil
}

func contentTypeOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".zip":
		return "application/zip"
	case ".dmg":
		return "application/x-apple-diskimage"
	}
	return "application/octet-stream"
}

// Location is where t sends a file named name, without credentials.
func (t Target) Location(name string) string {
	return Redact(strings.ReplaceAll(t.URL, FileName, url.PathEscape(name)))
}

// Redact drops a URL's query string, fragment and user information, where
// presigned URLs and basic authentication keep their credentials. It works
// on the text, so a ${file.name} placeholder is left as written.
func Redact(raw string) string {
	raw, _, _ = strings.Cut(raw, "#")
	raw, _, _ = strings.Cut(raw, "?")
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return raw
	}
	host, path, _ := strings.Cut(rest, "/")
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	if path != "" || strings.Contains(rest, "/") {
		path = "/" + path
	}
	return scheme + "://" + host + path
}
