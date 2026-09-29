package zapp

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"github.com/ironpark/zapp/internal/fsutil"
	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/archive"
	"github.com/ironpark/zapp/pkg/dep"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/ironpark/zapp/pkg/macpkg"
	"github.com/ironpark/zapp/pkg/signing"
	"github.com/ironpark/zapp/pkg/upload"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed iconfile.icns
var defaultIconFile []byte

type Step string

const (
	StepDep      Step = "dep"
	StepDMG      Step = "dmg"
	StepPKG      Step = "pkg"
	StepSign     Step = "sign"
	StepNotarize Step = "notarize"
	StepStaple   Step = "staple"
	StepZip      Step = "zip"
	StepUpload   Step = "upload"
	// StepChecksums lists the SHA-256 of the archives and installers built.
	StepChecksums Step = "checksums"
	// StepAppcast adds the release to a Sparkle appcast.
	StepAppcast Step = "appcast"
)

type Artifacts struct {
	App       string
	DMG       string
	PKG       string
	Zip       string
	Checksums string
	Appcast   string
	Uploads   []Uploaded
}

// Uploaded is one artifact sent to one endpoint. URL leaves out the query
// string, which may hold a presigned credential.
type Uploaded struct {
	Artifact, URL string
}

func (p *Plan) log(format string, args ...any) {
	if p.logger != nil {
		_, _ = p.logger.Printf(format, args...)
	}
}
func stepError(step Step, err error) error {
	if err == nil {
		return nil
	}
	return &StepError{Step: step, Err: err}
}
func (p *Plan) BundleDeps(ctx context.Context) error {
	if p.Dep == nil {
		return stepError(StepDep, fmt.Errorf("dep section is not configured"))
	}
	p.log("Bundling dependencies for %s\n", p.App)
	return stepError(StepDep, dep.Bundle(ctx, *p.Dep))
}
func (p *Plan) BuildDMG(ctx context.Context) (out string, err error) {
	defer func() { err = stepError(StepDMG, err) }()
	if p.DMG == nil {
		return "", fmt.Errorf("dmg section is not configured")
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	c := *p.DMG
	if (c.Icon == "" && p.App != "") || strings.EqualFold(filepath.Ext(c.Icon), ".png") {
		source := c.Icon
		disk := false
		if source == "" {
			source = p.App
			disk = !p.originalIcon
		}
		tmp, err := os.MkdirTemp("", "zapp-icon-*")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(tmp)
		c.Icon = filepath.Join(tmp, "icon.icns")
		if err = createIconSet(source, c.Icon, disk); err != nil {
			return "", fmt.Errorf("could not derive disk icon from %s: %w; supply --icon with an .icns or .png", source, err)
		}
	}
	if err = os.MkdirAll(filepath.Dir(c.FileName), 0755); err != nil {
		return "", err
	}
	p.log("Creating DMG %s\n", c.FileName)
	if err = dmg.CreateDMG(ctx, c); err != nil {
		return "", err
	}
	return c.FileName, nil
}
func (p *Plan) BuildPKG(ctx context.Context) (out string, err error) {
	defer func() { err = stepError(StepPKG, err) }()
	if p.PKG == nil {
		return "", fmt.Errorf("pkg section is not configured")
	}
	s := p.PKG
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(s.Output), 0755); err != nil {
		return "", err
	}
	p.log("Creating PKG %s\n", s.Output)
	if s.App != nil {
		err = macpkg.BuildApp(ctx, *s.App)
		if err != nil {
			return "", err
		}
		return s.Output, nil
	}
	if s.Product == nil {
		if len(s.Components) != 1 {
			return "", fmt.Errorf("component package requires one component")
		}
		c := s.Components[0]
		c.OutputPath = s.Output
		err = macpkg.BuildComponent(ctx, c)
		if err != nil {
			return "", err
		}
		return s.Output, nil
	}
	tmp, err := os.MkdirTemp("", "zapp-components-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	product := *s.Product
	product.Packages = nil
	if s.License != "" {
		resources := filepath.Join(tmp, "Resources")
		if product.ResourcesDir != "" {
			if err = os.CopyFS(resources, os.DirFS(product.ResourcesDir)); err != nil {
				return "", err
			}
		} else {
			if err = os.MkdirAll(resources, 0755); err != nil {
				return "", err
			}
		}
		if err = fsutil.CopyFile(s.License, filepath.Join(resources, filepath.Base(s.License))); err != nil {
			return "", err
		}
		product.ResourcesDir = resources
	}

	for i, c := range s.Components {
		c.OutputPath = filepath.Join(tmp, fmt.Sprintf("component-%d.pkg", i))
		if err = macpkg.BuildComponent(ctx, c); err != nil {
			return "", err
		}
		product.Packages = append(product.Packages, c.OutputPath)
	}
	if err = macpkg.BuildProduct(ctx, product); err != nil {
		return "", err
	}
	return s.Output, nil
}
func (p *Plan) Sign(ctx context.Context, target string) (err error) {
	if p.SignCredentials == nil {
		return stepError(StepSign, fmt.Errorf("signing is not configured"))
	}
	b := p.signBackend
	if b == nil {
		if b, err = signing.Select(*p.SignCredentials); err != nil {
			return stepError(StepSign, err)
		}
		// A backend made here is this call's to release: a PKCS#12
		// certificate lives in a temporary keychain or file until then.
		defer func() {
			if closeErr := signing.Close(b); closeErr != nil {
				err = errors.Join(err, stepError(StepSign, closeErr))
			}
		}()
	}
	p.log("Signing %s\n", target)
	return stepError(StepSign, b.Sign(ctx, target))
}
func (p *Plan) Notarize(ctx context.Context, target string) error {
	if p.NotarizeCredentials == nil {
		return stepError(StepNotarize, fmt.Errorf("notarization is not configured"))
	}
	b := p.notaryBackend
	var err error
	if b == nil {
		b, err = signing.Select(*p.NotarizeCredentials)
	}
	if err != nil {
		return stepError(StepNotarize, err)
	}
	p.log("Notarizing %s\n", target)
	if err = signing.Notarize(ctx, b, target, false); err != nil {
		return stepError(StepNotarize, err)
	}
	if p.Staple {
		return stepError(StepStaple, b.Staple(ctx, target))
	}
	return nil
}

// Build executes selected build sections in dependency order. Signing the app
// before packaging ensures the installed app carries its own signature.
func (p *Plan) Build(ctx context.Context, steps ...Step) (Artifacts, error) {
	a := Artifacts{App: p.App}
	actions, err := p.actions(steps)
	if err != nil {
		return a, err
	}
	for _, act := range actions {
		if err := act.run(ctx, &a); err != nil {
			return a, err
		}
	}
	return a, nil
}

// Action is one thing a build does: its step and what it does, in words.
type Action struct {
	Step Step
	What string
	run  func(context.Context, *Artifacts) error
}

// DryRun lists what Build would do with the same steps, in order, without
// doing any of it: nothing is built, signed or sent, and no keychain or
// network is consulted. It fails where Build would fail before starting.
func (p *Plan) DryRun(steps ...Step) ([]Action, error) {
	return p.actions(steps)
}

// actions decides what a build does and in what order, and checks what can
// be checked before it starts.
func (p *Plan) actions(steps []Step) ([]Action, error) {
	selected := map[Step]bool{}
	if len(steps) == 0 {
		selected[StepDep] = p.Dep != nil
		selected[StepDMG] = p.DMG != nil
		selected[StepPKG] = p.PKG != nil
		selected[StepZip] = p.Zip != nil
		selected[StepChecksums] = p.Checksums != nil
		selected[StepAppcast] = p.Appcast != nil
		selected[StepUpload] = len(p.Uploads) > 0
	} else {
		for _, s := range steps {
			switch s {
			case StepDep, StepDMG, StepPKG, StepZip, StepChecksums, StepAppcast, StepUpload:
				selected[s] = true
			default:
				return nil, stepError(s, fmt.Errorf("unknown build step %q", s))
			}
		}
	}
	for _, c := range []struct {
		step       Step
		configured bool
	}{{StepDep, p.Dep != nil}, {StepDMG, p.DMG != nil}, {StepPKG, p.PKG != nil}, {StepZip, p.Zip != nil}, {StepChecksums, p.Checksums != nil}, {StepAppcast, p.Appcast != nil}, {StepUpload, len(p.Uploads) > 0}} {
		if selected[c.step] && !c.configured {
			return nil, stepError(c.step, fmt.Errorf("%s section is not configured", c.step))
		}
	}
	// An app whose frameworks lost their links cannot be signed or shipped,
	// and is refused before anything is built from it.
	if p.App != "" && (p.SignCredentials != nil || selected[StepZip] || selected[StepDMG] || selected[StepPKG]) {
		if err := appbundle.CheckLinks(p.App); err != nil {
			return nil, err
		}
	}
	// A missing or wrong key or token is found before the build, not after.
	if selected[StepAppcast] {
		if _, err := p.sparkleKey(); err != nil {
			return nil, stepError(StepAppcast, err)
		}
	}
	if selected[StepUpload] && githubToken() == "" && slices.ContainsFunc(p.Uploads, func(u UploadConfig) bool { return u.GitHub != nil }) {
		return nil, stepError(StepUpload, fmt.Errorf("uploading to a GitHub release needs a token in GITHUB_TOKEN or GH_TOKEN"))
	}

	var list []Action
	add := func(step Step, what string, run func(context.Context, *Artifacts) error) {
		list = append(list, Action{step, what, run})
	}
	sign := func(path string, c *signing.Credentials) {
		if c != nil {
			add(StepSign, fmt.Sprintf("sign %s with %s", path, c.SigningSummary()), func(ctx context.Context, _ *Artifacts) error {
				return p.Sign(ctx, path)
			})
		}
	}
	notarize := func(path string) {
		what := fmt.Sprintf("notarize %s with %s", path, p.NotarizeCredentials.NotarySummary())
		if p.Staple {
			what += ", then staple it"
		}
		add(StepNotarize, what, func(ctx context.Context, _ *Artifacts) error { return p.Notarize(ctx, path) })
	}

	if selected[StepDep] {
		add(StepDep, "bundle the libraries "+p.App+" links into it", func(ctx context.Context, _ *Artifacts) error { return p.BundleDeps(ctx) })
	}
	if p.App != "" {
		sign(p.App, p.SignCredentials)
	}
	// The app is notarized itself when it ships bare: in the ZIP, or after
	// bundling with nothing to package it. Its ZIP is what gets submitted
	// unless the ticket is to be stapled, which changes the app, so the
	// archive has to be made afterwards; either way the bundle is archived
	// once. A stapled app also reaches the DMG and PKG built next.
	bare := selected[StepDep] && !selected[StepDMG] && !selected[StepPKG]
	notarizeApp := p.NotarizeCredentials != nil && (selected[StepZip] || bare)
	if notarizeApp && (!selected[StepZip] || p.Staple) {
		notarize(p.App)
		notarizeApp = false
	}
	if selected[StepZip] {
		add(StepZip, fmt.Sprintf("archive %s as %s", p.App, p.Zip.Output), func(ctx context.Context, a *Artifacts) (err error) {
			a.Zip, err = p.BuildZip(ctx)
			return err
		})
		if notarizeApp {
			notarize(p.Zip.Output)
		}
	}
	var installers []string
	if selected[StepDMG] {
		add(StepDMG, "create "+p.DMG.FileName, func(ctx context.Context, a *Artifacts) (err error) {
			a.DMG, err = p.BuildDMG(ctx)
			return err
		})
		installers = append(installers, p.DMG.FileName)
	}
	if selected[StepPKG] {
		add(StepPKG, "create "+p.PKG.Output, func(ctx context.Context, a *Artifacts) (err error) {
			a.PKG, err = p.BuildPKG(ctx)
			return err
		})
		installers = append(installers, p.PKG.Output)
	}
	// Every installer is signed before any is notarized.
	if c := p.SignCredentials; c != nil && len(installers) > 0 {
		x := *c
		x.Entitlements = "" // an app's alone
		for _, path := range installers {
			sign(path, &x)
		}
	}
	if p.NotarizeCredentials != nil {
		for _, path := range installers {
			notarize(path)
		}
	}
	// Listed once every artifact is final: signed, notarized and stapled.
	if selected[StepChecksums] {
		add(StepChecksums, "write the SHA-256 of the ZIP, DMG and PKG to "+p.Checksums.Output, func(ctx context.Context, a *Artifacts) (err error) {
			a.Checksums, err = p.WriteChecksums(ctx, *a)
			return err
		})
	}
	if selected[StepAppcast] {
		what := fmt.Sprintf("add the %s to appcast %s", p.Appcast.Artifact, p.Appcast.Output)
		if p.Appcast.Feed != "" {
			what += ", extending " + upload.Redact(p.Appcast.Feed)
		}
		add(StepAppcast, what, func(ctx context.Context, a *Artifacts) (err error) {
			a.Appcast, err = p.WriteAppcast(ctx, *a)
			return err
		})
	}
	if selected[StepUpload] {
		for i, u := range p.Uploads {
			names := "every artifact built"
			if len(u.Artifacts) > 0 {
				names = strings.Join(u.Artifacts, ", ")
			}
			add(StepUpload, fmt.Sprintf("upload %s to %s", names, u.endpoint()), func(ctx context.Context, a *Artifacts) (err error) {
				a.Uploads, err = p.uploadTo(ctx, i, *a, a.Uploads)
				return stepError(StepUpload, err)
			})
		}
	}
	return list, nil
}

func (p *Plan) BuildZip(ctx context.Context) (out string, err error) {
	if p.Zip == nil {
		return "", stepError(StepZip, fmt.Errorf("zip section is not configured"))
	}
	p.log("Archiving %s as %s\n", p.App, p.Zip.Output)
	if err = archive.Zip(ctx, p.App, p.Zip.Output); err != nil {
		return "", stepError(StepZip, err)
	}
	return p.Zip.Output, nil
}

// WriteChecksums lists the SHA-256 of the ZIP, DMG and PKG in a, by file
// name, as `shasum -a 256` prints them.
func (p *Plan) WriteChecksums(ctx context.Context, a Artifacts) (out string, err error) {
	defer func() { err = stepError(StepChecksums, err) }()
	if p.Checksums == nil {
		return "", fmt.Errorf("checksums section is not configured")
	}
	var list strings.Builder
	for _, path := range []string{a.Zip, a.DMG, a.PKG} {
		if path == "" {
			continue
		}
		if err = ctx.Err(); err != nil {
			return "", err
		}
		sum, err := sha256File(path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&list, "%x  %s\n", sum, filepath.Base(path))
	}
	if list.Len() == 0 {
		return "", fmt.Errorf("nothing to list; build a zip, dmg or pkg")
	}
	out = p.Checksums.Output
	if err = os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	p.log("Writing checksums to %s\n", out)
	return out, fsutil.WriteFileAtomic(out, []byte(list.String()), 0o644)
}

func sha256File(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// Upload sends a build's artifacts to every configured endpoint. An endpoint
// that names an artifact this build did not make skips it; one that receives
// nothing at all is an error.
func (p *Plan) Upload(ctx context.Context, a Artifacts) (sent []Uploaded, err error) {
	defer func() { err = stepError(StepUpload, err) }()
	for i := range p.Uploads {
		if sent, err = p.uploadTo(ctx, i, a, sent); err != nil {
			return sent, err
		}
	}
	return sent, nil
}

// uploadTo sends a's artifacts to endpoint i and appends where they went to
// sent.
func (p *Plan) uploadTo(ctx context.Context, i int, a Artifacts, sent []Uploaded) ([]Uploaded, error) {
	spec := p.Uploads[i]
	built := map[string]string{"zip": a.Zip, "dmg": a.DMG, "pkg": a.PKG, "checksums": a.Checksums, "appcast": a.Appcast}
	names := spec.Artifacts
	if len(names) == 0 {
		names = UploadArtifacts
	}
	var files []artifactFile
	for _, name := range names {
		if path := built[name]; path != "" {
			files = append(files, artifactFile{name, path})
		} else if len(spec.Artifacts) > 0 {
			p.log("Skipping %s upload: this build made no %s\n", name, name)
		}
	}
	if len(files) == 0 {
		return sent, fmt.Errorf("upload[%d] has nothing to send; build a %s first", i, strings.Join(names, ", "))
	}
	return p.send(ctx, spec, files, sent)
}

// UploadFiles sends files to every configured endpoint. Each is reported by
// its file name.
func (p *Plan) UploadFiles(ctx context.Context, paths ...string) (sent []Uploaded, err error) {
	defer func() { err = stepError(StepUpload, err) }()
	var files []artifactFile
	for _, path := range paths {
		files = append(files, artifactFile{filepath.Base(path), path})
	}
	for _, spec := range p.Uploads {
		if sent, err = p.send(ctx, spec, files, sent); err != nil {
			return sent, err
		}
	}
	return sent, nil
}

// artifactFile is an artifact to upload and the file it was built as.
type artifactFile struct{ artifact, path string }

// send uploads files to one endpoint and appends where they went to sent.
func (p *Plan) send(ctx context.Context, spec UploadConfig, files []artifactFile, sent []Uploaded) ([]Uploaded, error) {
	file := func(ctx context.Context, path string) (string, error) {
		return upload.File(ctx, p.httpClient, spec.Target(), path)
	}
	if spec.GitHub != nil {
		assets, err := upload.OpenRelease(ctx, p.httpClient, spec.GitHub.release())
		if err != nil {
			return sent, err
		}
		file = assets.Send
	}
	for _, f := range files {
		p.log("Uploading %s to %s\n", f.path, spec.destination(filepath.Base(f.path)))
		url, err := file(ctx, f.path)
		if err != nil {
			return sent, err
		}
		sent = append(sent, Uploaded{Artifact: f.artifact, URL: url})
	}
	return sent, nil
}
