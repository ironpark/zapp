package rcodesign

import (
	"testing"
)

func TestCredentialsConfigured(t *testing.T) {
	for name, tc := range map[string]struct {
		creds Options
		want  bool
	}{
		"empty":              {Options{}, false},
		"p12":                {Options{P12File: "/c.p12"}, true},
		"pem":                {Options{PEMFile: "/c.pem"}, true},
		"password only":      {Options{P12Password: "pw"}, false},
		"password file only": {Options{P12PasswordFile: "/pw"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.creds.Configured(); got != tc.want {
				t.Errorf("Configured() = %v, want %v", got, tc.want)
			}
		})
	}
}
