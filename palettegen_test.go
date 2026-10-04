package xterm

import (
	"math"
	"testing"

	"github.com/0magnet/xterm-go/vt"
)

var genTheme = vt.Theme{
	Background: "#1d1f21", Foreground: "#c5c8c6",
	Red: "#cc6666", Green: "#b5bd68", Yellow: "#f0c674",
	Blue: "#81a2be", Magenta: "#b294bb", Cyan: "#8abeb7",
}

func mustRGB(t *testing.T, css string) [3]int {
	t.Helper()
	r, g, b, _, ok := parseRGBA(css)
	if !ok {
		t.Fatalf("unreadable color %q", css)
	}
	return [3]int{r, g, b}
}

// near allows the one-step rounding a CIELAB round trip can introduce.
func near(a, b [3]int) bool {
	const tol = 1
	for i := range a {
		if d := a[i] - b[i]; d > tol || d < -tol {
			return false
		}
	}
	return true
}

func TestRGBToLabKnownValues(t *testing.T) {
	for _, tc := range []struct {
		in   [3]int
		want lab
	}{
		{[3]int{255, 255, 255}, lab{100, 0, 0}},
		{[3]int{0, 0, 0}, lab{0, 0, 0}},
		{[3]int{255, 0, 0}, lab{53.24, 80.09, 67.20}},
		{[3]int{0, 0, 255}, lab{32.30, 79.19, -107.86}},
	} {
		got := rgbToLab(tc.in)
		for i := range got {
			if math.Abs(got[i]-tc.want[i]) > 0.05 {
				t.Errorf("rgbToLab(%v) = %v, want %v", tc.in, got, tc.want)
				break
			}
		}
		if back := labToRGB(got); back != tc.in {
			t.Errorf("round trip %v -> %v", tc.in, back)
		}
	}
}

// TestGenerate256Off: without the option the palette is xterm's.
func TestGenerate256Off(t *testing.T) {
	p := BuildPalette(genTheme)
	if p[16] != "#000000" || p[196] != "#ff0000" || p[231] != "#ffffff" ||
		p[232] != "#080808" || p[255] != "#eeeeee" {
		t.Errorf("standard palette changed: %s %s %s %s %s", p[16], p[196], p[231], p[232], p[255])
	}
}

// TestGenerate256Cube checks every cube entry against the proposal's formula,
// written here as the explicit trilinear weighting of the eight corners.
func TestGenerate256Cube(t *testing.T) {
	th := genTheme
	th.Generate256 = true
	p := BuildPalette(th)

	css := [8]string{th.Background, th.Red, th.Green, th.Yellow, th.Blue, th.Magenta, th.Cyan, th.Foreground}
	var c [8]lab
	for i, s := range css {
		c[i] = rgbToLab(mustRGB(t, s))
	}
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				x, y, z := float64(r)/5, float64(g)/5, float64(b)/5
				var want lab
				for k := 0; k < 8; k++ {
					// corner k: bit0 = r, bit1 = g, bit2 = b
					w := 1.0
					w *= map[bool]float64{true: x, false: 1 - x}[k&1 != 0]
					w *= map[bool]float64{true: y, false: 1 - y}[k&2 != 0]
					w *= map[bool]float64{true: z, false: 1 - z}[k&4 != 0]
					for i := range want {
						want[i] += w * c[k][i]
					}
				}
				idx := 16 + 36*r + 6*g + b
				if got := mustRGB(t, p[idx]); !near(got, labToRGB(want)) {
					t.Errorf("p[%d] = %s, want %s", idx, p[idx], rgbCSS(labToRGB(want)))
				}
			}
		}
	}
	// the corners are the theme colors themselves
	for idx, s := range map[int]string{16: th.Background, 196: th.Red, 46: th.Green, 226: th.Yellow,
		21: th.Blue, 201: th.Magenta, 51: th.Cyan, 231: th.Foreground} {
		if !near(mustRGB(t, p[idx]), mustRGB(t, s)) {
			t.Errorf("corner p[%d] = %s, want %s", idx, p[idx], s)
		}
	}
	// the base 16 are untouched
	if p[1] != th.Red || p[0] != defaultAnsi16[0] {
		t.Errorf("base colors changed: %s %s", p[0], p[1])
	}
}

func TestGenerate256Greys(t *testing.T) {
	th := genTheme
	th.Generate256 = true
	p := BuildPalette(th)
	bg, fg := rgbToLab(mustRGB(t, th.Background)), rgbToLab(mustRGB(t, th.Foreground))
	for i := 0; i < 24; i++ {
		want := labToRGB(lerpLab(float64(i+1)/25, bg, fg))
		if got := mustRGB(t, p[232+i]); !near(got, want) {
			t.Errorf("p[%d] = %s, want %s", 232+i, p[232+i], rgbCSS(want))
		}
	}
	// default black/white theme: the ramp is neutral and rising
	p = BuildPalette(vt.Theme{Generate256: true})
	prev := -1
	for i := 232; i < 256; i++ {
		c := mustRGB(t, p[i])
		if !near(c, [3]int{c[0], c[0], c[0]}) || c[0] <= prev {
			t.Errorf("p[%d] = %s is not a rising grey", i, p[i])
		}
		prev = c[0]
	}
}

// TestGenerate256LightTheme: a dark-on-light theme keeps index 16 the dark end.
func TestGenerate256LightTheme(t *testing.T) {
	p := BuildPalette(vt.Theme{Background: "#ffffff", Foreground: "#000000", Generate256: true})
	if p[16] != "#000000" || p[231] != "#ffffff" {
		t.Errorf("light theme corners: 16=%s 231=%s", p[16], p[231])
	}
}

// TestGenerate256Unreadable: a color the parser cannot read leaves the
// standard palette in place rather than a half-generated one.
func TestGenerate256Unreadable(t *testing.T) {
	p := BuildPalette(vt.Theme{Background: "transparent", Generate256: true})
	if p[16] != "#000000" || p[255] != "#eeeeee" {
		t.Errorf("got %s %s", p[16], p[255])
	}
}
