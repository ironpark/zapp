package otool

import (
	"reflect"
	"testing"
)

func TestParseOtoolOutput(t *testing.T) {
	// A dylib lists its own install name first, and a fat binary repeats the
	// whole list once per architecture.
	const output = `libavformat.62.dylib:
	/opt/homebrew/opt/ffmpeg/lib/libavformat.62.dylib (compatibility version 62.0.0, current version 62.12.101)
	/opt/homebrew/Cellar/ffmpeg/8.1.1/lib/libavcodec.62.dylib (compatibility version 62.0.0, current version 62.28.101)
	/usr/lib/libSystem.B.dylib (compatibility version 1.0.0, current version 1356.0.0)
libavformat.62.dylib (architecture arm64):
	/opt/homebrew/opt/ffmpeg/lib/libavformat.62.dylib (compatibility version 62.0.0, current version 62.12.101)
	/opt/homebrew/Cellar/ffmpeg/8.1.1/lib/libavcodec.62.dylib (compatibility version 62.0.0, current version 62.28.101)
`
	got := parseOtoolOutput(output, "/opt/homebrew/opt/ffmpeg/lib/libavformat.62.dylib")
	want := []string{
		"/opt/homebrew/Cellar/ffmpeg/8.1.1/lib/libavcodec.62.dylib",
		"/usr/lib/libSystem.B.dylib",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseOtoolOutput() = %v, want %v", got, want)
	}
}

func TestParseOtoolOutputExecutable(t *testing.T) {
	// An executable has no LC_ID_DYLIB, so nothing must be dropped.
	const output = `Ultrasync:
	@executable_path/../Frameworks/libavformat.62.dylib (compatibility version 62.0.0, current version 62.12.101)
`
	got := parseOtoolOutput(output, "")
	want := []string{"@executable_path/../Frameworks/libavformat.62.dylib"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseOtoolOutput() = %v, want %v", got, want)
	}
}

func TestParseRPaths(t *testing.T) {
	const output = `Load command 12
          cmd LC_LOAD_DYLIB
      cmdsize 56
         name /usr/lib/libSystem.B.dylib (offset 24)
Load command 13
          cmd LC_RPATH
      cmdsize 40
         path @executable_path/../Frameworks (offset 12)
Load command 14
          cmd LC_RPATH
      cmdsize 32
         path @loader_path/. (offset 12)
`
	got := parseRPaths(output)
	want := []string{"@executable_path/../Frameworks", "@loader_path/."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseRPaths() = %v, want %v", got, want)
	}
}

func TestIsSystemLib(t *testing.T) {
	for name, want := range map[string]bool{
		"/usr/lib/libSystem.B.dylib": true,
		"/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation": true,
		"/opt/homebrew/opt/ffmpeg/lib/libavcodec.62.dylib":                              false,
		"@rpath/libavcodec.62.dylib":                                                    false,
		"/usr/local/lib/libfoo.dylib":                                                   false,
	} {
		if got := IsSystemLib(name); got != want {
			t.Errorf("IsSystemLib(%q) = %v, want %v", name, got, want)
		}
	}
}
