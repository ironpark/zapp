package zapp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

func TestPositionDecoding(t *testing.T) {
	for _, decode := range []struct {
		name string
		fn   func([]byte, any) error
	}{
		{"JSON", json.Unmarshal}, {"YAML", yaml.Unmarshal},
	} {
		t.Run(decode.name, func(t *testing.T) {
			for _, input := range []string{"[]", "[1]", "[1,2,3]", "[null,2]", "[1,1.5]", "[1,\"2\"]", "{}"} {
				var pos Position
				if err := decode.fn([]byte(input), &pos); err == nil {
					t.Errorf("accepted invalid position %s", input)
				}
			}
			var pos Position
			if err := decode.fn([]byte("[0,420]"), &pos); err != nil || pos != (Position{0, 420}) {
				t.Fatalf("position = %v, error = %v", pos, err)
			}
		})
	}
}

func TestPositionSerialization(t *testing.T) {
	p := &Project{Version: 1, DMG: &DMGConfig{Contents: map[string]Content{"/Applications": {Pos: &Position{0, 420}, Link: true}}}}
	data, err := p.YAML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "pos: [0, 420]") {
		t.Fatalf("expected inline position:\n%s", data)
	}
	roundtrip, err := Parse(strings.NewReader(string(data)), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if *roundtrip.DMG.Contents["/Applications"].Pos != (Position{0, 420}) {
		t.Fatal("position changed")
	}
}
