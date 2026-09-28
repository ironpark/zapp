package sparkle

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testSeed = base64.StdEncoding.EncodeToString(make([]byte, ed25519.SeedSize))

func TestKeySignsFile(t *testing.T) {
	k, err := ParseKey(testSeed + "\n")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "App.zip")
	if err := os.WriteFile(path, []byte("update"), 0o644); err != nil {
		t.Fatal(err)
	}
	length, sig, err := k.SignFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(sig)
	pub, _ := base64.StdEncoding.DecodeString(k.PublicKey())
	if length != 6 || !ed25519.Verify(pub, []byte("update"), raw) {
		t.Fatalf("length %d, signature does not verify", length)
	}
	for _, bad := range []string{"not base64!", base64.StdEncoding.EncodeToString(make([]byte, 64))} {
		if _, err := ParseKey(bad); err == nil {
			t.Errorf("ParseKey(%q) passed", bad)
		}
	}
}

func item(version string) Item {
	return Item{Version: version, ShortVersion: "1." + version, MinimumSystemVersion: "11.0", ReleaseNotes: "https://example.com/notes?v=" + version + "&x=1",
		Published: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC), URL: "https://example.com/App.zip", Length: 42, Signature: "c2ln"}
}

// parse decodes the fields a Sparkle client reads.
func parse(t *testing.T, feed []byte) []string {
	t.Helper()
	var rss struct {
		Channel struct {
			Title string `xml:"title"`
			Items []struct {
				Version   string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle version"`
				Enclosure struct {
					URL       string `xml:"url,attr"`
					Signature string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle edSignature,attr"`
				} `xml:"enclosure"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(feed, &rss); err != nil {
		t.Fatalf("%v\n%s", err, feed)
	}
	var versions []string
	for _, it := range rss.Channel.Items {
		if it.Enclosure.URL == "" || it.Enclosure.Signature == "" {
			t.Fatalf("item %s lacks its enclosure:\n%s", it.Version, feed)
		}
		versions = append(versions, it.Version)
	}
	return versions
}

func TestAdd(t *testing.T) {
	feed, err := Add(nil, "Demo & Co", item("1"))
	if err != nil {
		t.Fatal(err)
	}
	if got := parse(t, feed); strings.Join(got, ",") != "1" || !strings.Contains(string(feed), "Demo &amp; Co") {
		t.Fatalf("new feed:\n%s", feed)
	}
	if feed, err = Add(feed, "", item("2")); err != nil {
		t.Fatal(err)
	}
	if got := parse(t, feed); strings.Join(got, ",") != "2,1" {
		t.Fatalf("versions %v:\n%s", got, feed)
	}
	// Publishing a version again replaces its item.
	again := item("1")
	again.Signature = "bmV3"
	if feed, err = Add(feed, "", again); err != nil {
		t.Fatal(err)
	}
	if got := parse(t, feed); strings.Join(got, ",") != "1,2" || strings.Count(string(feed), `edSignature="c2ln"`) != 1 || !strings.Contains(string(feed), `edSignature="bmV3"`) {
		t.Fatalf("versions %v:\n%s", got, feed)
	}
}

// An older appcast keeps what zapp does not write, and its version may be an
// enclosure attribute.
func TestAddKeepsExistingFeed(t *testing.T) {
	old := `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <title>Demo</title>
    <link>https://example.com</link>
    <item>
      <title>Old</title>
      <enclosure url="https://example.com/old.zip" sparkle:version="1" sparkle:edSignature="b2xk" length="1" type="application/octet-stream"/>
    </item>
    <item>
      <title>Older</title>
      <enclosure url="https://example.com/older.zip" sparkle:version="0" sparkle:edSignature="b2xk" length="1" type="application/octet-stream"/>
    </item>
  </channel>
</rss>
`
	feed, err := Add([]byte(old), "", item("1"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(feed)
	if strings.Contains(s, "old.zip") || !strings.Contains(s, "older.zip") || !strings.Contains(s, "<link>https://example.com</link>") || !strings.Contains(s, "xmlns:dc") {
		t.Fatalf("merged feed:\n%s", s)
	}
	if !strings.Contains(s, "    <link>https://example.com</link>\n    <item>\n      <title>1.1</title>") {
		t.Fatalf("new item is not first, at the items' indentation:\n%s", s)
	}
	for _, bad := range []string{"<rss><channel>", "<rss><channel></channel></rss>", `<rss xmlns:sparkle="` + Namespace + `"></rss>`} {
		if _, err := Add([]byte(bad), "", item("1")); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}
