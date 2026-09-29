package appbundle

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// CheckLinks reports a versioned framework in the bundle at path whose
// symbolic links were replaced on the way here. Versions/Current and every
// entry beside Versions have to be links: a code signature seals them as
// links, so anything else fails verification on the Mac and the notary
// rejects it. The usual culprits are Git on Windows, which checks a link out
// as a text file holding its target, and copying or unpacking with a tool
// that follows links, which leaves a copy of what they pointed to.
func CheckLinks(path string) error {
	var problems []string
	checkedOut := false
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || filepath.Ext(p) != ".framework" {
			return nil
		}
		if versions, err := os.Lstat(filepath.Join(p, "Versions")); err != nil || !versions.IsDir() {
			return nil // a shallow framework has no links
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return err
		}
		links := []string{filepath.Join("Versions", "Current")}
		for _, e := range entries {
			if e.Name() != "Versions" {
				links = append(links, e.Name())
			}
		}
		rel, _ := filepath.Rel(path, p)
		for _, link := range links {
			problem, text := notLink(filepath.Join(p, link))
			if problem != "" {
				problems = append(problems, filepath.ToSlash(filepath.Join(rel, link))+" is "+problem)
				checkedOut = checkedOut || text
			}
		}
		return nil
	})
	if err != nil || len(problems) == 0 {
		return err
	}
	fix := "copy or unpack it with a tool that keeps symbolic links, such as tar or ditto"
	if checkedOut {
		fix = "on Windows enable Developer Mode, run `git config core.symlinks true` and check it out again"
	}
	return fmt.Errorf("%s has lost the symbolic links in its frameworks, which breaks their code signature:\n  %s\nBuild it on macOS or Linux, or %s",
		filepath.Base(path), strings.Join(problems, "\n  "), fix)
}

// notLink describes what stands where a link should be, and whether it is a
// text file holding the link's target. It returns "" for a link, or for
// nothing at all.
func notLink(path string) (problem string, text bool) {
	info, err := os.Lstat(path)
	switch {
	case err != nil || info.Mode()&fs.ModeSymlink != 0:
		return "", false
	case info.IsDir():
		return "a copy of the directory the link pointed to", false
	}
	if info.Mode().IsRegular() && info.Size() > 0 && info.Size() < 1024 {
		data, err := os.ReadFile(path)
		if err == nil && utf8.Valid(data) && !bytes.ContainsAny(data, "\x00\n") {
			return fmt.Sprintf("a text file holding %q, the link's target", data), true
		}
	}
	return "a copy of the file the link pointed to", false
}
