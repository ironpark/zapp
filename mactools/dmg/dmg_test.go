package dmg

import (
	"os"
	"testing"
)

func TestCreateDMG(t *testing.T) {
	os.Remove("SyncMaster.dmg")
	os.RemoveAll("/tmp/test")

	err := CreateDMG(Config{
		Title:            "SyncMaster",
		Icon:             "/Users/ironpark/Documents/Project/Personal/zapp/assets/test/SyncMaster 240704.app/Contents/Resources/iconfile.icns",
		LabelSize:        15,
		ContentsIconSize: 200,
		WindowWidth:      640,
		WindowHeight:     480,
		Background:       "/Users/ironpark/Documents/Project/Personal/zapp/mactools/dmg/bg.png",
		Contents: []Item{
			{X: int(float64(640) / 5 * 3), Y: 480 / 2, Type: Link, Path: "/Applications"},
			{X: int(float64(640) / 5 * 1), Y: 480 / 2, Type: Dir, Path: "/Users/ironpark/Documents/Project/Personal/zapp/assets/test/SyncMaster 240704.app"},
		},
	}, "/tmp/test")

	if err != nil {
		t.Fatal(err)
	}
}
