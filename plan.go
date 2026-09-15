package zapp

import (
	"github.com/ironpark/zapp/pkg/dep"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/ironpark/zapp/pkg/macpkg"
	"github.com/ironpark/zapp/pkg/signing"
)

type PKGSpec struct {
	// License is a project-relative external resource staged into Product.
	License    string
	App        *macpkg.AppConfig
	Components []macpkg.ComponentConfig
	Product    *macpkg.ProductConfig
	Output     string
}
type Plan struct {
	signBackend, notaryBackend signing.Backend
	App                        string
	DMG                        *dmg.Config
	PKG                        *PKGSpec
	Dep                        *dep.Config
	// Credential field names differ from method names because Go shares their namespace.
	SignCredentials     *signing.Credentials
	NotarizeCredentials *signing.Credentials
	Staple              bool
	logger              Logger
	originalIcon        bool
	project             *Project
}

// YAML presents normalized paths and defaults using the public file schema.
// Passwords never appear in this representation.
func (p *Plan) YAML() ([]byte, error) { return p.project.YAML() }
