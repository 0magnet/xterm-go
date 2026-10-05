//go:build js && wasm

package xterm

import (
	"syscall/js"
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

func TestCompletelyTransparent(t *testing.T) {
	pix := make([]byte, 4*4)
	for i := range pix {
		if i%4 != 3 {
			pix[i] = 0xff // color without coverage is still nothing
		}
	}
	if !completelyTransparent(pix) {
		t.Error("zero-alpha pixels reported as drawn")
	}
	pix[7] = 1
	if completelyTransparent(pix) {
		t.Error("a pixel with alpha reported as empty")
	}
}

// With allowTransparency a glyph is rasterized on a transparent background:
// its antialiased edges carry partial alpha in the text color instead of a
// blend with the (translucent) theme background baked in at full alpha.
func TestTransparentAtlasLeavesBackgroundOut(t *testing.T) {
	a := atlasForTest(t, 10, 20)
	a.dispose()
	cfg := a.cfg
	cfg.allowTransparency = true
	cfg.colors = NewColorSet(vt.Theme{Background: "#00000080", Foreground: "#ffffff"})
	a = newTextureAtlas(cfg)
	defer a.dispose()

	g := a.getRasterizedGlyph('M', 0, 0, 0)
	if g == nullRasterizedGlyph {
		t.Fatal("M rasterized empty")
	}
	data := a.ctx.Call("getImageData", g.texPosX, g.texPosY, g.sizeX, g.sizeY).Get("data")
	pix := make([]byte, data.Get("length").Int())
	js.CopyBytesToGo(pix, data)
	partial := 0
	for off := 0; off+3 < len(pix); off += 4 {
		al := pix[off+3]
		if al == 0 {
			continue
		}
		if al < 255 {
			partial++
		}
		// 2D canvases store premultiplied color, so a faint pixel reads back
		// with a little rounding; a baked-in black background would read as
		// dark grey at full alpha instead.
		if al > 64 && (pix[off] < 200 || pix[off+1] < 200 || pix[off+2] < 200) {
			t.Fatalf("pixel %d is rgba(%d,%d,%d,%d): background baked in", off/4, pix[off], pix[off+1], pix[off+2], al)
		}
	}
	if partial == 0 {
		t.Error("no partially transparent edge pixels")
	}
}
