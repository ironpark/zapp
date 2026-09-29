package homebrew

import "testing"

func TestRender(t *testing.T) {
	c := Cask{Token: "demo", Version: "2.3", SHA256: "abc123", URL: "https://github.com/me/demo/releases/download/v2.3/Demo.dmg",
		Name: "Demo", Desc: `Says "hi" #{now}`, Homepage: "https://example.com", App: "Demo.app", MinimumMacOS: "12.0", AutoUpdates: true}
	want := `cask "demo" do
  version "2.3"
  sha256 "abc123"

  url "https://github.com/me/demo/releases/download/v2.3/Demo.dmg"
  name "Demo"
  desc "Says \"hi\" \#{now}"
  homepage "https://example.com"

  auto_updates true
  depends_on macos: ">= :monterey"

  app "Demo.app"
end
`
	if got := c.Render(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	c = Cask{Token: "demo", Version: "1.0", SHA256: "abc", URL: "https://x/Demo.pkg", Name: "Demo", Homepage: "https://x", PKG: "Demo.pkg", PKGIDs: []string{"com.example.demo", "com.example.helper"}}
	want = `cask "demo" do
  version "1.0"
  sha256 "abc"

  url "https://x/Demo.pkg"
  name "Demo"
  homepage "https://x"

  pkg "Demo.pkg"

  uninstall pkgutil: ["com.example.demo", "com.example.helper"]
end
`
	if got := c.Render(); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestToken(t *testing.T) {
	for name, want := range map[string]string{"Demo": "demo", "My App.app": "my-app", "Foo+": "foo-plus", "Hello, World!": "hello-world", "Été 2": "t-2"} {
		if got := Token(name); got != want {
			t.Errorf("Token(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestMacOSSymbol(t *testing.T) {
	for version, want := range map[string]string{"12.0": "monterey", "11": "big_sur", "11.2.3": "big_sur", "10.15": "catalina", "10.15.7": "catalina", "26.0": "tahoe", "10.9": "", "": "", "x": ""} {
		if got := MacOSSymbol(version); got != want {
			t.Errorf("MacOSSymbol(%q) = %q, want %q", version, got, want)
		}
	}
}
