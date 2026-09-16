package comp

import (
	"errors"
	"github.com/hajimehoshi/ebiten/v2"
	"testing"
)

type memoryClipboard struct {
	text string
	err  error
}

func (c *memoryClipboard) ReadText() (string, error) { return c.text, c.err }
func (c *memoryClipboard) WriteText(s string) error {
	if c.err != nil {
		return c.err
	}
	c.text = s
	return nil
}
func key(k ebiten.Key) Keyboard     { return Keyboard{Pressed: []ebiten.Key{k}} }
func command(k ebiten.Key) Keyboard { return Keyboard{Command: true, Pressed: []ebiten.Key{k}} }

func TestUnicodeEditingAndDraftIsolation(t *testing.T) {
	i := NewInput(InputSpec{Value: "가나다"})
	i.SetCursor(1)
	i.Handle(Keyboard{Text: "🙂"}, nil)
	if i.Text() != "가🙂나다" || i.Cursor() != 2 {
		t.Fatalf("insertion: %q cursor %d", i.Text(), i.Cursor())
	}
	snapshot := i.Clone()
	i.Handle(key(ebiten.KeyBackspace), nil)
	i.Handle(key(ebiten.KeyDelete), nil)
	if i.Text() != "가다" || snapshot.Text() != "가🙂나다" {
		t.Fatal("rune editing or clone isolation failed")
	}
	if i.Spec.Value != "가나다" || !i.Dirty() {
		t.Fatal("draft changed committed value")
	}
	if i.Handle(key(ebiten.KeyEnter), nil).Intent != InputSubmit {
		t.Fatal("single-line Enter must request submit")
	}
	if i.Text() != "가다" {
		t.Fatal("submit lost the draft before validation")
	}
}

func TestMultilineNavigationAndSubmit(t *testing.T) {
	i := NewInput(InputSpec{Value: "가나다\nabcde\nxy", Multiline: true})
	i.SetCursor(8)
	i.Handle(key(ebiten.KeyArrowUp), nil)
	if i.Cursor() != 3 {
		t.Fatalf("up: %d", i.Cursor())
	}
	i.Handle(key(ebiten.KeyArrowDown), nil)
	if i.Cursor() != 7 {
		t.Fatalf("down: %d", i.Cursor())
	}
	if i.Handle(key(ebiten.KeyEnter), nil).Intent != InputIdle || i.Text() != "가나다\nabc\nde\nxy" {
		t.Fatal("multiline Enter must insert a newline")
	}
	if i.Handle(command(ebiten.KeyEnter), nil).Intent != InputSubmit {
		t.Fatal("command Enter must submit")
	}
	if i.Handle(key(ebiten.KeyTab), nil).Intent != InputNext {
		t.Fatal("Tab must request focus movement")
	}
	if i.Handle(Keyboard{Shift: true, Pressed: []ebiten.Key{ebiten.KeyTab}}, nil).Intent != InputPrevious {
		t.Fatal("Shift Tab must move backwards")
	}
	if i.Handle(key(ebiten.KeyEscape), nil).Intent != InputCancel {
		t.Fatal("Escape must request cancellation")
	}
}

func TestClipboardFailureDoesNotDestroySelection(t *testing.T) {
	i := NewInput(InputSpec{Value: "keep me"})
	i.SelectAll()
	c := &memoryClipboard{err: errors.New("clipboard unavailable")}
	if result := i.Handle(command(ebiten.KeyX), c); result.Err == nil || i.Text() != "keep me" {
		t.Fatal("failed cut destroyed the draft")
	}
	c.err = nil
	i.Handle(command(ebiten.KeyX), c)
	if i.Text() != "" || c.text != "keep me" {
		t.Fatal("cut after retry failed")
	}
	c.text = "a\tb\n\x00c"
	i.Handle(command(ebiten.KeyV), c)
	if i.Text() != "a bc" {
		t.Fatalf("single-line paste sanitation: %q", i.Text())
	}
	i.Handle(command(ebiten.KeyA), c)
	i.Handle(Keyboard{Text: "교체"}, c)
	if i.Text() != "교체" {
		t.Fatal("select-all replacement failed")
	}
}

func TestChoiceIsDraftUntilApplied(t *testing.T) {
	i := NewInput(InputSpec{Value: "", Choices: []string{"", "udzo", "ulfo"}})
	if r := i.Handle(key(ebiten.KeyEnter), nil); r.Intent != InputSubmit || i.Text() != "udzo" || i.Spec.Value != "" {
		t.Fatal("choice was not returned as a draft")
	}
	i.NextChoice()
	if i.Text() != "ulfo" {
		t.Fatal("next choice failed")
	}
	i.NextChoice()
	if i.Text() != "" {
		t.Fatal("choice did not wrap")
	}
}
