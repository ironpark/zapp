package hfsplus

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ironpark/zapp/internal/hdiutil"
)

// writeImage builds v into a file under a temporary directory and returns its path.
func writeImage(t *testing.T, v Volume) string {
	t.Helper()
	// hdiutil picks its reader from the extension, so the verification helpers
	// below only recognise the image under a name it expects.
	path := filepath.Join(t.TempDir(), "image.dmg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	n, err := Write(context.Background(), f, v)
	if err != nil {
		t.Fatal(err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != n {
		t.Fatalf("reported %d bytes but wrote %d", n, info.Size())
	}
	return path
}

// attach exposes an image as a device without mounting it, so that fsck_hfs can
// read it. Verification is only possible on macOS.
func attach(t *testing.T, image string, args ...string) string {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("verifying an image needs macOS")
	}
	// The claim covers the whole time the image stays attached, not just the
	// attach itself, so no other test binary has an image mounted meanwhile.
	release := hdiutil.Lock()
	out, err := hdiutil.Output(append([]string{"attach", image, "-readonly", "-nobrowse"}, args...)...)
	if err != nil {
		release()
		t.Fatalf("attach: %v", err)
	}
	device := strings.Fields(string(out))[0]
	t.Cleanup(func() {
		_, _ = hdiutil.Run("detach", device, "-force")
		release()
	})
	return device
}

// fsck runs the system checker over an image and fails if it reports anything
// other than a clean volume.
func fsck(t *testing.T, image string) {
	t.Helper()
	device := attach(t, image, "-nomount")
	raw := "/dev/r" + strings.TrimPrefix(device, "/dev/")
	out, _ := exec.Command("fsck_hfs", "-n", raw).CombinedOutput()
	if !strings.Contains(string(out), "appears to be OK") {
		t.Fatalf("fsck_hfs rejected the image:\n%s", out)
	}
}

// mount attaches an image and returns the directory it is readable at.
func mount(t *testing.T, image string) string {
	t.Helper()
	point := filepath.Join(t.TempDir(), "mnt")
	if err := os.MkdirAll(point, 0755); err != nil {
		t.Fatal(err)
	}
	attach(t, image, "-mountpoint", point)
	return point
}

func TestWriteAndMount(t *testing.T) {
	var finder [32]byte
	copy(finder[0:], "icnsMACS")
	finder[8] = 0x04 // kHasCustomIcon, the high byte of fdFlags.

	image := writeImage(t, Volume{
		Name:    "Zapp Test",
		Created: time.Unix(1700000000, 0),
		Root: &Node{Mode: fs.ModeDir, Children: []*Node{
			{Name: "hello.txt", Data: Bytes([]byte("hello world\n"))},
			{Name: "한글 이름.txt", Data: Bytes([]byte("korean\n"))},
			{Name: "Applications", Mode: fs.ModeSymlink, LinkTarget: "/Applications"},
			{Name: "custom", Data: Bytes([]byte("data\n")), ResourceFork: Bytes([]byte("resource")), FinderInfo: finder},
			{Name: "sub", Mode: fs.ModeDir, Children: []*Node{
				{Name: "inner.bin", Data: Bytes(make([]byte, 100_000))},
			}},
		}},
	})
	fsck(t, image)

	point := mount(t, image)
	for name, want := range map[string]string{
		"hello.txt":               "hello world\n",
		"한글 이름.txt":               "korean\n",
		"custom":                  "data\n",
		"sub/inner.bin":           strings.Repeat("\x00", 100_000),
		"custom/..namedfork/rsrc": "resource",
	} {
		got, err := os.ReadFile(filepath.Join(point, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("%s: contents differ", name)
		}
	}

	target, err := os.Readlink(filepath.Join(point, "Applications"))
	if err != nil || target != "/Applications" {
		t.Fatalf("symlink: %q %v", target, err)
	}
	// A custom icon is only honoured when the type, creator and flag bytes all
	// survive the round trip, so check the whole record rather than its prefix.
	out, err := exec.Command("xattr", "-px", "com.apple.FinderInfo", filepath.Join(point, "custom")).Output()
	if err != nil {
		t.Fatal(err)
	}
	got, err := hex.DecodeString(strings.Join(strings.Fields(string(out)), ""))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, finder[:]) {
		t.Fatalf("finder info came back as %x, want %x", got, finder)
	}
}

func TestLargeCatalogSpansIndexNodes(t *testing.T) {
	var children []*Node
	// Comfortably more entries than one catalog leaf node holds, so the tree
	// has to grow an index level above the leaves.
	for i := range 400 {
		children = append(children, &Node{
			Name: fmt.Sprintf("file-%03d.txt", i),
			Data: Bytes([]byte(fmt.Sprintf("contents %d\n", i))),
		})
	}
	image := writeImage(t, Volume{Name: "Many", Root: &Node{Mode: fs.ModeDir, Children: children}})
	fsck(t, image)

	point := mount(t, image)
	entries, err := os.ReadDir(point)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(children) {
		t.Fatalf("read back %d entries, want %d", len(entries), len(children))
	}
	// Spot check both ends, which sit in different leaf nodes.
	for _, i := range []int{0, 399} {
		name := fmt.Sprintf("file-%03d.txt", i)
		got, err := os.ReadFile(filepath.Join(point, name))
		if err != nil {
			t.Fatal(err)
		}
		if want := fmt.Sprintf("contents %d\n", i); string(got) != want {
			t.Fatalf("%s: got %q want %q", name, got, want)
		}
	}
}

func TestCompareNames(t *testing.T) {
	name := func(s string) []uint16 {
		t.Helper()
		units, err := encodeName(s)
		if err != nil {
			t.Fatal(err)
		}
		return units
	}
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"a", "b", -1},
		{"b", "a", 1},
		{"A", "a", 0}, // The catalog folds case.
		{"AbC", "aBc", 0},
		{"a", "ab", -1}, // A prefix sorts before the longer name.
		{"한글", "한글", 0},
		{"가", "나", -1},
		{"Apple", "apple ", -1},
	} {
		if got := compareNames(name(c.a), name(c.b)); got != c.want {
			t.Errorf("compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestAssignIDsIsStableAndIndependentOfContents(t *testing.T) {
	build := func(contents string) Volume {
		return Volume{Name: "V", Root: &Node{Mode: fs.ModeDir, Children: []*Node{
			{Name: "b", Mode: fs.ModeDir, Children: []*Node{{Name: "inner", Data: Bytes([]byte(contents))}}},
			{Name: "a", Data: Bytes([]byte(contents))},
		}}}
	}
	ids := func(v Volume) map[string]uint64 {
		if err := AssignIDs(&v); err != nil {
			t.Fatal(err)
		}
		out := map[string]uint64{}
		var walk func(string, *Node)
		walk = func(prefix string, n *Node) {
			for _, c := range n.Children {
				out[prefix+c.Name] = c.ID
				walk(prefix+c.Name+"/", c)
			}
		}
		walk("", v.Root)
		return out
	}
	first, second := ids(build("short")), ids(build(strings.Repeat("much longer", 100)))
	if len(first) != 3 {
		t.Fatalf("numbered %d entries, want 3", len(first))
	}
	for name, id := range first {
		if second[name] != id {
			t.Fatalf("%s was numbered %d and then %d; contents must not affect numbering", name, id, second[name])
		}
	}
	// Numbering follows catalog order, not the order the caller supplied.
	if first["a"] >= first["b"] {
		t.Fatalf("entries were not numbered in catalog order: %v", first)
	}
}

func TestRejectsBadInput(t *testing.T) {
	for name, v := range map[string]Volume{
		"no name":         {Root: &Node{Mode: fs.ModeDir}},
		"no root":         {Name: "V"},
		"file as root":    {Name: "V", Root: &Node{Data: Bytes(nil)}},
		"odd block size":  {Name: "V", Root: &Node{Mode: fs.ModeDir}, BlockSize: 6000},
		"tiny block size": {Name: "V", Root: &Node{Mode: fs.ModeDir}, BlockSize: 512},
		"empty file name": {Name: "V", Root: &Node{Mode: fs.ModeDir, Children: []*Node{{Name: ""}}}},
		"duplicate names": {Name: "V", Root: &Node{Mode: fs.ModeDir, Children: []*Node{
			{Name: "Same", Data: Bytes(nil)}, {Name: "same", Data: Bytes(nil)},
		}}},
		"link with no target": {Name: "V", Root: &Node{Mode: fs.ModeDir, Children: []*Node{
			{Name: "L", Mode: fs.ModeSymlink},
		}}},
		"negative free space": {Name: "V", Root: &Node{Mode: fs.ModeDir}, FreeSpace: -1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Write(context.Background(), new(bytes.Buffer), v); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestWriteHonoursContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v := Volume{Name: "V", Root: &Node{Mode: fs.ModeDir, Children: []*Node{{Name: "a", Data: Bytes([]byte("x"))}}}}
	if _, err := Write(ctx, new(bytes.Buffer), v); err != context.Canceled {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}
