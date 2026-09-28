// Package sparkle writes Sparkle appcasts: the RSS feed a Sparkle-enabled app
// reads to find its updates, with each download signed by an EdDSA key.
package sparkle

import (
	"bytes"
	"cmp"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Namespace is Sparkle's XML namespace.
const Namespace = "http://www.andymatuschak.org/xml-namespaces/sparkle"

// Item is one release in an appcast.
type Item struct {
	// Version is CFBundleVersion, which Sparkle compares; ShortVersion is
	// CFBundleShortVersionString, which it shows.
	Version      string
	ShortVersion string
	// MinimumSystemVersion is LSMinimumSystemVersion; empty leaves it out.
	MinimumSystemVersion string
	// ReleaseNotes is a link to the release notes; empty leaves it out.
	ReleaseNotes string
	Published    time.Time

	// URL, Length and Signature describe the download.
	URL       string
	Length    int64
	Signature string
}

// Key is a Sparkle EdDSA signing key.
type Key struct{ private ed25519.PrivateKey }

// ParseKey reads a private key as Sparkle's generate_keys -x exports it:
// the base64 of its 32-byte seed.
func ParseKey(text string) (Key, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil {
		return Key{}, fmt.Errorf("sparkle key is not base64: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return Key{}, fmt.Errorf("sparkle key holds %d bytes, not %d; export it with Sparkle's generate_keys -x", len(seed), ed25519.SeedSize)
	}
	return Key{ed25519.NewKeyFromSeed(seed)}, nil
}

// PublicKey is the key's public half, base64, as SUPublicEDKey holds it.
func (k Key) PublicKey() string {
	return base64.StdEncoding.EncodeToString(k.private.Public().(ed25519.PublicKey))
}

// SignFile returns the file's length and its signature, as an enclosure's
// length and sparkle:edSignature. Ed25519 signs the whole message, not a
// digest of it, so the file is read into memory, as Sparkle's own tools do.
func (k Key) SignFile(path string) (int64, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, "", err
	}
	return int64(len(data)), base64.StdEncoding.EncodeToString(ed25519.Sign(k.private, data)), nil
}

// Add returns the appcast feed with item as its newest release. An item for
// the same version is replaced and every other one kept, along with the rest
// of the feed as it was written. An empty feed starts a new one titled title.
func Add(feed []byte, title string, item Item) ([]byte, error) {
	entry := item.render()
	if len(bytes.TrimSpace(feed)) == 0 {
		var b bytes.Buffer
		b.WriteString(xml.Header)
		fmt.Fprintf(&b, "<rss version=\"2.0\" xmlns:sparkle=%q>\n  <channel>\n", Namespace)
		fmt.Fprintf(&b, "    <title>%s</title>\n", escape(title))
		b.WriteString(entry)
		b.WriteString("  </channel>\n</rss>\n")
		return b.Bytes(), nil
	}
	if err := xml.Unmarshal(feed, new(struct{})); err != nil {
		return nil, fmt.Errorf("existing appcast is not XML: %w", err)
	}
	doc := string(feed)
	if !strings.Contains(doc, Namespace) {
		return nil, fmt.Errorf("existing appcast does not declare Sparkle's namespace %s", Namespace)
	}
	for _, old := range itemPattern.FindAllStringIndex(doc, -1) {
		if itemVersion(doc[old[0]:old[1]]) == item.Version {
			doc = doc[:old[0]] + strings.TrimLeft(doc[old[1]:], " \t\r\n")
			break
		}
	}
	// The newest item goes first, at the first item's indentation.
	at := strings.Index(doc, "<item")
	if at < 0 {
		at = strings.Index(doc, "</channel>")
		if at < 0 {
			return nil, fmt.Errorf("existing appcast has no channel")
		}
	}
	lineStart := strings.LastIndex(doc[:at], "\n") + 1
	if strings.TrimSpace(doc[lineStart:at]) == "" {
		at = lineStart
	}
	return []byte(doc[:at] + entry + doc[at:]), nil
}

var (
	itemPattern    = regexp.MustCompile(`(?s)[ \t]*<item[\s>].*?</item>[ \t]*\r?\n?`)
	versionElement = regexp.MustCompile(`(?s)<sparkle:version>\s*(.*?)\s*</sparkle:version>`)
	versionAttr    = regexp.MustCompile(`sparkle:version\s*=\s*"([^"]*)"`)
)

// itemVersion finds the version an item was published for, as an element or,
// in older appcasts, an enclosure attribute.
func itemVersion(item string) string {
	if m := versionElement.FindStringSubmatch(item); m != nil {
		return m[1]
	}
	if m := versionAttr.FindStringSubmatch(item); m != nil {
		return m[1]
	}
	return ""
}

func (it Item) render() string {
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, "      "+format+"\n", args...) }
	b.WriteString("    <item>\n")
	line("<title>%s</title>", escape(cmp.Or(it.ShortVersion, it.Version)))
	line("<pubDate>%s</pubDate>", it.Published.UTC().Format(time.RFC1123Z))
	line("<sparkle:version>%s</sparkle:version>", escape(it.Version))
	if it.ShortVersion != "" {
		line("<sparkle:shortVersionString>%s</sparkle:shortVersionString>", escape(it.ShortVersion))
	}
	if it.MinimumSystemVersion != "" {
		line("<sparkle:minimumSystemVersion>%s</sparkle:minimumSystemVersion>", escape(it.MinimumSystemVersion))
	}
	if it.ReleaseNotes != "" {
		line("<sparkle:releaseNotesLink>%s</sparkle:releaseNotesLink>", escape(it.ReleaseNotes))
	}
	line(`<enclosure url="%s" length="%d" type="application/octet-stream" sparkle:edSignature="%s"/>`, escape(it.URL), it.Length, escape(it.Signature))
	b.WriteString("    </item>\n")
	return b.String()
}

func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
