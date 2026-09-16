package gui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ironpark/zapp"
)

func TestGUIBuildProducesArtifactWithoutSavingProject(t *testing.T) {
	g := testEditor(t)
	g.ctx = t.Context()
	g.s.Project.PKG = nil
	g.s.Project.DMG.Title = "GUI build"
	g.s.Project.Out = "artifacts"
	x, y := 100, 100
	g.s.Project.DMG.Contents = map[string]zapp.Content{"source.txt": {Pos: &zapp.Position{x, y}}}
	if err := os.WriteFile(filepath.Join(g.s.Dir, "source.txt"), []byte("build test"), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(g.s.Path)
	g.rebuild()
	g.startBuild()
	if g.build == nil {
		t.Fatalf("build did not start: %s", g.status)
	}
	job := g.build
	defer job.cancel()
	select {
	case result := <-job.done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if info, err := os.Stat(result.artifacts.DMG); err != nil || info.Size() == 0 {
			t.Fatalf("missing DMG: %v", err)
		}
		job.done <- result
	case <-time.After(30 * time.Second):
		t.Fatal("build timed out")
	}
	g.pollBuild()
	if !job.finished || g.failed {
		t.Fatal("completion was not reported")
	}
	after, _ := os.ReadFile(g.s.Path)
	if string(before) != string(after) {
		t.Fatal("build rewrote project file")
	}
}

func TestBuildCancellationAndNoSteps(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := runProjectBuild(ctx, &zapp.Project{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := runProjectBuild(t.Context(), &zapp.Project{}, nil); err == nil {
		t.Fatal("empty build accepted")
	}
	g := testEditor(t)
	ctx, cancel = context.WithCancel(t.Context())
	g.build = &buildJob{cancel: cancel, done: make(chan buildResult, 1), events: make(chan string, 1)}
	g.dismissBuild()
	if ctx.Err() == nil || !g.build.cancelling {
		t.Fatal("cancel did not reach worker")
	}
	g.build.done <- buildResult{err: context.Canceled}
	g.pollBuild()
	if !g.build.finished || g.buildDialog().Title != "Build cancelled" {
		t.Fatal("cancellation did not finish")
	}
}
