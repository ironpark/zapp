package zapp

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/ironpark/zapp/internal/fsutil"
	"github.com/ironpark/zapp/pkg/dep"
	"github.com/ironpark/zapp/pkg/dmg"
	"github.com/ironpark/zapp/pkg/macpkg"
	"github.com/ironpark/zapp/pkg/signing"
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
)

type Artifacts struct {
	App string
	DMG string
	PKG string
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
func (p *Plan) Sign(ctx context.Context, target string) error {
	if p.SignCredentials == nil {
		return stepError(StepSign, fmt.Errorf("signing is not configured"))
	}
	b := p.signBackend
	var err error
	if b == nil {
		b, err = signing.Select(*p.SignCredentials)
	}
	if err != nil {
		return stepError(StepSign, err)
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
	} else {
		for _, s := range steps {
			switch s {
			case StepDep, StepDMG, StepPKG:
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
	if len(targets) == 0 && selected[StepDep] {
		targets = append(targets, p.App)
	}
	for _, target := range targets {
		if p.NotarizeCredentials != nil {
			if err = p.Notarize(ctx, target); err != nil {
				return a, err
			}
		}
	}
	return a, nil
}
