package zapp

import (
	"github.com/ironpark/zapp/pkg/dep"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/ironpark/zapp/pkg/macpkg"
	"github.com/ironpark/zapp/pkg/signing"
	"github.com/ironpark/zapp/pkg/upload"
	"net/http"
	"time"
)

type PKGSpec struct {
	// License is a project-relative external resource staged into Product.
	License    string
	App        *macpkg.AppConfig
	Components []macpkg.ComponentConfig
	Product    *macpkg.ProductConfig
	Output     string
}

// ZipSpec is where the distributable app archive is written.
type ZipSpec struct {
	Output string
}

// AppcastSpec is a resolved appcast: where it is written, what it publishes
// and the app release it describes.
type AppcastSpec struct {
	Output, URL, Artifact, Feed, Title, ReleaseNotes, KeyFile string
	// From the app's Info.plist.
	Version, ShortVersion, MinimumSystemVersion, PublicKey string
	Published                                              time.Time
}

// HomebrewSpec is a resolved cask: what it installs, from where, and where
// it is written and committed.
type HomebrewSpec struct {
	Token, Artifact, URL, Name, Desc, Homepage, Tap, Branch, Output string
	// From the app's Info.plist.
	Version, MinimumMacOS string
	// AutoUpdates is set for an app that updates itself with Sparkle.
	AutoUpdates bool
}

// DefaultNotarizeTimeout is how long notarization waits for Apple's verdict
// unless the project says otherwise. Most submissions take minutes; a busy
// notary service can take much longer.
const DefaultNotarizeTimeout = "1h"

// ChecksumsSpec is where the checksum list is written.
type ChecksumsSpec struct {
	Output string
}

type Plan struct {
	signBackend, notaryBackend signing.Backend
	App                        string
	DMG                        *dmg.Config
	PKG                        *PKGSpec
	Dep                        *dep.Config
	Zip                        *ZipSpec
	Checksums                  *ChecksumsSpec
	Appcast                    *AppcastSpec
	Homebrew                   *HomebrewSpec
	Uploads                    []UploadConfig
	// Credential field names differ from method names because Go shares their namespace.
	SignCredentials     *signing.Credentials
	NotarizeCredentials *signing.Credentials
	Staple              bool
	// Verify checks the app and each artifact before anything is published.
	Verify       bool
	logger       Logger
	httpClient   *http.Client
	originalIcon bool
	project      *Project
}

// YAML presents normalized paths and defaults using the public file schema.
// Passwords never appear in this representation, nor do upload credentials:
// credential headers and the userinfo and query of upload URLs, where
// presigned URLs keep their signatures.
func (p *Plan) YAML() ([]byte, error) {
	q := p.project
	if len(q.Upload) > 0 {
		q = q.Clone()
		for i := range q.Upload {
			u := &q.Upload[i]
			u.URL = upload.Redact(u.URL)
			for name := range u.Headers {
				if secretHeader(name) {
					u.Headers[name] = "<redacted>"
				}
			}
		}
	}
	return q.YAML()
}
