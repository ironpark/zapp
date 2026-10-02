package gui

import "testing"

func TestDraftIsolation(t *testing.T) {
	i := NewInput(InputSpec{Value: "가나다"})
	i.SetText("가🙂나다")
	snapshot := i.Clone()
	i.SetText("가다")
	if i.Text() != "가다" || snapshot.Text() != "가🙂나다" {
		t.Fatal("clone isolation failed")
	}
	if i.Spec.Value != "가나다" || !i.Dirty() {
		t.Fatal("draft changed committed value")
	}
}

func TestSetTextSanitation(t *testing.T) {
	i := NewInput(InputSpec{})
	if i.SetText("a\tb\n\x00c"); i.Text() != "a bc" {
		t.Fatalf("single-line sanitation: %q", i.Text())
	}
	i = NewInput(InputSpec{Multiline: true})
	if i.SetText("a\tb\n\x00c"); i.Text() != "a b\nc" {
		t.Fatalf("multiline sanitation: %q", i.Text())
	}
}

func TestNumberStepping(t *testing.T) {
	i := NewInput(InputSpec{Value: "0", Number: &NumberSpec{Min: 16, Max: 128, Step: 1, Default: 64}})
	i.StepNumber(1, false)
	if i.Text() != "65" {
		t.Fatalf("default step: %s", i.Text())
	}
	i.StepNumber(-1, true)
	if i.Text() != "55" {
		t.Fatalf("large step: %s", i.Text())
	}
	i.SetText("128")
	if i.StepNumber(1, false); i.Text() != "128" {
		t.Fatal("exceeded maximum")
	}
	i.SetText("16")
	if i.StepNumber(-1, false); i.Text() != "16" {
		t.Fatal("below minimum")
	}
	i.SetText("invalid")
	if i.StepNumber(1, false); i.Text() != "invalid" {
		t.Fatal("invalid draft silently replaced")
	}
	i.SetText("")
	if i.StepNumber(-1, false); i.Text() != "63" {
		t.Fatalf("empty default: %s", i.Text())
	}
	if i.Spec.Value != "0" {
		t.Fatal("stepping mutated committed value")
	}
}

func TestPlaceholderIsNotAValue(t *testing.T) {
	i := NewInput(InputSpec{Placeholder: "MyApp.app"})
	if i.Text() != "" || i.Dirty() {
		t.Fatal("placeholder became input value")
	}
}
