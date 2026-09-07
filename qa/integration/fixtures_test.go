// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Shared vocabulary for the tests that pin websocketd's path boundaries:
// what --staticdir, --cgidir and --dir will and will not hand out. Those
// tests each build a different directory tree, on purpose — the trees are
// the thing under test — but they build them out of the same few pieces and
// they assert against them in the same few shapes. Both live here.

// cgiSourceMarker appears in the *source* of every script these fixtures
// write and never in its output, so one response body distinguishes
// "executed" from "disclosed" without a second request. Named for issue #453,
// where a plain GET returned CGI source as text.
const cgiSourceMarker = "ISSUE453-SOURCE-MUST-NOT-BE-SERVED"

// wsSourceMarker is cgiSourceMarker for --dir scripts, which are served over
// WebSocket rather than executed as CGI. Kept distinct so a leak names which
// handler leaked it.
const wsSourceMarker = "SCRIPTDIR-SOURCE-MUST-NOT-BE-SERVED"

// writeStaticFile writes a plain file, creating parent directories.
func writeStaticFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

// writeCGIScript writes an executable CGI script that prints body. Its source
// carries cgiSourceMarker and its output does not, so a response tells
// "executed" apart from "disclosed" on its own.
func writeCGIScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n# " + cgiSourceMarker + "\n" +
		"printf 'Content-Type: text/plain\\r\\n\\r\\n'\n" +
		"printf '" + body + "\\n'\n"
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
}

// writeWSScript writes an executable script that echoes stdin back, with
// wsSourceMarker in its source only — so a WebSocket session proves the
// script ran, and an HTTP body containing the marker proves it leaked.
func writeWSScript(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\n" +
		"# " + wsSourceMarker + "\n" +
		"while read line; do echo \"got $line\"; done\n"
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
}

// mustSymlink creates link -> target or fails the test.
func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// assertSameFile fails unless a and b really are one file, so an assertion
// about a URL being served is an assertion about the file the operator
// meant, not about whatever happened to answer.
func assertSameFile(t *testing.T, a, b string) {
	t.Helper()
	fa, err := os.Stat(a)
	if err != nil {
		t.Fatalf("stat %q: %v", a, err)
	}
	fb, err := os.Stat(b)
	if err != nil {
		t.Fatalf("stat %q: %v", b, err)
	}
	if !os.SameFile(fa, fb) {
		t.Fatalf("fixture cannot exhibit the property: %q and %q are different files", a, b)
	}
}

// caseFoldingFS reports whether dir is on a case-folding filesystem, which
// decides whether the case-spelling cases can exhibit the property at all.
func caseFoldingFS(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "CaseProbe")
	if err := os.MkdirAll(probe, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	fi, err := os.Stat(filepath.Join(dir, "caseprobe"))
	return err == nil && fi.IsDir()
}

// cgiSite is the layout the #453 family shares: a CGI directory nested inside
// a static root, a look-alike sibling that must not be diverted into the CGI
// handler, and traversal targets that live outside the root.
//
//	<Root>/releases/v1/index.html              "the index page"
//	<Root>/releases/v1/cgi-bin/hello.sh        executable, prints "hello-from-cgi"
//	<Root>/releases/v1/cgi-bindings/note.txt   "ordinary static file"
//	<Root>/outside/evil.sh                     executable, prints "PWNED"
//	<Root>/outside/secret.txt                  "PWNED-secret"
//
// Callers add whatever symlink their own case needs.
type cgiSite struct {
	Root   string // the temp dir everything hangs off
	Static string // the real static root
	Cgi    string // the real CGI directory, inside Static
}

func newCGISite(t *testing.T) cgiSite {
	t.Helper()
	site := cgiSite{Root: t.TempDir()}
	site.Static = filepath.Join(site.Root, "releases", "v1")
	site.Cgi = filepath.Join(site.Static, "cgi-bin")
	writeCGIScript(t, filepath.Join(site.Cgi, "hello.sh"), "hello-from-cgi")
	writeStaticFile(t, filepath.Join(site.Static, "index.html"), "the index page")
	writeStaticFile(t, filepath.Join(site.Static, "cgi-bindings", "note.txt"), "ordinary static file")
	writeCGIScript(t, filepath.Join(site.Root, "outside", "evil.sh"), "PWNED")
	writeStaticFile(t, filepath.Join(site.Root, "outside", "secret.txt"), "PWNED-secret")
	return site
}

// httpGetter is s.HTTPGet or s.HTTPGetFollow. Which one a test uses is part
// of what it asserts — HTTPGet refuses to follow a redirect — so the choice
// stays at the call site rather than being fixed inside the helpers below.
type httpGetter func(string) (*http.Response, string)

// assertNotSource fails if body is a script's source rather than its output.
// Every script these fixtures write carries a marker in its source and never
// in its output precisely so this can be decided from any single response,
// and every helper below checks it on every response it sees: handing out
// source instead of running it is the shape of #453 and of the --dir
// disclosure that followed it.
func assertNotSource(t *testing.T, path string, status int, body string) {
	t.Helper()
	for _, marker := range []string{cgiSourceMarker, wsSourceMarker} {
		if strings.Contains(body, marker) {
			t.Errorf("SECURITY: GET %s disclosed script source (status %d): %q", path, status, body)
		}
	}
}

// requireServes is a positive control, and it aborts rather than merely
// failing: every refusal asserted after it is only meaningful if the server
// was serving the intended root in the first place. Without one, a
// regression to a blanket 404 reads as a security improvement. subject names
// the configuration under test.
func requireServes(t *testing.T, get httpGetter, subject, path, want string) {
	t.Helper()
	resp, body := get(path)
	assertNotSource(t, path, resp.StatusCode, body)
	if resp.StatusCode != 200 || !strings.Contains(body, want) {
		t.Fatalf("control GET %s = %d %q, want 200 containing %q: %s is not serving "+
			"the intended root, so the assertions below would pass vacuously",
			path, resp.StatusCode, body, want, subject)
	}
}

// assertServes is requireServes as a non-fatal expectation, for the cases
// where being served *is* the property under test rather than a precondition
// for it. why says what makes this URL one that must answer.
//
// The source check matters most here: a CGI script's source contains the
// text its output prints, so "200 with the expected output" does not on its
// own distinguish a script that ran from one that was handed over.
func assertServes(t *testing.T, get httpGetter, why, path, want string) {
	t.Helper()
	resp, body := get(path)
	assertNotSource(t, path, resp.StatusCode, body)
	if resp.StatusCode != 200 || !strings.Contains(body, want) {
		t.Errorf("GET %s = %d %q, want 200 containing %q: %s", path, resp.StatusCode, body, want, why)
	}
}

// assertNoLeak fails if any target's response body carries any of markers —
// every marker is looked for in every response, so the pairing is the same
// one a hand-rolled loop makes. violation says what a leak would mean
// ("executed a script from outside --cgidir"), so a failure names the defect
// rather than a row number. Pass nil markers to assert only that no script
// source came back.
//
// Requests go out raw, without client-side dot-segment normalization, which
// would otherwise rewrite away exactly the traversal being tested.
func assertNoLeak(t *testing.T, s *Server, violation string, markers []string, targets ...string) {
	t.Helper()
	for _, target := range targets {
		resp, body := rawResolve(t, s, target)
		checkNoLeak(t, violation, markers, target, resp.StatusCode, body)
	}
}

// assertRefused is assertNoLeak plus the status half of a refusal: the
// response must not be 200. The two halves are reported separately, so a run
// says which one broke.
func assertRefused(t *testing.T, s *Server, violation string, markers []string, targets ...string) {
	t.Helper()
	for _, target := range targets {
		resp, body := rawResolve(t, s, target)
		checkNoLeak(t, violation, markers, target, resp.StatusCode, body)
		if resp.StatusCode == 200 {
			t.Errorf("GET %s = 200 %q, want a refusal: %s", target, body, violation)
		}
	}
}

func checkNoLeak(t *testing.T, violation string, markers []string, target string, status int, body string) {
	t.Helper()
	assertNotSource(t, target, status, body)
	for _, marker := range markers {
		if strings.Contains(body, marker) {
			t.Errorf("SECURITY: %s: GET %s (status %d): %q", violation, target, status, body)
		}
	}
}

// rawHTTPGet sends a request line verbatim, without any client-side URL
// normalization, and returns the response. Go's http.Client (like curl)
// rewrites dot segments before sending, which hides exactly the traversal
// these tests are about.
func rawHTTPGet(t *testing.T, port int, requestTarget string) *http.Response {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	req := "GET " + requestTarget + " HTTP/1.1\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n"
	if _, err := io.WriteString(conn, req); err != nil {
		t.Fatalf("write request: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response for %q: %v", requestTarget, err)
	}
	// Drain the body before the deferred conn.Close() discards whatever is
	// still in flight. Without this the caller reads from a closed socket and
	// silently gets a truncated body - which makes any "body does not contain
	// X" assertion vacuous for responses larger than one read.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body for %q: %v", requestTarget, err)
	}
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return string(body)
}

// rawResolve sends target verbatim to s and returns the response and body.
func rawResolve(t *testing.T, s *Server, target string) (*http.Response, string) {
	t.Helper()
	resp := rawHTTPGet(t, s.Port, target)
	return resp, readBody(t, resp)
}
