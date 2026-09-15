package security

import "testing"

func TestSecureStringShortOrMissingID(t *testing.T) {
	// A description that does not match descRegexp leaves DeveloperID empty,
	// and some identities carry IDs shorter than the visible prefix.
	cases := []struct {
		name string
		id   string
		want string
	}{
		{"empty", "", ""},
		{"shorter than prefix", "AB", "**"},
		{"exactly prefix", "ABCDE", "*****"},
		{"longer than prefix", "ABCDEFGHIJ", "ABCDE*****"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskID(tc.id); got != tc.want {
				t.Errorf("maskID(%q) = %q, want %q", tc.id, got, tc.want)
			}
			// Must not panic.
			Identity{Type: "Developer ID Application", DeveloperName: "Jane Doe", DeveloperID: tc.id}.SecureString()
		})
	}
}

func TestParseFindIdentityOutput(t *testing.T) {
	const output = `  1) A1B2C3D4E5F60718293A4B5C6D7E8F9012345678 "Developer ID Application: Jane Doe (ABCDE12345)"
  2) 00112233445566778899AABBCCDDEEFF00112233 "unparseable description"
     2 valid identities found`

	got, err := parseFindIdentityOutput(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d identities, want 2", len(got))
	}
	if got[0].Type != "Developer ID Application" || got[0].DeveloperName != "Jane Doe" || got[0].DeveloperID != "ABCDE12345" {
		t.Errorf("identity 0 parsed as %+v", got[0])
	}
	// The unparseable entry keeps only the raw description; SecureString must
	// still be safe on it.
	if got[1].DeveloperID != "" {
		t.Errorf("identity 1 DeveloperID = %q, want empty", got[1].DeveloperID)
	}
	got[1].SecureString()
}
