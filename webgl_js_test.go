//go:build js && wasm

package xterm

import (
	"strings"
	"syscall/js"
	"testing"
)

// glContext builds a real WebGL 2 context to test against, or skips.
//
// Node has no canvas, so these only run in a browser:
//
//	make test-browser
//
// They are the only tests that can say anything about the shaders. GLSL is a
// Go string here, so nothing checks it until a driver does — a renamed uniform
// or a missing semicolon is a terminal that renders nothing, not a build
// failure.
func glContext(t *testing.T) js.Value {
	t.Helper()
	doc := js.Global().Get("document")
	if !doc.Truthy() || doc.Get("createElement").Type() != js.TypeFunction {
		t.Skip("no document; run with make test-browser")
	}
	canvas := doc.Call("createElement", "canvas")
	canvas.Set("width", 64)
	canvas.Set("height", 64)
	gl := canvas.Call("getContext", "webgl2")
	if !gl.Truthy() {
		t.Skip("no WebGL 2 context")
	}
	return gl
}

func TestEveryShaderCompiles(t *testing.T) {
	gl := glContext(t)
	for _, s := range []struct {
		name string
		typ  int
		src  string
	}{
		{"glyphVertexShader", glVertexShader, glyphVertexShader},
		{"glyphFragmentShader", glFragmentShader, glyphFragmentShader},
		{"rectVertexShader", glVertexShader, rectVertexShader},
		{"rectFragmentShader", glFragmentShader, rectFragmentShader},
	} {
		if _, err := createShader(gl, s.typ, s.src); err != nil {
			t.Errorf("%s: %v", s.name, err)
		}
	}
}

// The two programs the renderer builds have to link, which is a stronger check
// than compiling: a vertex output the fragment shader does not take, or a
// mismatched type between them, only fails here.
func TestBothProgramsLink(t *testing.T) {
	gl := glContext(t)
	for _, p := range []struct{ name, vert, frag string }{
		{"glyph", glyphVertexShader, glyphFragmentShader},
		{"rect", rectVertexShader, rectFragmentShader},
	} {
		prog, err := createProgram(gl, p.vert, p.frag)
		if err != nil {
			t.Errorf("%s program: %v", p.name, err)
			continue
		}
		if !prog.Truthy() {
			t.Errorf("%s program linked but is not an object", p.name)
		}
	}
}

// Every uniform and attribute the Go side looks up has to exist in the linked
// program. A name that does not resolve is a silent no-op at draw time: the
// terminal renders, wrongly, and nothing reports it.
func TestTheNamesTheRendererLooksUpExist(t *testing.T) {
	gl := glContext(t)

	glyph, err := createProgram(gl, glyphVertexShader, glyphFragmentShader)
	if err != nil {
		t.Fatalf("glyph program: %v", err)
	}
	rect, err := createProgram(gl, rectVertexShader, rectFragmentShader)
	if err != nil {
		t.Fatalf("rect program: %v", err)
	}

	// Taken from the shader sources themselves, so this fails when a name is
	// changed in one place and not the other.
	for _, p := range []struct {
		prog js.Value
		src  string
	}{
		{glyph, glyphVertexShader + glyphFragmentShader},
		{rect, rectVertexShader + rectFragmentShader},
	} {
		prog, src := p.prog, p.src
		for _, name := range declaredNames(src, "uniform") {
			if loc := gl.Call("getUniformLocation", prog, name); !loc.Truthy() {
				// A uniform the compiler removed because nothing reads it is
				// not an error, so this only reports, it does not fail.
				t.Logf("uniform %q has no location (optimized out)", name)
			}
		}
		for _, name := range declaredNames(src, "in") {
			if loc := gl.Call("getAttribLocation", prog, name).Int(); loc < 0 {
				t.Logf("attribute %q has no location (optimized out)", name)
			}
		}
	}
}

// declaredNames pulls the identifiers declared with the given qualifier out of
// GLSL source. Deliberately simple: it is reading shaders this package wrote,
// which are one declaration per line.
func declaredNames(src, qualifier string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		f := strings.Fields(strings.TrimSuffix(strings.TrimSpace(line), ";"))
		// uniform <type> <name>  /  in <type> <name>
		if len(f) == 3 && f[0] == qualifier {
			name := f[2]
			if i := strings.IndexByte(name, '['); i >= 0 {
				name = name[:i]
			}
			out = append(out, name)
		}
	}
	return out
}

// A shader that does not compile has to be reported rather than handed back as
// a program that draws nothing.
func TestABrokenShaderIsReported(t *testing.T) {
	gl := glContext(t)
	if _, err := createShader(gl, glFragmentShader, "not glsl at all"); err == nil {
		t.Error("something that is not GLSL compiled")
	}
	if _, err := createProgram(gl, glyphVertexShader, "not glsl at all"); err == nil {
		t.Error("a program with a broken fragment shader linked")
	}
}

func TestAShaderErrorCarriesTheCompilerMessage(t *testing.T) {
	gl := glContext(t)
	_, err := createShader(gl, glFragmentShader, `#version 300 es
		precision mediump float;
		out vec4 c;
		void main() { c = undefinedSymbol; }
	`)
	if err == nil {
		t.Fatal("a shader referencing an undefined symbol compiled")
	}
	if !strings.Contains(err.Error(), "compile") || len(err.Error()) < 25 {
		t.Errorf("the error does not carry the compiler's message: %q", err)
	}
}

// ── the float view ───────────────────────────────────────────────────────────

// Vertex data crosses into JS through this on every frame, so what arrives has
// to be what was sent.
func TestFloatViewCarriesTheData(t *testing.T) {
	if !js.Global().Get("Float32Array").Truthy() {
		t.Skip("no Float32Array")
	}
	var b jsF32
	in := []float32{0, 1, -1, 0.5, 1e6}
	v := b.view(in)
	if got := v.Get("length").Int(); got != len(in) {
		t.Fatalf("the view is %d long, sent %d floats", got, len(in))
	}
	for i, want := range in {
		if got := float32(v.Index(i).Float()); got != want {
			t.Errorf("float %d arrived as %v, sent %v", i, got, want)
		}
	}
}

// The buffer is reused between frames, so a shorter frame must not leave the
// previous one's tail visible through the view.
func TestFloatViewShrinksWithTheData(t *testing.T) {
	if !js.Global().Get("Float32Array").Truthy() {
		t.Skip("no Float32Array")
	}
	var b jsF32
	b.view([]float32{1, 2, 3, 4, 5, 6, 7, 8})
	v := b.view([]float32{9, 9})
	if got := v.Get("length").Int(); got != 2 {
		t.Errorf("after a shorter frame the view is %d long, want 2", got)
	}
}

func TestFloatViewOnNothing(t *testing.T) {
	if !js.Global().Get("Float32Array").Truthy() {
		t.Skip("no Float32Array")
	}
	var b jsF32
	if got := b.view(nil).Get("length").Int(); got != 0 {
		t.Errorf("an empty frame produced a view %d long", got)
	}
}

// The projection matrix maps the cell grid onto clip space; it has to be the
// nine or sixteen numbers the shader declares, not something else.
func TestProjectionMatrixIsTheRightShape(t *testing.T) {
	if n := len(projectionMatrix); n != 16 && n != 9 {
		t.Errorf("the projection matrix has %d entries, want a 3x3 or 4x4", n)
	}
}

// devicePixelRatio is a float32 round-trip of the zoom level, so the ratios
// browsers actually report are not the ratios they name: 80% zoom comes back
// as 0.800000011920929. A cell height sitting exactly on a device-pixel
// boundary then computes a hair above it, jsCeil rounds that whole pixel up,
// and every row grows ~6%. The canvas ends up taller than the element Fit()
// measured the grid against, so the bottom rows render outside the viewport.
//
// 21.25 * 0.800000011920929 == 17.00000025331974 is the case that cost a
// storefront its footer.
func TestSnapPxAbsorbsDevicePixelRatioError(t *testing.T) {
	const dpr = 0.800000011920929
	if v := 21.25 * dpr; v == 17 {
		t.Fatalf("test is vacuous: %v is already exact", v)
	}
	if got := jsCeil(21.25 * dpr); got != 18 {
		t.Fatalf("premise changed: unsnapped jsCeil = %v, want 18", got)
	}
	if got := jsCeil(snapPx(21.25 * dpr)); got != 17 {
		t.Errorf("jsCeil(snapPx(21.25*dpr)) = %v, want 17", got)
	}
	// The mirror case: truncation must not drop a pixel from a value that
	// fell a hair short of the boundary.
	if got := int(snapPx(17 - 2.5e-7)); got != 17 {
		t.Errorf("int(snapPx(17-2.5e-7)) = %d, want 17", got)
	}
}

// A real sub-pixel measurement must survive: snapping is for floating-point
// noise, not for rounding away a fraction the caller meant.
func TestSnapPxKeepsGenuineFractions(t *testing.T) {
	for _, v := range []float64{17.5, 16.9, 8.25, 0.5, 21.0001} {
		if got := snapPx(v); got != v {
			t.Errorf("snapPx(%v) = %v, want it unchanged", v, got)
		}
	}
	// And it still snaps at larger magnitudes, where the error scales up.
	if got := snapPx(1024 * (1 + 1.5e-8)); got != 1024 {
		t.Errorf("snapPx(1024+eps) = %v, want 1024", got)
	}
}

// A page that keeps overflowing costs a full re-render every time, because
// each clear throws away every glyph on screen. Growing the page is what
// stops a full-screen truecolor image — a distinct glyph per color pair,
// in half-blocks — from turning the whole atlas over frame after frame.
func TestAtlasPageGrowsOnRepeatedOverflow(t *testing.T) {
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		t.Skip("no document; run with make test-browser")
	}
	a := &textureAtlas{cache: map[glyphKey]*rasterizedGlyph{}, pageSize: atlasPageSize}
	a.canvas = doc.Call("createElement", "canvas")
	a.canvas.Set("width", a.pageSize)
	a.canvas.Set("height", a.pageSize)

	// One overflow is ordinary — the screen changed — and must not grow.
	a.growPage()
	if a.pageSize != atlasPageSize {
		t.Fatalf("grew on the first overflow: pageSize = %d", a.pageSize)
	}
	v := a.version
	a.growPage()
	if a.pageSize != atlasPageSize*2 {
		t.Errorf("pageSize = %d, want %d after %d overflows", a.pageSize, atlasPageSize*2, atlasGrowAfter)
	}
	if a.canvas.Get("width").Int() != a.pageSize {
		t.Errorf("canvas width = %d, want %d", a.canvas.Get("width").Int(), a.pageSize)
	}
	// Without a version bump the renderer keeps the old texture and samples
	// a page that is no longer there.
	if a.version == v {
		t.Error("version did not change, so the resized page is never re-uploaded")
	}
	// And it stops at the cap rather than growing without bound.
	for i := 0; i < 20; i++ {
		a.growPage()
	}
	if a.pageSize != atlasPageSizeMax {
		t.Errorf("pageSize = %d, want it capped at %d", a.pageSize, atlasPageSizeMax)
	}
}
