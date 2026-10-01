package gui

import (
	"bytes"
	"image/png"
	"path/filepath"

	"github.com/ironpark/ggfx"
	"github.com/ironpark/zapp/pkg/appbundle"
)

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
	if g.appIconPaths == nil {
		g.appIconPaths = map[string]string{}
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
	if g.appIconPending == nil {
		g.appIconPending = make(map[string]bool)
	}
	g.appIconPending[path] = true
	ctx, post := g.ctx, g.poster()
	go func() {
		data := nativeAppIcon(ctx, path)
		post(func() { g.applyAppIcon(path, data) })
	}()
}

// applyAppIcon stores the icon the OS extracted for the bundle at path under
// every key still showing that bundle.
func (g *editor) applyAppIcon(path string, data []byte) {
	delete(g.appIconPending, path)
	if len(data) == 0 {
		return
	}
	keys := []string{}
	for key, p := range g.appIconPaths {
		if p == path {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return
	}
	texture := ggfx.NewImageFromImage(img)
	g.assets[nativeAppIconKey(path)] = texture
	for _, key := range keys {
		g.assets[key] = texture
	}
	g.pruneAssets()
	g.assetsChanged()
}
