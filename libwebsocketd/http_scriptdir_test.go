// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libwebsocketd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The static handler must never hand back the source of a file the operator
// configured to be *executed*. Issue #453 established that for --cgidir; the
// identical layout with --dir (--staticdir=/PAGE --dir=/PAGE/scripts) was
// left serving script source as text/plain, and --dir is the more common of
// the two flags. These tests cover the --dir half of that rule.
//
// scriptDirMarker appears only in a script's *source*, never in its output,
// so a response body distinguishes "disclosed" from "executed" on its own.
const scriptDirMarker = "SCRIPTDIR-SOURCE-MUST-NOT-BE-SERVED"

// writeMarkedScript writes an executable script whose source carries
// scriptDirMarker.
func writeMarkedScript(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\n# " + scriptDirMarker + "\necho hello\n"
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
}

// staticGet drives serveStatic for one config and returns status and body.
func staticGet(t *testing.T, config *Config, urlPath string) (int, string) {
	t.Helper()
	log := RootLogScope(LogNone, func(*LogScope, LogLevel, string, string, string, ...interface{}) {})
	h := &WebsocketdServer{Config: config, Log: log}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !h.serveStatic(w, req, log) {
			http.NotFound(w, req)
		}
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + urlPath)
	if err != nil {
		t.Fatalf("GET %s: %v", urlPath, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body of GET %s: %v", urlPath, err)
	}
	return resp.StatusCode, string(body)
}

// TestStaticRefusesScriptDirSource is the core assertion: with --dir nested
// inside --staticdir, a plain GET of a script must not return its source.
// The positive control runs first and in the same server, so a regression
// that merely breaks static serving (everything 404s) cannot pass this test
// by looking safe.
func TestStaticRefusesScriptDirSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripts here are /bin/sh")
	}
	root := t.TempDir()
	staticDir := filepath.Join(root, "PAGE")
	scriptDir := filepath.Join(staticDir, "scripts")
	writeMarkedScript(t, filepath.Join(scriptDir, "hello.sh"))
	writeFile(t, filepath.Join(staticDir, "index.html"), "the index page")
	// A sibling whose name merely starts with the script directory's name
	// must keep working: the exclusion is by file identity, not by prefix.
	writeFile(t, filepath.Join(staticDir, "scriptsy", "note.txt"), "ordinary static file")

	config := &Config{StaticDir: staticDir, ScriptDir: scriptDir, UsingScriptDir: true}

	// Positive control: the server really is serving from staticDir.
	if code, body := staticGet(t, config, "/index.html"); code != 200 || !strings.Contains(body, "the index page") {
		t.Fatalf("control GET /index.html = %d %q, want 200 with the index page", code, body)
	}
	if code, body := staticGet(t, config, "/scriptsy/note.txt"); code != 200 || !strings.Contains(body, "ordinary static file") {
		t.Fatalf("control GET /scriptsy/note.txt = %d %q, want 200 with the static file", code, body)
	}

	code, body := staticGet(t, config, "/scripts/hello.sh")
	if strings.Contains(body, scriptDirMarker) {
		t.Errorf("SECURITY: GET /scripts/hello.sh disclosed --dir script source (status %d): %q", code, body)
	}
	if code == 200 {
		t.Errorf("GET /scripts/hello.sh = 200, want a refusal")
	}
}

// TestStaticRefusesScriptDirThroughSymlink covers the spelling the URL
// prefix never sees: a symlink elsewhere in the static tree pointing into
// the script directory reaches the same file under a name no prefix rule
// matches. Membership has to be decided by identity for this to be refused.
func TestStaticRefusesScriptDirThroughSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripts here are /bin/sh")
	}
	root := t.TempDir()
	staticDir := filepath.Join(root, "PAGE")
	scriptDir := filepath.Join(staticDir, "scripts")
	writeMarkedScript(t, filepath.Join(scriptDir, "hello.sh"))
	writeFile(t, filepath.Join(staticDir, "index.html"), "the index page")
	if err := os.Symlink(scriptDir, filepath.Join(staticDir, "mirror")); err != nil {
		t.Fatal(err)
	}

	config := &Config{StaticDir: staticDir, ScriptDir: scriptDir, UsingScriptDir: true}

	if code, body := staticGet(t, config, "/index.html"); code != 200 || !strings.Contains(body, "the index page") {
		t.Fatalf("control GET /index.html = %d %q, want 200 with the index page", code, body)
	}

	code, body := staticGet(t, config, "/mirror/hello.sh")
	if strings.Contains(body, scriptDirMarker) {
		t.Errorf("SECURITY: GET /mirror/hello.sh disclosed --dir script source (status %d): %q", code, body)
	}
	if code == 200 {
		t.Errorf("GET /mirror/hello.sh = 200, want a refusal")
	}
}

// TestStaticScriptDirExclusionExemptions is the case that must *not* trigger
// the predicate. A --staticdir at or inside --dir would have every static
// file inside the script directory, so excluding it would leave the static
// handler with nothing to serve at all; that layout keeps its behavior, the
// same exemption #453 made for --cgidir. Without these cases an exclusion
// that simply refused everything would pass the tests above.
func TestStaticScriptDirExclusionExemptions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripts here are /bin/sh")
	}

	t.Run("equal directories", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "index.html"), "the index page")
		config := &Config{StaticDir: dir, ScriptDir: dir, UsingScriptDir: true}
		if code, body := staticGet(t, config, "/index.html"); code != 200 || !strings.Contains(body, "the index page") {
			t.Errorf("GET /index.html = %d %q, want 200 with the index page", code, body)
		}
	})

	t.Run("static dir inside script dir", func(t *testing.T) {
		root := t.TempDir()
		staticDir := filepath.Join(root, "htm")
		writeFile(t, filepath.Join(staticDir, "index.html"), "the index page")
		config := &Config{StaticDir: staticDir, ScriptDir: root, UsingScriptDir: true}
		if code, body := staticGet(t, config, "/index.html"); code != 200 || !strings.Contains(body, "the index page") {
			t.Errorf("GET /index.html = %d %q, want 200 with the index page", code, body)
		}
	})

	t.Run("unrelated script dir", func(t *testing.T) {
		root := t.TempDir()
		staticDir := filepath.Join(root, "PAGE")
		scriptDir := filepath.Join(root, "scripts")
		writeMarkedScript(t, filepath.Join(scriptDir, "hello.sh"))
		writeFile(t, filepath.Join(staticDir, "index.html"), "the index page")
		config := &Config{StaticDir: staticDir, ScriptDir: scriptDir, UsingScriptDir: true}
		if code, body := staticGet(t, config, "/index.html"); code != 200 || !strings.Contains(body, "the index page") {
			t.Errorf("GET /index.html = %d %q, want 200 with the index page", code, body)
		}
	})
}

// TestStaticExcludesBothExecDirs pins the interaction: --cgidir and --dir can
// both be nested in the static tree at once, and neither may be disclosed.
// A fix that replaced the #453 CGI exclusion with a --dir one rather than
// adding to it would pass every test above and fail this one.
func TestStaticExcludesBothExecDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripts here are /bin/sh")
	}
	root := t.TempDir()
	staticDir := filepath.Join(root, "PAGE")
	scriptDir := filepath.Join(staticDir, "scripts")
	cgiDir := filepath.Join(staticDir, "cgi-bin")
	writeMarkedScript(t, filepath.Join(scriptDir, "hello.sh"))
	writeMarkedScript(t, filepath.Join(cgiDir, "hello.sh"))
	writeFile(t, filepath.Join(staticDir, "index.html"), "the index page")

	config := &Config{
		StaticDir:      staticDir,
		ScriptDir:      scriptDir,
		UsingScriptDir: true,
		CgiDir:         cgiDir,
	}

	if code, body := staticGet(t, config, "/index.html"); code != 200 || !strings.Contains(body, "the index page") {
		t.Fatalf("control GET /index.html = %d %q, want 200 with the index page", code, body)
	}
	for _, target := range []string{"/scripts/hello.sh", "/cgi-bin/hello.sh"} {
		code, body := staticGet(t, config, target)
		if strings.Contains(body, scriptDirMarker) {
			t.Errorf("SECURITY: GET %s disclosed script source (status %d): %q", target, code, body)
		}
		if code == 200 {
			t.Errorf("GET %s = 200, want a refusal", target)
		}
	}
}
