package macpkg

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func put(t *testing.T, p string, b []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, mode); err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T) (string, ComponentConfig) {
	t.Helper()
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	put(t, filepath.Join(root, "dir", "hello & 한글"), []byte("hello"), 0751)
	if err := os.Link(filepath.Join(root, "dir", "hello & 한글"), filepath.Join(root, "hard")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("dir/hello & 한글", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "empty"), nil, 0640)
	return dir, ComponentConfig{Root: root, Identifier: "dev.zapp.test", Version: "1.2", InstallLocation: "/Applications", OutputPath: filepath.Join(dir, "component.pkg")}
}

type cpioRecord struct {
	name             string
	size             int64
	mode, ino, nlink uint64
	data             []byte
}

func readCPIO(t *testing.T, r io.Reader) []cpioRecord {
	t.Helper()
	z, err := gzip.NewReader(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = z.Close() }()
	var out []cpioRecord
	for {
		h := make([]byte, 76)
		if _, err = io.ReadFull(z, h); err != nil {
			t.Fatal(err)
		}
		if string(h[:6]) != "070707" {
			t.Fatalf("bad magic %q", h[:6])
		}
		parse := func(a, b int) uint64 {
			v, e := strconv.ParseUint(string(h[a:b]), 8, 64)
			if e != nil {
				t.Fatal(e)
			}
			return v
		}
		n := parse(59, 65)
		size := int64(parse(65, 76))
		name := make([]byte, n)
		if _, err = io.ReadFull(z, name); err != nil {
			t.Fatal(err)
		}
		if string(name) == "TRAILER!!!\x00" {
			break
		}
		if size > 16<<20 {
			t.Fatal("unexpected large record")
		}
		data := make([]byte, size)
		if _, err = io.ReadFull(z, data); err != nil {
			t.Fatal(err)
		}
		out = append(out, cpioRecord{string(name[:len(name)-1]), size, parse(18, 24), parse(12, 18), parse(36, 42), data})
	}
	return out
}

func TestComponent(t *testing.T) {
	t.Setenv("PATH", "/nonexistent") // Generation must not shell out to any tools.
	_, c := fixture(t)
	if err := BuildComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	a, err := openXAR(context.Background(), c.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.f.Close() }()
	b, err := a.read(context.Background(), "PackageInfo", maxMetadata)
	if err != nil {
		t.Fatal(err)
	}
	var pi packageInfo
	if err = xml.Unmarshal(b, &pi); err != nil {
		t.Fatal(err)
	}
	if pi.Identifier != c.Identifier || pi.Payload.Files != 6 || pi.MinOS != "10.9" {
		t.Fatalf("bad package info: %s", b)
	}
	p, err := a.read(context.Background(), "Payload", maxMetadata)
	if err != nil {
		t.Fatal(err)
	}
	records := readCPIO(t, bytes.NewReader(p))
	byName := map[string]cpioRecord{}
	for _, r := range records {
		byName[r.name] = r
	}
	first, hard := byName["./dir/hello & 한글"], byName["./hard"]
	if first.ino != hard.ino || first.nlink != 2 || string(first.data) != "hello" || first.mode != 0100751 {
		t.Fatalf("hardlink or mode lost: %+v %+v", first, hard)
	}
	if string(byName["./link"].data) != "dir/hello & 한글" {
		t.Fatal("symlink target changed")
	}
	if _, err = a.read(context.Background(), "Bom", maxMetadata); err != nil {
		t.Fatal(err)
	}
}

func TestProductAndScripts(t *testing.T) {
	dir, c := fixture(t)
	ctx := context.Background()
	if err := BuildComponent(ctx, c); err != nil {
		t.Fatal(err)
	}
	scripts := filepath.Join(dir, "scripts")
	put(t, filepath.Join(scripts, "postinstall"), []byte("#!/bin/sh\nexit 0\n"), 0755)
	second := filepath.Join(dir, "scripts.pkg")
	if err := BuildComponent(ctx, ComponentConfig{ScriptsDir: scripts, Identifier: "dev.zapp.scripts", Version: "1", OutputPath: second}); err != nil {
		t.Fatal(err)
	}
	resources := filepath.Join(dir, "resources")
	put(t, filepath.Join(resources, "ko.lproj", "license.txt"), []byte("약관"), 0644)
	out := filepath.Join(dir, "product.pkg")
	d := &Distribution{Title: "A < B & C", LicenseFile: "license.txt", Choices: []Choice{{ID: "app", Visible: true, Selected: true, PackageIDs: []string{c.Identifier}}, {ID: "scripts", Selected: true, PackageIDs: []string{"dev.zapp.scripts"}}}}
	pc := ProductConfig{OutputPath: out, Packages: []string{c.OutputPath, second}, ResourcesDir: resources, Distribution: d}
	if err := BuildProduct(ctx, pc); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "darwin" {
		expanded := filepath.Join(dir, "expanded")
		native(t, "pkgutil", "--expand-full", out, expanded)
		if _, err := os.Stat(filepath.Join(expanded, "scripts.pkg", "Scripts", "postinstall")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(expanded, "Resources", "ko.lproj", "license.txt")); err != nil {
			t.Fatal(err)
		}
	}
	a, err := openXAR(ctx, out)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.f.Close() }()
	b, err := a.read(ctx, "Distribution", maxMetadata)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("A &lt; B &amp; C")) {
		t.Fatalf("XML not escaped: %s", b)
	}
	if a.entries["component.pkg/Payload"] == nil || a.entries["scripts.pkg/Scripts"] == nil || a.entries["scripts.pkg/Payload"] != nil || a.entries["Resources/ko.lproj/license.txt"] == nil {
		t.Fatal("missing product members")
	}
	pc.Distribution = nil
	pc.DistributionXML = b
	pc.OutputPath = filepath.Join(dir, "custom.pkg")
	if err := BuildProduct(ctx, pc); err != nil {
		t.Fatal(err)
	}
	pc.DistributionXML = bytes.ReplaceAll(b, []byte("#component.pkg"), []byte("https://example.com/component.pkg"))
	if err := BuildProduct(ctx, pc); err == nil {
		t.Fatal("accepted remote package reference")
	}
	pc.DistributionXML = []byte(string(b) + "<extra/>")
	if err := BuildProduct(ctx, pc); err == nil {
		t.Fatal("accepted two roots")
	}
	pc.DistributionXML = b
	pc.Distribution = d
	if err := BuildProduct(ctx, pc); err == nil {
		t.Fatal("accepted both distribution inputs")
	}
}

func TestFailuresKeepOutput(t *testing.T) {
	dir, c := fixture(t)
	original := []byte("existing output")
	put(t, c.OutputPath, original, 0644)
	cases := []struct {
		name   string
		change func(*ComponentConfig)
	}{
		{"identifier", func(c *ComponentConfig) { c.Identifier = "" }},
		{"invalid XML", func(c *ComponentConfig) { c.Identifier = "dev.\x01invalid" }},
		{"relative install", func(c *ComponentConfig) { c.InstallLocation = "Applications" }},
		{"root escape", func(c *ComponentConfig) { c.RootEntry = "../root" }},
		{"bad mode", func(c *ComponentConfig) { c.PayloadMode = 99 }},
		{"old OS", func(c *ComponentConfig) { c.PayloadMode = Large; c.MinOSVersion = "11.0" }},
		{"unexecutable script", func(c *ComponentConfig) {
			c.ScriptsDir = filepath.Join(dir, "badscript")
			put(t, filepath.Join(c.ScriptsDir, "preinstall"), []byte("exit 0"), 0644)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x := c
			tc.change(&x)
			if err := BuildComponent(context.Background(), x); err == nil {
				t.Fatal("accepted invalid config")
			}
			b, _ := os.ReadFile(c.OutputPath)
			if !bytes.Equal(b, original) {
				t.Fatal("replaced output on failure")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := BuildComponent(ctx, c); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	c.RootEntry = "missing"
	if err := BuildComponent(context.Background(), c); err == nil {
		t.Fatal("accepted missing root entry")
	}
	if err := atomicBuild(context.Background(), c.OutputPath, func(f *os.File, _ string) error {
		if _, err := f.Write([]byte("partial")); err != nil {
			return err
		}
		return io.ErrClosedPipe
	}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(c.OutputPath)
	if !bytes.Equal(b, original) {
		t.Fatal("partial output committed")
	}
}

func TestChecksumsAndBounds(t *testing.T) {
	s := &posixSum{}
	_, _ = s.Write([]byte("hello")) // posixSum.Write cannot fail.
	if s.Sum32() != 3287646509 {
		t.Fatalf("cksum = %d", s.Sum32())
	}
	for _, size := range []int64{-1, legacyLimit} {
		if err := cpioHeader(io.Discard, fileEntry{}, "x", size, 1); err == nil {
			t.Fatal("accepted overflow")
		}
	}
	if err := cpioHeader(io.Discard, fileEntry{nlink: 1}, "x", legacyLimit-1, 1); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"../evil", "/evil", "a/../b", "a\\b", "x\x00y", "x\x01y", "."} {
		if validArchivePath(p) {
			t.Fatalf("accepted %q", p)
		}
	}
	_, c := fixture(t)
	if err := BuildComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(c.OutputPath)
	for _, which := range []string{"toc", "size", "heap"} {
		t.Run(which, func(t *testing.T) {
			bad := bytes.Clone(b)
			switch which {
			case "toc":
				bad[30] ^= 0xff
			case "size":
				be.PutUint64(bad[16:], maxTOC+1)
			case "heap":
				bad[28+int(be.Uint64(bad[8:]))] ^= 0xff
			}
			p := filepath.Join(t.TempDir(), "bad.pkg")
			put(t, p, bad, 0644)
			a, err := openXAR(context.Background(), p)
			if err == nil {
				if err := a.f.Close(); err != nil {
					t.Fatal(err)
				}
				t.Fatal("accepted corrupt archive")
			}
		})
	}
}

func native(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func TestAppleCompatibility(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Apple tools require macOS")
	}
	dir, c := fixture(t)
	for i := range 1200 {
		put(t, filepath.Join(c.Root, "many", fmt.Sprintf("file%04d", i)), []byte("hello"), 0644)
	}
	if err := BuildComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	expanded := filepath.Join(dir, "expanded")
	native(t, "pkgutil", "--expand-full", c.OutputPath, expanded)
	first := filepath.Join(expanded, "Payload", "dir", "hello & 한글")
	b, err := os.ReadFile(first)
	if err != nil || string(b) != "hello" {
		t.Fatalf("restored data: %q %v", b, err)
	}
	a, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	h, err := os.Stat(filepath.Join(expanded, "Payload", "hard"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(a, h) {
		t.Fatal("Apple expansion lost hard links")
	}
	if a.Mode().Perm() != 0751 {
		t.Fatal("Apple expansion lost permissions")
	}
	target, err := os.Readlink(filepath.Join(expanded, "Payload", "link"))
	if err != nil || target != "dir/hello & 한글" {
		t.Fatalf("symlink: %q %v", target, err)
	}
	listing := native(t, "lsbom", filepath.Join(expanded, "Bom"))
	if !strings.Contains(listing, "file1199") || !strings.Contains(listing, "3287646509") {
		t.Fatal("BOM traversal/checksum mismatch")
	}
	if got := len(strings.Split(strings.TrimSpace(listing), "\n")); got != 1207 {
		t.Fatalf("BOM paths = %d, want 1207", got)
	}
	prod := filepath.Join(dir, "product.pkg")
	if err := BuildProduct(context.Background(), ProductConfig{OutputPath: prod, Packages: []string{c.OutputPath}}); err != nil {
		t.Fatal(err)
	}
	native(t, "pkgutil", "--expand-full", prod, filepath.Join(dir, "product-expanded"))
	native(t, "installer", "-showChoicesXML", "-pkg", prod, "-target", "/")
	// Apple-created, zlib-encoded component members must survive raw import.
	reference := filepath.Join(dir, "apple.pkg")
	native(t, "pkgbuild", "--root", c.Root, "--identifier", "dev.zapp.apple", "--version", "1", reference)
	if err := BuildProduct(context.Background(), ProductConfig{OutputPath: filepath.Join(dir, "imported.pkg"), Packages: []string{reference}}); err != nil {
		t.Fatal(err)
	}
	native(t, "pkgutil", "--expand-full", filepath.Join(dir, "imported.pkg"), filepath.Join(dir, "imported"))
	// Apple productbuild must also accept our component.
	native(t, "productbuild", "--package", c.OutputPath, filepath.Join(dir, "apple-product.pkg"))
}

func TestLargeModeSmallPayload(t *testing.T) {
	dir, c := fixture(t)
	c.PayloadMode = Large
	if err := BuildComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	a, err := openXAR(context.Background(), c.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.f.Close() }()
	if a.entries["LargeSegmentedPayload"] == nil {
		t.Fatal("large member missing")
	}
	b, err := a.read(context.Background(), "PackageInfo", maxMetadata)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`minimumSystemVersion="12.0"`)) {
		t.Fatal("large minimum missing")
	}
	if runtime.GOOS == "darwin" {
		native(t, "pkgutil", "--expand-full", c.OutputPath, filepath.Join(dir, "expanded"))
		if _, err := os.Stat(filepath.Join(dir, "expanded", "LargeSegmentedPayload", "hard")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRootEntryAndTime(t *testing.T) {
	dir, c := fixture(t)
	c.RootEntry = "dir"
	stamp := time.Unix(1600000000, 0)
	if err := os.Chtimes(filepath.Join(c.Root, "dir", "hello & 한글"), stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := BuildComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	a, err := openXAR(context.Background(), c.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.f.Close() }()
	b, err := a.read(context.Background(), "Payload", maxMetadata)
	if err != nil {
		t.Fatal(err)
	}
	records := readCPIO(t, bytes.NewReader(b))
	if len(records) != 3 {
		t.Fatal("included siblings")
	}
	if runtime.GOOS == "darwin" {
		native(t, "pkgutil", "--expand-full", c.OutputPath, filepath.Join(dir, "expanded"))
		st, err := os.Stat(filepath.Join(dir, "expanded", "Payload", "dir", "hello & 한글"))
		if err != nil || !st.ModTime().Equal(stamp) {
			t.Fatalf("mtime lost: %v %v", st, err)
		}
	}
}
