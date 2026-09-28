package gui

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/ironpark/zapp"
)

// dropFiles handles a file dropped onto the window where no field or preview
// took it: an app bundle becomes the project's app, and a certificate becomes
// the signing credential, on the tab that shows it.
func (g *editor) dropFiles(paths []string) {
	if len(paths) != 1 {
		g.report(errors.New("Drop one file at a time"), "")
		return
	}
	if !g.commit() {
		return
	}
	path := pickedPath(g.s.Path, paths[0])
	switch strings.ToLower(filepath.Ext(paths[0])) {
	case ".app":
		g.s.checkpoint()
		g.s.Project.App = path
		g.tab = tabProject
		g.report(nil, "App bundle set to "+path)
	case ".p12", ".pfx":
		g.useCertificate(func(c *zapp.SignConfig) { c.P12File = path }, 1)
		g.report(nil, "Signing with the PKCS#12 certificate "+path)
	case ".pem":
		g.useCertificate(func(c *zapp.SignConfig) { c.PEMFile = path }, 2)
		g.report(nil, "Signing with the PEM certificate "+path)
	default:
		g.report(errors.New("Drop a .app bundle, or a .p12 or .pem certificate"), "")
		return
	}
	g.selected = ""
	g.rebuild()
}

// useCertificate switches signing to the certificate method mode and lets set
// fill it in, enabling signing if it was off. The other methods' values are
// cleared, as choosing a method in the tab would.
func (g *editor) useCertificate(set func(*zapp.SignConfig), mode int) {
	g.s.checkpoint()
	if g.s.Project.Sign == nil {
		g.s.Project.Sign = &zapp.SignConfig{}
	}
	c := g.s.Project.Sign
	c.Identity, c.P12File, c.P12Base64, c.PEMFile = "", "", "", ""
	if mode != 1 {
		c.P12Password, c.P12PasswordFile = "", ""
	}
	set(c)
	g.signMode, g.signModeSet = mode, true
	g.tab = tabSign
}
