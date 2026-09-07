// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration

import (
	"path/filepath"
	"runtime"
	"testing"
)

// Issue #453 stopped `--staticdir=/PAGE --cgidir=/PAGE/cgi-bin` handing out
// CGI source. The identical layout with `--dir` — `--staticdir=/PAGE
// --dir=/PAGE/scripts` — was left doing exactly that, and `--dir` is the
// flag websocketd is actually built around. A plain GET (no Upgrade header)
// never reaches the WebSocket handler, so every script in the tree was
// downloadable as text.
//
// Driven against the real binary end to end, because the whole defect is in
// how the WebSocket and static handlers hand off to each other.

func TestScriptDirInsideStaticDirIsNotDisclosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripts here are /bin/sh")
	}
	t.Parallel()

	root := t.TempDir()
	staticDir := filepath.Join(root, "PAGE")
	scriptDir := filepath.Join(staticDir, "scripts")
	writeWSScript(t, filepath.Join(scriptDir, "hello.sh"))
	writeStaticFile(t, filepath.Join(staticDir, "index.html"), "the index page")
	mustSymlink(t, scriptDir, filepath.Join(staticDir, "mirror"))

	s := startServerArgsInDir(t, "", []string{"--dir=" + scriptDir, "--staticdir=" + staticDir})

	// Positive control A: the static handler really is serving from
	// staticDir. Without this a regression to a blanket 404 would look
	// like a security improvement.
	t.Run("control: static files are served", func(t *testing.T) {
		requireServes(t, s.HTTPGet, "--staticdir="+staticDir, "/", "the index page")
	})

	// Positive control B: --dir really is wired up and the script really
	// runs, at the mapping --dir has always documented.
	t.Run("control: the script runs over WebSocket", func(t *testing.T) {
		ws := s.Connect("/hello.sh")
		defer ws.Close()
		ws.Send("ping")
		ws.ExpectMessage("got ping")
	})

	t.Run("a plain GET must not return the script source", func(t *testing.T) {
		assertRefused(t, s, "--dir is nested inside --staticdir", nil, "/scripts/hello.sh")
	})

	t.Run("nor through a symlink into the script tree", func(t *testing.T) {
		assertRefused(t, s, "reached the --dir tree through a symlink", nil, "/mirror/hello.sh")
	})
}

// TestScriptDirEqualToStaticDirStillServes is the exemption, driven against
// the real binary: --dir and --staticdir pointing at one directory is the
// oldest demo layout there is, and excluding the script tree there would
// leave the static handler with nothing at all to serve.
func TestScriptDirEqualToStaticDirStillServes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripts here are /bin/sh")
	}
	t.Parallel()

	dir := t.TempDir()
	writeWSScript(t, filepath.Join(dir, "hello.sh"))
	writeStaticFile(t, filepath.Join(dir, "index.html"), "the index page")

	s := startServerArgsInDir(t, "", []string{"--dir=" + dir, "--staticdir=" + dir})

	assertServes(t, s.HTTPGet, "--dir and --staticdir name one directory", "/", "the index page")

	ws := s.Connect("/hello.sh")
	defer ws.Close()
	ws.Send("ping")
	ws.ExpectMessage("got ping")
}
