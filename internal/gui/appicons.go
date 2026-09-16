package gui

import (
	"bytes"
	"image/png"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ironpark/zapp/pkg/appbundle"
)

type appIconResult struct {
	path string
	png  []byte
}

// nativeAppIconKey names the cache slot holding the icon the OS extracted for a
// bundle, so the writer and the reader cannot spell it differently.
func nativeAppIconKey(path string) string { return assetCachePrefix + "native-app:" + path }

func (g *editor) loadAppIcon(key, path string) {
	if bundle, err := appbundle.Open(path); err == nil {
		if icon, err := bundle.IconFilePath(); err == nil {
			_ = g.loadAsset(key, icon)
		}
		// Modern apps may name an icon separately or use a PNG resource.
		if g.assets[key] == nil {
			for _, nameKey := range []string{"CFBundleIconFile", "CFBundleIconName"} {
				name, _ := bundle.GetString(nameKey)
				if name == "" {
					continue
				}
				for _, suffix := range []string{"", ".icns", ".png"} {
					if g.loadAsset(key, filepath.Join(path, "Contents", "Resources", name+suffix)) == nil {
						break
					}
				}
				if g.assets[key] != nil {
					break
				}
			}
		}
	}
	if g.assets[key] != nil {
		return
	}
	g.appIconPaths[key] = path
	cache := nativeAppIconKey(path)
	if img := g.assets[cache]; img != nil {
		g.assets[key] = img
		return
	}
	if g.ctx == nil || g.appIconPending[path] {
		return
	}
	if g.appIconResults == nil {
		g.appIconResults = make(chan appIconResult, 32)
		g.appIconPending = make(map[string]bool)
	}
	g.appIconPending[path] = true
	ctx := g.ctx
	go func() {
		data := nativeAppIcon(ctx, path)
		select {
		case g.appIconResults <- appIconResult{path, data}:
		case <-ctx.Done():
		}
	}()
}

func (g *editor) pollAppIcons() {
	for {
		select {
		case result := <-g.appIconResults:
			delete(g.appIconPending, result.path)
			if len(result.png) == 0 {
				continue
			}
			keys := []string{}
			for key, path := range g.appIconPaths {
				if path == result.path {
					keys = append(keys, key)
				}
			}
			if len(keys) == 0 {
				continue
			}
			img, err := png.Decode(bytes.NewReader(result.png))
			if err != nil {
				continue
			}
			texture := ebiten.NewImageFromImage(img)
			g.assets[nativeAppIconKey(result.path)] = texture
			for _, key := range keys {
				g.assets[key] = texture
			}
			g.pruneAssets()
		default:
			return
		}
	}
}
