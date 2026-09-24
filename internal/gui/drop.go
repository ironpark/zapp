package gui

import (
	"image"
	"math"
	"os"
	"path/filepath"

	"github.com/ironpark/zapp"
)

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
		g.s.Project.DMG.Contents[key] = zapp.Content{Pos: &zapp.Position{nx, ny}}
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
