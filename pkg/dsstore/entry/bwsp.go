package entry

import (
	"fmt"

	"github.com/ironpark/zapp/pkg/plist"
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
	data, err := plist.MarshalBinary(map[string]any{
		"ContainerShowSidebar": true,
		"ShowPathbar":          false,
		"ShowSidebar":          true,
		"ShowStatusBar":        false,
		"ShowTabView":          false,
		"ShowToolbar":          false,
		"SidebarWidth":         0,
		"WindowBounds":         fmt.Sprintf("{{%d, %d}, {%d, %d}}", w.X, w.Y, w.Width, w.Height),
	})
	if err != nil {
		return nil
	}
	return plistWrap(data)
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
