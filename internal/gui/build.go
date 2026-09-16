package gui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
)

type buildResult struct {
	artifacts zapp.Artifacts
	err       error
}
type buildJob struct {
	cancel                               context.CancelFunc
	events                               chan string
	done                                 chan buildResult
	message                              string
	finished, cancelling, closeRequested bool
	err                                  error
}

type buildLogger struct{ events chan string }

func (l buildLogger) Printf(format string, args ...any) (int, error) {
	s := fmt.Sprintf(format, args...)
	select {
	case l.events <- strings.TrimSpace(s):
	default:
	}
	return len(s), nil
}

func runProjectBuild(ctx context.Context, project *zapp.Project, logger zapp.Logger) (zapp.Artifacts, error) {
	if err := ctx.Err(); err != nil {
		return zapp.Artifacts{}, err
	}
	if project.Dep == nil && project.DMG == nil && project.PKG == nil {
		return zapp.Artifacts{}, fmt.Errorf("enable DMG, PKG or Dependencies before building")
	}
	plan, err := project.Resolve(zapp.WithLogger(logger))
	if err != nil {
		return zapp.Artifacts{}, err
	}
	return plan.Build(ctx)
}

func (g *editor) startBuild() {
	if g.build != nil || !g.commit() || !g.validatePaths() {
		return
	}
	ctx := g.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	job := &buildJob{cancel: cancel, events: make(chan string, 32), done: make(chan buildResult, 1), message: "Resolving current project settings…"}
	g.build = job
	project := g.s.Project.Clone()
	g.report(nil, "Building project…")
	go func() {
		artifacts, err := runProjectBuild(ctx, project, buildLogger{job.events})
		job.done <- buildResult{artifacts, err}
	}()
}

func (g *editor) pollBuild() {
	j := g.build
	if j == nil || j.finished {
		return
	}
drain:
	for {
		select {
		case message := <-j.events:
			if !j.cancelling {
				j.message = message
				g.report(nil, message)
			}
		default:
			break drain
		}
	}
	select {
	case result := <-j.done:
		j.finished, j.err = true, result.err
		j.cancel()
		switch {
		case errors.Is(result.err, context.Canceled):
			j.message = "Build cancelled."
		case result.err != nil:
			j.message = result.err.Error()
		default:
			paths := []string{}
			for _, path := range []string{result.artifacts.DMG, result.artifacts.PKG} {
				if path != "" {
					paths = append(paths, path)
				}
			}
			if len(paths) == 0 {
				paths = append(paths, result.artifacts.App)
			}
			j.message = "Built: " + strings.Join(paths, " · ")
		}
		g.report(result.err, j.message)
		// Builds can change app resources; refresh the preview on the next rebuild.
		g.previewSig = ""
		if j.closeRequested {
			g.finishClose()
		}
	default:
	}
}

// finishClose resolves the window close that was deferred while a build ran.
func (g *editor) finishClose() {
	g.build = nil
	if g.dirty() {
		g.confirmClose = true
	} else {
		g.quit = true
	}
}

func (g *editor) dismissBuild() {
	if g.build.finished {
		g.build = nil
		g.rebuild()
		return
	}
	g.build.cancelling = true
	g.build.message = "Cancelling build… Waiting for cleanup."
	g.build.cancel()
}

func (g *editor) buildDialog() comp.Dialog {
	j := g.build
	title, action := "Building project", "Cancel build"
	if j.finished {
		title, action = "Build complete", "Close"
		if j.err != nil {
			title = "Build failed"
		}
		if errors.Is(j.err, context.Canceled) {
			title = "Build cancelled"
		}
	}
	return comp.Dialog{Visible: true, Bounds: comp.Center(comp.Box(0, 0, g.w, g.h), 600, 220), Title: title, Message: j.message, OnCancel: g.dismissBuild, Actions: []comp.Button{{Label: action, Disabled: j.cancelling && !j.finished, OnClick: g.dismissBuild}}}
}
