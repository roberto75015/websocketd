package integration

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestCGI001_ScriptExecuted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()
	cgiDir := t.TempDir()
	writeCGIScript(t, filepath.Join(cgiDir, "hello.sh"), "hello-from-cgi")
	writeCGIScript(t, filepath.Join(cgiDir, "sub", "nested.sh"), "hello-from-nested")

	s := startServerOpts(t, []string{"--cgidir=" + cgiDir}, "echo")

	assertServes(t, s.HTTPGet, "it is inside --cgidir", "/hello.sh", "hello-from-cgi")
	assertServes(t, s.HTTPGet, "it is inside --cgidir", "/sub/nested.sh", "hello-from-nested")

	// Dot segments that stay inside the directory still resolve (the client
	// follows the mux's normalization redirect).
	assertServes(t, s.HTTPGetFollow, "the dot segment stays inside --cgidir",
		"/sub/../hello.sh", "hello-from-cgi")

	resp, _ := s.HTTPGet("/nosuch.sh")
	if resp.StatusCode != 404 {
		t.Errorf("expected 404 for missing script, got %d", resp.StatusCode)
	}
}

// TestCGI002_PathTraversal is the regression test for the CGI directory
// escape: nothing outside --cgidir may be executed, however the request
// path is spelled.
func TestCGI002_PathTraversal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()

	// An executable sitting outside the CGI directory. It is a sibling of
	// cgiDir so a short "../" hop reaches it.
	base := t.TempDir()
	cgiDir := filepath.Join(base, "cgi")
	outsideDir := filepath.Join(base, "outside")
	writeCGIScript(t, filepath.Join(outsideDir, "evil.sh"), "PWNED")
	writeCGIScript(t, filepath.Join(cgiDir, "ok.sh"), "ok")

	s := startServerOpts(t, []string{"--cgidir=" + cgiDir}, "echo")

	assertRefused(t, s, "escaped the CGI directory and executed", []string{"PWNED"},
		"/../outside/evil.sh",
		"/%2e%2e/outside/evil.sh",
		"/.%2e/outside/evil.sh",
		"/ok.sh/../../outside/evil.sh",
		"/sub/../../outside/evil.sh",
		"/..%2Foutside%2Fevil.sh",
	)

	// A symlink inside the CGI directory pointing outside it must not be
	// executed either.
	mustSymlink(t, filepath.Join(outsideDir, "evil.sh"), filepath.Join(cgiDir, "escape.sh"))
	assertNoLeak(t, s, "a symlink out of the CGI directory executed", []string{"PWNED"}, "/escape.sh")

	// The legitimate script is unaffected.
	assertServes(t, s.HTTPGet, "it is inside --cgidir", "/ok.sh", "ok")
}
