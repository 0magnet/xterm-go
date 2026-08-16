package xterm

import (
	"testing"

	"github.com/0magnet/xterm-go/vt"
)

func TestParseRGBA(t *testing.T) {
	for _, tc := range []struct {
		in      string
		r, g, b int
		alpha   float64
		ok      bool
	}{
		{in: "rgba(255, 255, 255, 0.3)", r: 255, g: 255, b: 255, alpha: 0.3, ok: true},
		{in: "rgb(1,2,3)", r: 1, g: 2, b: 3, alpha: 1, ok: true},
		{in: "#ffffff4d", r: 255, g: 255, b: 255, alpha: 0x4d / 255.0, ok: true},
		{in: "#102030", r: 0x10, g: 0x20, b: 0x30, alpha: 1, ok: true},
		{in: "not a color", ok: false},
	} {
		r, g, b, alpha, ok := parseRGBA(tc.in)
		if ok != tc.ok {
			t.Errorf("parseRGBA(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if !ok {
			continue
		}
		if r != tc.r || g != tc.g || b != tc.b || alpha-tc.alpha > 0.001 || tc.alpha-alpha > 0.001 {
			t.Errorf("parseRGBA(%q) = %d,%d,%d,%v; want %d,%d,%d,%v",
				tc.in, r, g, b, alpha, tc.r, tc.g, tc.b, tc.alpha)
		}
	}
}

// TestCompositeOver: neither renderer can draw a translucent selection, so the
// blend has to happen here. 30% white over black is a dark grey.
func TestCompositeOver(t *testing.T) {
	if got := compositeOver("rgba(255, 255, 255, 0.3)", "#000000"); got != "#4d4d4d" {
		t.Errorf("got %q, want %q", got, "#4d4d4d")
	}
	if got := compositeOver("rgba(0, 0, 0, 0.5)", "#ffffff"); got != "#808080" {
		t.Errorf("got %q, want %q", got, "#808080")
	}
	// an opaque selection color is used as it stands
	if got := compositeOver("#123456", "#000000"); got != "#123456" {
		t.Errorf("got %q, want %q", got, "#123456")
	}
}

// TestDefaultSelectionIsVisibleOnTheDefaultBackground guards the case that
// actually reaches a user: the theme names no selection color, so the default
// translucent white has to end up as something that is neither the background
// nor the foreground.
func TestDefaultSelectionIsVisibleOnTheDefaultBackground(t *testing.T) {
	cs := NewColorSet(vt.Theme{Background: "#0c0c0c", Foreground: "#e6e6e6"})
	if cs.SelectionBgOpaque == cs.Background {
		t.Errorf("the selection is the same color as the background: %q", cs.SelectionBgOpaque)
	}
	if cs.SelectionBgOpaque == cs.Foreground {
		t.Errorf("the selection is the same color as the text: %q", cs.SelectionBgOpaque)
	}
	if _, _, _, _, ok := parseRGBA(cs.SelectionBgOpaque); !ok {
		t.Errorf("the blended selection color is unreadable: %q", cs.SelectionBgOpaque)
	}
}

// TestSelectionForegroundIsOptional — leaving it unset is what keeps colored
// output readable through a selection.
func TestSelectionForegroundIsOptional(t *testing.T) {
	if fg := NewColorSet(vt.Theme{}).SelectionFg; fg != "" {
		t.Errorf("an unset selection foreground resolved to %q", fg)
	}
	if fg := NewColorSet(vt.Theme{SelectionForeground: "#ff0000"}).SelectionFg; fg != "#ff0000" {
		t.Errorf("got %q, want %q", fg, "#ff0000")
	}
}
