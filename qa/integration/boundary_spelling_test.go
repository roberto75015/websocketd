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

// The static and CGI boundary check ended in a string prefix over two
// filepath.EvalSymlinks results. EvalSymlinks resolves symlinks but does not
// canonicalize case, so on a case-folding filesystem a symlink whose target
// spells a parent directory differently from the way --staticdir spells it
// resolved to a real path that names a file plainly inside the configured
// directory yet shared no string prefix with it. websocketd 404ed the
// operator's own file. Fail-closed, so not a disclosure — but the file was
// inside the directory they configured.
//
// Driven against the real binary, because what an operator sees is the
// status code.

// spellingSite lays out one directory reachable under two case spellings.
//
//	<base>/PAGE/sub/x.txt              the real file
//	<base>/PAGE/alias-samecase.txt  -> sub/x.txt              (control)
//	<base>/PAGE/alias-abs.txt       -> <base>/page/sub/x.txt  (absolute target)
//	<base>/PAGE/alias-rel.txt       -> ../page/sub/x.txt      (relative target)
//	<base>/OUT/loot.txt                genuinely outside
//	<base>/PAGE/escape-abs.txt      -> <base>/out/loot.txt
//	<base>/PAGE/escape-rel.txt      -> ../out/loot.txt
//
// Both symlink target forms are built on purpose: absolute and relative
// targets take different routes through EvalSymlinks.
func spellingSite(t *testing.T) (base, page string) {
	t.Helper()
	base = t.TempDir()
	page = filepath.Join(base, "PAGE")
	writeStaticFile(t, filepath.Join(page, "sub", "x.txt"), "REAL-CONTENT")
	writeStaticFile(t, filepath.Join(page, "index.html"), "the index page")
	writeStaticFile(t, filepath.Join(base, "OUT", "loot.txt"), "PWNED-loot")
	for _, l := range []struct{ target, name string }{
		{filepath.Join("sub", "x.txt"), "alias-samecase.txt"},
		{filepath.Join(base, "page", "sub", "x.txt"), "alias-abs.txt"},
		{filepath.Join("..", "page", "sub", "x.txt"), "alias-rel.txt"},
		{filepath.Join(base, "out", "loot.txt"), "escape-abs.txt"},
		{filepath.Join("..", "out", "loot.txt"), "escape-rel.txt"},
	} {
		mustSymlink(t, l.target, filepath.Join(page, l.name))
	}
	return base, page
}

// TestBoundarySpelling_StaticAliasWithCaseVariantTarget: three URLs naming
// one file, all of which must be served.
func TestBoundarySpelling_StaticAliasWithCaseVariantTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require admin on Windows")
	}
	t.Parallel()
	base, page := spellingSite(t)
	if !caseFoldingFS(t, base) {
		t.Skip("SKIPPING, NOT PASSING: filesystem is case-sensitive, so <base>/page and <base>/PAGE are genuinely different directories and this case cannot arise here")
	}
	real := filepath.Join(page, "sub", "x.txt")
	for _, name := range []string{"alias-samecase.txt", "alias-abs.txt", "alias-rel.txt"} {
		assertSameFile(t, filepath.Join(page, name), real)
	}

	s := startServerOpts(t, []string{"--staticdir=" + page}, "echo")

	// Controls. Without these a blanket 404 regression would read as a
	// blanket "safe" and the assertions below would prove nothing.
	requireServes(t, s.HTTPGet, "--staticdir="+page, "/sub/x.txt", "REAL-CONTENT")
	requireServes(t, s.HTTPGet, "--staticdir="+page, "/alias-samecase.txt", "REAL-CONTENT")

	for _, name := range []string{"/alias-abs.txt", "/alias-rel.txt"} {
		assertServes(t, s.HTTPGet, "it is the same file as /sub/x.txt", name, "REAL-CONTENT")
	}
}

// TestBoundarySpelling_EscapeStaysRefused is the twin that must not
// trigger, and it runs on every filesystem. Tolerating a differently
// spelled parent must not make anything outside --staticdir reachable,
// including a symlink whose target is spelled in exactly the same
// case-variant style as the aliases above.
func TestBoundarySpelling_EscapeStaysRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require admin on Windows")
	}
	t.Parallel()
	_, page := spellingSite(t)
	s := startServerOpts(t, []string{"--staticdir=" + page}, "echo")

	requireServes(t, s.HTTPGet, "--staticdir="+page, "/sub/x.txt", "REAL-CONTENT")

	assertNoLeak(t, s, "disclosed a file outside --staticdir", []string{"PWNED"},
		"/escape-abs.txt",
		"/escape-rel.txt",
		"/../OUT/loot.txt",
		"/%2e%2e/OUT/loot.txt",
		"/..%2FOUT%2Floot.txt",
		"/sub/../../OUT/loot.txt",
	)
}

// TestBoundarySpelling_CGIAliasWithCaseVariantTarget is the same defect on
// the CGI side, where the consequence is a script that will not run rather
// than a file that will not load.
func TestBoundarySpelling_CGIAliasWithCaseVariantTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()
	base := t.TempDir()
	cgi := filepath.Join(base, "CGI")
	writeCGIScript(t, filepath.Join(cgi, "sub", "hello.sh"), "hello-from-cgi")
	writeCGIScript(t, filepath.Join(base, "OUT", "evil.sh"), "PWNED")
	if !caseFoldingFS(t, base) {
		t.Skip("SKIPPING, NOT PASSING: filesystem is case-sensitive, so <base>/cgi and <base>/CGI are genuinely different directories and this case cannot arise here")
	}
	mustSymlink(t, filepath.Join(base, "cgi", "sub", "hello.sh"), filepath.Join(cgi, "alias-abs.sh"))
	mustSymlink(t, filepath.Join("..", "cgi", "sub", "hello.sh"), filepath.Join(cgi, "alias-rel.sh"))
	mustSymlink(t, filepath.Join(base, "out", "evil.sh"), filepath.Join(cgi, "escape.sh"))
	assertSameFile(t, filepath.Join(cgi, "alias-abs.sh"), filepath.Join(cgi, "sub", "hello.sh"))
	assertSameFile(t, filepath.Join(cgi, "alias-rel.sh"), filepath.Join(cgi, "sub", "hello.sh"))

	s := startServerOpts(t, []string{"--cgidir=" + cgi}, "echo")

	requireServes(t, s.HTTPGet, "--cgidir="+cgi, "/sub/hello.sh", "hello-from-cgi")

	// assertServes checks the source marker too, which is what separates a
	// script that ran from one that was handed over: the source contains the
	// text the output prints.
	for _, name := range []string{"/alias-abs.sh", "/alias-rel.sh"} {
		assertServes(t, s.HTTPGet, "it is the same file as /sub/hello.sh", name, "hello-from-cgi")
	}

	// The twin: a CGI symlink out of the tree stays refused.
	assertNoLeak(t, s, "executed a script outside --cgidir", []string{"PWNED"}, "/escape.sh")
}
