// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The #453 mount prefix — the URL position at which --cgidir sits inside
// --staticdir — was derived by comparing the two flag values as text. That
// works only while the operator spells both the same way. A release symlink
// ("current" -> "releases/v1") pointed at by one flag and not the other, or
// two spellings differing only in case on a case-insensitive filesystem,
// made the same pair of directories look unrelated: no prefix was derived,
// the CGI handler did not recognize the script's natural URL, and the static
// handler correctly refused to disclose an exec directory's contents. The
// operator got a 404 for a script that was configured and executable.
//
// These drive the real binary, because the bug is entirely in how two
// handlers hand off to each other.

// mountSpellingSite is the shared cgiSite plus the release symlink these
// cases are spelled through: <Root>/current -> <Root>/releases/v1.
func mountSpellingSite(t *testing.T) cgiSite {
	t.Helper()
	site := newCGISite(t)
	mustSymlink(t, site.Static, filepath.Join(site.Root, "current"))
	return site
}

// assertMountSpellingRouting is the whole contract, driven against one
// spelling of the pair. The refusals are only meaningful next to the two
// positive controls, which fail if the server was not really serving CGI
// and static content from the intended roots.
func assertMountSpellingRouting(t *testing.T, s *Server) {
	t.Helper()

	t.Run("control: the direct mapping runs the script", func(t *testing.T) {
		requireServes(t, s.HTTPGet, "--cgidir", "/hello.sh", "hello-from-cgi")
	})

	t.Run("control: static content is served", func(t *testing.T) {
		requireServes(t, s.HTTPGet, "--staticdir", "/", "the index page")
	})

	t.Run("the script runs at its URL inside the static tree", func(t *testing.T) {
		assertServes(t, s.HTTPGet, "--cgidir is mounted here inside --staticdir",
			"/cgi-bin/hello.sh", "hello-from-cgi")
	})

	t.Run("a sibling sharing the prefix text is not diverted", func(t *testing.T) {
		assertServes(t, s.HTTPGet, "it is an ordinary file, not the CGI directory",
			"/cgi-bindings/note.txt", "ordinary static file")
	})

	t.Run("the cgi directory is neither listed nor served", func(t *testing.T) {
		assertRefused(t, s, "listed the CGI directory", []string{"hello.sh"}, "/cgi-bin", "/cgi-bin/")
	})

	t.Run("traversal through the derived prefix cannot escape", func(t *testing.T) {
		assertNoLeak(t, s, "escaped the tree", []string{"PWNED", "root:"},
			"/cgi-bin/../../outside/evil.sh",
			"/cgi-bin/../../outside/secret.txt",
			"/cgi-bin/%2e%2e/%2e%2e/outside/evil.sh",
			"/cgi-bin/..%2F..%2Foutside%2Fevil.sh",
			"/cgi-bin/sub/../../../outside/evil.sh",
			"/cgi-bin/../../../../../../etc/passwd",
		)
	})

	t.Run("dotfiles stay refused", func(t *testing.T) {
		assertRefused(t, s, "served a dotfile from the CGI directory", []string{"CGI_SECRET"}, "/cgi-bin/.env")
	})
}

// TestCGIMountSpelling_StaticDirIsASymlink: --staticdir is the release
// symlink, --cgidir is the real directory inside it.
func TestCGIMountSpelling_StaticDirIsASymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()
	site := mountSpellingSite(t)
	writeStaticFile(t, filepath.Join(site.Cgi, ".env"), "CGI_SECRET="+cgiSourceMarker)
	s := startServerOpts(t, []string{
		"--staticdir=" + filepath.Join(site.Root, "current"),
		"--cgidir=" + site.Cgi,
	}, "echo")
	assertMountSpellingRouting(t, s)
}

// TestCGIMountSpelling_CgiDirViaSymlink: the reverse — --staticdir is the
// real directory, --cgidir is reached through the release symlink.
func TestCGIMountSpelling_CgiDirViaSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()
	site := mountSpellingSite(t)
	writeStaticFile(t, filepath.Join(site.Cgi, ".env"), "CGI_SECRET="+cgiSourceMarker)
	s := startServerOpts(t, []string{
		"--staticdir=" + site.Static,
		"--cgidir=" + filepath.Join(site.Root, "current", "cgi-bin"),
	}, "echo")
	assertMountSpellingRouting(t, s)
}

// TestCGIMountSpelling_CaseDiffersOnly: one directory, two spellings that
// differ only in case. Meaningful only where the filesystem folds case.
func TestCGIMountSpelling_CaseDiffersOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()
	site := mountSpellingSite(t)
	if !caseFoldingFS(t, site.Root) {
		t.Skip("filesystem is case-sensitive; the two spellings are genuinely different directories")
	}
	writeStaticFile(t, filepath.Join(site.Cgi, ".env"), "CGI_SECRET="+cgiSourceMarker)
	// site.Static ends in "v1"; "V1" names the same directory here.
	upper := filepath.Join(filepath.Dir(site.Static), strings.ToUpper(filepath.Base(site.Static)))
	s := startServerOpts(t, []string{
		"--staticdir=" + site.Static,
		"--cgidir=" + filepath.Join(upper, "cgi-bin"),
	}, "echo")
	assertMountSpellingRouting(t, s)
}

// TestCGIMountSpelling_OutsideTreeStaysRefused is the adversarial twin.
// Resolving the two flag values must not turn "the CGI directory is
// *reachable* from the static tree through a symlink" into "the CGI
// directory is *inside* the static tree". --cgidir here really lives
// outside --staticdir, joined only by a symlink named "scripts".
// /scripts/hello.sh must stay refused; /hello.sh must keep working.
func TestCGIMountSpelling_OutsideTreeStaysRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()

	root := t.TempDir()
	staticDir := filepath.Join(root, "static")
	cgiDir := filepath.Join(root, "elsewhere", "cgi")
	writeCGIScript(t, filepath.Join(cgiDir, "hello.sh"), "hello-from-cgi")
	writeStaticFile(t, filepath.Join(staticDir, "index.html"), "the index page")
	mustSymlink(t, cgiDir, filepath.Join(staticDir, "scripts"))

	s := startServerOpts(t, []string{"--staticdir=" + staticDir, "--cgidir=" + cgiDir}, "echo")

	// Controls, so the refusal below is a refusal and not a dead server.
	requireServes(t, s.HTTPGet, "--cgidir="+cgiDir, "/hello.sh", "hello-from-cgi")
	requireServes(t, s.HTTPGet, "--staticdir="+staticDir, "/", "the index page")

	assertRefused(t, s, "--cgidir is reachable from --staticdir only through a symlink, "+
		"which does not put it inside", nil, "/scripts/hello.sh")
}
