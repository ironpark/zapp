package gui

import (
	"fmt"
	"runtime"
)

type validationIssue struct {
	tab            int
	label, message string
	component      int
	item           string
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

func (g *editor) signMethod() int {
	c := g.s.Project.Sign
	if c != nil {
		if c.P12File != "" || c.P12PasswordFile != "" {
			return 1
		}
		if c.PEMFile != "" {
			return 2
		}
		if c.Identity != "" {
			return 0
		}
	}
	if g.signModeSet {
		return g.signMode
	}
	if runtime.GOOS != "darwin" {
		return 1
	}
	return 0
}
func (g *editor) notaryMethod() int {
	c := g.s.Project.Notarize
	if c != nil {
		if c.APIKeyFile != "" {
			return 2
		}
		if c.Profile != "" {
			return 0
		}
		if c.AppleID != "" || c.TeamID != "" || c.Password != "" {
			return 1
		}
	}
	if g.notaryModeSet {
		return g.notaryMode
	}
	if runtime.GOOS != "darwin" {
		return 2
	}
	return 0
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
	g.componentIndex = issue.component
	g.revealComponent()
	g.selected = issue.item
	g.showFieldError(issue.tab, issue.label, fmt.Errorf("%s", issue.message))
}
