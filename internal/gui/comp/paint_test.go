package comp

import "testing"

func testPainter(t *testing.T) *Painter {
	t.Helper()
	p, err := NewPainter(nil, DarkTheme())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// Fit binary-searches the prefix length, which is only correct while measured
// width grows monotonically with the prefix. Assert the resulting invariant:
// the returned string fits, and one more rune of the source would not.
func TestFitReturnsLongestFittingPrefix(t *testing.T) {
	p := testPainter(t)
	inputs := []string{
		"", "a", "hello world", "/Users/someone/Projects/zapp/dist/MyApplication.app",
		"한국어 텍스트 라벨", "iiiiiiiiiiiiiiiiiiii", "WWWWWWWWWWWWWWWWWWWW",
		"mixed 한글 and ASCII with spaces everywhere here",
	}
	for _, s := range inputs {
		for width := -2; width <= 320; width += 3 {
			for _, size := range []int{11, 14, 17} {
				got := p.Fit(s, width, size)
				if width <= 0 {
					if got != "" {
						t.Fatalf("Fit(%q, %d, %d) = %q, want empty", s, width, size, got)
					}
					continue
				}
				if got == s {
					continue // Untruncated: the whole string already fit.
				}
				if got != "" && p.Measure(got, size) > width {
					t.Fatalf("Fit(%q, %d, %d) = %q, which overflows by %d", s, width, size, got, p.Measure(got, size)-width)
				}
				r := []rune(s)
				n := len([]rune(got))
				if got != "" {
					n-- // Drop the ellipsis to recover the source prefix length.
				}
				if n < len(r) {
					if longer := string(r[:n+1]) + "…"; p.Measure(longer, size) <= width {
						t.Fatalf("Fit(%q, %d, %d) = %q, but %q also fits", s, width, size, got, longer)
					}
				}
			}
		}
	}
}
