package xterm

import (
	"reflect"
	"strings"
	"testing"

	"github.com/0magnet/xterm-go/vt"
)

func TestA11yRowTextColumns(t *testing.T) {
	opts := vt.NewOptions()
	opts.Cols, opts.Rows = 20, 3
	core := vt.NewTerminal(opts)
	// A wide character takes two columns but is one UTF-16 unit; an emoji
	// outside the BMP is two units in one cell.
	core.WriteString("a你b\U0001F600c")
	text, cols := a11yRowText(core.Buffer().Lines.Get(0))
	if text != "a你b\U0001F600c" {
		t.Fatalf("text %q", text)
	}
	want := []int{0, 1, 3, 4, 4, 5, 6}
	if !reflect.DeepEqual(cols, want) {
		t.Errorf("cols %v, want %v", cols, want)
	}

	text, cols = a11yRowText(core.Buffer().Lines.Get(1))
	if text != "" || !reflect.DeepEqual(cols, []int{0}) {
		t.Errorf("blank row: %q %v", text, cols)
	}
}

func TestA11yAnnouncerSkipsTypedEcho(t *testing.T) {
	var a a11yAnnouncer
	a.key("l")
	a.key("s")
	for _, c := range []string{"l", "s", "\n", "x"} {
		a.char(c)
	}
	// The screen reader read l and s out as they were typed; their echoes are
	// not read again, and what the command printed is.
	if got := a.take(); got != "\nx" {
		t.Errorf("announced %q", got)
	}
	if got := a.take(); got != "" {
		t.Errorf("second take %q", got)
	}
	// Control keys are not remembered: Enter's echo is output like any other.
	a.key("\r")
	a.char("\n")
	if got := a.take(); got != "\n" {
		t.Errorf("after enter %q", got)
	}
}

func TestA11yAnnouncerStopsAfterTooMuchOutput(t *testing.T) {
	var a a11yAnnouncer
	a.tab(2)
	for range a11yMaxRowsToRead + 5 {
		a.char("y")
		a.char("\n")
	}
	got := a.take()
	if !strings.HasPrefix(got, "  y\n") {
		t.Errorf("tab not spaces: %q", got[:8])
	}
	if strings.Count(got, "\n") != a11yMaxRowsToRead+1 || !strings.HasSuffix(got, a11yTooMuchOutput) {
		t.Errorf("did not stop after %d lines: %q", a11yMaxRowsToRead, got)
	}
	a.reset()
	a.char("z")
	if got := a.take(); got != "z" {
		t.Errorf("after reset %q", got)
	}
}

func TestSelectRange(t *testing.T) {
	s, _ := newSel(t, 10, 3, "0123456789abcdefghij")
	s.has = false
	s.selectRange(pos{8, 0}, pos{0, 1})
	if got := s.text(); got != "89" {
		t.Errorf("to end of row: %q", got)
	}
	s.selectRange(pos{2, 0}, pos{3, 1})
	if got := s.text(); got != "23456789abc" {
		t.Errorf("across a wrap: %q", got)
	}
}
