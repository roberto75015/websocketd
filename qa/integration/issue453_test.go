package integration

import (
	"path/filepath"
	"runtime"
	"testing"
)

// Issue #453: `--cgidir=<root>/cgi-bin --staticdir=<root>` returned the CGI
// scripts' source code as plain text instead of running them. The URL a
// browser forms for a script inside the static tree is /cgi-bin/script.sh,
// but --cgidir mapped the whole URL path into the CGI directory, looked for
// <root>/cgi-bin/cgi-bin/script.sh, missed, and fell through to the static
// handler — which happily served the script it had been told to execute.
//
// This drives the real binary end to end, because the whole bug is in how
// the two handlers hand off to each other.

func TestIssue453_CGIDirInsideStaticDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()

	site := newCGISite(t)
	mustSymlink(t, site.Cgi, filepath.Join(site.Static, "mirror"))

	s := startServerOpts(t, []string{"--cgidir=" + site.Cgi, "--staticdir=" + site.Static}, "echo")

	t.Run("script runs at its URL inside the static tree", func(t *testing.T) {
		assertServes(t, s.HTTPGet, "--cgidir sits at this URL inside --staticdir",
			"/cgi-bin/hello.sh", "hello-from-cgi")
	})

	t.Run("the documented direct mapping is unchanged", func(t *testing.T) {
		assertServes(t, s.HTTPGet, "--cgidir has always mapped the whole URL path",
			"/hello.sh", "hello-from-cgi")
	})

	t.Run("a symlink into the cgi dir is refused, not disclosed", func(t *testing.T) {
		assertRefused(t, s, "reached the CGI directory through a symlink", nil, "/mirror/hello.sh")
	})

	t.Run("static files are still served", func(t *testing.T) {
		assertServes(t, s.HTTPGet, "the static root still has content", "/", "the index page")
		// A sibling whose name merely starts with the CGI directory's name
		// must not be diverted into the CGI handler.
		assertServes(t, s.HTTPGet, "it is an ordinary file, not the CGI directory",
			"/cgi-bindings/note.txt", "ordinary static file")
	})

	t.Run("the cgi directory is neither listed nor served", func(t *testing.T) {
		assertRefused(t, s, "listed the CGI directory", []string{"hello.sh"}, "/cgi-bin", "/cgi-bin/")
	})

	t.Run("traversal spelled through the new prefix cannot escape", func(t *testing.T) {
		assertNoLeak(t, s, "escaped the tree", []string{"PWNED"},
			"/cgi-bin/../../outside/evil.sh",
			"/cgi-bin/../../outside/secret.txt",
			"/cgi-bin/%2e%2e/%2e%2e/outside/evil.sh",
			"/cgi-bin/..%2F..%2Foutside%2Fevil.sh",
			"/cgi-bin/sub/../../../outside/evil.sh",
		)
	})
}
