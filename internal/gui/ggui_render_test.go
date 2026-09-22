package gui

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/ggui"
	uitheme "github.com/ironpark/ggui/ui/theme"
	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/internal/gui/comp"
)

// Opt-in GPU snapshots follow ggui/examples/sqlite's render harness. The normal
// test suite stays headless; snapshots run on the engine's main thread.
func TestMain(m *testing.M) {
	if dir := os.Getenv("ZAPP_GUI_RENDER_DIR"); dir != "" {
		ebiten.SetWindowSize(320, 240)
		ebiten.SetWindowTitle("Zapp layout previews")
		game := &desktopRenderer{directory: dir}
		err := ebiten.RunGame(game)
		if err == nil {
			err = game.err
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type desktopRenderer struct {
	directory string
	done      bool
	err       error
}

func (r *desktopRenderer) Layout(int, int) (int, int) { return 320, 240 }
func (r *desktopRenderer) Update() error {
	if r.done {
		return ebiten.Termination
	}
	return nil
}
func (r *desktopRenderer) Draw(*ebiten.Image) {
	if !r.done {
		r.err = r.render()
		r.done = true
	}
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
		painter, err := comp.NewPainter(nil, comp.DarkTheme())
		if err != nil {
			return err
		}
		g := &editor{ctx: context.Background(), s: s, w: width, h: 760, active: -1, ui: painter, assets: map[string]*ebiten.Image{}, status: "Edit settings, then Save. Validation checks build inputs without building."}
		g.rebuild()
		m := newDesktopModel(g)
		p := ggui.ProbeBuilder(func() ggui.Widget { return ggui.Provide(ggui.ReducedMotionKey, true, desktopView(m)) }, ggui.Sz(width, 760))
		p.Setup(func() { uitheme.Bind(m.Dark, editorTheme(true), editorTheme(false)) })
		for _, name := range []string{"project", "dmg", "pkg", "components", "help", "json", "dependencies", "signing", "notarization", "light", "close"} {
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
			case "notarization":
				g.switchTab(tabNotarize)
			case "light":
				m.Dark.Set(false)
				g.switchTab(tabPKG)
				g.pkgRaw = false
				g.rebuild()
			case "close":
				g.confirmClose = true
			}
			m.sync()
			p.Frame()
			img := ebiten.NewImage(width, 760)
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
