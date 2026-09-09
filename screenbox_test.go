package xterm

import (
	"math"
	"testing"
)

// The canvas must cover the screen exactly — no more, no less.
//
// The bug this guards was 41px of WebGL canvas hanging below the terminal,
// which put a scrollbar on a winbox window whose content fitted. It happened
// because the canvas box was reconstructed from the ceil-ed device buffer
// (rows * ceil(cellH*dpr) / dpr) instead of from the cell size, and that round
// trip does not come back to where it started at a fractional pixel ratio.
func TestScreenBoxIsExactlyCellTimesCount(t *testing.T) {
	for _, tc := range []struct {
		name         string
		cellW, cellH float64
		cols, rows   int
	}{
		{"integral cells", 8, 16, 80, 24},
		{"the reported case", 9.6, 19.6923, 193, 46},
		{"fractional height", 7.2, 12.5, 100, 30},
		{"one cell", 10.5, 21.25, 1, 1},
		{"no rows", 8, 16, 80, 0},
	} {
		w, h := screenBoxPx(tc.cellW, tc.cellH, tc.cols, tc.rows)
		if want := tc.cellW * float64(tc.cols); w != want {
			t.Errorf("%s: width %v, want %v", tc.name, w, want)
		}
		if want := tc.cellH * float64(tc.rows); h != want {
			t.Errorf("%s: height %v, want %v", tc.name, h, want)
		}
	}
}

// The old computation, kept as the thing NOT to do, so the test says what the
// regression would look like rather than only that the new code agrees with
// itself. A fractional dpr is the case that breaks it; at dpr 1 the two agree,
// which is why this went unnoticed on an ordinary display.
func TestDeviceRoundTripInflatesTheBox(t *testing.T) {
	const cellH, rows = 19.6957, 46
	exact := cellH * float64(rows)

	roundTrip := func(dpr float64) float64 {
		deviceCell := math.Ceil(cellH * dpr)
		return math.Round(float64(rows) * deviceCell / dpr)
	}
	// It inflates at an ORDINARY pixel ratio too, which is the part worth
	// knowing: ceil of a fractional cell height adds up to a pixel per row
	// whatever the display is, so this was never a fractional-dpr curiosity.
	if got := roundTrip(1); got <= exact {
		t.Fatalf("expected inflation at dpr 1: got %v, exact %v", got, exact)
	} else {
		t.Logf("dpr 1:      round trip %v vs exact %v — %.0f px over", got, exact, got-exact)
	}
	// The display this was found on. The old code produced a canvas ~45px
	// taller than the screen here; the shared expression cannot.
	got := roundTrip(0.8125)
	if got <= exact+10 {
		t.Fatalf("expected the round trip to inflate the box at dpr 0.8125: got %v, exact %v", got, exact)
	}
	t.Logf("dpr 0.8125: round trip %v vs exact %v — %v px of canvas past the screen",
		got, exact, got-exact)
	w, h := screenBoxPx(9.6, cellH, 193, rows)
	if h != exact {
		t.Errorf("screenBoxPx must not inflate: got %v, want %v", h, exact)
	}
	if w != 9.6*193 {
		t.Errorf("width: got %v, want %v", w, 9.6*193)
	}
}
