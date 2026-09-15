package norm_test

import (
	"testing"

	"github.com/ironpark/zapp/internal/thirdparty/text/unicode/norm"
)

// These cases cover the filename normalization used by DSStore, including
// canonical ordering and supplementary-plane characters in the bundled tables.
func TestVendoredNFD(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"ASCII", "MyApp.app", "MyApp.app"},
		{"Latin", "caf\u00e9", "cafe\u0301"},
		{"Hangul", "\uac01", "\u1100\u1161\u11a8"},
		{"combining order", "a\u0315\u0300", "a\u0300\u0315"},
		{"supplementary", "\U0001d15e", "\U0001d157\U0001d165"},
		{"invalid UTF-8", "\xff\u00e9", "\xffe\u0301"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := norm.NFD.String(tc.input); got != tc.want {
				t.Fatalf("NFD(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
