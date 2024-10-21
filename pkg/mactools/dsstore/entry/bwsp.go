package entry

import (
	"bytes"
	"fmt"
	"howett.net/plist"
)

type WorkspaceSettingsEntry struct {
	ContainerShowSidebar bool
	ShowPathbar          bool
	ShowSidebar          bool
	ShowStatusBar        bool
	ShowTabView          bool
	ShowToolbar          bool
	SidebarWidth         int
	X                    int
	Y                    int
	Width                int
	Height               int
}

func (w *WorkspaceSettingsEntry) Bytes() []byte {
	buffer := &bytes.Buffer{}
	_ = plist.NewBinaryEncoder(buffer).Encode(map[string]any{
		"ContainerShowSidebar": true,
		"ShowPathbar":          false,
		"ShowSidebar":          true,
		"ShowStatusBar":        false,
		"ShowTabView":          false,
		"ShowToolbar":          false,
		"SidebarWidth":         0,
		"WindowBounds":         fmt.Sprintf("{{%d, %d}, {%d, %d}}", w.X, w.Y, w.Width, w.Height),
	})
	return plistWrap(buffer.Bytes())
}

func (w *WorkspaceSettingsEntry) Filename() string {
	return "."
}

func (w *WorkspaceSettingsEntry) EntryType() string {
	return TypeWorkspaceSettings
}

func (w *WorkspaceSettingsEntry) DataType() string {
	return "blob"
}

// NewWorkspaceSettingsEntry creates a new workspace settings entry.
func NewWorkspaceSettingsEntry(x, y, width, height int) *WorkspaceSettingsEntry {
	return &WorkspaceSettingsEntry{
		X:                    x,
		Y:                    y,
		Width:                width,
		Height:               height,
		ContainerShowSidebar: true,
		ShowPathbar:          false,
		ShowSidebar:          true,
		ShowStatusBar:        false,
		ShowTabView:          false,
		ShowToolbar:          false,
		SidebarWidth:         0,
	}
}
