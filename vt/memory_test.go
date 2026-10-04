package vt

import (
	"strings"
	"testing"
)

// TestScrollingDoesNotAllocatePerLine: once the scrollback is full, a line
// leaving the top is recycled for the new one, and plain text needs neither
// of a line's maps, so scrolling allocates nothing per line. Before the maps
// were made lazy, Fill and CopyFrom allocated two maps for every line.
func TestScrollingDoesNotAllocatePerLine(t *testing.T) {
	term := newTestTerminal(80, 24)
	chunk := []byte(strings.Repeat(strings.Repeat("x", 79)+"\r\n", 100))
	for i := 0; i < 11; i++ {
		term.Write(chunk) // fill the scrollback
	}
	if allocs := testing.AllocsPerRun(20, func() { term.Write(chunk) }); allocs > 10 {
		t.Errorf("%v allocations to scroll 100 lines", allocs)
	}
}

// TestPlainLinesHaveNoMaps: the per-line maps exist only once a cell needs
// one.
func TestPlainLinesHaveNoMaps(t *testing.T) {
	term := newTestTerminal(20, 3)
	term.WriteString("plain\r\n\x1b[4:3mcurly\x1b[0m é\r\n\x1b[2J")
	b := term.Buffer()
	l := b.Lines.Get(b.YBase)
	if l.combined != nil || l.extendedAttrs != nil {
		t.Errorf("an erased line kept its maps")
	}

	term = newTestTerminal(20, 3)
	term.WriteString("plain\r\n\x1b[4:3mcurly\x1b[0m é")
	b = term.Buffer()
	if l := b.Lines.Get(b.YBase); l.combined != nil || l.extendedAttrs != nil {
		t.Errorf("a plain line has maps")
	}
	l = b.Lines.Get(b.YBase + 1)
	if len(l.extendedAttrs) == 0 || len(l.combined) != 1 {
		t.Errorf("line 2: %d extended, %d combined", len(l.extendedAttrs), len(l.combined))
	}
	if got := l.TranslateToString(true, 0, -1); got != "curly é" {
		t.Errorf("line 2 = %q", got)
	}
	// a copy keeps them and an empty source copies as nil
	if c := l.Clone(); len(c.combined) != 1 || len(c.extendedAttrs) == 0 {
		t.Errorf("clone lost the maps")
	}
	if c := b.Lines.Get(b.YBase).Clone(); c.combined != nil || c.extendedAttrs != nil {
		t.Errorf("clone of a plain line has maps")
	}
}
