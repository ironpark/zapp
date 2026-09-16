package apfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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

func testVolume() Volume {
	return Volume{Name: "APFS Test", Created: time.Unix(1700000000, 0), Root: &Node{Mode: fs.ModeDir | 0755}}
}

func writeTestImage(t *testing.T, v Volume) (string, *Image) {
	t.Helper()
	i, err := Plan(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "image.dmg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	n, err := i.WriteTo(context.Background(), f)
	closeErr := f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if n != i.Size() {
		t.Fatalf("size %d != %d", n, i.Size())
	}
	return path, i
}

func checkImage(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		return
	}
	out, err := exec.Command("/sbin/fsck_apfs", "-n", path).CombinedOutput()
	if err != nil || bytes.Contains(out, []byte("error:")) || bytes.Contains(out, []byte("Fix ")) || bytes.Contains(out, []byte("warning: apfs_")) {
		t.Fatalf("fsck_apfs: %v\n%s", err, out)
	}
}

func mountImage(t *testing.T, path string) string {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("mounting needs macOS")
	}
	point := filepath.Join(t.TempDir(), "mnt")
	if err := os.Mkdir(point, 0755); err != nil {
		t.Fatal(err)
	}
	// The claim covers the whole time the image stays attached, not just the
	// attach itself, so no other test binary has an image mounted meanwhile.
	release := hdiutil.Lock()
	out, err := hdiutil.Run("attach", path, "-readonly", "-nobrowse", "-mountpoint", point)
	if err != nil {
		release()
		t.Fatalf("attach: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		defer release()
		if out, err := hdiutil.Run("detach", point, "-force"); err != nil {
			t.Errorf("detach: %v\n%s", err, out)
		}
	})
	return point
}

func TestMountedFiles(t *testing.T) {
	for _, sensitive := range []bool{false, true} {
		t.Run(fmt.Sprintf("case-sensitive=%v", sensitive), func(t *testing.T) {
			v := testVolume()
			v.CaseSensitive = sensitive
			v.Root.Children = []*Node{
				{Name: "hello.txt", Mode: 0751, Data: Bytes([]byte("hello\n")), ModTime: v.Created},
				{Name: "empty", Mode: 0600, Data: Bytes(nil)},
				{Name: "empty-dir", Mode: fs.ModeDir | 0700},
				{Name: "Applications", Mode: fs.ModeSymlink, LinkTarget: "/Applications"},
				{Name: "한글-é-😀", Mode: 0644, Data: Bytes([]byte("unicode"))},
				{Name: "Straße", Mode: 0644, Data: Bytes([]byte("folded"))},
				{Name: "\u1fb7", Mode: 0644, Data: Bytes([]byte("greek"))},
				{Name: "resource", Mode: 0644, Data: Bytes([]byte("data")), ResourceFork: Bytes(bytes.Repeat([]byte("resource"), 1000))},
			}
			if sensitive {
				v.Root.Children = append(v.Root.Children, &Node{Name: "HELLO.txt", Mode: 0644, Data: Bytes([]byte("distinct"))})
			}
			path, _ := writeTestImage(t, v)
			checkImage(t, path)
			point := mountImage(t, path)
			for name, want := range map[string]string{"hello.txt": "hello\n", "empty": "", "한글-é-😀": "unicode", "Straße": "folded"} {
				got, err := os.ReadFile(filepath.Join(point, name))
				if err != nil || string(got) != want {
					t.Fatalf("%s: %q, %v", name, got, err)
				}
			}
			for _, name := range []string{"한글-é-😀"} {
				if _, err := os.Stat(filepath.Join(point, name)); err != nil {
					t.Fatal(err)
				}
			}
			lookup := "HELLO.txt"
			want := "hello\n"
			if sensitive {
				want = "distinct"
			}
			if got, err := os.ReadFile(filepath.Join(point, lookup)); err != nil || string(got) != want {
				t.Fatalf("case lookup: %q %v", got, err)
			}
			if !sensitive {
				if got, err := os.ReadFile(filepath.Join(point, "\u03b1\u0345\u0342")); err != nil || string(got) != "greek" {
					t.Fatalf("combining case fold: %q %v", got, err)
				}
				if got, err := os.ReadFile(filepath.Join(point, "STRASSE")); err != nil || string(got) != "folded" {
					t.Fatalf("full case fold: %q %v", got, err)
				}
			}
			if info, err := os.Stat(filepath.Join(point, "hello.txt")); err != nil {
				t.Fatal(err)
			} else if info.Mode().Perm() != 0751 || !info.ModTime().Equal(v.Created) {
				t.Fatalf("metadata: %v %v", info.Mode(), info.ModTime())
			}
			if info, err := os.Stat(filepath.Join(point, "empty-dir")); err != nil || info.Mode().Perm() != 0700 {
				t.Fatalf("directory permissions: %v %v", info, err)
			}
			if got, err := os.Readlink(filepath.Join(point, "Applications")); err != nil || got != "/Applications" {
				t.Fatalf("link: %q %v", got, err)
			}
			if got, err := os.ReadFile(filepath.Join(point, "resource", "..namedfork", "rsrc")); err != nil || !bytes.Equal(got, bytes.Repeat([]byte("resource"), 1000)) {
				t.Fatalf("resource fork: len %d %v", len(got), err)
			}
		})
	}
}

func TestLargeDirectory(t *testing.T) {
	v := testVolume()
	for i := range 12000 {
		v.Root.Children = append(v.Root.Children, &Node{Name: fmt.Sprintf("file-%05d", i), Mode: 0644, Data: Bytes([]byte(fmt.Sprintf("payload-%d", i)))})
	}
	path, image := writeTestImage(t, v)
	if image.layout.fsTree.root.level < 2 {
		t.Fatal("fixture must exercise a multi-level tree")
	}
	checkImage(t, path)
	point := mountImage(t, path)
	for _, i := range []int{0, 17, 350, 6000, 11999} {
		got, err := os.ReadFile(filepath.Join(point, fmt.Sprintf("file-%05d", i)))
		if err != nil || string(got) != fmt.Sprintf("payload-%d", i) {
			t.Fatalf("entry %d: %q %v", i, got, err)
		}
	}
	entries, err := os.ReadDir(point)
	if err != nil || len(entries) != 12000 {
		t.Fatalf("directory count %d: %v", len(entries), err)
	}
}

// A generated source checks multi-chunk allocation without retaining a large
// byte slice in memory. Reads exercise both ends of the extent after mounting.
type zeroSource int64

func (s zeroSource) Size() int64 { return int64(s) }
func (s zeroSource) Open() (io.ReadCloser, error) {
	return io.NopCloser(io.LimitReader(zeroReader{}, int64(s))), nil
}

type zeroReader struct{}

func (zeroReader) Read(b []byte) (int, error) { clear(b); return len(b), nil }

func TestMultipleAllocationChunks(t *testing.T) {
	v := testVolume()
	v.Root.Children = []*Node{{Name: "large", Mode: 0644, Data: zeroSource(140 << 20)}}
	path, image := writeTestImage(t, v)
	if image.layout.chunks < 2 {
		t.Fatal("fixture must span allocation chunks")
	}
	checkImage(t, path)
	point := mountImage(t, path)
	f, err := os.Open(filepath.Join(point, "large"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var b [10]byte
	if n, err := f.ReadAt(b[:], (140<<20)-10); err != nil || n != 10 || b != [10]byte{} {
		t.Fatalf("tail: %x %d %v", b, n, err)
	}
}

func TestNameRules(t *testing.T) {
	for _, names := range [][]string{{"a", "A"}, {"é", "e\u0301"}, {"Straße", "STRASSE"}, {"한글", "한글"}, {"\u1fb7", "\u03b1\u0345\u0342"}} {
		v := testVolume()
		for _, name := range names {
			v.Root.Children = append(v.Root.Children, &Node{Name: name})
		}
		if err := AssignIDs(&v); err == nil {
			t.Errorf("accepted colliding names %q", names)
		}
	}
	v := testVolume()
	v.CaseSensitive = true
	v.Root.Children = []*Node{{Name: "A"}, {Name: "a"}}
	if err := AssignIDs(&v); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", ".", "..", "a/b", "a\x00b", "\xff", strings.Repeat("x", 256)} {
		v := testVolume()
		v.Root.Children = []*Node{{Name: name}}
		if err := AssignIDs(&v); err == nil {
			t.Errorf("accepted invalid name %q", name)
		}
	}
	if got := nameHash("root", true) << 10; got != 0xb671e400 {
		t.Fatalf("native APFS root hash = %x", got)
	}
	if got := nameHash("private-dir", true) << 10; got != 0xaca68c00 {
		t.Fatalf("native APFS private-dir hash = %x", got)
	}
}

func TestChecksumsAndReferences(t *testing.T) {
	v := testVolume()
	v.Root.Children = []*Node{{Name: "data", Mode: 0644, Data: Bytes([]byte("x"))}}
	i, err := Plan(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	l := i.layout
	for _, tree := range []*tree{l.fsTree, l.extentTree, l.snapTree, l.volumeMapTree, l.containerMapTree} {
		for _, n := range tree.nodes {
			b := l.blocks[n.addr]
			if le.Uint64(b) != checksum(b) {
				t.Fatalf("checksum for block %d", n.addr)
			}
			copyOf := bytes.Clone(b)
			copyOf[64] ^= 1
			if checksum(copyOf) == le.Uint64(copyOf) {
				t.Fatal("corruption was undetected")
			}
			if le.Uint64(b[8:]) != n.oid || le.Uint64(b[16:]) != transaction {
				t.Fatal("object header mismatch")
			}
		}
	}
	// Every referenced main-area block is marked used, and the first free one
	// is available. This is independent of the writer's free-count field.
	bm := l.blocks[l.ipBase+l.cibs+l.cabs]
	for n := uint64(0); n <= l.used; n++ {
		if marked := bm[n/8]&(1<<(n%8)) != 0; marked != (n < l.used) {
			t.Fatalf("allocation bit %d", n)
		}
	}
}

func TestReproducibleAndCancelled(t *testing.T) {
	v := testVolume()
	v.Root.Children = []*Node{{Name: "payload", Mode: 0644, Data: Bytes([]byte("payload"))}}
	i, err := Plan(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	var a, b bytes.Buffer
	if _, err := i.WriteTo(context.Background(), &a); err != nil {
		t.Fatal(err)
	}
	j, err := Plan(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.WriteTo(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("fixed input changed image bytes")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Plan(ctx, v); !errors.Is(err, context.Canceled) {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := i.WriteTo(ctx, io.Discard); !errors.Is(err, context.Canceled) {
		t.Fatalf("WriteTo: %v", err)
	}
	if _, err := i.WriteTo(context.Background(), shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return len(b) / 2, nil }

func TestSourceChangedAfterPlanning(t *testing.T) {
	for _, initial := range []string{"", "original"} {
		path := filepath.Join(t.TempDir(), "source")
		if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
			t.Fatal(err)
		}
		source, err := FromFile(path)
		if err != nil {
			t.Fatal(err)
		}
		v := testVolume()
		v.Root.Children = []*Node{{Name: "source", Data: source}}
		image, err := Plan(context.Background(), v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(initial+"changed"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := image.WriteTo(context.Background(), io.Discard); err == nil {
			t.Fatal("changed source was accepted")
		}
	}
}

// Exercise a second chunk-info block without writing gigabytes of zeroes.
// The sparse file has exactly the metadata and zero payload described by Plan.
func TestMultipleChunkInfoBlocks(t *testing.T) {
	const size = int64(17 << 30)
	v := testVolume()
	v.Root.Children = []*Node{{Name: "large", Mode: 0644, Data: zeroSource(size)}}
	image, err := Plan(context.Background(), v)
	if err != nil {
		t.Fatal(err)
	}
	if image.layout.cibs < 2 {
		t.Fatal("fixture did not require multiple chunk-info blocks")
	}
	path := filepath.Join(t.TempDir(), "large.dmg")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(image.Size()); err != nil {
		t.Fatal(err)
	}
	for address, block := range image.layout.blocks {
		if _, err := f.WriteAt(block, int64(address)*blockSize); err != nil {
			t.Fatal(err)
		}
	}
	checkImage(t, path)
	point := mountImage(t, path)
	info, err := os.Stat(filepath.Join(point, "large"))
	if err != nil || info.Size() != size {
		t.Fatalf("large file: %v %v", info, err)
	}
}
