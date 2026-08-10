//go:build js && wasm

// Demo for xterm-go: a terminal with a small in-wasm echo shell that
// exercises colors, unicode, scrolling and titles.
package main

import (
	"fmt"
	"strings"
	"syscall/js"

	xterm "github.com/0magnet/xterm-go"
	"github.com/0magnet/xterm-go/vt"
)

const prompt = "\x1b[1;32mxterm-go\x1b[0m:\x1b[1;34m~\x1b[0m$ "

type shell struct {
	term *xterm.Terminal
	line []rune
}

func (s *shell) write(data string) { s.term.WriteString(data) }

func (s *shell) handleInput(data string) {
	for _, r := range data {
		switch r {
		case '\r':
			s.write("\r\n")
			s.exec(strings.TrimSpace(string(s.line)))
			s.line = s.line[:0]
			s.write(prompt)
		case 0x7f, '\b': // backspace
			if len(s.line) > 0 {
				s.line = s.line[:len(s.line)-1]
				s.write("\b \b")
			}
		case 0x03: // ctrl+c
			s.write("^C\r\n" + prompt)
			s.line = s.line[:0]
		case 0x0c: // ctrl+l
			s.write("\x1b[2J\x1b[H" + prompt + string(s.line))
		default:
			if r >= 32 || r == '\t' {
				s.line = append(s.line, r)
				s.write(string(r))
			}
		}
	}
}

func (s *shell) exec(cmd string) {
	switch cmd {
	case "":
	case "help":
		s.write("commands: \x1b[1mhelp colors truecolor unicode ls scroll title clear\x1b[0m\r\n")
	case "colors":
		for i := 0; i < 16; i++ {
			s.write(fmt.Sprintf("\x1b[48;5;%dm  ", i))
		}
		s.write("\x1b[0m\r\n")
		for row := 0; row < 6; row++ {
			for col := 0; col < 36; col++ {
				s.write(fmt.Sprintf("\x1b[48;5;%dm ", 16+row*36+col))
			}
			s.write("\x1b[0m\r\n")
		}
		for i := 232; i < 256; i++ {
			s.write(fmt.Sprintf("\x1b[48;5;%dm ", i))
		}
		s.write("\x1b[0m\r\n")
	case "truecolor":
		for x := 0; x < 72; x++ {
			r := 255 - x*255/72
			g := x * 510 / 72
			b := x * 255 / 72
			if g > 255 {
				g = 510 - g
			}
			s.write(fmt.Sprintf("\x1b[48;2;%d;%d;%dm ", r, g, b))
		}
		s.write("\x1b[0m\r\n")
	case "unicode":
		s.write("wide: \x1b[33m你好，世界\x1b[0m  combining: e\u0301 a\u0308  box: ┌─┬─┐\r\n")
		s.write("                                        │ │ │\r\n")
		s.write("                                        └─┴─┘\r\n")
	case "ls":
		s.write("\x1b[1;34mbin\x1b[0m  \x1b[1;34mdocs\x1b[0m  \x1b[1;32mmain.wasm\x1b[0m  README.md  go.mod\r\n")
	case "scroll":
		for i := 1; i <= 50; i++ {
			s.write(fmt.Sprintf("\x1b[3%dmline %d of 50 — scroll back up with the wheel or shift+pgup\x1b[0m\r\n", i%8, i))
		}
	case "title":
		s.write("\x1b]0;title set by the shell\x07window title updated\r\n")
	case "clear":
		s.write("\x1b[2J\x1b[H")
	default:
		s.write("command not found: " + cmd + " (try \x1b[1mhelp\x1b[0m)\r\n")
	}
}

func main() {
	doc := js.Global().Get("document")
	container := doc.Call("getElementById", "terminal")

	opts := vt.NewOptions()
	opts.Scrollback = 1000
	term := xterm.New(opts)
	term.Open(container)
	term.Fit()

	// use the WebGL renderer unless ?renderer=dom is given
	rendererName := "webgl"
	search := js.Global().Get("location").Get("search").String()
	if strings.Contains(search, "renderer=dom") {
		rendererName = "dom"
	} else if err := term.EnableWebGL(); err != nil {
		rendererName = "dom (webgl unavailable: " + err.Error() + ")"
	}

	term.OnTitleChange = func(title string) {
		doc.Set("title", title)
	}

	sh := &shell{term: term}
	term.Core.OnData = sh.handleInput

	resize := js.FuncOf(func(js.Value, []js.Value) any {
		term.Fit()
		return nil
	})
	js.Global().Call("addEventListener", "resize", resize)

	sh.write("\x1b[1;36m╔══════════════════════════════════════╗\r\n")
	sh.write("║  xterm-go — xterm.js ported to Go    ║\r\n")
	sh.write("╚══════════════════════════════════════╝\x1b[0m\r\n")
	sh.write("renderer: \x1b[1;33m" + rendererName + "\x1b[0m (\x1b[4m?renderer=dom\x1b[0m to switch)\r\n")
	sh.write("type \x1b[1mhelp\x1b[0m for demo commands\r\n\r\n")
	sh.write(prompt)

	select {}
}
