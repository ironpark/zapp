package rcodesign

import (
	"slices"
	"testing"
)

func TestCredentialsArgs(t *testing.T) {
	for name, tc := range map[string]struct {
		creds Options
		want  []string
	}{
		"nothing": {Options{}, nil},
		"p12 alone": {
			Options{P12File: "/c.p12"},
			[]string{"--p12-file", "/c.p12"},
		},
		// A password file keeps the secret off the command line, so it wins
		// when both are given.
		"password file preferred": {
			Options{P12File: "/c.p12", P12Password: "pw", P12PasswordFile: "/pw.txt"},
			[]string{"--p12-file", "/c.p12", "--p12-password-file", "/pw.txt"},
		},
		"inline password": {
			Options{P12File: "/c.p12", P12Password: "pw"},
			[]string{"--p12-file", "/c.p12", "--p12-password", "pw"},
		},
		"pem": {
			Options{PEMFile: "/c.pem"},
			[]string{"--pem-file", "/c.pem"},
		},
		// A password with no bundle to open is not passed on.
		"orphan password": {
			Options{P12Password: "pw"},
			nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := tc.creds.args(); !slices.Equal(got, tc.want) {
				t.Errorf("args() = %q, want %q", got, tc.want)
			}
		})
	}
}

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
