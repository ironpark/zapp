package gui

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ironpark/zapp"
)

// Every label a method owns names one of its tab's fields, so switching to
// the method a problem is on shows the problem's field.
func TestMethodFieldsNameRealFields(t *testing.T) {
	g := testEditor(t)
	g.s.Project.Sign, g.s.Project.Notarize = &zapp.SignConfig{}, &zapp.NotarizeConfig{}
	for _, c := range []struct {
		all     []field
		methods [][]string
	}{{g.signAllFields(), signMethodFields}, {g.notaryAllFields(), notaryMethodFields}} {
		for _, labels := range c.methods {
			for _, label := range labels {
				if !slices.ContainsFunc(c.all, func(f field) bool { return f.Label == label }) {
					t.Errorf("no field is labelled %q", label)
				}
			}
		}
	}
}

// A missing component root is shown on that component's Root directory
// field, and the health summary says which component it is.
func TestValidateFindsTheComponentWithAMissingRoot(t *testing.T) {
	g := testEditor(t)
	g.tab = tabPKG
	if err := os.Mkdir(g.assetPath("payload"), 0o700); err != nil {
		t.Fatal(err)
	}
	g.s.Project.PKG.Components = []zapp.Component{{ID: "app", Root: "payload"}, {ID: "support", Root: "missing"}}
	g.componentIndex = 0
	g.rebuild()
	if issue := g.health.Issues[tabPKG]; !strings.HasPrefix(issue, "Component 2 root directory: ") {
		t.Errorf("PKG health issue = %q", issue)
	}
	g.validate()
	if g.tab != tabPKG || g.componentIndex != 1 || g.active < 0 || g.fields[g.active].Label != labelRootDirectory {
		t.Fatalf("validate went to tab %d, component %d, field %d", g.tab, g.componentIndex, g.active)
	}
}

// A DMG item whose source is missing has no field to show it on, so
// locating the problem selects the item instead.
func TestAMissingItemSourceSelectsTheItem(t *testing.T) {
	g := testEditor(t)
	g.s.Project.DMG.Contents = map[string]zapp.Content{"gone.txt": {Pos: &zapp.Position{50, 50}}}
	g.rebuild()
	loc, ok := g.locateIssue(&os.PathError{Op: "stat", Path: g.assetPath("gone.txt"), Err: os.ErrNotExist})
	if !ok || loc.tab != tabDMG || loc.item != "gone.txt" {
		t.Fatalf("located %+v, %v", loc, ok)
	}
	g.showIssue(loc, os.ErrNotExist)
	if g.tab != tabDMG || g.selected != "gone.txt" {
		t.Fatalf("tab %d, selected %q; want the DMG tab with gone.txt selected", g.tab, g.selected)
	}
}
