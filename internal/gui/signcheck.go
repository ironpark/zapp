package gui

import (
	"context"
	"reflect"
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
	identities []string
	listed     bool // identities were looked up, even if none were found
	listing    bool
	results    chan func()

	checking bool
	checked  *zapp.SignConfig // the settings the result describes
	ok       bool
	message  string
}

// post hands work finished in the background to the UI thread.
func (g *editor) post(fn func()) {
	if g.signing.results == nil {
		g.signing.results = make(chan func(), 4)
	}
	wake := g.wakeFunc()
	go func() {
		g.signing.results <- fn
		wake()
	}()
}

// pollSigning applies finished background signing work on the UI thread.
func (g *editor) pollSigning() {
	for {
		select {
		case fn := <-g.signing.results:
			fn()
		default:
			return
		}
	}
}

// listIdentities looks up the keychain's signing identities once per session,
// the first time the Keychain method is shown. Only macOS has a keychain.
func (g *editor) listIdentities() {
	a := &g.signing
	if runtime.GOOS != "darwin" || a.listed || a.listing || g.ctx == nil {
		return
	}
	a.listing = true
	ctx := g.ctx
	go func() {
		identities, err := macos.ListIdentities(ctx)
		names := []string{}
		if err == nil {
			for _, identity := range identities {
				names = append(names, identity.String())
			}
		}
		g.post(func() { a.identities, a.listed, a.listing = names, true, false })
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
	if a.checking || !g.commit() || g.s.Project.Sign == nil {
		return
	}
	p := g.s.Project.Clone()
	settings := *p.Sign
	installer := p.PKG != nil
	p.Dep, p.DMG, p.PKG, p.Notarize = nil, nil, nil, nil
	plan, err := p.Resolve()
	if err != nil {
		a.checked, a.ok, a.message = &settings, false, err.Error()
		return
	}
	a.checking, a.checked = true, &settings
	ctx := g.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	creds := *plan.SignCredentials
	go func() {
		ok, message := describeSigning(ctx, creds, installer)
		g.post(func() { a.checking, a.ok, a.message = false, ok, message })
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
func (g *editor) signingCheck() (shown, checking, ok bool, message string) {
	a := g.signing
	if g.s.Project.Sign == nil || a.checked == nil || !reflect.DeepEqual(*a.checked, *g.s.Project.Sign) {
		return false, false, false, ""
	}
	return true, a.checking, a.ok, a.message
}
