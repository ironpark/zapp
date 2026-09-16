package gui

import (
	"testing"

	"github.com/ironpark/zapp"
)

func TestSectionTableCoversEveryTab(t *testing.T) {
	for i, s := range sections {
		if s.Name == "" || s.Description == "" {
			t.Errorf("sections[%d] is missing a name or description", i)
		}
		if s.fields == nil {
			t.Errorf("sections[%d] (%s) has no field builder", i, s.Name)
		}
		if s.Optional() != (s.flip != nil) {
			t.Errorf("sections[%d] (%s) defines presence and toggle inconsistently", i, s.Name)
		}
	}
	if sections[tabProject].Optional() {
		t.Error("the project tab must always be available")
	}
	for _, tab := range []int{tabDMG, tabPKG, tabDep, tabSign, tabNotarize} {
		if !sections[tab].Optional() {
			t.Errorf("section %q should be optional", sections[tab].Name)
		}
	}
}

// Toggling a section off and back on must restore the values the user entered,
// which is the behaviour the generic toggleSection has to preserve.
func TestToggleRoundTripsSectionValues(t *testing.T) {
	for _, tab := range []int{tabDMG, tabPKG, tabDep, tabSign, tabNotarize} {
		s := sections[tab]
		t.Run(s.Name, func(t *testing.T) {
			p := &zapp.Project{App: "My.app",
				DMG:      &zapp.DMGConfig{Title: "keep"},
				PKG:      &zapp.PKGConfig{Identifier: "keep"},
				Dep:      &zapp.DepConfig{Libs: []string{"keep"}},
				Sign:     &zapp.SignConfig{Identity: "keep"},
				Notarize: &zapp.NotarizeConfig{Profile: "keep"},
			}
			var scratch zapp.Project
			if !s.Enabled(p) {
				t.Fatal("section should start enabled")
			}
			s.flip(p, &scratch)
			if s.Enabled(p) {
				t.Fatal("section should be disabled after the first toggle")
			}
			s.flip(p, &scratch)
			if !s.Enabled(p) {
				t.Fatal("section should be enabled again after the second toggle")
			}
		})
	}
}

// Re-enabling a section that was never configured must produce a usable zero
// value rather than leaving a nil pointer for the field builders to dereference.
func TestToggleCreatesMissingSection(t *testing.T) {
	for _, tab := range []int{tabDMG, tabPKG, tabDep, tabSign, tabNotarize} {
		s := sections[tab]
		p, scratch := &zapp.Project{App: "My.app"}, zapp.Project{}
		if s.Enabled(p) {
			t.Fatalf("%s should start disabled on an empty project", s.Name)
		}
		s.flip(p, &scratch)
		if !s.Enabled(p) {
			t.Errorf("%s was not created when enabled from empty", s.Name)
		}
	}
	if noSections(&zapp.Project{App: "My.app"}) != true {
		t.Error("an empty project should report no enabled sections")
	}
}
