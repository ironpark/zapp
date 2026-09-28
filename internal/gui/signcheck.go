package gui

import (
	"context"
	"runtime"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/pkg/signing"
	"github.com/ironpark/zapp/pkg/signing/macos"
)

// signingAssist holds the Signing tab's background work: the keychain
// identities offered for the Keychain method, and the result of checking the
// configured credentials without building.
type signingAssist struct {
	identities string // one per line
	listed     bool   // identities were looked up, even if none were found
	listing    bool

	checked signCheckInputs // what the result describes
	result  signCheck
}

// signCheckInputs is everything a credential check depends on.
type signCheckInputs struct {
	sign      zapp.SignConfig
	installer bool
}

// signCheck is what the Signing tab shows for a credential check.
type signCheck struct {
	Shown, Checking, OK bool
	Message             string
}

func (g *editor) signCheckInputs() signCheckInputs {
	return signCheckInputs{*g.s.Project.Sign, g.s.Project.PKG != nil}
}

// listIdentities looks up the keychain's signing identities once per session,
// the first time the Keychain method is shown. Only macOS has a keychain.
func (g *editor) listIdentities() {
	a := &g.signing
	if runtime.GOOS != "darwin" || a.listed || a.listing || g.ctx == nil ||
		g.tab != tabSign || !g.enabled() || g.signMethod() != 0 {
		return
	}
	a.listing = true
	ctx, post := g.ctx, g.poster()
	go func() {
		identities, err := macos.ListIdentities(ctx)
		names := []string{}
		if err == nil {
			for _, identity := range identities {
				names = append(names, identity.String())
			}
		}
		post(func() { a.identities, a.listed, a.listing = strings.Join(names, "\n"), true, false })
	}()
}

// useIdentity fills the Keychain method's identity from a suggestion.
func (g *editor) useIdentity(name string) {
	if !g.commit() || g.s.Project.Sign == nil {
		return
	}
	g.s.checkpoint()
	g.s.Project.Sign.Identity = name
	g.rebuild()
}

// checkSigning resolves the signing settings and asks the signing backend
// which certificate it would use, without signing anything. On macOS a
// PKCS#12 or PEM certificate is imported into a temporary keychain for the
// check and removed again.
func (g *editor) checkSigning() {
	a := &g.signing
	if a.result.Checking || !g.commit() || g.s.Project.Sign == nil {
		return
	}
	inputs := g.signCheckInputs()
	p := g.s.Project.Clone()
	p.Dep, p.DMG, p.PKG, p.Notarize = nil, nil, nil, nil
	plan, err := p.Resolve()
	a.checked = inputs
	if err != nil {
		a.result = signCheck{Shown: true, Message: err.Error()}
		return
	}
	a.result = signCheck{Shown: true, Checking: true}
	ctx, post := g.ctx, g.poster()
	if ctx == nil {
		ctx = context.Background()
	}
	creds := *plan.SignCredentials
	go func() {
		ok, message := describeSigning(ctx, creds, inputs.installer)
		post(func() { a.result = signCheck{Shown: true, OK: ok, Message: message} })
	}()
}

// describeSigning reports the certificate the app, and the installer when a
// PKG is built, would be signed with. Apple uses a different certificate for
// each, so either can be missing on its own.
func describeSigning(ctx context.Context, creds signing.Credentials, installer bool) (bool, string) {
	b, err := signing.Select(creds)
	if err != nil {
		return false, err.Error()
	}
	defer func() { _ = signing.Close(b) }()
	targets := [][2]string{{"App", "App.app"}}
	if installer {
		targets = append(targets, [2]string{"Installer", "Installer.pkg"})
	}
	ok, lines := true, []string{}
	for _, target := range targets {
		description, err := b.Describe(ctx, target[1])
		if err != nil {
			ok, description = false, err.Error()
		}
		lines = append(lines, target[0]+": "+description)
	}
	return ok, strings.Join(lines, "\n")
}

// signingCheck is the result to show for the current settings, if any: a
// result for settings that have since changed is stale and hidden.
func (g *editor) signingCheck() signCheck {
	if g.s.Project.Sign == nil || g.signing.checked != g.signCheckInputs() {
		return signCheck{}
	}
	return g.signing.result
}
