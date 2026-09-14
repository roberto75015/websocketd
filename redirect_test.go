// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestRedirectAddress(t *testing.T) {
	cases := []struct {
		addr    string
		redirTo int
		want    string
		wantErr bool
	}{
		{addr: "127.0.0.1:8090", redirTo: 8098, want: "127.0.0.1:8098"},
		{addr: ":8090", redirTo: 8098, want: ":8098"},
		// Bracketed IPv6 literals: the first colon is inside the brackets, so
		// naive first-colon splitting produced a malformed address that failed
		// to bind (and killed every listener).
		{addr: "[::1]:8090", redirTo: 8098, want: "[::1]:8098"},
		{addr: "example.com:80", redirTo: 443, want: "example.com:443"},
		{addr: "no-port-here", redirTo: 8098, wantErr: true},
	}
	for _, c := range cases {
		got, err := redirectAddress(c.addr, c.redirTo)
		if c.wantErr {
			if err == nil {
				t.Errorf("redirectAddress(%q) = %q, want error", c.addr, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("redirectAddress(%q) unexpected error: %v", c.addr, err)
			continue
		}
		if got != c.want {
			t.Errorf("redirectAddress(%q, %d) = %q, want %q", c.addr, c.redirTo, got, c.want)
		}
	}
}

func TestRedirectLocation(t *testing.T) {
	cases := []struct {
		clientHost string
		listenAddr string
		ssl        bool
		want       string
	}{
		{clientHost: "example.com", listenAddr: "127.0.0.1:8090", want: "http://example.com:8090/"},
		// client's own port is discarded, canonical server port wins
		{clientHost: "example.com:9999", listenAddr: "127.0.0.1:8090", want: "http://example.com:8090/"},
		{clientHost: "example.com", listenAddr: "127.0.0.1:8090", ssl: true, want: "https://example.com:8090/"},
		// IPv6 client host must stay bracketed in the Location header
		{clientHost: "[::1]:9999", listenAddr: "[::1]:8090", want: "http://[::1]:8090/"},
		{clientHost: "[::1]", listenAddr: "[::1]:8090", want: "http://[::1]:8090/"},
	}
	for _, c := range cases {
		if got := redirectLocation(c.clientHost, c.listenAddr, c.ssl); got != c.want {
			t.Errorf("redirectLocation(%q, %q, %v) = %q, want %q", c.clientHost, c.listenAddr, c.ssl, got, c.want)
		}
	}
}

// TestRedirectTarget covers the Location header the redirect server actually
// sends. redirectLocation above supplies the origin; redirectTarget adds the
// path and query the client asked for, so that a link into the site does not
// land on the front page.
//
// The request targets are run through url.ParseRequestURI, which is exactly
// what net/http does to the request line, so r.URL here has the same shape the
// handler sees.
func TestRedirectTarget(t *testing.T) {
	cases := []struct {
		name       string
		clientHost string
		listenAddr string
		ssl        bool
		target     string
		want       string
	}{
		// The path and query are what the flag is for: a client that asked
		// for a page must arrive at that page.
		{"root", "example.com", "127.0.0.1:8090", false, "/", "http://example.com:8090/"},
		{"deep path with query", "example.com", "127.0.0.1:8090", false, "/docs/page.html?q=1", "http://example.com:8090/docs/page.html?q=1"},
		{"path only", "example.com", "127.0.0.1:8090", false, "/a/b/c", "http://example.com:8090/a/b/c"},
		{"trailing slash kept", "example.com", "127.0.0.1:8090", false, "/dir/", "http://example.com:8090/dir/"},
		{"query only", "example.com", "127.0.0.1:8090", false, "/?q=1&r=2", "http://example.com:8090/?q=1&r=2"},
		{"empty query kept", "example.com", "127.0.0.1:8090", false, "/p?", "http://example.com:8090/p?"},
		{"client port still discarded", "example.com:9999", "127.0.0.1:8090", false, "/a", "http://example.com:8090/a"},
		{"ssl scheme with path", "example.com", "127.0.0.1:8090", true, "/a?b=c", "https://example.com:8090/a?b=c"},
		{"ipv6 host with path", "[::1]:9999", "[::1]:8090", false, "/a/b?c=d", "http://[::1]:8090/a/b?c=d"},
		{"ipv6 host no client port", "[::1]", "[::1]:8090", false, "/a", "http://[::1]:8090/a"},

		// A path beginning "//" is the classic way an appended request target
		// turns a Location into a protocol-relative URL. It cannot here: the
		// authority is already present, so "//evil.com" stays a path.
		{"protocol relative", "example.com", "127.0.0.1:8090", false, "//evil.com/", "http://example.com:8090//evil.com/"},
		{"protocol relative triple", "example.com", "127.0.0.1:8090", false, "///evil.com/", "http://example.com:8090///evil.com/"},
		{"backslash authority", "example.com", "127.0.0.1:8090", false, "/\\evil.com/", "http://example.com:8090/%5Cevil.com/"},
		{"encoded slashes not decoded", "example.com", "127.0.0.1:8090", false, "/%2f%2fevil.com/", "http://example.com:8090/%2f%2fevil.com/"},

		// Header injection: percent-encoded control characters must stay
		// percent-encoded rather than being decoded into the header value.
		{"encoded CRLF stays encoded", "example.com", "127.0.0.1:8090", false, "/a%0d%0aX-Injected:%20yes", "http://example.com:8090/a%0d%0aX-Injected:%20yes"},
		{"encoded NUL stays encoded", "example.com", "127.0.0.1:8090", false, "/a%00b", "http://example.com:8090/a%00b"},

		// Dot segments: literal ones are resolved (and cannot climb above the
		// root); encoded ones are not decoded into new ones.
		{"dot segments resolved", "example.com", "127.0.0.1:8090", false, "/a/../../etc/passwd", "http://example.com:8090/etc/passwd"},
		{"encoded dots kept encoded", "example.com", "127.0.0.1:8090", false, "/a/%2e%2e/%2e%2e/etc/passwd", "http://example.com:8090/a/%2e%2e/%2e%2e/etc/passwd"},
		{"doubly encoded dots", "example.com", "127.0.0.1:8090", false, "/a/%252e%252e/b", "http://example.com:8090/a/%252e%252e/b"},

		// Queries pass through byte for byte: no re-encoding, no truncation at
		// a "#" or a second "?".
		{"fragment in query", "example.com", "127.0.0.1:8090", false, "/p?a=1#frag", "http://example.com:8090/p?a=1#frag"},
		{"encoded hash in query", "example.com", "127.0.0.1:8090", false, "/p?a=%231", "http://example.com:8090/p?a=%231"},
		{"second question mark", "example.com", "127.0.0.1:8090", false, "/p?a=1?b=2", "http://example.com:8090/p?a=1?b=2"},
		{"already encoded not doubled", "example.com", "127.0.0.1:8090", false, "/a%20b%2Fc?q=%41%26b", "http://example.com:8090/a%20b%2Fc?q=%41%26b"},

		// Non-UTF-8 bytes are escaped rather than emitted raw into a header.
		{"non utf8 byte escaped", "example.com", "127.0.0.1:8090", false, "/a\xffb", "http://example.com:8090/a%FFb"},

		// An absolute-form request target parses with a host of its own. Only
		// its path and query are used; the host in the Location still comes
		// from the Host header the server resolved.
		{"absolute form target", "example.com", "127.0.0.1:8090", false, "http://other.example/zzz?k=v", "http://example.com:8090/zzz?k=v"},

		// An opaque target ("http:foo") has no path at all. Reading Path and
		// RawQuery degrades to the origin; r.URL.RequestURI() would instead
		// splice "http:foo" into the middle of the Location.
		{"opaque target", "example.com", "127.0.0.1:8090", false, "http:foo", "http://example.com:8090/"},

		// A nil request URL (never produced by net/http, but the function is
		// exported to the handler) must not panic.
		{"nil request url", "example.com", "127.0.0.1:8090", false, "", "http://example.com:8090/"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var reqURL *url.URL
			if c.target != "" {
				var err error
				reqURL, err = url.ParseRequestURI(c.target)
				if err != nil {
					t.Fatalf("fixture cannot exhibit the property: url.ParseRequestURI(%q) failed: %v", c.target, err)
				}
			}
			got := redirectTarget(c.clientHost, c.listenAddr, c.ssl, reqURL)
			if got != c.want {
				t.Errorf("redirectTarget(%q, %q, %v, %q)\n  = %q\nwant %q",
					c.clientHost, c.listenAddr, c.ssl, c.target, got, c.want)
			}
		})
	}
}

// TestRedirectTargetKeepsOwnAuthority is the open-redirect guard proper. For
// every hostile request target, the Location must parse as an absolute URL on
// the host the client itself sent - never on a host the request target names,
// and never with a control character in it.
//
// The final block is the case that must not trigger the guard: a legitimate
// deep link is not flagged. Without it, a function that returned a bare
// "http://example.com:8090/" for everything would pass this test.
func TestRedirectTargetKeepsOwnAuthority(t *testing.T) {
	const clientHost = "example.com"
	const listenAddr = "127.0.0.1:8090"
	const wantHost = "example.com:8090"

	hostile := []string{
		"//evil.com/",
		"///evil.com/",
		"////evil.com/",
		"/\\evil.com/",
		"/\\\\evil.com/",
		"//evil.com:80/a?b=c",
		"//user:pass@evil.com/",
		"/%2f%2fevil.com/",
		"/a%0d%0aLocation:%20http://evil.com/",
		"/a%00b",
		"/a/../../../../evil",
		"/a/%2e%2e/%2e%2e/b",
		"/p?a=1#@evil.com",
		"/p?a=1&b=//evil.com",
		"/@evil.com/",
		"/a\xffb",
		"http://other.example//evil.com/",
	}

	for _, target := range hostile {
		reqURL, err := url.ParseRequestURI(target)
		if err != nil {
			t.Fatalf("fixture cannot exhibit the property: url.ParseRequestURI(%q) failed: %v", target, err)
		}
		got := redirectTarget(clientHost, listenAddr, false, reqURL)

		if strings.ContainsAny(got, "\r\n\x00") {
			t.Errorf("target %q produced a Location with a control character: %q", target, got)
			continue
		}
		u, err := url.Parse(got)
		if err != nil {
			t.Errorf("target %q produced an unparseable Location %q: %v", target, got, err)
			continue
		}
		if u.Scheme != "http" {
			t.Errorf("target %q produced Location %q with scheme %q, want %q", target, got, u.Scheme, "http")
		}
		if u.User != nil {
			t.Errorf("target %q produced Location %q carrying userinfo %q", target, got, u.User)
		}
		if u.Host != wantHost {
			t.Errorf("target %q produced Location %q on host %q, want %q - open redirect",
				target, got, u.Host, wantHost)
		}
	}

	// The companion that keeps the guard above from passing vacuously: a
	// benign deep link must survive intact, so a redirectTarget that threw
	// every path away (or escaped the whole thing) would fail here.
	reqURL, err := url.ParseRequestURI("/docs/page.html?q=1")
	if err != nil {
		t.Fatalf("fixture cannot exhibit the property: %v", err)
	}
	if got, want := redirectTarget(clientHost, listenAddr, false, reqURL), "http://example.com:8090/docs/page.html?q=1"; got != want {
		t.Errorf("benign deep link\n  = %q\nwant %q", got, want)
	}
}
