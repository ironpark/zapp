package gui

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/ironpark/ggfx"
	"github.com/ironpark/ggui"
	uitheme "github.com/ironpark/ggui/ui/theme"
	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
)

// Opt-in GPU snapshots follow ggui's example render harness. The normal test
// suite stays headless. The graphics driver only draws and reads back pixels
// inside a frame, so the snapshots run in the frames of a hidden window.
func TestMain(m *testing.M) {
	if dir := os.Getenv("ZAPP_GUI_RENDER_DIR"); dir != "" {
		r := &desktopRenderer{directory: dir}
		if err := r.run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type desktopRenderer struct {
	directory string
}

// run renders every snapshot in the first frame of a window that is never
// shown, then ends the loop.
func (r *desktopRenderer) run() error {
	return ggfx.Run(ggfx.HandlerFunc(func(ev ggfx.Event) error {
		switch ev.(type) {
		case ggfx.StartEvent:
			_, err := ggfx.NewWindow(&ggfx.WindowOptions{Title: "Zapp layout previews", Width: 320, Height: 240, Hidden: true})
			return err
		case ggfx.FrameEvent:
			if err := r.render(); err != nil {
				return err
			}
			return ggfx.Termination
		}
		return nil
	}), nil)
}
func (r *desktopRenderer) render() error {
	if err := os.MkdirAll(r.directory, 0755); err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "zapp-desktop-preview-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	path := filepath.Join(temp, ".zapp.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nout: dist\ndmg: {}\npkg: {}\ndep: {}\nsign: {}\nnotarize: {}\n"), 0600); err != nil {
		return err
	}
	for _, width := range []int{1200, 1080} {
		s, err := Open(path)
		if err != nil {
			return err
		}
		s.Dir = "~/Projects/Example"
		s.Project.DMG.Contents = map[string]zapp.Content{"Example.app": {Pos: &zapp.Position{120, 160}}, "/Applications": {Pos: &zapp.Position{380, 160}, Link: true}}
		painter, err := comp.NewPainter(nil)
		if err != nil {
			return err
		}
		g := &editor{ctx: context.Background(), s: s, w: width, h: 760, active: -1, ui: painter, assets: map[string]*ggfx.Image{}, status: "Edit settings, then Save. Validation checks build inputs without building."}
		g.rebuild()
		m := newDesktopModel(g)
		p := ggui.ProbeBuilder(func() ggui.Widget { return ggui.Provide(ggui.ReducedMotionKey, true, desktopView(m)) }, ggui.Sz(width, 760))
		p.Setup(func() { uitheme.Bind(m.Dark, editorTheme(true), editorTheme(false)) })
		for _, name := range []string{"project", "dmg", "pkg", "components", "help", "json", "dependencies", "signing", "notarization", "distribution", "build", "light", "close"} {
			switch name {
			case "project":
				g.switchTab(tabProject)
			case "dmg":
				g.switchTab(tabDMG)
			case "pkg":
				g.switchTab(tabPKG)
			case "components":
				g.s.Project.PKG.Components = []zapp.Component{{ID: "app", Root: "build", Entry: "Example.app", InstallLocation: "/Applications"}, {ID: "support", Root: "resources", InstallLocation: "/Library/Application Support/Example"}}
				g.rebuild()
			case "help":
				g.helpOpen = true
			case "json":
				g.helpOpen = false
				g.pkgRaw = true
				g.rebuild()
			case "dependencies":
				g.switchTab(tabDep)
			case "signing":
				g.switchTab(tabSign)
				g.signing.identities, g.signing.listed = "Developer ID Application: Example (TEAMID1234)", true
			case "notarization":
				g.switchTab(tabNotarize)
			case "distribution":
				g.s.Project.Zip = &zapp.ZipConfig{}
				g.s.Project.Upload = []zapp.UploadConfig{{URL: "https://releases.example.com/${app.version}/${file.name}", Headers: map[string]string{"Authorization": "Bearer ${env:RELEASE_TOKEN}"}, Artifacts: []string{"zip", "dmg"}}, {GitHub: &zapp.GitHubRelease{Tag: "v${app.version}"}}}
				g.switchTab(tabDistribution)
			case "build":
				g.build = &buildJob{finished: true, message: "Built dist/Example.dmg", artifacts: zapp.Artifacts{DMG: "dist/Example.dmg"},
					log: []string{"Bundling libraries for Example.app", "Signing Example.app", "Creating DMG dist/Example.dmg", "Signing dist/Example.dmg", "Submitting dist/Example.dmg for notarization", "Notarization accepted"}}
			case "light":
				g.build = nil
				m.Dark.Set(false)
				g.switchTab(tabPKG)
				g.pkgRaw = false
				g.rebuild()
			case "close":
				g.confirmClose = true
			}
			m.sync()
			p.Frame()
			img := ggfx.NewImage(width, 760)
			img.Fill(ggui.Untrack(uitheme.Use).Bg)
			p.Draw(img)
			file, err := os.Create(filepath.Join(r.directory, fmt.Sprintf("%d-%s.png", width, name)))
			if err != nil {
				return err
			}
			err = png.Encode(file, img)
			closeErr := file.Close()
			img.Deallocate()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
		p.Close()
		painter.Close()
		if g.previewSurface != nil {
			g.previewSurface.Deallocate()
		}
	}
	return nil
}
