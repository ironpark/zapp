package gui

import (
	"fmt"
	"image"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp"
)

// droppedPaths descends only through virtual grouping directories. An actual
// dropped directory (including an app bundle) is one item, never its children.
func droppedPaths(files fs.FS) ([]string, error) {
	var paths []string
	var visit func(string) error
	visit = func(name string) error {
		entries, err := fs.ReadDir(files, name)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			child := path.Join(name, entry.Name())
			file, err := files.Open(child)
			if err != nil {
				return err
			}
			source, real := file.(ebiten.AbsPather)
			var absolute string
			if real {
				absolute = source.AbsPath()
			}
			file.Close()
			if real && filepath.IsAbs(absolute) {
				paths = append(paths, absolute)
				continue
			}
			if !entry.IsDir() {
				return fmt.Errorf("cannot determine the original path of %s", child)
			}
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	err := visit(".")
	return paths, err
}

func (g *editor) dropFiles(files fs.FS, point image.Point) {
	if g.tab != tabDMG || !g.enabled() || !point.In(g.previewArea()) {
		return
	}
	if !g.commit() {
		return
	}
	paths, err := droppedPaths(files)
	if err != nil {
		g.report(err, "")
		return
	}
	count, err := g.addDroppedPaths(paths, point)
	if err != nil {
		g.report(err, "")
		return
	}
	if count == 0 {
		g.report(nil, "These items are already in the layout.")
		return
	}
	g.report(nil, fmt.Sprintf("Added %d item(s). Drag to arrange; Undo removes this batch.", count))
}

func (g *editor) addDroppedPaths(paths []string, point image.Point) (int, error) {
	if g.tab != tabDMG || !g.enabled() || !point.In(g.previewArea()) {
		return 0, nil
	}
	l := g.s.layout()
	seen := map[string]bool{}
	for _, item := range l.Items {
		absolute, err := filepath.Abs(g.assetPath(item.Path))
		if err == nil {
			seen[absolute] = true
		}
	}
	var keys []string
	for _, source := range paths {
		absolute, err := filepath.Abs(source)
		if err != nil {
			return 0, err
		}
		if seen[absolute] {
			continue
		}
		if _, err := os.Stat(absolute); err != nil {
			return 0, err
		}
		key, err := filepath.Rel(filepath.Dir(g.s.Path), absolute)
		if err != nil {
			key = absolute
		}
		keys = append(keys, key)
		seen[absolute] = true
	}
	if len(keys) == 0 {
		return 0, nil
	}
	before := g.s.Project.Clone()
	g.s.materialize()
	x, y := g.transform().content(float64(point.X), float64(point.Y))
	nx := max(0, min(l.W-1, int(math.Round(x))))
	ny := max(0, min(l.H-1, int(math.Round(y))))
	spacing := l.IconSize + 32
	for _, key := range keys {
		// Keep the first item at the drop point and lay out additional items
		// with enough space to distinguish them before the user arranges them.
		itemX, itemY := nx, ny
		g.s.Project.DMG.Contents[key] = zapp.Content{Pos: &zapp.Position{itemX, itemY}}
		nx += spacing
		if nx >= l.W {
			nx = min(l.W-1, l.IconSize/2)
			ny += spacing
			if ny >= l.H {
				ny = min(l.H-1, l.IconSize/2)
			}
		}
	}
	if err := g.s.validateLayout(); err != nil {
		g.s.Project = before
		g.rebuild()
		return 0, err
	}
	g.s.push(before)
	g.selected = keys[len(keys)-1]
	g.rebuild()
	return len(keys), nil
}
