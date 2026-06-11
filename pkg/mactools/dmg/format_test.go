package dmg

import (
	"testing"

	"github.com/ironpark/zapp/pkg/mactools/hdiutil"
)

func TestResolveOutputFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    hdiutil.Format
		wantErr bool
	}{
		{"", hdiutil.UDZO, false},
		{"UDZO", hdiutil.UDZO, false},
		{"udzo", hdiutil.UDZO, false},
		{"UDRO", hdiutil.UDRO, false},
		{"UDBZ", hdiutil.UDBZ, false},
		{"UDCO", hdiutil.UDCO, false},
		{"UDRW", "", true},
	}

	for _, tt := range tests {
		got, err := resolveOutputFormat(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("resolveOutputFormat(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Fatalf("resolveOutputFormat(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Fatalf("resolveOutputFormat(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
