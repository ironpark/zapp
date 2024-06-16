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
		},
	}, "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
}
