package plist

import "testing"

// Entitlements keys contain dots, so an exact key must beat reading the same
// name as a path into nested dictionaries.
func TestResolvePrefersAnExactKeyOverAPath(t *testing.T) {
	root := map[string]any{
		"com.apple.security.app-sandbox": true,
		"a":                              map[string]any{"b": map[string]any{"c": "deep"}},
		"list":                           []any{"zero", "one"},
	}
	for _, c := range []struct {
		path  string
		want  any
		found bool
	}{
		{"com.apple.security.app-sandbox", true, true},
		{"a.b.c", "deep", true},
		{"list.1", "one", true},
		{"a.b.missing", nil, false},
		{"nope", nil, false},
	} {
		_, _, value, found := Resolve(root, c.path)
		if found != c.found || value != c.want {
			t.Fatalf("Resolve(%q) = %#v,%v want %#v,%v", c.path, value, found, c.want, c.found)
		}
	}
}

// Both halves of an edit must read the path the same way: the container Resolve
// reports for a missing leaf is where a new key belongs.
func TestResolveReportsWhereAMissingKeyBelongs(t *testing.T) {
	root := map[string]any{"a": map[string]any{"b": "value"}}

	container, name, _, found := Resolve(root, "a.new")
	if found || name != "new" {
		t.Fatalf("nested leaf: name=%q found=%v", name, found)
	}
	if err := Put(container, name, "written"); err != nil {
		t.Fatal(err)
	}
	if got := root["a"].(map[string]any)["new"]; got != "written" {
		t.Fatalf("nested create put the value at %#v", root)
	}

	// A dotted name that matches nothing stays one literal key, rather than
	// being invented as a nested structure.
	container, name, _, found = Resolve(root, "x.y")
	if found || name != "x.y" {
		t.Fatalf("unmatched path: name=%q found=%v", name, found)
	}
	if err := Put(container, name, "flat"); err != nil {
		t.Fatal(err)
	}
	if root["x.y"] != "flat" {
		t.Fatalf("literal dotted key became %#v", root)
	}
}

func TestPutAndDeleteReportBadContainers(t *testing.T) {
	if err := Put([]any{"only"}, "5", "x"); err == nil {
		t.Fatal("out-of-range index accepted")
	}
	if err := Put("scalar", "k", "x"); err == nil {
		t.Fatal("scalar container accepted")
	}
	// Removing from an array would renumber every key path after it.
	if err := Delete([]any{"a"}, "0"); err == nil {
		t.Fatal("array element removal accepted")
	}
}
