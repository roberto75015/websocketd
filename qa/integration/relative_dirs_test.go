// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// relDirTree builds the fixture the relative-root tests share:
//
//	base/secret/loot.txt   — content that must never be served
//	base/secretbin/pwn.sh  — a script that must never be executed
//	base/serve/index.html  — a legitimate static file
//	base/serve/escape      — symlink to ../secret
//	base/serve/sub/        — a directory to run websocketd from
//	base/cgi/hello.sh      — a legitimate CGI script
//	base/cgi/escape        — symlink to ../secretbin
//	base/cgi/sub/          — a directory to run websocketd from
func relDirTree(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	for _, d := range []string{"secret", "secretbin", "serve/sub", "cgi/sub"} {
		if err := os.MkdirAll(filepath.Join(base, filepath.FromSlash(d)), 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeStaticFile(t, filepath.Join(base, "secret", "loot.txt"), "TOP-SECRET-LOOT")
	writeStaticFile(t, filepath.Join(base, "serve", "index.html"), "<html>RELATIVE-ROOT-OK</html>")
	writeCGIScript(t, filepath.Join(base, "secretbin", "pwn.sh"), "PWNED")
	writeCGIScript(t, filepath.Join(base, "cgi", "hello.sh"), "CGI-RELATIVE-OK")
	mustSymlink(t, filepath.Join("..", "secret"), filepath.Join(base, "serve", "escape"))
	mustSymlink(t, filepath.Join("..", "secretbin"), filepath.Join(base, "cgi", "escape"))
	return base
}

// TestRelDir001_StaticDirRelativeServesFiles covers the most natural thing an
// operator types: cd into the directory you want to serve and run
// `websocketd --staticdir=. ...`. Every spelling of the same directory must
// serve the same file — only "." and "./" used to 404, because the boundary
// check compared a relative resolved path against a relative boundary.
func TestRelDir001_StaticDirRelativeServesFiles(t *testing.T) {
	t.Parallel()
	base := relDirTree(t)
	serve := filepath.Join(base, "serve")

	cases := []struct {
		name    string
		cwd     string
		flag    string
		urlPath string
	}{
		{"absolute (control)", serve, serve, "/index.html"},
		{"dot", serve, ".", "/index.html"},
		{"dot slash", serve, "./", "/index.html"},
		{"relative subdir", base, "serve", "/index.html"},
		{"dot slash subdir", base, "./serve", "/index.html"},
		{"dotdot", filepath.Join(serve, "sub"), "..", "/index.html"},
		{"dot, directory index", serve, ".", "/"},
	}
	for _, tc := range cases {
		tc := tc // go.mod is go1.21: loop vars are shared, and subtests run parallel
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := startServerInDir(t, tc.cwd, []string{"--staticdir=" + tc.flag})
			assertServes(t, s.HTTPGetFollow, "--staticdir="+tc.flag+" names the directory holding it",
				tc.urlPath, "RELATIVE-ROOT-OK")
		})
	}
}

// TestRelDir002_StaticDirRelativeRefusesEscape is the security half: a
// relative static root must enforce the same boundary an absolute one does.
// With a ".." root the old comparison accepted anything still spelled with a
// leading "../" — which a symlink out of the tree resolves to — so the check
// failed open and served the file.
//
// Each case first proves the server is serving from the intended root, so a
// test that cannot reach its subject fails rather than passing vacuously on a
// 404 it would have got anyway.
func TestRelDir002_StaticDirRelativeRefusesEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require admin on Windows")
	}
	t.Parallel()
	base := relDirTree(t)
	serve := filepath.Join(base, "serve")

	cases := []struct {
		name string
		cwd  string
		flag string
	}{
		{"absolute (control)", serve, serve},
		{"dot", serve, "."},
		{"relative subdir", base, "serve"},
		{"dotdot", filepath.Join(serve, "sub"), ".."},
	}
	for _, tc := range cases {
		tc := tc // go.mod is go1.21: loop vars are shared, and subtests run parallel
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := startServerInDir(t, tc.cwd, []string{"--staticdir=" + tc.flag})

			// Positive control: the root really is being served.
			requireServes(t, s.HTTPGetFollow, "--staticdir="+tc.flag, "/index.html", "RELATIVE-ROOT-OK")

			assertNoLeak(t, s, "--staticdir="+tc.flag+" served a file from outside the root",
				[]string{"TOP-SECRET-LOOT"},
				"/escape/loot.txt",
				"/escape/../secret/loot.txt",
				"/%2e%2e/secret/loot.txt",
				"/..%2Fsecret%2Floot.txt",
				"/sub/../escape/loot.txt",
			)
		})
	}
}

// TestRelDir003_CgiDirRelativeRunsScripts is TestRelDir001 for --cgidir,
// which shares the same boundary check and so shared the same 404.
func TestRelDir003_CgiDirRelativeRunsScripts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()
	base := relDirTree(t)
	cgi := filepath.Join(base, "cgi")

	cases := []struct {
		name string
		cwd  string
		flag string
	}{
		{"absolute (control)", cgi, cgi},
		{"dot", cgi, "."},
		{"dot slash", cgi, "./"},
		{"relative subdir", base, "cgi"},
		{"dotdot", filepath.Join(cgi, "sub"), ".."},
	}
	for _, tc := range cases {
		tc := tc // go.mod is go1.21: loop vars are shared, and subtests run parallel
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := startServerInDir(t, tc.cwd, []string{"--cgidir=" + tc.flag})
			assertServes(t, s.HTTPGet, "--cgidir="+tc.flag+" names the directory holding it",
				"/hello.sh", "CGI-RELATIVE-OK")
		})
	}
}

// TestRelDir004_CgiDirRelativeRefusesEscape is the security half for
// --cgidir. The fail-open here is worse than the static one: the escaped
// file is not merely disclosed, it is executed.
func TestRelDir004_CgiDirRelativeRefusesEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()
	base := relDirTree(t)
	cgi := filepath.Join(base, "cgi")

	cases := []struct {
		name string
		cwd  string
		flag string
	}{
		{"absolute (control)", cgi, cgi},
		{"dot", cgi, "."},
		{"relative subdir", base, "cgi"},
		{"dotdot", filepath.Join(cgi, "sub"), ".."},
	}
	for _, tc := range cases {
		tc := tc // go.mod is go1.21: loop vars are shared, and subtests run parallel
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := startServerInDir(t, tc.cwd, []string{"--cgidir=" + tc.flag})

			// Positive control: the root really is being served.
			requireServes(t, s.HTTPGet, "--cgidir="+tc.flag, "/hello.sh", "CGI-RELATIVE-OK")

			assertNoLeak(t, s, "--cgidir="+tc.flag+" executed a script from outside the root",
				[]string{"PWNED"},
				"/escape/pwn.sh",
				"/../secretbin/pwn.sh",
				"/%2e%2e/secretbin/pwn.sh",
				"/..%2Fsecretbin%2Fpwn.sh",
				"/hello.sh/../escape/pwn.sh",
			)
		})
	}
}

// TestRelDir005_DotfilesStillRefusedUnderRelativeRoot guards issue #476's
// hardening against being re-opened by the relative-root fix: a dotfile is
// refused however the static root is spelled.
func TestRelDir005_DotfilesStillRefusedUnderRelativeRoot(t *testing.T) {
	t.Parallel()
	base := relDirTree(t)
	serve := filepath.Join(base, "serve")
	writeStaticFile(t, filepath.Join(serve, ".git", "config"), "DOTFILE-LEAK")
	writeStaticFile(t, filepath.Join(serve, ".env"), "DOTFILE-LEAK")

	for _, flag := range []string{".", serve} {
		flag := flag // go.mod is go1.21: loop vars are shared, and subtests run parallel
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			s := startServerInDir(t, serve, []string{"--staticdir=" + flag})

			requireServes(t, s.HTTPGetFollow, "--staticdir="+flag, "/index.html", "RELATIVE-ROOT-OK")

			assertNoLeak(t, s, "--staticdir="+flag+" served a dotfile", []string{"DOTFILE-LEAK"},
				"/.env", "/.git/config")

			// And no directory listing.
			resp, body := s.HTTPGet("/sub/")
			if resp.StatusCode == 200 && strings.Contains(body, "<a href=") {
				t.Errorf("--staticdir=%s: directory listing served for /sub/", flag)
			}
		})
	}
}

// TestRelDir006_MixedRelativeAndAbsoluteRoots runs --staticdir and --cgidir
// together with one spelled relative and the other absolute. Each root must
// enforce its own boundary regardless of how the other is written: the two
// go through the same check, and a per-process rather than per-root frame of
// reference would let one spelling contaminate the other.
func TestRelDir006_MixedRelativeAndAbsoluteRoots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("CGI scripts here are /bin/sh")
	}
	t.Parallel()
	base := relDirTree(t)
	serve := filepath.Join(base, "serve")
	cgi := filepath.Join(base, "cgi")

	cases := []struct {
		name      string
		cwd       string
		staticArg string
		cgiArg    string
	}{
		{"static relative, cgi absolute", base, "serve", cgi},
		{"static absolute, cgi relative", base, serve, "cgi"},
		{"static dot, cgi absolute", serve, ".", cgi},
		{"static absolute, cgi dot", cgi, serve, "."},
	}
	for _, tc := range cases {
		tc := tc // go.mod is go1.21: loop vars are shared, and subtests run parallel
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := startServerInDir(t, tc.cwd, []string{
				"--staticdir=" + tc.staticArg,
				"--cgidir=" + tc.cgiArg,
			})

			// Both roots must actually be live, or the refusals below prove
			// nothing.
			requireServes(t, s.HTTPGetFollow, "--staticdir="+tc.staticArg, "/index.html", "RELATIVE-ROOT-OK")
			requireServes(t, s.HTTPGet, "--cgidir="+tc.cgiArg, "/hello.sh", "CGI-RELATIVE-OK")

			// Neither root may be escaped.
			pair := "static/" + tc.staticArg + " cgi/" + tc.cgiArg
			assertNoLeak(t, s, pair+" served a file from outside the root", []string{"TOP-SECRET-LOOT"},
				"/escape/loot.txt", "/escape/../secret/loot.txt")
			assertNoLeak(t, s, pair+" executed a script from outside the root", []string{"PWNED"},
				"/escape/pwn.sh", "/../secretbin/pwn.sh")
		})
	}
}
