package zapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp/internal/fsutil"
	"github.com/ironpark/zapp/pkg/sparkle"
	"github.com/ironpark/zapp/pkg/upload"
)

// sparkleKey reads the appcast's signing key from ZAPP_SPARKLE_KEY or its key
// file, and checks it against the SUPublicEDKey the app ships with: a feed
// signed with another key offers updates the app then refuses.
func (p *Plan) sparkleKey() (sparkle.Key, error) {
	c := p.Appcast
	text := os.Getenv("ZAPP_SPARKLE_KEY")
	if text == "" && c.KeyFile != "" {
		data, err := os.ReadFile(c.KeyFile)
		if err != nil {
			return sparkle.Key{}, err
		}
		text = string(data)
	}
	if text == "" {
		return sparkle.Key{}, errors.New("signing the appcast needs Sparkle's private EdDSA key in ZAPP_SPARKLE_KEY or appcast.keyFile; export it with generate_keys -x")
	}
	key, err := sparkle.ParseKey(text)
	if err != nil {
		return sparkle.Key{}, err
	}
	switch {
	case c.PublicKey == "":
		p.log("Warning: %s has no SUPublicEDKey; Sparkle cannot verify updates without it\n", filepath.Base(p.App))
	case c.PublicKey != key.PublicKey():
		return sparkle.Key{}, fmt.Errorf("the Sparkle key does not match the app's SUPublicEDKey %s", c.PublicKey)
	}
	return key, nil
}

// WriteAppcast adds the build's ZIP or DMG to the appcast as the newest
// release, signed, and returns the appcast's path.
func (p *Plan) WriteAppcast(ctx context.Context, a Artifacts) (out string, err error) {
	defer func() { err = stepError(StepAppcast, err) }()
	c := p.Appcast
	if c == nil {
		return "", fmt.Errorf("appcast section is not configured")
	}
	file := map[string]string{"zip": a.Zip, "dmg": a.DMG}[c.Artifact]
	if file == "" {
		return "", fmt.Errorf("the appcast publishes the %s; build one", c.Artifact)
	}
	key, err := p.sparkleKey()
	if err != nil {
		return "", err
	}
	length, signature, err := key.SignFile(file)
	if err != nil {
		return "", err
	}
	feed, err := p.readFeed(ctx)
	if err != nil {
		return "", err
	}
	item := sparkle.Item{
		Version: c.Version, ShortVersion: c.ShortVersion, MinimumSystemVersion: c.MinimumSystemVersion,
		ReleaseNotes: c.ReleaseNotes, Published: c.Published,
		URL:    strings.ReplaceAll(c.URL, upload.FileName, url.PathEscape(filepath.Base(file))),
		Length: length, Signature: signature,
	}
	data, err := sparkle.Add(feed, c.Title, item)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(c.Output), 0o755); err != nil {
		return "", err
	}
	p.log("Adding %s %s to appcast %s\n", filepath.Base(file), releaseName(c), c.Output)
	return c.Output, fsutil.WriteFileAtomic(c.Output, data, 0o644)
}

// releaseName shows a release as Sparkle does, by its short version.
func releaseName(c *AppcastSpec) string {
	if c.ShortVersion != "" && c.ShortVersion != c.Version {
		return c.ShortVersion + " (" + c.Version + ")"
	}
	return c.Version
}

// readFeed reads the appcast the release joins: the published feed, or the
// one written last time. None yet is not an error.
func (p *Plan) readFeed(ctx context.Context) ([]byte, error) {
	c := p.Appcast
	source := c.Feed
	if source == "" {
		source = c.Output
	}
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		data, err := os.ReadFile(source)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return data, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	client := p.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("reading the published appcast: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		p.log("No appcast at %s yet; starting one\n", upload.Redact(source))
		return nil, nil
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("reading the published appcast %s: %s", upload.Redact(source), resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
}
