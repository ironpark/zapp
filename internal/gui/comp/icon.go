package comp

import (
	"embed"
	"io/fs"
)

type Icon string

const (
	IconUndo Icon = "undo"
	IconRedo Icon = "redo"
)

//go:embed icons/*.svg
var iconFiles embed.FS

// IconFiles holds the icons' SVG sources; File names an icon's source within it.
func IconFiles() fs.FS { return iconFiles }

func (i Icon) File() string { return "icons/" + string(i) + ".svg" }
