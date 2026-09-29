// Package verify checks that an app, DMG, PKG or ZIP is ready to ship: signed
// with a Developer ID and a secure timestamp, with the hardened runtime, and
// notarized. What can be read from the files is checked on every platform;
// on macOS Apple's own codesign, spctl and pkgutil check the rest, and
// elsewhere the signature library checks each binary's code digests.
package verify

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/ironpark/zapp/pkg/appbundle"
	"github.com/ironpark/zapp/pkg/archive"
	"github.com/ironpark/zapp/pkg/macho"
)

// Status is the outcome of one check.
type Status int

const (
	Pass Status = iota
	// Warn is a problem that does not stop the artifact from working.
	Warn
	Fail
	// Skip is a check this platform or build cannot make.
	Skip
)

func (s Status) String() string {
	return [...]string{"ok", "warn", "FAIL", "skip"}[s]
}

// Check is one thing verified about an artifact.
type Check struct {
	Name   string
	Status Status
	Detail string
}

// Report is what was found about one artifact.
type Report struct {
	// Path is the artifact; for an app inside a ZIP, the ZIP and the app.
	Path   string
	Checks []Check
}

// Failed reports whether any check failed.
func (r Report) Failed() bool {
	return slices.ContainsFunc(r.Checks, func(c Check) bool { return c.Status == Fail })
}

func (r *Report) add(name string, status Status, format string, args ...any) {
	r.Checks = append(r.Checks, Check{name, status, fmt.Sprintf(format, args...)})
}

// String renders the report as the artifact, then one line per check.
func (r Report) String() string {
	var b strings.Builder
	b.WriteString(r.Path + "\n")
	for _, c := range r.Checks {
		b.WriteString(strings.TrimRight(fmt.Sprintf("  %-4s  %-19s %s", c.Status, c.Name, c.Detail), " ") + "\n")
	}
	return b.String()
}

// Path verifies the artifact at path, telling it by its extension: .app,
// .dmg, .pkg or .zip. A ZIP is reported per app it holds.
func Path(ctx context.Context, path string) ([]Report, error) {
	switch strings.ToLower(filepath.Ext(filepath.Clean(path))) {
	case ".app":
		return []Report{App(ctx, path)}, nil
	case ".dmg":
		return []Report{DMG(ctx, path)}, nil
	case ".pkg":
		return []Report{PKG(ctx, path)}, nil
	case ".zip":
		return Zip(ctx, path)
	}
	return nil, fmt.Errorf("%s is not an app, DMG, PKG or ZIP", path)
}

// Zip extracts the archive and verifies each app at its top level.
func Zip(ctx context.Context, path string) ([]Report, error) {
	dir, err := os.MkdirTemp("", "zapp-verify-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := archive.Unzip(ctx, path, dir); err != nil {
		return nil, fmt.Errorf("extracting %s: %w", path, err)
	}
	apps, _ := filepath.Glob(filepath.Join(dir, "*.app"))
	if len(apps) == 0 {
		return nil, fmt.Errorf("%s holds no app at its top level", path)
	}
	var reports []Report
	for _, app := range apps {
		r := App(ctx, app)
		r.Path = path + ": " + filepath.Base(app)
		reports = append(reports, r)
	}
	return reports, nil
}

// App verifies an app bundle.
func App(ctx context.Context, path string) Report {
	r := Report{Path: path}
	info, err := appbundle.Open(path)
	if err != nil {
		r.add("bundle", Fail, "%v", err)
		return r
	}
	exeName, err := info.BundleExecutable()
	if err != nil {
		r.add("bundle", Fail, "%v", err)
		return r
	}
	exe := filepath.Join(path, "Contents", "MacOS", exeName)
	switch st, err := os.Stat(exe); {
	case err != nil:
		r.add("bundle", Fail, "CFBundleExecutable names Contents/MacOS/%s, which is missing", exeName)
		return r
	case runtime.GOOS != "windows" && st.Mode()&0o111 == 0:
		r.add("bundle", Fail, "Contents/MacOS/%s is not executable", exeName)
	default:
		id, _ := info.BundleID()
		version, _ := info.Version()
		r.add("bundle", Pass, "%s %s", id, version)
	}
	if err := appbundle.CheckLinks(path); err != nil {
		r.add("framework links", Fail, "%v", err)
	}

	binaries, err := machOFiles(path)
	if err != nil {
		r.add("signature", Fail, "%v", err)
		return r
	}
	var s sigChecks
	for _, file := range binaries {
		rel, _ := filepath.Rel(path, file)
		s.read(filepath.ToSlash(rel), file, file == exe)
	}
	s.report(&r, "Developer ID Application", len(binaries), true)

	main, _ := macho.ReadImages(exe)
	var archs []string
	needs, needsArch := "", ""
	for _, img := range main {
		archs = append(archs, img.Arch)
		if newer(img.MinOS, needs) {
			needs, needsArch = img.MinOS, img.Arch
		}
	}
	if len(archs) > 0 {
		r.add("architectures", Pass, "%s", strings.Join(archs, ", "))
	}
	declared, _ := info.GetString("LSMinimumSystemVersion")
	switch {
	case declared != "" && newer(needs, declared):
		r.add("minimum macOS", Warn, "Info.plist says %s, but Contents/MacOS/%s needs %s on %s; older Macs offer to open it and it fails", declared, exeName, needs, needsArch)
	case declared != "":
		r.add("minimum macOS", Pass, "%s", declared)
	case needs != "":
		r.add("minimum macOS", Pass, "%s, from the executable; Info.plist has no LSMinimumSystemVersion", needs)
	}

	if _, err := os.Stat(filepath.Join(path, "Contents", "CodeResources")); err == nil {
		r.add("stapled", Pass, "the notarization ticket is attached")
	} else {
		r.add("stapled", Warn, "no notarization ticket attached; Gatekeeper has to reach Apple the first time it opens")
	}
	deepApp(ctx, &r, path)
	return r
}

// sigChecks gathers what the signatures of an artifact's code say, and the
// files at fault for each check.
type sigChecks struct {
	unsigned, adHoc, wrongKind, noTimestamp, noRuntime, debuggable, unreadable []string
	signers, teams                                                             []string
	kinds                                                                      []kinded
}

// kinded is a signature and the common name of its certificate.
type kinded struct{ where, name string }

func (s *sigChecks) read(rel, file string, main bool) {
	images, err := macho.ReadImages(file)
	if err != nil {
		s.unreadable = append(s.unreadable, fmt.Sprintf("%s: %v", rel, err))
		return
	}
	for _, img := range images {
		if img.Unloaded {
			continue
		}
		where := rel
		if len(images) > 1 {
			where += " (" + img.Arch + ")"
		}
		s.signature(where, img.Signature, img.Executable, main)
	}
}

// signature records one code signature: of a Mach-O image, or of a disk
// image, which is neither an executable nor an app's main one.
func (s *sigChecks) signature(where string, sig *macho.Signature, executable, main bool) {
	switch {
	case sig == nil || !sig.Signed:
		s.unsigned = append(s.unsigned, where)
		return
	case sig.AdHoc || len(sig.CMS) == 0:
		s.adHoc = append(s.adHoc, where)
		return
	}
	if executable && !sig.Runtime {
		s.noRuntime = append(s.noRuntime, where)
	}
	if main && getTaskAllow(sig.Entitlements) {
		s.debuggable = append(s.debuggable, where)
	}
	signer, err := parseSigner(sig.CMS)
	if err != nil {
		s.unreadable = append(s.unreadable, fmt.Sprintf("%s: %v", where, err))
		return
	}
	s.signer(where, signer)
	team := sig.TeamID
	if team == "" && len(signer.Leaf.Subject.OrganizationalUnit) > 0 {
		team = signer.Leaf.Subject.OrganizationalUnit[0]
	}
	if team != "" && !slices.Contains(s.teams, team) {
		s.teams = append(s.teams, team)
	}
}

func (s *sigChecks) signer(where string, signer *signer) {
	name := signer.Leaf.Subject.CommonName
	if !slices.Contains(s.signers, name) {
		s.signers = append(s.signers, name)
	}
	s.kinds = append(s.kinds, kinded{where, name})
	if signer.Timestamp.IsZero() {
		s.noTimestamp = append(s.noTimestamp, where)
	}
}

// report adds the signature checks to r, given the certificate kind the
// notary accepts for this artifact and how many files were read. code is set
// for Mach-O code, which the hardened runtime and entitlements apply to.
func (s *sigChecks) report(r *Report, kind string, files int, code bool) {
	for _, k := range s.kinds {
		if !strings.HasPrefix(k.name, kind+":") {
			s.wrongKind = append(s.wrongKind, k.where+" by "+k.name)
		}
	}
	switch {
	case files == 0:
		r.add("signature", Fail, "no code found to verify")
		return
	case len(s.unsigned)+len(s.adHoc)+len(s.unreadable) > 0:
		var problems []string
		if len(s.unsigned) > 0 {
			problems = append(problems, "not signed: "+list(s.unsigned))
		}
		if len(s.adHoc) > 0 {
			problems = append(problems, "signed ad hoc, with no certificate: "+list(s.adHoc))
		}
		if len(s.unreadable) > 0 {
			problems = append(problems, "unreadable: "+list(s.unreadable))
		}
		r.add("signature", Fail, "%s", strings.Join(problems, "; "))
	default:
		r.add("signature", Pass, "%s", strings.Join(s.signers, ", "))
	}
	if len(s.kinds) == 0 {
		return
	}
	if len(s.wrongKind) > 0 {
		r.add("Developer ID", Fail, "the notary accepts only a %s certificate; signed %s", kind, list(s.wrongKind))
	} else {
		r.add("Developer ID", Pass, "%s", kind)
	}
	if len(s.teams) > 1 {
		r.add("team", Fail, "signed by more than one team, %s; the app cannot load code from another team", strings.Join(s.teams, ", "))
	}
	if len(s.noTimestamp) > 0 {
		r.add("secure timestamp", Fail, "the notary needs one; missing on %s", list(s.noTimestamp))
	} else {
		r.add("secure timestamp", Pass, "")
	}
	if code {
		if len(s.noRuntime) > 0 {
			r.add("hardened runtime", Fail, "the notary needs it on every executable; missing on %s", list(s.noRuntime))
		} else {
			r.add("hardened runtime", Pass, "")
		}
		if len(s.debuggable) > 0 {
			r.add("entitlements", Fail, "com.apple.security.get-task-allow is set on %s; the notary rejects a debuggable app", list(s.debuggable))
		}
	}
}

// list names up to five files, then counts the rest.
func list(items []string) string {
	if len(items) <= 5 {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(items[:5], ", "), len(items)-5)
}

var getTaskAllowKey = []byte("<key>com.apple.security.get-task-allow</key>")

// getTaskAllow reports an entitlements plist that lets a debugger attach.
func getTaskAllow(entitlements []byte) bool {
	_, after, ok := bytes.Cut(entitlements, getTaskAllowKey)
	return ok && bytes.HasPrefix(bytes.TrimSpace(after), []byte("<true/>"))
}

// machOFiles lists the Mach-O files in a bundle, not following links.
func machOFiles(bundle string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(bundle, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		header := make([]byte, macho.HeaderSize)
		n, err := io.ReadFull(f, header)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return err
		}
		if macho.IsHeader(header[:n]) {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

// newer reports whether version a is later than b, comparing numerically
// part by part. An empty a is never newer.
func newer(a, b string) bool {
	if a == "" {
		return false
	}
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			_, _ = fmt.Sscan(pa[i], &x)
		}
		if i < len(pb) {
			_, _ = fmt.Sscan(pb[i], &y)
		}
		if x != y {
			return x > y
		}
	}
	return false
}
