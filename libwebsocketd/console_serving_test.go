// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libwebsocketd

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
)

// This file guards the dev console's HTTP-serving contract laid out in
// ASKS.md A1: a strict CSP with hashes that actually match the served
// content, no-sniff, a byte-identical response regardless of request
// target (the structural fix that retires the old {{addr}} substitution
// and the reflected-XSS class it carried), a size budget, no external
// resource references, and basic HTML well-formedness. It deliberately
// does not touch console.html, console.go or http.go — those belong to
// the implementation lane.
//
// It only exercises the stable public surface (NewWebsocketdServer's
// http.Handler), not any unexported serving function, so it keeps working
// whatever the implementation lane calls its internals.

// newDevConsoleTestServer starts a real HTTP server (httptest.NewServer,
// on an OS-assigned free port) wrapping a WebsocketdServer configured the
// way `websocketd --devconsole <cmd>` would be, minus a real backend
// command: the console-serving path never touches CommandName, and leaving
// it empty makes serveWebSocket's upgrade check short-circuit so every
// plain GET falls through to the console.
func newDevConsoleTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	config := &Config{
		DevConsole:  true,
		StartupTime: time.Now(),
	}
	log := RootLogScope(LogError, func(*LogScope, LogLevel, string, string, string, ...interface{}) {})
	handler := NewWebsocketdServer(config, log, 0)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// getConsole issues one GET against srv, optionally overriding the Host
// header (empty leaves the client's default), and returns the response
// with its body fully read.
func getConsole(t *testing.T, srv *httptest.Server, path, host string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("building request for %s: %v", path, err)
	}
	if host != "" {
		req.Host = host
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body of %s: %v", path, err)
	}
	return resp, body
}

// ---- CSP parsing -----------------------------------------------------

// parseCSP splits a Content-Security-Policy header value into
// directive -> tokens, e.g. "script-src 'self' 'sha256-x'" ->
// {"script-src": ["'self'", "'sha256-x'"]}.
func parseCSP(header string) map[string][]string {
	directives := map[string][]string{}
	for _, part := range strings.Split(header, ";") {
		fields := strings.Fields(part)
		if len(fields) == 0 {
			continue
		}
		directives[fields[0]] = fields[1:]
	}
	return directives
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func containsAll(list []string, wants ...string) bool {
	for _, w := range wants {
		if !contains(list, w) {
			return false
		}
	}
	return true
}

var sha256TokenRe = regexp.MustCompile(`^'sha256-[A-Za-z0-9+/]+=*'$`)

func hasSha256Token(list []string) bool {
	for _, v := range list {
		if sha256TokenRe.MatchString(v) {
			return true
		}
	}
	return false
}

// TestDevConsoleSecurityHeaders pins the header contract from ASKS.md A1:
// "Strict CSP: default-src 'none', script and style pinned by SHA-256 hash
// computed at init, connect-src ws: wss: ..., frame-ancestors 'none', plus
// X-Content-Type-Options: nosniff. There are currently no security headers
// on the console response." That last sentence is the failing input: main
// today sends none of this.
//
// The CSP checks parse the header into directives (parseCSP) rather than
// substring-matching the raw string: a substring match can pass against a
// malformed policy that happens to contain the right words in the wrong
// place (e.g. "script-src 'none'; default-src 'sha256-x'" would satisfy
// naive Contains checks for both "default-src 'none'" reordered text and a
// script-src hash, while actually being backwards). This absorbs three
// checks that used to live in a same-named test in console_test.go
// (Content-Type, ETag strength, unsafe-eval) so the two never diverge.
func TestDevConsoleSecurityHeaders(t *testing.T) {
	srv := newDevConsoleTestServer(t)
	resp, _ := getConsole(t, srv, "/", "")

	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Errorf("Content-Type = %q, want text/html...", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want %q", got, "nosniff")
	}
	if etag := resp.Header.Get("ETag"); etag == "" {
		t.Error("no ETag on the console response; the body is a constant and should carry a strong validator")
	} else if strings.HasPrefix(etag, "W/") {
		t.Errorf("ETag %q is weak; the body is byte-stable so the validator should be strong", etag)
	}

	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("no Content-Security-Policy header on the dev console response; " +
			"a browser is currently trusted to sandbox arbitrary content received " +
			"from whatever WebSocket endpoint the user points the console at")
	}
	d := parseCSP(csp)

	if got := d["default-src"]; len(got) != 1 || got[0] != "'none'" {
		t.Errorf("default-src = %v, want [\"'none'\"]", got)
	}
	if got := d["frame-ancestors"]; len(got) != 1 || got[0] != "'none'" {
		t.Errorf("frame-ancestors = %v, want [\"'none'\"]", got)
	}
	if connect := d["connect-src"]; !containsAll(connect, "ws:", "wss:") {
		t.Errorf("connect-src = %v, want it to permit both ws: and wss: "+
			"(the user may type either kind of URL into #url)", connect)
	}
	if !hasSha256Token(d["script-src"]) {
		t.Errorf("script-src = %v, want a 'sha256-...' pinned hash", d["script-src"])
	}
	if !hasSha256Token(d["style-src"]) {
		t.Errorf("style-src = %v, want a 'sha256-...' pinned hash", d["style-src"])
	}
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP %q loosens script/style execution", csp)
	}
}

// ---- CSP hash correctness --------------------------------------------

var scriptTagRe = regexp.MustCompile(`(?is)<script([^>]*)>(.*?)</script>`)
var styleTagRe = regexp.MustCompile(`(?is)<style([^>]*)>(.*?)</style>`)
var srcAttrRe = regexp.MustCompile(`(?i)\bsrc\s*=`)

// inlineElementHashes finds every <script>/<style> element in body whose
// content is inline (no src= attribute, which would make it an external
// reference rather than something the CSP hash mechanism covers) and
// returns the CSP 'sha256-...' token for each, computed independently from
// the exact bytes browsers hash: everything between the tag's '>' and its
// own '</tag>', unmodified.
func inlineElementHashes(body []byte, tagRe *regexp.Regexp) []string {
	var hashes []string
	for _, m := range tagRe.FindAllSubmatch(body, -1) {
		attrs, content := m[1], m[2]
		if srcAttrRe.Match(attrs) {
			continue
		}
		sum := sha256.Sum256(content)
		hashes = append(hashes, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
	}
	return hashes
}

// TestDevConsoleCSPHashesMatchBody recomputes the SHA-256 of every inline
// <script>/<style> element straight from the response body and checks it
// against the CSP header from the *same* response — not against any
// constant this test file or the production code holds. A stale or
// mismatched hash (e.g. CSP computed once at init but the body edited
// afterwards, or hashed over the wrong byte range) is the failing input.
func TestDevConsoleCSPHashesMatchBody(t *testing.T) {
	srv := newDevConsoleTestServer(t)
	resp, body := getConsole(t, srv, "/", "")
	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("no CSP header to check hashes against (see TestDevConsoleSecurityHeaders)")
	}
	d := parseCSP(csp)

	scriptHashes := inlineElementHashes(body, scriptTagRe)
	if len(scriptHashes) == 0 {
		t.Fatal("no inline <script> element found in the served body to hash")
	}
	for _, h := range scriptHashes {
		if !contains(d["script-src"], h) {
			t.Errorf("script-src does not list %s, the SHA-256 independently "+
				"recomputed from the actual served <script> content; script-src = %v",
				h, d["script-src"])
		}
	}

	styleHashes := inlineElementHashes(body, styleTagRe)
	if len(styleHashes) == 0 {
		t.Fatal("no inline <style> element found in the served body to hash")
	}
	for _, h := range styleHashes {
		if !contains(d["style-src"], h) {
			t.Errorf("style-src does not list %s, the SHA-256 independently "+
				"recomputed from the actual served <style> content; style-src = %v",
				h, d["style-src"])
		}
	}
}

// TestInlineElementHashesCatchesTamperedContent proves the recomputation
// machinery itself is honest, independent of whether the live server has a
// CSP at all yet: it hashes fabricated markup and checks the result is the
// *correct* hash of the real content, then shows that comparing it against
// an unrelated ("stale") hash is correctly detected as a mismatch. Answers
// "what input would make this fail": a script/style body edited after the
// CSP hash was computed.
func TestInlineElementHashesCatchesTamperedContent(t *testing.T) {
	body := []byte(`<html><head><style>body{color:red}</style></head>` +
		`<body><script>console.log("hi")</script></body></html>`)

	scriptHashes := inlineElementHashes(body, scriptTagRe)
	if len(scriptHashes) != 1 {
		t.Fatalf("expected exactly one script hash, got %v", scriptHashes)
	}
	wantSum := sha256.Sum256([]byte(`console.log("hi")`))
	want := "'sha256-" + base64.StdEncoding.EncodeToString(wantSum[:]) + "'"
	if scriptHashes[0] != want {
		t.Fatalf("inlineElementHashes computed %s, want %s (hand-computed over the exact script bytes)", scriptHashes[0], want)
	}

	staleSum := sha256.Sum256([]byte("stale content"))
	staleCSP := []string{"'sha256-" + base64.StdEncoding.EncodeToString(staleSum[:]) + "'"}
	if contains(staleCSP, scriptHashes[0]) {
		t.Fatal("a stale/unrelated hash was wrongly accepted as matching the real script content")
	}
}

// ---- size budget --------------------------------------------------------
//
// Request-independence has its own test in console_test.go
// (TestDevConsoleBodyIsRequestIndependent), which is the stronger of two
// versions this file originally also carried: it additionally covers a raw
// quote-breakout injection in the request target (net/http's client would
// otherwise percent-encode the very bytes that make the old {{addr}} bug
// reachable, hiding it) and a bare, no-port Host, and checks the served
// body equals ConsoleContent exactly rather than only comparing responses
// to each other.

// consoleSizeBudget covers the served response, which is bigger than the
// ~25KB ASKS.md A1 named: that figure describes console.html as authored
// (see TestConsoleSizeBudget in console_test.go, which checks exactly
// that, unexpanded, and does stay under 25KB), but every response also
// carries the full BSD license text expanded into the leading HTML comment
// in place of the tiny {{license}} placeholder - roughly 1.3KB, and true
// of the console this replaced as well, not something this rebuild added.
// 30KB keeps a real ceiling (something that would actually catch bloat)
// while being honest about what is actually sent over the wire.
const consoleSizeBudget = 30 * 1024

func TestDevConsoleSizeBudget(t *testing.T) {
	srv := newDevConsoleTestServer(t)
	_, body := getConsole(t, srv, "/", "")
	if len(body) > consoleSizeBudget {
		t.Errorf("console response is %d bytes, over the %d byte budget", len(body), consoleSizeBudget)
	}
	if len(body) == 0 {
		t.Fatal("console response body is empty")
	}
}

// ---- absence of external resources --------------------------------------

// findExternalResourceRefs scans HTML/CSS text for src=/href= attributes,
// CSS url(...), and @import that point at an absolute external origin
// (http://, https://) or a protocol-relative one (//host/...). It
// deliberately does not flag bare "http://" text that isn't in one of
// those resource-loading positions (e.g. a URL mentioned in a comment),
// since ASKS.md A1's requirement is "no external URL ... resource
// references" — a load, not a text mention. Go's RE2 has no backreferences,
// so each quote style gets its own pattern rather than one with \1.
func findExternalResourceRefs(body []byte) []string {
	var found []string
	check := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || strings.HasPrefix(v, "#") || strings.HasPrefix(v, "data:") {
			return
		}
		if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "//") {
			found = append(found, v)
		}
	}
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:src|href)\s*=\s*"([^"]*)"`),
		regexp.MustCompile(`(?i)\b(?:src|href)\s*=\s*'([^']*)'`),
		regexp.MustCompile(`(?i)url\(\s*"([^"]*)"\s*\)`),
		regexp.MustCompile(`(?i)url\(\s*'([^']*)'\s*\)`),
		regexp.MustCompile(`(?i)url\(\s*([^"'()\s][^)]*)\s*\)`),
		regexp.MustCompile(`(?i)@import\s+"([^"]*)"`),
		regexp.MustCompile(`(?i)@import\s+'([^']*)'`),
	} {
		for _, m := range re.FindAllSubmatch(body, -1) {
			check(string(m[1]))
		}
	}
	return found
}

func TestFindExternalResourceRefsDetectsInjectedRemote(t *testing.T) {
	cases := []struct{ name, body string }{
		{"script src", `<script src="https://evil.example/x.js"></script>`},
		{"protocol-relative img", `<img src="//evil.example/pixel.gif">`},
		{"css import double-quoted", `<style>@import "http://evil.example/x.css";</style>`},
		{"css url() unquoted", `<style>body{background:url(http://evil.example/bg.png)}</style>`},
		{"link stylesheet", `<link rel="stylesheet" href="https://evil.example/x.css">`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := findExternalResourceRefs([]byte(c.body)); len(got) == 0 {
				t.Errorf("findExternalResourceRefs did not flag an external reference in: %s", c.body)
			}
		})
	}
}

func TestFindExternalResourceRefsIgnoresBenignRefs(t *testing.T) {
	cases := []struct{ name, body string }{
		{"fragment href", `<a href="#detail">jump</a>`},
		{"data uri icon", `<link rel="icon" href="data:image/png;base64,AAAA">`},
		{"license text mention, not a resource load", `<!-- Full documentation at https://websocketd.com/ -->`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := findExternalResourceRefs([]byte(c.body)); len(got) != 0 {
				t.Errorf("findExternalResourceRefs false-flagged a benign reference: %v", got)
			}
		})
	}
}

// TestDevConsoleNoExternalResources pins ASKS.md A1's "Single embedded
// file, zero dependencies, no build step, no network requests" and the
// explicit test list item "absence of external URLs."
func TestDevConsoleNoExternalResources(t *testing.T) {
	srv := newDevConsoleTestServer(t)
	_, body := getConsole(t, srv, "/", "")
	if got := findExternalResourceRefs(body); len(got) != 0 {
		t.Errorf("console body references external resources, violating the zero-dependency/no-network-request requirement: %v", got)
	}
}

// ---- HTML well-formedness ------------------------------------------------
//
// golang.org/x/net/html would be the obvious tool here, but it is not
// already a dependency of this module (checked via `go list -m all` before
// writing this file: only golang.org/x/sys appears, pulled in by chromedp
// for qa/browser) and the task instructions are explicit that it may only
// be used if it is already an indirect dependency. Adding a new module just
// for this one check was judged not worth doing, so this is a hand-rolled,
// real tag-balance scanner instead of a full HTML5 tokenizer. It:
//   - strips comments before scanning, so URLs or markup-looking text
//     inside them is never mistaken for a tag;
//   - treats <script>/<style> as raw-text elements, so a "<" in JS/CSS
//     (e.g. `if (a < b)`) is not parsed as markup;
//   - understands the standard HTML5 void elements (br, img, input, ...)
//     which never need a closing tag;
//   - requires literal, explicit <html>, <head> and <body> opening tags —
//     not merely the implicit elements browsers synthesize — because the
//     DOM contract requires attributes on <html> (data-theme) that only
//     exist if it is written explicitly in the source.
//
// Known limitation: like most regexp-based tag matchers, it does not
// handle a literal '>' character appearing inside a quoted attribute value
// (real tokenizers track quote state; this one does not). The console
// source is not expected to need that, and no case in this file's tests
// exercises it.
var wellformedTagRe = regexp.MustCompile(`(?s)<(/?)([a-zA-Z][a-zA-Z0-9-]*)([^>]*)>`)
var wellformedCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

var rawTextElements = map[string]bool{"script": true, "style": true}

// checkHTMLWellformed returns a list of human-readable problems, or nil if
// none were found.
func checkHTMLWellformed(body []byte) []string {
	var problems []string
	s := wellformedCommentRe.ReplaceAllString(string(body), "")

	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(strings.ToLower(trimmed), "<!doctype html>") {
		problems = append(problems, "document does not start with <!DOCTYPE html>")
	}

	var stack []string
	openedTagNames := map[string]bool{} // exact tag names seen opened, for the required-tag check below
	pos := 0
	for pos < len(s) {
		loc := wellformedTagRe.FindStringSubmatchIndex(s[pos:])
		if loc == nil {
			break
		}
		full := s[pos+loc[0] : pos+loc[1]]
		closing := s[pos+loc[2]:pos+loc[3]] == "/"
		name := strings.ToLower(s[pos+loc[4] : pos+loc[5]])
		attrs := s[pos+loc[6] : pos+loc[7]]
		selfClosed := strings.HasSuffix(strings.TrimSpace(attrs), "/")
		next := pos + loc[1]

		switch {
		case closing:
			if len(stack) == 0 || stack[len(stack)-1] != name {
				problems = append(problems, fmt.Sprintf("closing tag %s does not match currently open element (open stack: %v)", full, stack))
			} else {
				stack = stack[:len(stack)-1]
			}
		case voidElements[name] || selfClosed:
			// nothing pushed; these never need a closing tag.
			openedTagNames[name] = true
		default:
			stack = append(stack, name)
			openedTagNames[name] = true
			if rawTextElements[name] {
				closeTag := "</" + name
				idx := strings.Index(strings.ToLower(s[next:]), closeTag)
				if idx < 0 {
					problems = append(problems, fmt.Sprintf("<%s> is opened but never closed", name))
					pos = len(s)
					continue
				}
				// Skip the raw-text content; the closing tag itself is
				// picked up on the next loop iteration and pops the stack.
				next += idx
			}
		}
		pos = next
	}
	if len(stack) > 0 {
		problems = append(problems, fmt.Sprintf("unclosed element(s) at end of document: %v", stack))
	}

	// Exact tag-name membership, not a substring search: "<header>" must
	// not satisfy a requirement for "<head>", and "<htmlx>" (not a real
	// tag name per wellformedTagRe's char class anyway) must not satisfy
	// "<html>".
	for _, required := range []string{"html", "head", "body"} {
		if !openedTagNames[required] {
			problems = append(problems, fmt.Sprintf(
				"no literal <%s> tag found; the DOM contract requires an explicit <html data-theme=\"auto|light|dark\"> element, not one browsers synthesize implicitly", required))
		}
	}
	return problems
}

func TestCheckHTMLWellformedDetectsMismatchedTags(t *testing.T) {
	bad := `<!DOCTYPE html><html><head></head><body><div><span></div></body></html>`
	if problems := checkHTMLWellformed([]byte(bad)); len(problems) == 0 {
		t.Fatal("checkHTMLWellformed did not flag mismatched <div>/<span> nesting")
	}
}

func TestCheckHTMLWellformedDetectsUnclosedElement(t *testing.T) {
	bad := `<!DOCTYPE html><html><head></head><body><div></body></html>`
	if problems := checkHTMLWellformed([]byte(bad)); len(problems) == 0 {
		t.Fatal("checkHTMLWellformed did not flag an unclosed <div>")
	}
}

// TestCheckHTMLWellformedRejectsHeaderAsHead is a regression test for a bug
// caught while writing this file: a naive substring search for "<head"
// matches "<header class=...>", so a document with a <header> element but
// no real <head> was wrongly accepted. checkHTMLWellformed tracks exact
// opened tag names instead.
func TestCheckHTMLWellformedRejectsHeaderAsHead(t *testing.T) {
	bad := `<!DOCTYPE html><html><body><header class="header">console</header></body></html>`
	problems := checkHTMLWellformed([]byte(bad))
	found := false
	for _, p := range problems {
		if strings.Contains(p, "<head>") {
			found = true
		}
	}
	if !found {
		t.Fatalf("checkHTMLWellformed accepted a <header> element as satisfying the <head> requirement; problems = %v", problems)
	}
}

func TestCheckHTMLWellformedAcceptsWellformedDoc(t *testing.T) {
	good := `<!DOCTYPE html><html data-theme="auto"><head><title>x</title></head>` +
		`<body><script>if (1<2) { /* < in JS text, not markup */ }</script>` +
		`<style>a{color:red}</style><br><input type="text"></body></html>`
	if problems := checkHTMLWellformed([]byte(good)); len(problems) != 0 {
		t.Fatalf("checkHTMLWellformed false-flagged a well-formed doc: %v", problems)
	}
}

// TestDevConsoleHTMLWellformed pins the "HTML wellformedness" item from
// ASKS.md A1's test list.
func TestDevConsoleHTMLWellformed(t *testing.T) {
	srv := newDevConsoleTestServer(t)
	_, body := getConsole(t, srv, "/", "")
	if problems := checkHTMLWellformed(body); len(problems) > 0 {
		t.Errorf("console HTML is not well-formed:\n%s", strings.Join(problems, "\n"))
	}
}
