package gui

import (
	"bytes"
	"embed"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed assets/fileicons/*.png
var fileIconFiles embed.FS

// itemKindNames maps the icon kinds above to the labels the inspector shows.
var itemKindNames = map[string]string{
	"app": "App", "folder": "Folder", "applications": "Folder", "file": "File",
	"text": "Text", "pdf": "PDF", "image": "Image", "audio": "Audio", "video": "Video",
	"archive": "Archive", "disk": "Disk image", "package": "Package", "font": "Font",
	"script": "Script", "source": "Source", "executable": "Executable",
}

func fileIconType(path string, directory bool) string {
	if filepath.Clean(path) == "/Applications" {
		return "applications"
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".app" {
		return "app"
	}
	if directory {
		return "folder"
	}
	switch ext {
	case ".txt", ".md", ".rtf", ".log", ".csv":
		return "text"
	case ".pdf":
		return "pdf"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".heic", ".tiff", ".tif", ".bmp", ".svg", ".icns", ".ico":
		return "image"
	case ".mp3", ".m4a", ".aac", ".wav", ".aiff", ".flac", ".ogg":
		return "audio"
	case ".mov", ".mp4", ".m4v", ".avi", ".mkv", ".webm":
		return "video"
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar", ".tgz":
		return "archive"
	case ".dmg", ".iso", ".img", ".sparseimage", ".sparsebundle":
		return "disk"
	case ".pkg", ".mpkg":
		return "package"
	case ".ttf", ".otf", ".ttc", ".woff", ".woff2":
		return "font"
	case ".sh", ".bash", ".zsh", ".command", ".applescript", ".scpt":
		return "script"
	case ".go", ".swift", ".c", ".h", ".cpp", ".rs", ".py", ".js", ".ts", ".tsx", ".jsx", ".json", ".yaml", ".yml", ".xml", ".html", ".css":
		return "source"
	case ".exe", ".bin", ".dylib", ".so":
		return "executable"
	}
	return "file"
}

func fileIconForPath(path string) string {
	info, err := os.Stat(path)
	directory := err == nil && info.IsDir()
	kind := fileIconType(path, directory)
	if kind == "file" && err == nil && info.Mode()&0111 != 0 {
		return "executable"
	}
	return kind
}

func (g *editor) defaultFileIcon(name string) *ebiten.Image {
	key := assetCachePrefix + "embedded:" + name
	if img := g.assets[key]; img != nil {
		return img
	}
	data, err := fileIconFiles.ReadFile("assets/fileicons/" + name + ".png")
	if err != nil {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	texture := ebiten.NewImageFromImage(img)
	g.assets[key] = texture
	return texture
}

func drawFileIcon(dst *ebiten.Image, icon *ebiten.Image, bounds image.Rectangle) {
	if icon == nil || bounds.Empty() {
		return
	}
	ratio := min(float64(bounds.Dx())/float64(icon.Bounds().Dx()), float64(bounds.Dy())/float64(icon.Bounds().Dy()))
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(ratio, ratio)
	op.GeoM.Translate(float64(bounds.Min.X)+(float64(bounds.Dx())-float64(icon.Bounds().Dx())*ratio)/2, float64(bounds.Min.Y)+(float64(bounds.Dy())-float64(icon.Bounds().Dy())*ratio)/2)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(icon, op)
}
