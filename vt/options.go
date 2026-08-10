package vt

// Options is the subset of the xterm.js options relevant to the core
// (port of the OptionsService raw options; browser-only options live in
// the wasm layer).
type Options struct {
	// Cols/Rows of the terminal. Defaults: 80x24.
	Cols int
	Rows int
	// Scrollback line count. Default: 1000.
	Scrollback int
	// TabStopWidth. Default: 8.
	TabStopWidth int
	// CursorBlink toggles cursor blinking. Default: false.
	CursorBlink bool
	// ConvertEol treats LF as CRLF. Default: false.
	ConvertEol bool
	// DisableStdin ignores keyboard input. Default: false.
	DisableStdin bool
	// ScreenReaderMode. Default: false.
	ScreenReaderMode bool
	// WindowsMode disables reflow (conpty quirk mode). Default: false.
	WindowsMode bool
	// ReflowCursorLine reflows the cursor line on resize. Default: false.
	ReflowCursorLine bool
	// FontFamily/FontSize used by the renderer.
	FontFamily string
	FontSize   float64
	// LineHeight multiplier. Default: 1.
	LineHeight float64
	// LetterSpacing in px. Default: 0.
	LetterSpacing float64
	// Theme colors (CSS color strings; empty = defaults).
	Theme Theme
}

// Theme holds the terminal colors (port of ITheme).
type Theme struct {
	Foreground          string
	Background          string
	Cursor              string
	CursorAccent        string
	SelectionBackground string
	Black               string
	Red                 string
	Green               string
	Yellow              string
	Blue                string
	Magenta             string
	Cyan                string
	White               string
	BrightBlack         string
	BrightRed           string
	BrightGreen         string
	BrightYellow        string
	BrightBlue          string
	BrightMagenta       string
	BrightCyan          string
	BrightWhite         string
}

// NewOptions returns options with xterm.js defaults.
func NewOptions() *Options {
	return &Options{
		Cols:          80,
		Rows:          24,
		Scrollback:    1000,
		TabStopWidth:  8,
		FontFamily:    "monospace",
		FontSize:      15,
		LineHeight:    1.0,
		LetterSpacing: 0,
	}
}
