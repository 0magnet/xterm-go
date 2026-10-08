package xterm

import "testing"

// TestXParseColor: a color query is answered in XParseColor's 16-bit form.
func TestXParseColor(t *testing.T) {
	for rgb, want := range map[uint32]string{
		0x000000: "rgb:0000/0000/0000",
		0xff8000: "rgb:ffff/8080/0000",
		0x2e3436: "rgb:2e2e/3434/3636",
	} {
		if got := xParseColor(rgb); got != want {
			t.Errorf("%06x: %s, want %s", rgb, got, want)
		}
	}
}
