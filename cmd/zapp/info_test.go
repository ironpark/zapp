package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

type failingWriter struct{ remaining int }

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.remaining == 0 {
		return 0, io.ErrClosedPipe
	}
	w.remaining--
	return len(p), nil
}

func TestInfoOutputErrors(t *testing.T) {
	// Fail after the header to exercise body output as well.
	if err := printInfo(&cli.Command{Writer: &failingWriter{remaining: 1}}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("printInfo: %v", err)
	}
	if err := print3rdPartyLicenseOverview(&cli.Command{Writer: &failingWriter{}}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("licenses: %v", err)
	}
}

func TestInfoUsesCommandWriter(t *testing.T) {
	var out bytes.Buffer
	if err := printInfo(&cli.Command{Writer: &out}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[Build Info]", Version, Commit, "MIT License"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q", want)
		}
	}
}
