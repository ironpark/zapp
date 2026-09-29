package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/ironpark/zapp"
	"github.com/urfave/cli/v3"
)

func buildFlags() []cli.Flag {
	var out []cli.Flag
	seen := map[string]bool{}
	for _, flags := range [][]cli.Flag{dmgCommand.Flags, pkgCommand.Flags, {&cli.StringSliceFlag{Name: "libs", Aliases: []string{"l"}}}, distributionFlags()} {
		for _, f := range flags {
			if !seen[f.Names()[0]] {
				seen[f.Names()[0]] = true
				// Each command owns independent parsing state.
				switch v := f.(type) {
				case *cli.StringFlag:
					x := *v
					out = append(out, &x)
				case *cli.IntFlag:
					x := *v
					out = append(out, &x)
				case *cli.BoolFlag:
					x := *v
					out = append(out, &x)
				case *cli.StringSliceFlag:
					x := *v
					out = append(out, &x)
				}
			}
		}
	}
	return out
}
func buildCommand() *cli.Command {
	flags := append(buildFlags(), &cli.BoolFlag{Name: "dry-run", Usage: "List what the build would do, in order and with which credentials, without doing it"}, &cli.StringFlag{Name: "artifacts", Usage: "Append the artifact paths to this file as app=, zip=, dmg=, pkg=, checksums=, appcast= and homebrew= lines, and upload URLs as zip-url= and so on, e.g. $GITHUB_OUTPUT"})
	return &cli.Command{Name: "build", Usage: "Build project sections in deployment order", ArgsUsage: "[dep|zip|dmg|pkg|verify|checksums|appcast|upload|homebrew ...]", Flags: flags, Action: func(ctx context.Context, c *cli.Command) error {
		p, err := loadProject(c, "build")
		if err != nil {
			return err
		}
		pl, err := p.Resolve(zapp.WithLogger(newAppLogger(c.Root())))
		if err != nil {
			return err
		}
		if !p.Builds() {
			return fmt.Errorf("build requires a project with dep, zip, dmg, or pkg sections")
		}
		if c.Bool("dry-run") {
			return dryRun(c.Root().Writer, pl, buildSteps(c, p))
		}
		artifacts, err := pl.Build(ctx, buildSteps(c, p)...)
		if err != nil {
			return err
		}
		if file := c.String("artifacts"); file != "" {
			return writeArtifacts(file, artifacts)
		}
		return nil
	}}
}

// dryRun prints what the build would do, one numbered line per action.
func dryRun(w io.Writer, pl *zapp.Plan, steps []zapp.Step) error {
	actions, err := pl.DryRun(steps...)
	if err != nil {
		return err
	}
	if len(actions) == 0 {
		_, err = fmt.Fprintln(w, "Nothing to do.")
		return err
	}
	fmt.Fprintln(w, "The build would, in order:")
	for i, a := range actions {
		fmt.Fprintf(w, "%3d. %-9s %s\n", i+1, a.Step, a.What)
	}
	return nil
}

// buildSteps are the steps named on the command line. Asking for a ZIP,
// verification, checksums, an appcast, a cask, an upload endpoint or a GitHub release alongside them runs those
// steps too, as the project's own
// sections would with no steps named. p is the project with the command line
// applied, so --zip=false and --no-upload have already had their say.
func buildSteps(c *cli.Command, p *zapp.Project) []zapp.Step {
	var steps []zapp.Step
	for _, arg := range c.Args().Slice() {
		steps = append(steps, zapp.Step(arg))
	}
	if len(steps) == 0 {
		return nil
	}
	for _, step := range []zapp.Step{zapp.StepZip, zapp.StepVerify, zapp.StepChecksums, zapp.StepAppcast, zapp.StepHomebrew} {
		if on, _, _ := flagBool(c, string(step)); on && !slices.Contains(steps, step) {
			steps = append(steps, step)
		}
	}
	url, _ := flagValue(c, "upload-url")
	tag, _ := flagValue(c, "github-release")
	if url+tag != "" && len(p.Upload) > 0 && !slices.Contains(steps, zapp.StepUpload) {
		steps = append(steps, zapp.StepUpload)
	}
	return steps
}

// writeArtifacts appends the artifact paths as key=value lines, the form
// GitHub Actions reads from $GITHUB_OUTPUT. Paths are absolute so a later step
// in another directory can use them; a step that was not built is empty. An
// uploaded artifact's URL follows as <artifact>-url, from the first endpoint
// that received it.
func writeArtifacts(file string, a zapp.Artifacts) error {
	var lines, urls string
	seen := map[string]bool{}
	for _, u := range a.Uploads {
		if !seen[u.Artifact] {
			seen[u.Artifact] = true
			urls += u.Artifact + "-url=" + u.URL + "\n"
		}
	}
	for _, artifact := range [][2]string{{"app", a.App}, {"zip", a.Zip}, {"dmg", a.DMG}, {"pkg", a.PKG}, {"checksums", a.Checksums}, {"appcast", a.Appcast}, {"homebrew", a.Homebrew}} {
		path := artifact[1]
		if path != "" {
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			path = abs
		}
		lines += artifact[0] + "=" + path + "\n"
	}
	lines += urls
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(lines); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
