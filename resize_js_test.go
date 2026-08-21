//go:build js && wasm

package xterm

import (
	"syscall/js"
	"testing"

	"github.com/0magnet/xterm-go/vt"
)

// The regression guard for a trap that cost a long debugging session in a
// consumer: assigning Core.OnResize looks like the obvious way to hear about a
// resize — Attach assigns Core.OnData, after all — but Open owns that field,
// and its handler is what reallocates the renderer for the new grid. Taking it
// leaves the render model sized for the old grid and the next frame panics,
// which in wasm ends the whole program.
//
// Terminal.OnResize is the safe hook. These tests say it fires, and that it
// cannot displace the handler it is fanned out from.

func openTestTerminal(t *testing.T) *Terminal {
	t.Helper()
	doc := js.Global().Get("document")
	if !doc.Truthy() {
		t.Skip("no document; run with make test-browser")
	}
	host := doc.Call("createElement", "div")
	host.Get("style").Set("width", "640px")
	host.Get("style").Set("height", "480px")
	doc.Get("body").Call("appendChild", host)
	t.Cleanup(func() { host.Call("remove") })

	term := New(vt.NewOptions())
	term.Open(host)
	t.Cleanup(term.Dispose)
	return term
}

func TestOnResizeFires(t *testing.T) {
	term := openTestTerminal(t)

	var gotCols, gotRows, calls int
	term.OnResize = func(cols, rows int) {
		gotCols, gotRows = cols, rows
		calls++
	}

	term.Core.Resize(100, 30)

	if calls == 0 {
		t.Fatal("OnResize never fired")
	}
	if gotCols != 100 || gotRows != 30 {
		t.Errorf("OnResize got %dx%d, want 100x30", gotCols, gotRows)
	}
}

func TestOnResizeDoesNotDisplaceTheRenderer(t *testing.T) {
	// The failure this exists for is not "the hook did not fire" — it is the
	// renderer never hearing about the new grid. So the assertion is that the
	// terminal is CONSISTENT afterwards: the core agrees with what the hook was
	// told, and drawing at the new size does not panic.
	term := openTestTerminal(t)

	var told int
	term.OnResize = func(cols, _ int) { told = cols }

	term.Core.Resize(120, 40)

	if term.Core.Cols() != 120 || term.Core.Rows() != 40 {
		t.Fatalf("core is %dx%d after a resize to 120x40", term.Core.Cols(), term.Core.Rows())
	}
	if told != term.Core.Cols() {
		t.Errorf("the hook was told %d columns and the core has %d", told, term.Core.Cols())
	}

	// Write enough to fill the wider grid and force a render. Before the fix,
	// a consumer that had taken Core.OnResize panicked here rather than at the
	// resize itself, which is what made the cause hard to see.
	term.WriteString("\x1b[2J\x1b[H")
	for i := 0; i < 40; i++ {
		term.WriteString("0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890\r\n")
	}
	term.render()
}

func TestOnResizeIsOptional(t *testing.T) {
	// A nil hook is the common case and must not be called.
	term := openTestTerminal(t)
	term.Core.Resize(90, 25)
	if term.Core.Cols() != 90 {
		t.Errorf("core is %d columns, want 90", term.Core.Cols())
	}
}
