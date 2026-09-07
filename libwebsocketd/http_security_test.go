// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libwebsocketd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestResolveCgiPath checks that a request URL can only ever name a file
// inside the configured CGI directory. req.URL.Path is percent-decoded by
// net/http, and libwebsocketd is usable as a plain http.Handler with no mux
// in front of it, so dot segments can reach us verbatim and must not be
// able to climb out of the directory. (The websocketd binary does register
// on DefaultServeMux, which redirects an unclean path first — that is a
// second line, not the one under test here.)
func TestResolveCgiPath(t *testing.T) {
	cgiDir := filepath.FromSlash("/srv/cgi")

	tests := []struct {
		name    string
		urlPath string
		want    string // "" means the request must be refused
	}{
		{"simple script", "/hello.sh", "/srv/cgi/hello.sh"},
		{"nested script", "/sub/dir/hello.sh", "/srv/cgi/sub/dir/hello.sh"},
		{"duplicate slashes", "//sub//hello.sh", "/srv/cgi/sub/hello.sh"},
		{"interior dot segment", "/sub/../hello.sh", "/srv/cgi/hello.sh"},
		{"single dot", "/./hello.sh", "/srv/cgi/hello.sh"},
		{"root", "/", ""},
		{"empty", "", ""},
		{"leading dot segments", "/../../../bin/sh", "/srv/cgi/bin/sh"},
		{"dot segments after prefix", "/sub/../../../bin/sh", "/srv/cgi/bin/sh"},
		{"trailing dot segment", "/hello.sh/..", ""},
		{"bare dot segment", "/..", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveCgiPath(cgiDir, tt.urlPath)
			if tt.want == "" {
				if err == nil {
					t.Errorf("SECURITY: resolveCgiPath(%q, %q) = %q, want refusal", cgiDir, tt.urlPath, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCgiPath(%q, %q) = error %v, want %q", cgiDir, tt.urlPath, err, tt.want)
			}
			if want := filepath.FromSlash(tt.want); got != want {
				t.Errorf("resolveCgiPath(%q, %q) = %q, want %q", cgiDir, tt.urlPath, got, want)
			}
		})
	}
}

// TestResolveCgiPathAssertionNeverFires pins containsPath's status as an
// invariant assertion rather than a second containment check: for every URL
// path a request can carry, resolveCgiPath either refuses on the "names no
// script" rule or returns a path lexically inside cgiDir, so the assertion
// has nothing left to catch. A check that never fires is only safe to keep
// while that is a proven property of its input rather than an accident —
// and it is only safe to delete once the property is written down here.
//
// The second half is the case that does trip it, so the first half cannot
// be passing vacuously: an empty cgiDir, where filepath.Rel cannot relate a
// relative directory to the absolute child it was joined into. serveCGI
// returns before calling resolveCgiPath with one, which is why no request
// reaches it.
func TestResolveCgiPathAssertionNeverFires(t *testing.T) {
	urls := []string{
		"/", "", ".", "..", "/hello.sh", "//sub//hello.sh", "/sub/../hello.sh",
		"/./hello.sh", "/../../../bin/sh", "/sub/../../../bin/sh", "/hello.sh/..",
		"/..", "/.", "/a/./b/../c", "/a//../..", "/.git/config", "/cgi-bin/hello.sh",
		"/x%2f..%2f", "/ ", "/-", "//", "///../", "/..%2f..%2f", "/a/../../..",
		`/sub\hello.bat`, `/..\..\evil`, `\..\..\evil`, "/\u00e9/../x",
		"/very/deep/../../../../../../../../etc/passwd",
	}
	dirs := []string{
		filepath.FromSlash("/srv/cgi"), filepath.FromSlash("/"), ".", "cgi-bin",
		filepath.FromSlash("./cgi/"), filepath.FromSlash("/srv/cgi/"),
		filepath.FromSlash("/srv/../srv/cgi"),
	}
	for _, dir := range dirs {
		for _, u := range urls {
			got, err := resolveCgiPath(dir, u)
			if err != nil {
				// The only refusal resolveCgiPath is allowed to produce is
				// the one for a path that names no script at all. If the
				// assertion ever starts refusing, this fails and says so.
				if !strings.Contains(err.Error(), "no CGI script named") {
					t.Errorf("resolveCgiPath(%q, %q) refused with %v; the lexical assertion is not supposed to be reachable", dir, u, err)
				}
				continue
			}
			rel, relErr := filepath.Rel(dir, got)
			if relErr != nil {
				t.Errorf("resolveCgiPath(%q, %q) = %q, which filepath.Rel cannot relate to the directory: %v", dir, u, got, relErr)
				continue
			}
			if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Errorf("SECURITY: resolveCgiPath(%q, %q) = %q, which escapes the directory", dir, u, got)
			}
		}
	}

	// The must-fire case, so the loop above cannot be trivially satisfied.
	if _, err := resolveCgiPath("", "/hello.sh"); err == nil {
		t.Error("resolveCgiPath(\"\", \"/hello.sh\") returned no error; the lexical assertion is unreachable even where it should fire, so the loop above proves nothing")
	}
}

// TestResolveCgiPathBackslash covers Windows, where a backslash also
// separates path elements. filepath.ToSlash rewrites those backslashes
// before path.Clean runs, so a "..\" segment folds back inside cgiDir just
// like "../" does above rather than being refused — the security property
// is containment, not rejection. (An earlier version of this test asserted
// refusal and failed on Windows CI while the resolved paths it printed were
// in fact inside the CGI directory.)
func TestResolveCgiPathBackslash(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("backslash is an ordinary filename character outside Windows")
	}
	cgiDir := `C:\srv\cgi`

	tests := []struct {
		name    string
		urlPath string
		want    string // "" means the request must be refused
	}{
		{"nested script", `/sub\hello.bat`, `C:\srv\cgi\sub\hello.bat`},
		{"leading dot segments", `/..\..\..\windows\system32\cmd.exe`, `C:\srv\cgi\windows\system32\cmd.exe`},
		{"dot segments after prefix", `/sub\..\..\evil.bat`, `C:\srv\cgi\evil.bat`},
		{"reduces to the directory itself", `/sub\..`, ""},
		{"bare separator", `/\`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveCgiPath(cgiDir, tt.urlPath)
			if tt.want == "" {
				if err == nil {
					t.Errorf("SECURITY: resolveCgiPath(%q, %q) = %q, want refusal", cgiDir, tt.urlPath, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveCgiPath(%q, %q) = error %v, want %q", cgiDir, tt.urlPath, err, tt.want)
			}
			if got != tt.want {
				t.Errorf("resolveCgiPath(%q, %q) = %q, want %q", cgiDir, tt.urlPath, got, tt.want)
			}
			// The property that actually matters, asserted independently of
			// the exact expected string above.
			if err := containsPath(cgiDir, got); err != nil {
				t.Errorf("SECURITY: resolveCgiPath(%q, %q) = %q, which escapes %q", cgiDir, tt.urlPath, got, cgiDir)
			}
		})
	}
}

// TestCgiSymlinkEscape covers a link inside the CGI directory that points
// outside it — the path is textually contained but the file executed is not.
func TestCgiSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require admin on Windows")
	}
	cgiDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "evil.sh")
	if err := os.WriteFile(outside, []byte("#!/bin/sh\necho pwned"), 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(cgiDir, "escape.sh")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	filePath, err := resolveCgiPath(cgiDir, "/escape.sh")
	if err != nil {
		t.Fatalf("resolveCgiPath: %v", err)
	}
	if err := checkPathBoundary(filePath, cgiDir); err == nil {
		t.Error("SECURITY: symlink out of the CGI directory should be refused")
	}
}
