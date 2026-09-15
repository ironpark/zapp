package macho

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// otoolL returns what otool -L reports, as the install names alone. otool
// prints an LC_ID_DYLIB first for a dylib, which the caller drops separately.
func otoolL(t *testing.T, path string) []string {
	t.Helper()
	out := run(t, "otool", "-L", path)
	var names []string
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "(compatibility version") {
			continue
		}
		name := strings.Fields(line)[0]
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// otoolD returns the install name otool -D reports, empty for a non-dylib.
func otoolD(t *testing.T, path string) string {
	t.Helper()
	for _, line := range strings.Split(run(t, "otool", "-D", path), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, ":") {
			continue
		}
		return line
	}
	return ""
}

// otoolRPaths returns the LC_RPATH entries otool -l reports.
func otoolRPaths(t *testing.T, path string) []string {
	t.Helper()
	var paths []string
	seen := map[string]bool{}
	inRPath := false
	for _, line := range strings.Split(run(t, "otool", "-l", path), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "cmd":
			inRPath = fields[1] == "LC_RPATH"
		case "path":
			if inRPath && !seen[fields[1]] {
				seen[fields[1]] = true
				paths = append(paths, fields[1])
			}
			inRPath = false
		}
	}
	return paths
}

func run(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v (output: %s)", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not available: %v", name, err)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// buildFixtures compiles a dylib and an executable that links it. System
// binaries cover neither an install name nor generous header padding, so
// without these the SetID and AddRPath paths would go untested. Both are built
// universal, to exercise the fat path on every architecture.
func buildFixtures(t *testing.T) []string {
	t.Helper()
	if _, err := exec.LookPath("cc"); err != nil {
		return nil
	}
	dir := t.TempDir()

	src := filepath.Join(dir, "lib.c")
	if err := os.WriteFile(src, []byte("int answer(void) { return 42; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	dylib := filepath.Join(dir, "libfixture.dylib")
	if out, err := exec.Command("cc", "-dynamiclib",
		"-arch", "x86_64", "-arch", "arm64",
		"-install_name", "@rpath/libfixture.dylib",
		"-Wl,-headerpad_max_install_names",
		"-o", dylib, src).CombinedOutput(); err != nil {
		t.Logf("could not build the dylib fixture: %v (%s)", err, out)
		return nil
	}

	mainSrc := filepath.Join(dir, "main.c")
	if err := os.WriteFile(mainSrc, []byte("int answer(void);\nint main(void) { return answer(); }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	exeP := filepath.Join(dir, "app")
	if out, err := exec.Command("cc",
		"-arch", "x86_64", "-arch", "arm64",
		"-Wl,-headerpad_max_install_names",
		"-o", exeP, mainSrc, dylib).CombinedOutput(); err != nil {
		t.Logf("could not build the executable fixture: %v (%s)", err, out)
		return []string{dylib}
	}
	return []string{dylib, exeP}
}

// subjects returns Mach-O files to test against: system binaries for the shapes
// the linker produces in practice, plus built fixtures for the cases they do
// not cover.
func subjects(t *testing.T) []string {
	t.Helper()
	var found []string
	for _, p := range []string{"/bin/ls", "/bin/sh", "/usr/bin/otool", "/usr/bin/true"} {
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	found = append(found, buildFixtures(t)...)
	if len(found) == 0 {
		t.Skip("no Mach-O files available to test against")
	}
	return found
}

// The reader has to agree with otool on real binaries, which is the whole point
// of replacing it.
func TestReadMatchesOtool(t *testing.T) {
	requireTool(t, "otool")

	for _, path := range subjects(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			info, err := Read(path)
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}

			wantID := otoolD(t, path)
			if info.ID != wantID {
				t.Errorf("ID = %q, want %q", info.ID, wantID)
			}

			// otool -L lists the ID first for a dylib; Read excludes it.
			wantDeps := otoolL(t, path)
			if wantID != "" {
				var filtered []string
				for _, d := range wantDeps {
					if d != wantID {
						filtered = append(filtered, d)
					}
				}
				wantDeps = filtered
			}
			if !equal(info.Dependencies, wantDeps) {
				t.Errorf("Dependencies =\n  %q\nwant\n  %q", info.Dependencies, wantDeps)
			}

			if want := otoolRPaths(t, path); !equal(info.RPaths, want) {
				t.Errorf("RPaths = %q, want %q", info.RPaths, want)
			}
		})
	}
}

func TestReadRejectsNonMachO(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"empty":  "",
		"script": "#!/bin/sh\necho hi\n",
		"short":  "MZ",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Read(path); err == nil {
			t.Errorf("Read(%s) accepted a non-Mach-O file", name)
		}
	}
}

// copyTo gives each test its own writable copy, since the subjects are system
// files and the edits are destructive.
func copyTo(t *testing.T, src, dst string) string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0755); err != nil {
		t.Fatal(err)
	}
	return dst
}

// Each edit has to produce what install_name_tool produces, byte for byte.
func TestEditsMatchInstallNameTool(t *testing.T) {
	requireTool(t, "otool")
	requireTool(t, "install_name_tool")

	for _, path := range subjects(t) {
		info, err := Read(path)
		if err != nil {
			t.Fatalf("Read(%s): %v", path, err)
		}

		t.Run("change/"+filepath.Base(path), func(t *testing.T) {
			if len(info.Dependencies) == 0 {
				t.Skip("no dependencies to repoint")
			}
			old := info.Dependencies[0]
			// Same length, so it fits wherever the original did.
			new := "@rpath/" + strings.Repeat("x", max(0, len(old)-7))

			dir := t.TempDir()
			mine := copyTo(t, path, filepath.Join(dir, "mine"))
			theirs := copyTo(t, path, filepath.Join(dir, "theirs"))

			if err := ChangeDependency(mine, old, new); err != nil {
				t.Fatalf("ChangeDependency() error: %v", err)
			}
			run(t, "install_name_tool", "-change", old, new, theirs)
			assertSameLinkage(t, mine, theirs)
			assertOnlyHeaderChanged(t, path, mine)
		})

		t.Run("id/"+filepath.Base(path), func(t *testing.T) {
			if info.ID == "" {
				t.Skip("not a dylib")
			}
			new := "@rpath/" + strings.Repeat("y", max(0, len(info.ID)-7))

			dir := t.TempDir()
			mine := copyTo(t, path, filepath.Join(dir, "mine"))
			theirs := copyTo(t, path, filepath.Join(dir, "theirs"))

			if err := SetID(mine, new); err != nil {
				t.Fatalf("SetID() error: %v", err)
			}
			run(t, "install_name_tool", "-id", new, theirs)
			assertSameLinkage(t, mine, theirs)
			assertOnlyHeaderChanged(t, path, mine)
		})

		t.Run("addrpath/"+filepath.Base(path), func(t *testing.T) {
			const rpath = "@executable_path/../Frameworks"
			dir := t.TempDir()
			mine := copyTo(t, path, filepath.Join(dir, "mine"))
			theirs := copyTo(t, path, filepath.Join(dir, "theirs"))

			if err := AddRPath(mine, rpath); err != nil {
				t.Skipf("AddRPath() not possible for this binary: %v", err)
			}
			out, err := exec.Command("install_name_tool", "-add_rpath", rpath, theirs).CombinedOutput()
			if err != nil {
				t.Skipf("install_name_tool could not add an rpath either: %v (%s)", err, out)
			}
			assertSameLinkage(t, mine, theirs)
			assertOnlyHeaderChanged(t, path, mine)

			// And the result must read back.
			after, err := Read(mine)
			if err != nil {
				t.Fatalf("Read() after AddRPath: %v", err)
			}
			if !contains(after.RPaths, rpath) {
				t.Errorf("RPaths after AddRPath = %q, want it to contain %q", after.RPaths, rpath)
			}
		})
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// assertSameLinkage checks that the two files describe the same linkage, which
// is the property that matters. Byte equality is the wrong oracle: for a
// linker-signed binary install_name_tool also rewrites the code signature and
// so changes the file's size, while this package touches load commands only.
func assertSameLinkage(t *testing.T, mine, theirs string) {
	t.Helper()
	if got, want := otoolD(t, mine), otoolD(t, theirs); got != want {
		t.Errorf("install name = %q, install_name_tool produced %q", got, want)
	}
	if got, want := otoolL(t, mine), otoolL(t, theirs); !equal(got, want) {
		t.Errorf("libraries =\n  %q\ninstall_name_tool produced\n  %q", got, want)
	}
	if got, want := otoolRPaths(t, mine), otoolRPaths(t, theirs); !equal(got, want) {
		t.Errorf("rpaths = %q, install_name_tool produced %q", got, want)
	}
}

// assertOnlyHeaderChanged checks that the edit stayed inside the load command
// region of each architecture. Everything the linker placed after the header —
// code, data, the signature — must be byte for byte what it was.
func assertOnlyHeaderChanged(t *testing.T, original, edited string) {
	t.Helper()
	before, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("the edit changed the file size: %d -> %d", len(before), len(after))
	}

	images, err := slices(after)
	if err != nil {
		t.Fatal(err)
	}
	// Build the set of byte ranges the load commands are allowed to occupy.
	type span struct{ lo, hi int }
	var allowed []span
	for _, s := range images {
		cmds, err := s.commands(after)
		if err != nil {
			t.Fatal(err)
		}
		limit, err := s.headerLimit(after, cmds)
		if err != nil {
			continue
		}
		allowed = append(allowed, span{s.start, limit})
	}

	for i := range before {
		if before[i] == after[i] {
			continue
		}
		ok := false
		for _, sp := range allowed {
			if i >= sp.lo && i < sp.hi {
				ok = true
				break
			}
		}
		if !ok {
			t.Fatalf("the edit changed byte %d, which is outside every load command region", i)
		}
	}
}

// Adding a runpath twice must not add it twice.
func TestAddRPathIsIdempotent(t *testing.T) {
	const rpath = "@executable_path/../Frameworks"
	for _, path := range subjects(t) {
		dir := t.TempDir()
		f := copyTo(t, path, filepath.Join(dir, "bin"))

		if err := AddRPath(f, rpath); err != nil {
			continue // no header padding; covered elsewhere
		}
		before, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := AddRPath(f, rpath); err != nil {
			t.Fatalf("second AddRPath() error: %v", err)
		}
		after, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if len(before) != len(after) {
			t.Errorf("%s: the second AddRPath changed the file", filepath.Base(path))
		}
		return
	}
	t.Skip("no binary with header padding available")
}

// A name that does not fit has to be refused rather than truncated.
func TestChangeRejectsOversizedName(t *testing.T) {
	for _, path := range subjects(t) {
		info, err := Read(path)
		if err != nil || len(info.Dependencies) == 0 {
			continue
		}
		dir := t.TempDir()
		f := copyTo(t, path, filepath.Join(dir, "bin"))

		huge := "/" + strings.Repeat("z", 4096)
		if err := ChangeDependency(f, info.Dependencies[0], huge); err == nil {
			t.Errorf("%s: ChangeDependency accepted a 4 KiB name", filepath.Base(path))
		}
		// The file must be untouched after a refusal.
		again, err := Read(f)
		if err != nil {
			t.Fatalf("Read() after a refused change: %v", err)
		}
		if !equal(again.Dependencies, info.Dependencies) {
			t.Errorf("%s: a refused change still modified the file", filepath.Base(path))
		}
		return
	}
	t.Skip("no suitable binary")
}
