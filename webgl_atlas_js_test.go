//go:build js && wasm

package xterm

import (
	"testing"

	"github.com/0magnet/xterm-go/vt"
)

// atlasForTest builds an atlas over a real 2D canvas, or skips.
//
// Node has no canvas, so this only runs in a browser:
//
//	make test-browser
func atlasForTest(t *testing.T, cellW, cellH int) *textureAtlas {
	t.Helper()
	doc := document
	if !doc.Truthy() || doc.Get("createElement").Type().String() != "function" {
		t.Skip("no document; run with make test-browser")
	}
	probe := doc.Call("createElement", "canvas")
	if !probe.Call("getContext", "2d").Truthy() {
		t.Skip("no 2d canvas context; run with make test-browser")
	}
	probe.Call("remove")
	return newTextureAtlas(atlasConfig{
		deviceCellWidth:  cellW,
		deviceCellHeight: cellH,
		deviceCharWidth:  cellW,
		deviceCharHeight: cellH,
		fontSize:         float64(cellH) / 2,
		fontFamily:       "monospace",
		dpr:              1,
		lineHeight:       1,
		colors:           NewColorSet(vt.Theme{}),
	})
}

// TestCombinedCharDoesNotOverrunTmpCanvas rasterizes combined characters long
// enough to grow the temp canvas to exactly allowedWidth.
//
// At that size the bounding-box scan's x bound, padding+width, lies past the
// last pixel read back. Upstream is in JavaScript, where the overrun reads
// undefined out of the Uint8ClampedArray and merely reports a glyph a few
// pixels too wide; Go indexes the same bytes and panics, so the first ZWJ
// emoji on screen took the whole renderer down with it — the tcell unicode
// demo rendered a blank terminal for exactly this reason.
//
// The rune counts here are the ones that matter: two fits the canvas the atlas
// starts with, three is the first that grows it, and the flag and family
// sequences are what real text actually contains.
func TestCombinedCharDoesNotOverrunTmpCanvas(t *testing.T) {
	a := atlasForTest(t, 9, 20)
	defer a.dispose()

	for _, chars := range []string{
		"e\u0301",                          // 2 runes, fits the canvas as created
		"a\u0301\u0302",                    // 3 runes, the first size that grows it
		"\U0001F3F3\uFE0F\u200D\U0001F308", // rainbow flag, 4
		"\U0001F468\u200D\U0001F469\u200D\U0001F467\u200D\U0001F466", // family, 7
	} {
		g := a.getRasterizedGlyphCombinedChar(chars, 0, 0xffffff, 0)
		if g == nil {
			t.Fatalf("%q: nil glyph", chars)
		}
		// A glyph may legitimately rasterize empty, but a non-empty one must
		// not claim more width than the canvas it was measured on.
		if w := a.tmpCanvas.Get("width").Int(); int(g.sizeX) > w {
			t.Errorf("%q: sizeX %v exceeds temp canvas width %d", chars, g.sizeX, w)
		}
	}
}
