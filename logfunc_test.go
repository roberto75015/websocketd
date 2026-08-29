// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/joewalnes/websocketd/libwebsocketd"
)

// TestLogfuncEscapesControls proves the log boundary escapes control
// characters in both the message and the associated values. Child stderr is
// relayed into the log verbatim, so a wrapped program echoing remote input on
// stderr could otherwise inject terminal control sequences (screen clearing,
// title/OSC changes) or forge log lines with embedded newlines into whatever
// consumes websocketd's log stream.
func TestLogfuncEscapesControls(t *testing.T) {
	l := libwebsocketd.RootLogScope(libwebsocketd.LogError, logfunc)
	l.Associate("url", "http://x/\x1b[2J\x1b]50;injection\x07")

	// Swap in a pipe to capture logfunc's stdout output.
	keep := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	os.Stdout = w
	l.Error("stderr", "line\x1b[2Jcleared\nforged\tline\x07")
	logfuncFlush(l)
	w.Close()
	os.Stdout = keep

	out, _ := io.ReadAll(r)
	if strings.ContainsRune(string(out), '\x1b') || strings.ContainsRune(string(out), '\x07') {
		t.Errorf("raw control characters reached the log: %q", out)
	}
	if strings.Contains(string(out), "\nforged") {
		t.Errorf("newline in log data forged a log line: %q", out)
	}
	if !strings.Contains(string(out), `line\x1b[2Jcleared`) {
		t.Errorf("expected escaped control sequence in log, got: %q", out)
	}
	if !strings.Contains(string(out), `url:'http://x/\x1b[2J\x1b]50;injection\x07'`) {
		t.Errorf("expected escaped control sequence in associated value, got: %q", out)
	}
}

// logfuncFlush drains pending output. fmt.Printf writes are unbuffered on
// all platforms Go supports, so this only needs to keep the compiler honest.
func logfuncFlush(_ *libwebsocketd.LogScope) {}

func TestEscapeControls(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain", "plain"},
		{"tab\tstays", "tab\tstays"},          // tab is not a terminal escape
		{"esc\x1b[2J", `esc\x1b[2J`},          // ESC
		{"bell\x07", `bell\x07`},              // BEL
		{"cr\rlf\n", "cr\\x0dlf\\x0a"},      // newline forgery
		{"del\x7f", `del\x7f`},                // DEL
		{"ok Café ✓", "ok Café ✓"},            // UTF-8 passes through untouched
	}
	for _, c := range cases {
		if got := escapeControls(c.in); got != c.want {
			t.Errorf("escapeControls(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
