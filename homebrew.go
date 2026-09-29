package zapp

import (
	"cmp"
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ironpark/zapp/internal/fsutil"
	"github.com/ironpark/zapp/pkg/homebrew"
	"github.com/ironpark/zapp/pkg/upload"
)

func homebrewToken() string { return cmp.Or(os.Getenv("ZAPP_HOMEBREW_TOKEN"), githubToken()) }

// tapFile is where the cask goes in its tap, with this run's credentials.
func (c *HomebrewSpec) tapFile() upload.RepoFile {
	return upload.RepoFile{Repo: c.Tap, Path: "Casks/" + c.Token + ".rb", Branch: c.Branch, Token: homebrewToken(), API: os.Getenv("GITHUB_API_URL")}
}

// checkHomebrew reports, before the build, a cask that could not be written:
// its artifact is not built, nothing says where it is downloaded from, or
// there is no token for its tap.
func (p *Plan) checkHomebrew(selected map[Step]bool) error {
	c := p.Homebrew
	built := map[string]bool{"dmg": selected[StepDMG], "zip": selected[StepZip], "pkg": selected[StepPKG]}
	if !built[c.Artifact] {
		return fmt.Errorf("the cask installs the %s; build one", c.Artifact)
	}
	if c.URL == "" && !(selected[StepUpload] && slices.ContainsFunc(p.Uploads, func(u UploadConfig) bool {
		return len(u.Artifacts) == 0 || slices.Contains(u.Artifacts, c.Artifact)
	})) {
		return fmt.Errorf("the cask needs the %s's download URL: upload it, or set homebrew.url", c.Artifact)
	}
	if c.Tap != "" && homebrewToken() == "" {
		return fmt.Errorf("committing the cask to %s needs a token in ZAPP_HOMEBREW_TOKEN, GITHUB_TOKEN or GH_TOKEN", c.Tap)
	}
	return nil
}

// WriteCask writes the Homebrew cask for the build's artifact and, with a
// tap, commits it there. It returns the cask's path.
func (p *Plan) WriteCask(ctx context.Context, a Artifacts) (out string, err error) {
	defer func() { err = stepError(StepHomebrew, err) }()
	c := p.Homebrew
	if c == nil {
		return "", fmt.Errorf("homebrew section is not configured")
	}
	file := map[string]string{"dmg": a.DMG, "zip": a.Zip, "pkg": a.PKG}[c.Artifact]
	if file == "" {
		return "", fmt.Errorf("the cask installs the %s; build one", c.Artifact)
	}
	download := strings.ReplaceAll(c.URL, upload.FileName, url.PathEscape(filepath.Base(file)))
	if download == "" {
		for _, u := range a.Uploads {
			if u.Artifact == c.Artifact {
				download = u.URL
				break
			}
		}
	}
	if download == "" {
		return "", fmt.Errorf("the cask needs the %s's download URL: upload it, or set homebrew.url", c.Artifact)
	}
	sum, err := sha256File(file)
	if err != nil {
		return "", err
	}
	cask := homebrew.Cask{Token: c.Token, Version: c.Version, SHA256: hex.EncodeToString(sum), URL: download,
		Name: c.Name, Desc: c.Desc, Homepage: c.Homepage, MinimumMacOS: c.MinimumMacOS, AutoUpdates: c.AutoUpdates}
	if c.Artifact == "pkg" {
		cask.PKG, cask.PKGIDs = filepath.Base(file), p.pkgIdentifiers()
	} else {
		cask.App = filepath.Base(p.App)
	}
	data := []byte(cask.Render())
	if err := os.MkdirAll(filepath.Dir(c.Output), 0o755); err != nil {
		return "", err
	}
	p.log("Writing Homebrew cask %s for %s\n", c.Output, filepath.Base(file))
	if err := fsutil.WriteFileAtomic(c.Output, data, 0o644); err != nil {
		return "", err
	}
	if c.Tap != "" {
		f := c.tapFile()
		p.log("Committing it as %s\n", f.Location())
		page, err := f.Commit(ctx, p.httpClient, data, c.Token+" "+c.Version)
		if err != nil {
			return "", err
		}
		p.log("Cask: %s\n", page)
	}
	return c.Output, nil
}

// pkgIdentifiers are the receipts the PKG leaves, which uninstalling the
// cask removes.
func (p *Plan) pkgIdentifiers() []string {
	var ids []string
	add := func(id string) {
		if id != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if s := p.PKG; s != nil {
		if s.App != nil {
			add(s.App.Identifier)
		}
		for _, c := range s.Components {
			add(c.Identifier)
		}
	}
	return ids
}
