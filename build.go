package zapp

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"github.com/ironpark/zapp/internal/fsutil"
	"github.com/ironpark/zapp/pkg/archive"
	"github.com/ironpark/zapp/pkg/dep"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/ironpark/zapp/pkg/macpkg"
	"github.com/ironpark/zapp/pkg/signing"
	"github.com/ironpark/zapp/pkg/upload"
	"io"
	"os"
	"path/filepath"
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
)

type Artifacts struct {
	App       string
	DMG       string
	PKG       string
	Zip       string
	Checksums string
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
	selected := map[Step]bool{}
	if len(steps) == 0 {
		selected[StepDep] = p.Dep != nil
		selected[StepDMG] = p.DMG != nil
		selected[StepPKG] = p.PKG != nil
		selected[StepZip] = p.Zip != nil
		selected[StepChecksums] = p.Checksums != nil
		selected[StepUpload] = len(p.Uploads) > 0
	} else {
		for _, s := range steps {
			switch s {
			case StepDep, StepDMG, StepPKG, StepZip, StepChecksums, StepUpload:
				selected[s] = true
			default:
				return a, stepError(s, fmt.Errorf("unknown build step %q", s))
			}
		}
	}
	if selected[StepDep] && p.Dep == nil {
		return a, stepError(StepDep, fmt.Errorf("dep section is not configured"))
	}
	if selected[StepDMG] && p.DMG == nil {
		return a, stepError(StepDMG, fmt.Errorf("dmg section is not configured"))
	}
	if selected[StepPKG] && p.PKG == nil {
		return a, stepError(StepPKG, fmt.Errorf("pkg section is not configured"))
	}
	if selected[StepZip] && p.Zip == nil {
		return a, stepError(StepZip, fmt.Errorf("zip section is not configured"))
	}
	if selected[StepChecksums] && p.Checksums == nil {
		return a, stepError(StepChecksums, fmt.Errorf("checksums section is not configured"))
	}
	if selected[StepUpload] && len(p.Uploads) == 0 {
		return a, stepError(StepUpload, fmt.Errorf("upload section is not configured"))
	}
	if selected[StepDep] {
		if err := p.BundleDeps(ctx); err != nil {
			return a, err
		}
	}
	if p.SignCredentials != nil && p.App != "" {
		if err := p.Sign(ctx, p.App); err != nil {
			return a, err
		}
	}
	var err error
	// The app is notarized itself when it ships bare: in the ZIP, or after
	// bundling with nothing to package it. Its ZIP is what gets submitted
	// unless the ticket is to be stapled, which changes the app, so the
	// archive has to be made afterwards; either way the bundle is archived
	// once. A stapled app also reaches the DMG and PKG built next.
	bare := selected[StepDep] && !selected[StepDMG] && !selected[StepPKG]
	notarizeApp := p.NotarizeCredentials != nil && (selected[StepZip] || bare)
	if notarizeApp && (!selected[StepZip] || p.Staple) {
		if err = p.Notarize(ctx, p.App); err != nil {
			return a, err
		}
		notarizeApp = false
	}
	if selected[StepZip] {
		if a.Zip, err = p.BuildZip(ctx); err != nil {
			return a, err
		}
		if notarizeApp {
			if err = p.Notarize(ctx, a.Zip); err != nil {
				return a, err
			}
		}
	}
	if selected[StepDMG] {
		a.DMG, err = p.BuildDMG(ctx)
		if err != nil {
			return a, err
		}
	}
	if selected[StepPKG] {
		a.PKG, err = p.BuildPKG(ctx)
		if err != nil {
			return a, err
		}
	}
	targets := []string{}
	if a.DMG != "" {
		targets = append(targets, a.DMG)
	}
	if a.PKG != "" {
		targets = append(targets, a.PKG)
	}
	for _, target := range targets {
		if p.SignCredentials != nil {
			if err = p.Sign(ctx, target); err != nil {
				return a, err
			}
		}
	}
	for _, target := range targets {
		if p.NotarizeCredentials != nil {
			if err = p.Notarize(ctx, target); err != nil {
				return a, err
			}
		}
	}
	// Listed once every artifact is final: signed, notarized and stapled.
	if selected[StepChecksums] {
		if a.Checksums, err = p.WriteChecksums(ctx, a); err != nil {
			return a, err
		}
	}
	if selected[StepUpload] {
		if a.Uploads, err = p.Upload(ctx, a); err != nil {
			return a, err
		}
	}
	return a, nil
}

// BuildZip archives the app, as it stands, for distribution. Build notarizes
// and staples the app around it.
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
	built := map[string]string{"zip": a.Zip, "dmg": a.DMG, "pkg": a.PKG, "checksums": a.Checksums}
	for i, spec := range p.Uploads {
		target := spec.Target()
		names := spec.Artifacts
		if len(names) == 0 {
			names = UploadArtifacts
		}
		count := 0
		for _, name := range names {
			path := built[name]
			if path == "" {
				if len(spec.Artifacts) > 0 {
					p.log("Skipping %s upload: this build made no %s\n", name, name)
				}
				continue
			}
			p.log("Uploading %s to %s\n", path, target.Location(filepath.Base(path)))
			url, err := upload.File(ctx, p.httpClient, target, path)
			if err != nil {
				return sent, err
			}
			sent = append(sent, Uploaded{Artifact: name, URL: url})
			count++
		}
		if count == 0 {
			return sent, fmt.Errorf("upload[%d] has nothing to send; build a %s first", i, strings.Join(names, ", "))
		}
	}
	return sent, nil
}
