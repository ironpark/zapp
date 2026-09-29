package rcodesign

import (
	"errors"
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

func TestReadableLog(t *testing.T) {
	err := readableLog(errors.New(`notarization submission 2efe ended Invalid; notary log: {"status":"Invalid","statusSummary":"Archive contains critical validation errors","issues":[{"severity":"error","path":"Demo.zip/Demo.app/Contents/MacOS/Demo","message":"The signature does not include a secure timestamp.","architecture":"arm64"}]}`))
	want := "notarization submission 2efe ended Invalid: Archive contains critical validation errors\n  Demo.app/Contents/MacOS/Demo (arm64): The signature does not include a secure timestamp."
	if err.Error() != want {
		t.Fatalf("got %q", err)
	}
	plain := errors.New("an App Store Connect API key file is required")
	if readableLog(plain) != plain {
		t.Fatal("rewrote an error without a log")
	}
}
