package rcodesign

import (
	"testing"
)

func TestCredentialsConfigured(t *testing.T) {
	for name, tc := range map[string]struct {
		creds Options
		want  bool
	}{
		"empty": {Options{}, false},
		"pem":   {Options{PEMFile: "/c.pem"}, true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.creds.Configured(); got != tc.want {
				t.Errorf("Configured() = %v, want %v", got, tc.want)
			}
		})
	}
}
