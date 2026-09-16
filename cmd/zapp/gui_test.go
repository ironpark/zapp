package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestGUIHelpDoesNotOpenWindow(t *testing.T) {
	app := newApp()
	var output bytes.Buffer
	app.Writer = &output
	if err := app.Run(context.Background(), []string{"zapp", "gui", "--help"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "--config") {
		t.Fatalf("missing GUI config flag: %s", output.String())
	}
}

func TestGUIRejectsPositionalArguments(t *testing.T) {
	err := newApp().Run(context.Background(), []string{"zapp", "gui", "unexpected"})
	if err == nil || !strings.Contains(err.Error(), "no positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}
