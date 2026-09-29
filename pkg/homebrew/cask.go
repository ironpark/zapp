// Package homebrew writes Homebrew casks: the Ruby files `brew install
// --cask` reads to download and install a Mac app.
package homebrew

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Cask describes one release of an app, as a cask installs it.
type Cask struct {
	// Token names the cask: `brew install --cask <token>`.
	Token   string
	Version string
	// SHA256 is the download's digest, in hex.
	SHA256 string
	URL    string
	// Name is the app's name as people know it; Desc a one-line summary.
	Name, Desc, Homepage string
	// App is the bundle a DMG or ZIP holds, such as Demo.app. PKG, when set
	// instead, is an installer package to run, and PKGIDs the receipts it
	// leaves, which uninstalling removes.
	App    string
	PKG    string
	PKGIDs []string
	// MinimumMacOS is LSMinimumSystemVersion; empty leaves it out.
	MinimumMacOS string
	// AutoUpdates tells Homebrew the app updates itself, with Sparkle.
	AutoUpdates bool
}

// Render writes the cask as Ruby, in the order `brew style` wants.
func (c Cask) Render() string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, "  "+format+"\n", args...) }
	fmt.Fprintf(&b, "cask %s do\n", ruby(c.Token))
	line("version %s", ruby(c.Version))
	line("sha256 %s", ruby(c.SHA256))
	b.WriteString("\n")
	line("url %s", ruby(c.URL))
	line("name %s", ruby(c.Name))
	if c.Desc != "" {
		line("desc %s", ruby(c.Desc))
	}
	line("homepage %s", ruby(c.Homepage))
	extra := false
	if c.AutoUpdates {
		b.WriteString("\n")
		extra = true
		line("auto_updates true")
	}
	if symbol := MacOSSymbol(c.MinimumMacOS); symbol != "" {
		if !extra {
			b.WriteString("\n")
		}
		line("depends_on macos: %s", ruby(">= :"+symbol))
	}
	b.WriteString("\n")
	if c.PKG != "" {
		line("pkg %s", ruby(c.PKG))
		if len(c.PKGIDs) > 0 {
			b.WriteString("\n")
			ids := make([]string, len(c.PKGIDs))
			for i, id := range c.PKGIDs {
				ids[i] = ruby(id)
			}
			if len(ids) == 1 {
				line("uninstall pkgutil: %s", ids[0])
			} else {
				line("uninstall pkgutil: [%s]", strings.Join(ids, ", "))
			}
		}
	} else {
		line("app %s", ruby(c.App))
	}
	b.WriteString("end\n")
	return b.String()
}

// ruby quotes s as a double-quoted Ruby string, which would otherwise
// interpolate #{...}.
func ruby(s string) string {
	q := strconv.Quote(s)
	return strings.ReplaceAll(q, "#", `\#`)
}

// Token turns an app's name into a cask token as Homebrew names them:
// lower case, with hyphens between words, such as "My App" to "my-app".
func Token(name string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(strings.TrimSuffix(name, ".app")) {
		switch {
		case r == '+':
			b.WriteString("-plus")
			hyphen = false
		case r == '@':
			b.WriteString("-at-")
			hyphen = true
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			hyphen = false
		case b.Len() > 0 && !hyphen:
			b.WriteByte('-')
			hyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// symbols are the macOS releases Homebrew names, newest first.
var symbols = []struct {
	major, minor int
	name         string
}{
	{26, 0, "tahoe"}, {15, 0, "sequoia"}, {14, 0, "sonoma"}, {13, 0, "ventura"},
	{12, 0, "monterey"}, {11, 0, "big_sur"}, {10, 15, "catalina"}, {10, 14, "mojave"},
	{10, 13, "high_sierra"}, {10, 12, "sierra"}, {10, 11, "el_capitan"},
}

// MacOSSymbol is Homebrew's name for the oldest release a version runs on,
// such as monterey for 12.0, or "" for one it has no name for.
func MacOSSymbol(version string) string {
	parts := strings.SplitN(version, ".", 3)
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return ""
	}
	minor := 0
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	for _, s := range symbols {
		if major == s.major && (major > 10 || minor == s.minor) {
			return s.name
		}
	}
	return ""
}
