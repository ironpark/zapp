package dmg

import (
	"os"
	"testing"
)

func TestCreateDMG(t *testing.T) {
	os.Remove("asd.dmg")
	os.RemoveAll("/tmp/test")
	err := CreateDMG(Config{
		Title:      "asd",
		Icon:       "",
		Background: "",
		Contents: []Item{
			{X: 300, Y: 100, Type: Link, Path: "/Applications"},
			{X: 600, Y: 100, Type: Dir, Path: "/Users/ironpark/Documents/Project/Personal/zapp/assets/test/SyncMaster 240704.app"},
		},
	}, "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
}
