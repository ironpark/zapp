package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

// Release is a GitHub release that takes files as assets.
type Release struct {
	// Repo is owner/name.
	Repo string
	Tag  string
	// Draft makes a release this creates a draft. A release that already
	// exists is left as it is.
	Draft bool
	// Token authenticates to the API; it needs write access to contents.
	Token string
	// API is the REST API root. Empty means https://api.github.com; GitHub
	// Enterprise Server has its own.
	API string
}

// Check reports a release that cannot be reached, before anything is built.
func (r Release) Check() error {
	owner, name, ok := strings.Cut(r.Repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("github repo must be written owner/name, not %q", r.Repo)
	}
	if r.Tag == "" {
		return errors.New("github release needs a tag")
	}
	return nil
}

// Location describes the release, for logging.
func (r Release) Location() string {
	return fmt.Sprintf("GitHub release %s of %s", r.Tag, r.Repo)
}

// ReleaseAssets uploads files to one release.
type ReleaseAssets struct {
	r      Release
	client *http.Client
	upload string           // the release's asset upload URL
	assets map[string]int64 // the assets it has, by name
}

type githubRelease struct {
	TagName   string        `json:"tag_name"`
	UploadURL string        `json:"upload_url"`
	Assets    []githubAsset `json:"assets"`
}

type githubAsset struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// OpenRelease finds the release r names, drafts included, and creates it
// when there is none. client nil means http.DefaultClient.
func OpenRelease(ctx context.Context, client *http.Client, r Release) (*ReleaseAssets, error) {
	if err := r.Check(); err != nil {
		return nil, err
	}
	if r.Token == "" {
		return nil, errors.New("uploading to a GitHub release needs a token in GITHUB_TOKEN or GH_TOKEN")
	}
	if client == nil {
		client = http.DefaultClient
	}
	a := &ReleaseAssets{r: r, client: client, assets: map[string]int64{}}
	var rel githubRelease
	status, err := a.call(ctx, http.MethodGet, "releases/tags/"+url.PathEscape(r.Tag), nil, &rel)
	if status == http.StatusNotFound {
		// The tag lookup does not see drafts; the list does.
		var list []githubRelease
		if _, err = a.call(ctx, http.MethodGet, "releases?per_page=100", nil, &list); err != nil {
			return nil, err
		}
		found := false
		for _, l := range list {
			if l.TagName == r.Tag {
				rel, found = l, true
				break
			}
		}
		if !found {
			create := map[string]any{"tag_name": r.Tag, "name": r.Tag, "draft": r.Draft}
			_, err = a.call(ctx, http.MethodPost, "releases", create, &rel)
		}
	}
	if err != nil {
		return nil, err
	}
	// upload_url is a URI template: .../assets{?name,label}.
	a.upload, _, _ = strings.Cut(rel.UploadURL, "{")
	if a.upload == "" {
		return nil, fmt.Errorf("%s has no upload URL", r.Location())
	}
	for _, asset := range rel.Assets {
		a.assets[asset.Name] = asset.ID
	}
	return a, nil
}

// Send uploads the file at path as an asset, replacing one of the same name
// so that a build can be run again, and returns its download URL.
func (a *ReleaseAssets) Send(ctx context.Context, path string) (string, error) {
	name := filepath.Base(path)
	// GitHub stores a name with spaces with dots in their place.
	for _, stored := range []string{name, strings.ReplaceAll(name, " ", ".")} {
		id, ok := a.assets[stored]
		if !ok {
			continue
		}
		if _, err := a.call(ctx, http.MethodDelete, fmt.Sprintf("releases/assets/%d", id), nil, nil); err != nil {
			return "", fmt.Errorf("replacing %s: %w", stored, err)
		}
		delete(a.assets, stored)
	}
	t := Target{URL: a.upload + "?name=" + url.QueryEscape(name), Method: http.MethodPost, Headers: a.headers(), raw: true}
	body, err := deliver(ctx, a.client, t, path)
	if err != nil {
		return "", err
	}
	var asset githubAsset
	if err := json.Unmarshal(body, &asset); err != nil {
		return "", fmt.Errorf("reading GitHub's answer to %s: %w", name, err)
	}
	a.assets[asset.Name] = asset.ID
	return asset.DownloadURL, nil
}

func (a *ReleaseAssets) headers() map[string]string {
	return map[string]string{
		"Authorization":        "Bearer " + a.r.Token,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}
}

// call makes one request to the repository's API, decoding a JSON answer
// into out. It returns the status even when it is an error.
func (a *ReleaseAssets) call(ctx context.Context, method, path string, in, out any) (int, error) {
	api := strings.TrimSuffix(a.r.API, "/")
	if api == "" {
		api = "https://api.github.com"
	}
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, api+"/repos/"+a.r.Repo+"/"+path, body)
	if err != nil {
		return 0, err
	}
	for name, value := range a.headers() {
		req.Header.Set(name, value)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		err := fmt.Errorf("%s: GitHub answered %s", a.r.Location(), resp.Status)
		if text := strings.TrimSpace(string(detail)); text != "" {
			err = fmt.Errorf("%w: %s", err, text)
		}
		return resp.StatusCode, err
	}
	if out == nil {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out)
}
