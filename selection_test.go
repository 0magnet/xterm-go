package xterm

import (
	"testing"

	"github.com/0magnet/xterm-go/vt"
)

// newSel builds a terminal of the given size, writes text into it, and returns
// a selection over it. The model takes its two dependencies as functions
// precisely so this can be done without a browser.
func newSel(t *testing.T, cols, rows int, text string) (*selection, *vt.Terminal) {
	t.Helper()
	opts := vt.NewOptions()
	opts.Cols, opts.Rows = cols, rows
	core := vt.NewTerminal(opts)
	core.WriteString(text)
	s := newSelection(core.Cols, core.Buffer)
	s.buf = core.Buffer()
	s.has = true
	return s, core
}

// at is a position in the first buffer line, for tests that stay on one row.
func at(col, line int) pos { return pos{col: col, line: line} }

func TestSelectionAcrossOneRow(t *testing.T) {
	s, _ := newSel(t, 20, 5, "hello world")
	s.mode = selectChar
	s.anchor, s.focus = at(0, 0), at(4, 0)
	if got := s.text(); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

// TestSelectionIsInclusiveOfTheCellUnderThePointer is the difference between
// selecting what you dragged over and selecting one character less than that.
func TestSelectionIsInclusiveOfTheCellUnderThePointer(t *testing.T) {
	s, _ := newSel(t, 20, 5, "abcdef")
	s.mode = selectChar
	s.anchor, s.focus = at(1, 0), at(3, 0)
	if got := s.text(); got != "bcd" {
		t.Errorf("dragging b→d gave %q, want %q", got, "bcd")
	}
	// dragging the other way selects the same three cells
	s.anchor, s.focus = at(3, 0), at(1, 0)
	if got := s.text(); got != "bcd" {
		t.Errorf("dragging d→b gave %q, want %q", got, "bcd")
	}
}

// TestTrailingBlanksAreNotCopied covers the reason for reading the buffer
// rather than the DOM: the blanks to the right of a line are padding the
// terminal added, and nobody selected them.
func TestTrailingBlanksAreNotCopied(t *testing.T) {
	s, _ := newSel(t, 40, 5, "short\r\nline two")
	s.mode = selectChar
	s.anchor, s.focus = at(0, 0), at(39, 1)
	want := "short\nline two"
	if got := s.text(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestWrappedLineComesBackAsOneLine is the other half of that reason. Thirty
// characters written into a twenty-column terminal occupy two rows, but they
// were one line and no newline was ever typed between them.
func TestWrappedLineComesBackAsOneLine(t *testing.T) {
	const line = "0123456789abcdefghijklmnopqrst" // 30 chars into 20 columns
	s, core := newSel(t, 20, 5, line)
	if !core.Buffer().Lines.Get(1).IsWrapped {
		t.Fatal("the second row is not marked as a continuation; the test is wrong")
	}
	s.mode = selectChar
	s.anchor, s.focus = at(0, 0), at(9, 1)
	if got := s.text(); got != line {
		t.Errorf("got %q, want %q", got, line)
	}
}

func TestWordSelection(t *testing.T) {
	s, _ := newSel(t, 40, 5, "one two three")
	s.mode = selectWord
	for _, tc := range []struct {
		col  int
		want string
	}{
		{0, "one"}, {2, "one"},
		{4, "two"}, {6, "two"},
		{8, "three"}, {12, "three"},
		{3, " "}, // landing on the separator selects just it
	} {
		s.anchor, s.focus = at(tc.col, 0), at(tc.col, 0)
		if got := s.text(); got != tc.want {
			t.Errorf("double-click at column %d gave %q, want %q", tc.col, got, tc.want)
		}
	}
}

// TestWordSelectionCrossesAWrap matters because the words worth double-clicking
// in a terminal — paths, URLs, hashes — are the long ones, and the long ones
// are the ones that wrap.
func TestWordSelectionCrossesAWrap(t *testing.T) {
	const path = "/home/someone/a/very/long/path/to/a/file.txt"
	s, _ := newSel(t, 20, 5, path)
	s.mode = selectWord
	s.anchor, s.focus = at(5, 1), at(5, 1) // somewhere in the middle of it
	if got := s.text(); got != path {
		t.Errorf("got %q, want %q", got, path)
	}
}

// TestWordSeparatorsDoNotIncludeTheSlash — a path is one word.
func TestWordSeparatorsDoNotIncludeTheSlash(t *testing.T) {
	s, _ := newSel(t, 40, 5, "cd /usr/local/bin")
	s.mode = selectWord
	s.anchor, s.focus = at(8, 0), at(8, 0)
	if got := s.text(); got != "/usr/local/bin" {
		t.Errorf("got %q, want %q", got, "/usr/local/bin")
	}
}

func TestLineSelectionTakesTheWholeLogicalLine(t *testing.T) {
	const line = "0123456789abcdefghijklmnopqrst"
	s, _ := newSel(t, 20, 5, line)
	s.mode = selectLine
	s.anchor, s.focus = at(3, 1), at(3, 1) // clicked on the continuation row
	if got := s.text(); got != line {
		t.Errorf("got %q, want %q", got, line)
	}
}

// TestDraggingBackOverTheOriginKeepsTheFirstUnit: word mode unions the word
// under the anchor with the word under the focus, so pulling the mouse back
// past where it started does not shrink the selection to nothing.
func TestDraggingBackOverTheOriginKeepsTheFirstUnit(t *testing.T) {
	s, _ := newSel(t, 40, 5, "one two three")
	s.mode = selectWord
	s.anchor = at(5, 0) // "two"
	s.focus = at(0, 0)  // dragged left onto "one"
	if got := s.text(); got != "one two" {
		t.Errorf("got %q, want %q", got, "one two")
	}
	s.focus = at(10, 0) // dragged right onto "three"
	if got := s.text(); got != "two three" {
		t.Errorf("got %q, want %q", got, "two three")
	}
}

func TestContains(t *testing.T) {
	s, _ := newSel(t, 20, 5, "abcdef\r\nghijkl")
	s.mode = selectChar
	s.anchor, s.focus = at(2, 0), at(3, 1)

	for _, tc := range []struct {
		col, line int
		want      bool
	}{
		{1, 0, false}, // before the start on the first row
		{2, 0, true},
		{19, 0, true}, // the first row runs to its end
		{0, 1, true},
		{3, 1, true},  // the focus cell itself is selected
		{4, 1, false}, // past the end on the last row
		{0, 2, false}, // a row below the selection
	} {
		if got := s.contains(tc.col, tc.line); got != tc.want {
			t.Errorf("contains(%d,%d) = %v, want %v", tc.col, tc.line, got, tc.want)
		}
	}
}

// TestSelectionDoesNotSurviveABufferSwitch: the alternate screen has its own
// lines, and the coordinates of a selection made on the normal screen point at
// unrelated text there.
func TestSelectionDoesNotSurviveABufferSwitch(t *testing.T) {
	s, core := newSel(t, 20, 5, "hello")
	s.mode = selectChar
	s.anchor, s.focus = at(0, 0), at(4, 0)
	if !s.valid() {
		t.Fatal("the selection should be valid on the buffer it was made in")
	}
	core.WriteString("\x1b[?1049h") // switch to the alternate screen
	if s.valid() {
		t.Error("the selection outlived the buffer it was made in")
	}
	if got := s.text(); got != "" {
		t.Errorf("it also still reports text: %q", got)
	}
}

func TestSelectAllTakesTheScrollback(t *testing.T) {
	s, core := newSel(t, 20, 3, "one\r\ntwo\r\nthree\r\nfour\r\nfive")
	if core.Buffer().Lines.Length() <= core.Rows() {
		t.Fatal("nothing was scrolled back; the test is wrong")
	}
	s.selectAll()
	want := "one\ntwo\nthree\nfour\nfive"
	if got := s.text(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestABareClickSelectsNothing — a click is how you focus a terminal and how
// you dismiss a selection, not how you select one character.
func TestABareClickSelectsNothing(t *testing.T) {
	s, _ := newSel(t, 20, 5, "hello")
	s.has = false
	s.begin(at(2, 0), selectChar)
	if s.has {
		t.Error("a single click selected something")
	}
	s.extend(at(4, 0))
	if !s.has {
		t.Fatal("dragging did not start a selection")
	}
	if got := s.text(); got != "llo" {
		t.Errorf("got %q, want %q", got, "llo")
	}
}

// TestADoubleClickShowsItsWordImmediately — unlike a single click, it has
// already selected something and should say so without waiting for a drag.
func TestADoubleClickShowsItsWordImmediately(t *testing.T) {
	s, _ := newSel(t, 20, 5, "hello world")
	s.has = false
	s.begin(at(1, 0), selectWord)
	if !s.has {
		t.Fatal("a double click selected nothing")
	}
	if got := s.text(); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}
