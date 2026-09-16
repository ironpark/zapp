// Package gui implements the interactive project settings and DMG layout editor.
package gui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ironpark/zapp"
	"github.com/ironpark/zapp/pkg/dmg"
)

type Session struct {
	Path       string
	filePath   string
	Project    *zapp.Project
	original   []byte
	exists     bool
	saved      []byte
	undo, redo []*zapp.Project
}

// Open discovers an existing project or starts an unsaved project. Merely
// opening the editor never creates or rewrites a file.
func Open(name string) (*Session, error) {
	if name == "" {
		var err error
		name, err = zapp.Discover(".")
		if errors.Is(err, zapp.ErrNotFound) {
			name = ".zapp.yaml"
		} else if err != nil {
			return nil, err
		}
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	// Preserve the config path's directory as the base for relative paths,
	// matching zapp.Load, while saving through (rather than replacing) links.
	target := abs
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		target = resolved
	}
	s := &Session{Path: abs, filePath: target}
	s.original, err = os.ReadFile(abs)
	if err == nil {
		s.exists = true
		s.Project, err = zapp.Load(abs)
		if err != nil {
			return nil, err
		}
		if s.Project.Legacy() {
			return nil, fmt.Errorf("GUI requires a version: 1 project with a dmg section; migrate the legacy flat configuration first")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		s.Project, err = zapp.Parse(strings.NewReader("version: 1\nout: dist\ndmg: {}\n"), filepath.Dir(abs))
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	s.saved, err = s.encode()
	return s, err
}

func (s *Session) encode() ([]byte, error) {
	// An explicit null section distinguishes an all-disabled project from the
	// supported legacy flat DMG format when it is subsequently loaded.
	if strings.EqualFold(filepath.Ext(s.Path), ".json") {
		data, err := json.Marshal(s.Project)
		if err != nil {
			return nil, err
		}
		var object map[string]any
		if err := json.Unmarshal(data, &object); err != nil {
			return nil, err
		}
		if noSections(s.Project) {
			object["dmg"] = nil
		}
		data, err = json.MarshalIndent(object, "", "  ")
		return append(data, '\n'), err
	}
	data, err := s.Project.YAML()
	if noSections(s.Project) {
		data = append(data, []byte("dmg: null\n")...)
	}
	return data, err
}

// undoLimit caps the retained history so long sessions stay bounded.
const undoLimit = 100

func (s *Session) Dirty() bool { b, err := s.encode(); return err != nil || !bytes.Equal(b, s.saved) }

// push records a snapshot as the new undo top and drops any redo branch.
func (s *Session) push(before *zapp.Project) {
	s.undo = append(s.undo, before)
	if len(s.undo) > undoLimit {
		s.undo = s.undo[1:]
	}
	s.redo = nil
}
func (s *Session) checkpoint()   { s.push(s.Project.Clone()) }
func (s *Session) CanUndo() bool { return len(s.undo) > 0 }
func (s *Session) CanRedo() bool { return len(s.redo) > 0 }

// pop makes the top of from current, pushing the outgoing project onto to.
func (s *Session) pop(from, to *[]*zapp.Project) {
	if len(*from) == 0 {
		return
	}
	*to = append(*to, s.Project.Clone())
	s.Project = (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
}
func (s *Session) Undo() { s.pop(&s.undo, &s.redo) }
func (s *Session) Redo() { s.pop(&s.redo, &s.undo) }

func (s *Session) Save() error {
	if err := s.validateLayout(); err != nil {
		return err
	}
	data, err := s.encode()
	if err != nil {
		return err
	}
	if _, err = zapp.Parse(bytes.NewReader(data), filepath.Dir(s.Path)); err != nil {
		return err
	}
	current, err := os.ReadFile(s.filePath)
	if s.exists {
		if err != nil {
			return err
		}
		if !bytes.Equal(current, s.original) {
			return fmt.Errorf("configuration changed on disk; reopen the GUI to avoid overwriting external edits")
		}
	} else if err == nil || !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("configuration appeared on disk or cannot be read; reopen the GUI")
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(s.filePath); err == nil {
		mode = info.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(s.filePath), ".zapp-gui-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if s.exists {
		err = os.Rename(f.Name(), s.filePath)
	} else {
		// Link is exclusive: don't overwrite a file created since our read.
		err = os.Link(f.Name(), s.filePath)
	}
	if err != nil {
		return err
	}
	s.original = data
	s.saved = append([]byte(nil), data...)
	s.exists = true
	return nil
}

// dmgLayout is the resolved preview geometry: window size, icon and label
// metrics, and the ordered items placed inside the window.
type dmgLayout struct {
	W, H, IconSize, LabelSize int
	Items                     []layoutItem
}

// find returns the item with the given path, if it is present in the layout.
func (l dmgLayout) find(path string) (layoutItem, bool) {
	for _, i := range l.Items {
		if i.Path == path {
			return i, true
		}
	}
	return layoutItem{}, false
}

func layout(c *zapp.DMGConfig, app string) dmgLayout {
	// Geometry and automatic placement come from the same helpers Resolve uses,
	// so the preview cannot drift from what a build actually produces.
	w, h, size, label := c.Metrics()
	var items []layoutItem
	if c.Contents == nil {
		if app != "" {
			appX, linkX, y := c.DefaultPositions()
			items = []layoutItem{{app, "", false, appX, y}, {"/Applications", "", true, linkX, y}}
		}
	} else {
		for _, key := range slices.Sorted(maps.Keys(c.Contents)) {
			v := c.Contents[key]
			items = append(items, layoutItem{key, v.Name, v.Link, coord(v.X), coord(v.Y)})
		}
	}
	return dmgLayout{w, h, size, label, items}
}

type layoutItem struct {
	Path, Name string
	Link       bool
	X, Y       int
}

func (i layoutItem) title() string {
	return dmg.Item{Name: i.Name, Path: i.Path}.ImageName()
}
func (s *Session) materialize() {
	c := s.Project.DMG
	if c.Contents != nil {
		return
	}
	items := layout(c, s.Project.App).Items
	c.Contents = map[string]zapp.Content{}
	for _, item := range items {
		x, y := item.X, item.Y
		c.Contents[item.Path] = zapp.Content{X: &x, Y: &y, Name: item.Name, Link: item.Link}
	}
}

func (s *Session) move(key string, x, y int) {
	s.materialize()
	c := s.Project.DMG
	l := layout(c, s.Project.App)
	item, ok := c.Contents[key]
	if !ok {
		return
	}
	x = max(0, min(l.W, x))
	y = max(0, min(l.H, y))
	item.X = &x
	item.Y = &y
	c.Contents[key] = item
}

func (s *Session) validateLayout() error {
	c := s.Project.DMG
	if c == nil {
		return nil
	}
	l := layout(c, s.Project.App)
	if l.W < 1 || l.H < 1 {
		return fmt.Errorf("window dimensions must be positive")
	}
	if _, err := dmg.ParseFileSystem(c.FS); err != nil {
		return err
	}
	switch strings.ToLower(c.Format) {
	case "", "udzo", "ulfo", "zlib", "lzfse":
	default:
		return fmt.Errorf("format must be udzo or ulfo")
	}
	if l.IconSize < dmg.MinIconSize || l.IconSize > dmg.MaxIconSize {
		return fmt.Errorf("icon size must be %d–%d", dmg.MinIconSize, dmg.MaxIconSize)
	}
	if l.LabelSize < dmg.MinLabelSize || l.LabelSize > dmg.MaxLabelSize {
		return fmt.Errorf("label size must be %d–%d", dmg.MinLabelSize, dmg.MaxLabelSize)
	}
	if c.Contents != nil {
		for key, v := range c.Contents {
			if v.X == nil || v.Y == nil {
				return fmt.Errorf("contents[%q] requires x and y", key)
			}
		}
		// Validate names/coordinates without requiring source files to already
		// exist. Full build-input validation is a separate explicit UI action.
		d := dmg.Config{Title: "Preview", WindowWidth: l.W, WindowHeight: l.H, ContentsIconSize: l.IconSize, LabelSize: l.LabelSize}
		for _, i := range l.Items {
			d.Contents = append(d.Contents, dmg.Item{Path: i.Path, Name: i.Name, X: i.X, Y: i.Y, Type: dmg.Link})
		}
		return d.Validate()
	}
	return nil
}
