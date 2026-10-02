package gui

import (
	"errors"
	"runtime"
	"slices"
)

type validationIssue struct {
	issueLocation
	message string
}

// issueOn reports whether the outstanding validation issue belongs to tab.
func (g *editor) issueOn(tab int) bool { return g.issue != nil && g.issue.tab == tab }

// clearIssue drops the outstanding validation issue when it belongs to tab, so
// every mutation path states the rule the same way.
func (g *editor) clearIssue(tab int) {
	if g.issueOn(tab) {
		g.issue = nil
	}
}

// switchMethod moves credential values between a live config and its stash when
// the user picks a different method. groups[i] lists the (live, stash) pointer
// pairs owned by method i; fields outside every group are left untouched.
func switchMethod(current, next int, groups [][][2]*string) {
	for _, pair := range groups[current] {
		*pair[1] = *pair[0]
	}
	for _, group := range groups {
		for _, pair := range group {
			*pair[0] = ""
		}
	}
	for _, pair := range groups[next] {
		*pair[0] = *pair[1]
	}
}

// The Signing tab's methods, in the order its switch shows them.
const (
	signKeychain = iota
	signP12
	signPEM
)

var signMethodLabels = []string{"Keychain", "PKCS#12", "PEM"}

// signMethodFields lists the fields each signing method owns, by label.
var signMethodFields = [][]string{
	signKeychain: {labelSigningIdentity},
	signP12:      {labelP12Certificate, labelPasswordFile},
	signPEM:      {labelPEMCertificate},
}

// The Notarization tab's methods, in the order its switch shows them.
const (
	notaryProfile = iota
	notaryAppleID
	notaryAPIKey
)

var notaryMethodLabels = []string{"Profile", "Apple ID", "API key"}

// notaryMethodFields lists the fields each notarization method owns, by
// label.
var notaryMethodFields = [][]string{
	notaryProfile: {labelKeychainProfile},
	notaryAppleID: {labelAppleID, labelTeamID, labelAppPassword},
	notaryAPIKey:  {labelAPIKeyFile},
}

// methodOwning returns the method whose fields include label.
func methodOwning(methods [][]string, label string) (int, bool) {
	for method, labels := range methods {
		if slices.Contains(labels, label) {
			return method, true
		}
	}
	return 0, false
}

// methodFields keeps, of all, the fields method owns and those no method
// does.
func methodFields(all []field, methods [][]string, method int) []field {
	return slices.DeleteFunc(all, func(f field) bool {
		owner, owned := methodOwning(methods, f.Label)
		return owned && owner != method
	})
}

func (g *editor) signMethod() int {
	c := g.s.Project.Sign
	if c != nil {
		if c.P12File != "" || c.P12PasswordFile != "" {
			return signP12
		}
		if c.PEMFile != "" {
			return signPEM
		}
		if c.Identity != "" {
			return signKeychain
		}
	}
	if g.signModeSet {
		return g.signMode
	}
	if runtime.GOOS != "darwin" {
		return signP12
	}
	return signKeychain
}

func (g *editor) notaryMethod() int {
	c := g.s.Project.Notarize
	if c != nil {
		if c.APIKeyFile != "" {
			return notaryAPIKey
		}
		if c.Profile != "" {
			return notaryProfile
		}
		if c.AppleID != "" || c.TeamID != "" || c.Password != "" {
			return notaryAppleID
		}
	}
	if g.notaryModeSet {
		return g.notaryMode
	}
	if runtime.GOOS != "darwin" {
		return notaryAPIKey
	}
	return notaryProfile
}

func (g *editor) selectSignMethod(index int) {
	if !g.commit() || index == g.signMethod() {
		return
	}
	g.s.checkpoint()
	g.setSignMethod(index)
	g.rebuild()
}

// setSignMethod switches the signing credentials to method index, stashing
// the current method's values so switching back restores them.
func (g *editor) setSignMethod(index int) {
	c, stash := g.s.Project.Sign, &g.signStash
	switchMethod(g.signMethod(), index, [][][2]*string{
		{{&c.Identity, &stash.Identity}},
		{{&c.P12File, &stash.P12File}, {&c.P12PasswordFile, &stash.P12PasswordFile}, {&c.P12Password, &stash.P12Password}},
		{{&c.PEMFile, &stash.PEMFile}},
	})
	g.issue = nil
	g.signMode, g.signModeSet = index, true
}

func (g *editor) selectNotaryMethod(index int) {
	if !g.commit() || index == g.notaryMethod() {
		return
	}
	c, stash := g.s.Project.Notarize, &g.notaryStash
	g.s.checkpoint()
	// Staple is not owned by any method, so switchMethod leaves it in place.
	switchMethod(g.notaryMethod(), index, [][][2]*string{
		{{&c.Profile, &stash.Profile}},
		{{&c.AppleID, &stash.AppleID}, {&c.TeamID, &stash.TeamID}, {&c.Password, &stash.Password}},
		{{&c.APIKeyFile, &stash.APIKeyFile}},
	})
	g.issue = nil
	g.notaryMode, g.notaryModeSet = index, true
	g.rebuild()
}

func (g *editor) componentListVisible() bool {
	return g.tab == tabPKG && g.s.Project.PKG != nil && g.s.Project.PKG.HasFullForm()
}

func (g *editor) goToIssue() {
	if g.issue == nil {
		return
	}
	issue := *g.issue
	if !g.commit() {
		return
	}
	g.showIssue(issue.issueLocation, errors.New(issue.message))
}
