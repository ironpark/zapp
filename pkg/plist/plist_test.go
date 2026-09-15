package plist_test

import (
	"bytes"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/ironpark/zapp/pkg/plist"
)

var sample = map[string]interface{}{
	"CFBundleName":    "Zäpp ✨",
	"CFBundleVersion": "1.0",
	"count":           int64(42),
	"big":             int64(70000),
	"neg":             int64(-7),
	"yes":             true,
	"no":              false,
	"ratio":           1.5,
	"blob":            []byte{0, 1, 2, 3, 250},
	"when":            time.Date(2020, 3, 4, 5, 6, 7, 0, time.UTC),
	"list":            []interface{}{"a", int64(1), map[string]interface{}{"k": "v"}},
	"empty":           map[string]interface{}{},
	"long":            "0123456789012345678901234567890123456789",
}

func TestXMLRoundTrip(t *testing.T) {
	data, err := plist.MarshalXML(sample)
	if err != nil {
		t.Fatal(err)
	}
	got, err := plist.ParseDict(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, sample) {
		t.Fatalf("xml round trip mismatch:\n%#v", got)
	}
	if err := os.WriteFile("/tmp/zapp_xml.plist", data, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("plutil", "-lint", "/tmp/zapp_xml.plist").CombinedOutput(); err != nil {
		t.Fatalf("plutil rejected our XML: %v\n%s", err, out)
	}
}

func TestBinaryRoundTrip(t *testing.T) {
	data, err := plist.MarshalBinary(sample)
	if err != nil {
		t.Fatal(err)
	}
	got, err := plist.ParseDict(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, sample) {
		t.Fatalf("binary round trip mismatch:\n%#v", got)
	}
	if err := os.WriteFile("/tmp/zapp_bin.plist", data, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("plutil", "-lint", "/tmp/zapp_bin.plist").CombinedOutput(); err != nil {
		t.Fatalf("plutil rejected our binary plist: %v\n%s", err, out)
	}
}

// TestAgainstPlutil parses what the system encoder produces, in both formats.
func TestAgainstPlutil(t *testing.T) {
	xmlData, err := plist.MarshalXML(sample)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"xml1", "binary1"} {
		if err := os.WriteFile("/tmp/zapp_src.plist", xmlData, 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("plutil", "-convert", format, "/tmp/zapp_src.plist", "-o", "/tmp/zapp_conv.plist").CombinedOutput()
		if err != nil {
			t.Fatalf("plutil convert %s: %v\n%s", format, err, out)
		}
		conv, err := os.ReadFile("/tmp/zapp_conv.plist")
		if err != nil {
			t.Fatal(err)
		}
		got, err := plist.ParseDict(conv)
		if err != nil {
			t.Fatalf("parse plutil %s output: %v", format, err)
		}
		if !reflect.DeepEqual(got, sample) {
			t.Fatalf("%s mismatch:\n%#v", format, got)
		}
	}
}

// TestSystemInfoPlist reads a real, system-provided binary Info.plist.
func TestSystemInfoPlist(t *testing.T) {
	path := "/System/Applications/Calculator.app/Contents/Info.plist"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skip(err)
	}
	dict, err := plist.ParseDict(raw)
	if err != nil {
		t.Fatal(err)
	}
	if dict["CFBundleIdentifier"] != "com.apple.calculator" {
		t.Fatalf("unexpected bundle id: %v", dict["CFBundleIdentifier"])
	}
	// Compare the whole dictionary against plutil's XML rendering of the file.
	xmlOut, err := exec.Command("plutil", "-convert", "xml1", path, "-o", "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	viaXML, err := plist.ParseDict(xmlOut)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dict, viaXML) {
		t.Fatal("binary and XML readings of Info.plist differ")
	}
}

func TestMalformed(t *testing.T) {
	cases := [][]byte{
		[]byte(""),
		[]byte("<plist/>"),
		[]byte(`<plist version="1.0"><dict><key>a</key></dict></plist>`),
		[]byte(`<plist version="1.0"><dict><string>a</string></dict></plist>`),
		[]byte("bplist00"),
		append([]byte("bplist00"), bytes.Repeat([]byte{0xff}, 32)...),
	}
	for i, c := range cases {
		if _, err := plist.Parse(c); err == nil {
			t.Errorf("case %d: expected an error", i)
		}
	}
}
