package upload

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// RepoFile is a file in a GitHub repository, such as a cask in a Homebrew
// tap.
type RepoFile struct {
	// Repo is owner/name; Path is the file's, with /.
	Repo, Path string
	// Branch is where it is committed; empty means the default branch.
	Branch string
	// Token needs write access to the repository's contents.
	Token string
	// API is the REST API root; empty means https://api.github.com.
	API string
}

// Location describes the file, for logging.
func (f RepoFile) Location() string {
	where := f.Repo
	if f.Branch != "" {
		where += "@" + f.Branch
	}
	return f.Path + " in " + where
}

// Commit writes content to the file in one commit, creating or replacing
// it, and returns the file's page on GitHub. Content the file already holds
// makes no commit. client nil means http.DefaultClient.
func (f RepoFile) Commit(ctx context.Context, client *http.Client, content []byte, message string) (string, error) {
	if err := CheckRepo(f.Repo); err != nil {
		return "", err
	}
	if f.Token == "" {
		return "", errors.New("committing to a GitHub repository needs a token")
	}
	api := newGitHubAPI(client, f.API, f.Repo, f.Token, f.Location())
	var segments []string
	for _, s := range strings.Split(strings.Trim(f.Path, "/"), "/") {
		segments = append(segments, url.PathEscape(s))
	}
	path := "contents/" + strings.Join(segments, "/")
	query := ""
	if f.Branch != "" {
		query = "?ref=" + url.QueryEscape(f.Branch)
	}
	var current struct {
		SHA     string `json:"sha"`
		Content string `json:"content"`
		HTMLURL string `json:"html_url"`
	}
	status, err := api.call(ctx, http.MethodGet, path+query, nil, &current)
	switch {
	case status == http.StatusNotFound:
		current.SHA = ""
	case err != nil:
		return "", err
	default:
		old, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(current.Content), ""))
		if err == nil && bytes.Equal(old, content) {
			return current.HTMLURL, nil
		}
	}
	put := map[string]any{"message": message, "content": base64.StdEncoding.EncodeToString(content)}
	if current.SHA != "" {
		put["sha"] = current.SHA
	}
	if f.Branch != "" {
		put["branch"] = f.Branch
	}
	var written struct {
		Content struct {
			HTMLURL string `json:"html_url"`
		} `json:"content"`
	}
	if _, err := api.call(ctx, http.MethodPut, path, put, &written); err != nil {
		return "", fmt.Errorf("committing: %w", err)
	}
	return written.Content.HTMLURL, nil
}
