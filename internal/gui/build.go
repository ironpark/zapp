package gui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ironpark/zapp"
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
	log                                  []string // every line reported, oldest first
	logText                              string   // log joined, as of the last text call
	logDirty                             bool
	artifacts                            zapp.Artifacts
	finished, cancelling, closeRequested bool
	err                                  error
}

// maxBuildLog bounds the lines a build dialog keeps; older ones are dropped.
const maxBuildLog = 1000

func (j *buildJob) record(line string) {
	j.log = append(j.log, line)
	if len(j.log) > maxBuildLog {
		j.log = j.log[len(j.log)-maxBuildLog:]
	}
	j.logDirty = true
}

// text is the log as the dialog shows it, joined again only after new lines.
func (j *buildJob) text() string {
	if j.logDirty {
		j.logText, j.logDirty = strings.Join(j.log, "\n"), false
	}
	return j.logText
}

type buildLogger struct {
	events chan string
	wake   func()
}

func (l buildLogger) Printf(format string, args ...any) (int, error) {
	s := fmt.Sprintf(format, args...)
	select {
	case l.events <- strings.TrimSpace(s):
		l.wake()
	default:
	}
	return len(s), nil
}

func runProjectBuild(ctx context.Context, project *zapp.Project, logger zapp.Logger) (zapp.Artifacts, error) {
	if err := ctx.Err(); err != nil {
		return zapp.Artifacts{}, err
	}
	if !project.Builds() {
		return zapp.Artifacts{}, fmt.Errorf("enable DMG, PKG, ZIP or Dependencies before building")
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
	job := &buildJob{cancel: cancel, events: make(chan string, 256), done: make(chan buildResult, 1), message: "Resolving current project settings…"}
	if g.dirty() {
		job.record("Building with unsaved settings; " + g.s.Name + " is unchanged until you save.")
	}
	g.build = job
	project := g.s.Project.Clone()
	g.report(nil, "Building project…")
	wake := g.wakeFunc()
	go func() {
		artifacts, err := runProjectBuild(ctx, project, buildLogger{job.events, wake})
		job.done <- buildResult{artifacts, err}
		wake()
	}()
}

func (g *editor) pollBuild() {
	j := g.build
	if j == nil || j.finished {
		return
	}
	// Take the result before draining: the builder sends done after its
	// last line, so draining afterwards cannot leave final lines behind.
	var result buildResult
	finished := false
	select {
	case result = <-j.done:
		finished = true
	default:
	}
	// A burst of log lines shows only its last one in the status bar, so
	// report once after draining rather than syncing the UI per line.
	latest := ""
drain:
	for {
		select {
		case message := <-j.events:
			j.record(message)
			if !j.cancelling {
				latest = message
			}
		default:
			break drain
		}
	}
	if latest != "" {
		j.message = latest
		g.report(nil, latest)
	}
	if !finished {
		return
	}
	j.finished, j.err, j.artifacts = true, result.err, result.artifacts
	j.cancel()
	switch {
	case errors.Is(result.err, context.Canceled):
		j.message = "Build cancelled."
	case result.err != nil:
		j.message = result.err.Error()
	default:
		a := result.artifacts
		paths := []string{}
		for _, path := range []string{a.Zip, a.DMG, a.PKG, a.Checksums, a.Appcast} {
			if path != "" {
				paths = append(paths, path)
			}
		}
		if len(paths) == 0 {
			paths = append(paths, a.App)
		}
		j.message = "Built " + strings.Join(paths, " · ")
		if len(a.Uploads) > 0 {
			j.message += fmt.Sprintf(" · uploaded %d", len(a.Uploads))
		}
	}
	g.report(result.err, j.message)
	if result.err != nil && !errors.Is(result.err, context.Canceled) {
		g.locateValidationError(result.err)
	} else {
		g.issue = nil
	}
	// Builds can change app resources; refresh the preview on the next rebuild.
	g.previewSig = ""
	g.recheckHealth()
	if j.closeRequested {
		g.finishClose()
	}
}

// finishClose resolves the window close that was deferred while a build ran.
func (g *editor) finishClose() {
	g.build = nil
	if g.dirty() {
		g.confirmClose = true
	} else {
		g.requestQuit()
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

// buildDialog returns the build modal's title and message.
func (g *editor) buildDialog() (title, message string) {
	j := g.build
	title = "Building project"
	if j.finished {
		title = "Build complete"
		if j.err != nil {
			title = "Build failed"
		}
		if errors.Is(j.err, context.Canceled) {
			title = "Build cancelled"
		}
	}
	return title, j.message
}
