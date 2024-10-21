package alias

import "testing"

func TestGetVolumeName(t *testing.T) {
	vn, err := Create("/Users/ironpark/Documents/Project/Personal/zapp/aa/dmg")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(vn)
}
