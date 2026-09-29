package notarylog

import "testing"

func TestSummary(t *testing.T) {
	log := `{"logFormatVersion":1,"jobId":"2efe2717","status":"Invalid","statusSummary":"Archive contains critical validation errors","statusCode":4000,"archiveFilename":"Demo.zip","issues":[
{"severity":"error","code":null,"path":"Demo.zip/Demo.app/Contents/MacOS/Demo","message":"The binary is not signed with a valid Developer ID certificate.","docUrl":"https://developer.apple.com/a","architecture":"x86_64"},
{"severity":"error","code":null,"path":"Demo.zip/Demo.app/Contents/MacOS/Demo","message":"The binary is not signed with a valid Developer ID certificate.","docUrl":"https://developer.apple.com/a","architecture":"arm64"},
{"severity":"warning","code":null,"path":"Demo.zip/Demo.app/Contents/Frameworks/libx.dylib","message":"The executable does not have the hardened runtime enabled.","docUrl":null,"architecture":"arm64"}]}`
	want := "Archive contains critical validation errors\n" +
		"  Demo.app/Contents/MacOS/Demo (x86_64, arm64): The binary is not signed with a valid Developer ID certificate.\n" +
		"    https://developer.apple.com/a\n" +
		"  warning: Demo.app/Contents/Frameworks/libx.dylib (arm64): The executable does not have the hardened runtime enabled."
	if got := Summary([]byte(log)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	for _, bad := range []string{"", "not json", `{"error":"no log"}`} {
		if got := Summary([]byte(bad)); got != "" {
			t.Errorf("Summary(%q) = %q", bad, got)
		}
	}
}
