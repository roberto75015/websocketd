// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// rawRedirect sends requestLine verbatim to the redirect server on port and
// returns the status line and the Location header.
//
// It deliberately does not use net/http's client: that client parses and
// re-serialises the request target, so a hostile path (a protocol-relative
// "//evil.com", an embedded CR/LF, a doubled percent escape) is normalised or
// rejected before it ever reaches websocketd. A payload the client rewrote
// has tested nothing. Every byte here goes onto the socket as written.
func rawRedirect(t *testing.T, port int, requestLine, hostHeader string) (status int, statusLine, location string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 5*time.Second)
	if err != nil {
		t.Fatalf("dial redirect port %d: %v", port, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}

	raw := requestLine + "\r\nHost: " + hostHeader + "\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(raw)); err != nil {
		t.Fatalf("write raw request %q: %v", requestLine, err)
	}

	br := bufio.NewReader(conn)
	statusLineBytes, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read status line for %q: %v", requestLine, err)
	}
	statusLine = strings.TrimRight(statusLineBytes, "\r\n")

	fields := strings.SplitN(statusLine, " ", 3)
	if len(fields) < 2 {
		t.Fatalf("malformed status line %q for request %q", statusLine, requestLine)
	}
	status, err = strconv.Atoi(fields[1])
	if err != nil {
		t.Fatalf("malformed status code in %q for request %q", statusLine, requestLine)
	}

	// Read headers verbatim rather than through textproto, so that a CR or LF
	// smuggled into the Location value shows up as an extra header line here
	// instead of being folded away by a forgiving parser.
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(name), "location") {
			location = strings.TrimSpace(value)
		}
		if strings.HasPrefix(strings.ToLower(line), "x-injected") {
			t.Errorf("request %q injected a header into the response: %q", requestLine, line)
		}
	}
	return status, statusLine, location
}

// startRedirectServer starts websocketd with --redirport and returns the main
// and redirect ports. stdout and stderr are captured into separate buffers:
// which stream a message lands on is part of the behaviour, so they are never
// merged.
func startRedirectServer(t *testing.T) (mainPort, redirPort int, stderr *bytes.Buffer) {
	t.Helper()
	mainPort = freePort(t)
	redirPort = freePort(t)

	cmd := exec.Command(websocketdBin,
		"--port="+strconv.Itoa(mainPort),
		"--address=127.0.0.1",
		"--redirport="+strconv.Itoa(redirPort),
		"--loglevel=error",
		// The redirect listener has nothing to do with origin policy; silencing
		// the startup banner keeps the captured stderr readable when a case fails.
		"--anyorigin",
		testcmdBin, "echo")

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start websocketd: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("websocketd stdout:\n%s", outBuf.String())
			t.Logf("websocketd stderr:\n%s", errBuf.String())
		}
	})

	waitForPort(t, redirPort, 10*time.Second)
	return mainPort, redirPort, &errBuf
}

// TestRedirPortPreservesPathAndQuery drives the real binary: a request to the
// --redirport listener for a deep path must redirect to that same path on the
// canonical port, not to the front page.
func TestRedirPortPreservesPathAndQuery(t *testing.T) {
	t.Parallel()
	mainPort, redirPort, _ := startRedirectServer(t)
	origin := fmt.Sprintf("http://127.0.0.1:%d", mainPort)

	cases := []struct {
		name        string
		requestLine string
		want        string
	}{
		{"root", "GET / HTTP/1.1", origin + "/"},
		{"deep path with query", "GET /docs/page.html?q=1 HTTP/1.1", origin + "/docs/page.html?q=1"},
		{"path only", "GET /deeply/nested/thing HTTP/1.1", origin + "/deeply/nested/thing"},
		{"query only", "GET /?q=1&r=2 HTTP/1.1", origin + "/?q=1&r=2"},
		{"trailing slash kept", "GET /dir/ HTTP/1.1", origin + "/dir/"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, statusLine, loc := rawRedirect(t, redirPort, c.requestLine, fmt.Sprintf("127.0.0.1:%d", redirPort))
			if status != http.StatusMovedPermanently {
				t.Fatalf("request %q: got %q, want 301 - the test never reached its subject",
					c.requestLine, statusLine)
			}
			if loc != c.want {
				t.Errorf("request %q\n  Location = %q\n      want = %q", c.requestLine, loc, c.want)
			}
		})
	}
}

// TestRedirPortHostileTargets drives every hostile request target the redirect
// server can be handed. The invariant under test is narrow and absolute: the
// Location header must always be an absolute URL whose authority is this
// server's own canonical origin. http.Redirect is called with a #nosec G710
// annotation asserting this is not an open redirect; these are the cases that
// make that assertion mean something now that client-controlled bytes reach
// the header.
func TestRedirPortHostileTargets(t *testing.T) {
	t.Parallel()
	mainPort, redirPort, _ := startRedirectServer(t)
	wantHost := fmt.Sprintf("127.0.0.1:%d", mainPort)
	hostHeader := fmt.Sprintf("127.0.0.1:%d", redirPort)

	// The Go http server rejects these at the request-line parser, before any
	// handler runs. Asserting the 400 is what proves rawRedirect really put
	// unnormalised bytes on the wire: net/http's client would never have sent
	// them, so a green here is evidence the fixture can exhibit the property.
	rejected := []struct {
		name        string
		requestLine string
	}{
		{"raw CRLF in path", "GET /a\r\nX-Injected: yes\r\nZ: /b HTTP/1.1"},
		{"raw LF in path", "GET /a\nX-Injected: yes HTTP/1.1"},
		{"raw space in path", "GET /a b HTTP/1.1"},
	}
	for _, c := range rejected {
		t.Run("rejected/"+c.name, func(t *testing.T) {
			status, statusLine, loc := rawRedirect(t, redirPort, c.requestLine, hostHeader)
			if status != http.StatusBadRequest {
				t.Errorf("request %q: got %q (Location %q), want 400 from the request-line parser",
					c.requestLine, statusLine, loc)
			}
		})
	}

	// These reach the handler. Each must come back as a 301 whose Location
	// parses as an absolute URL on our own authority.
	redirected := []struct {
		name        string
		requestLine string
		want        string
	}{
		{"protocol relative", "GET //evil.com/ HTTP/1.1", "http://" + wantHost + "//evil.com/"},
		{"protocol relative triple slash", "GET ///evil.com/ HTTP/1.1", "http://" + wantHost + "///evil.com/"},
		{"backslash authority", "GET /\\evil.com/ HTTP/1.1", "http://" + wantHost + "/%5Cevil.com/"},
		{"encoded slashes", "GET /%2f%2fevil.com/ HTTP/1.1", "http://" + wantHost + "/%2f%2fevil.com/"},
		{"encoded CRLF stays encoded", "GET /a%0d%0aX-Injected:%20yes HTTP/1.1", "http://" + wantHost + "/a%0d%0aX-Injected:%20yes"},
		{"encoded NUL", "GET /a%00b HTTP/1.1", "http://" + wantHost + "/a%00b"},
		{"dot segments resolved", "GET /a/../../etc/passwd HTTP/1.1", "http://" + wantHost + "/etc/passwd"},
		{"encoded dot segments kept encoded", "GET /a/%2e%2e/%2e%2e/etc/passwd HTTP/1.1", "http://" + wantHost + "/a/%2e%2e/%2e%2e/etc/passwd"},
		{"doubly encoded dots", "GET /a/%252e%252e/b HTTP/1.1", "http://" + wantHost + "/a/%252e%252e/b"},
		{"fragment in query", "GET /p?a=1#frag HTTP/1.1", "http://" + wantHost + "/p?a=1#frag"},
		{"encoded hash in query", "GET /p?a=%231 HTTP/1.1", "http://" + wantHost + "/p?a=%231"},
		{"second question mark", "GET /p?a=1?b=2 HTTP/1.1", "http://" + wantHost + "/p?a=1?b=2"},
		{"already percent encoded", "GET /a%20b%2Fc?q=%41%26b HTTP/1.1", "http://" + wantHost + "/a%20b%2Fc?q=%41%26b"},
		{"non utf8 byte escaped", "GET /a\xffb HTTP/1.1", "http://" + wantHost + "/a%FFb"},
		{"empty query forced", "GET /p? HTTP/1.1", "http://" + wantHost + "/p?"},
	}
	for _, c := range redirected {
		t.Run("redirected/"+c.name, func(t *testing.T) {
			status, statusLine, loc := rawRedirect(t, redirPort, c.requestLine, hostHeader)
			if status != http.StatusMovedPermanently {
				t.Fatalf("request %q: got %q, want 301 - the test never reached its subject",
					c.requestLine, statusLine)
			}
			assertOwnAuthority(t, c.requestLine, loc, wantHost)
			if loc != c.want {
				t.Errorf("request %q\n  Location = %q\n      want = %q", c.requestLine, loc, c.want)
			}
		})
	}

	// Absolute-form and authority-form request targets carry a host of their
	// own in r.URL. The Go server derives r.Host from the absolute form, so
	// the redirect follows the client to the host it named - self-inflicted,
	// and the same behaviour the plain Host header already has. What must not
	// happen is the request-target host leaking into the path or the port
	// surviving unrewritten.
	t.Run("absolute form request target", func(t *testing.T) {
		status, statusLine, loc := rawRedirect(t, redirPort, "GET http://other.example/zzz?k=v HTTP/1.1", hostHeader)
		if status != http.StatusMovedPermanently {
			t.Fatalf("got %q, want 301 - the test never reached its subject", statusLine)
		}
		want := fmt.Sprintf("http://other.example:%d/zzz?k=v", mainPort)
		if loc != want {
			t.Errorf("absolute-form target\n  Location = %q\n      want = %q", loc, want)
		}
	})

	t.Run("opaque request target", func(t *testing.T) {
		// "http:foo" parses with a non-empty URL.Opaque and an empty Path.
		// r.URL.RequestURI() would splice "http:foo" into the middle of the
		// Location; reading Path/RawQuery instead degrades to the origin.
		status, statusLine, loc := rawRedirect(t, redirPort, "GET http:foo HTTP/1.1", hostHeader)
		if status != http.StatusMovedPermanently {
			t.Fatalf("got %q, want 301 - the test never reached its subject", statusLine)
		}
		assertOwnAuthority(t, "GET http:foo", loc, wantHost)
		if want := "http://" + wantHost + "/"; loc != want {
			t.Errorf("opaque target\n  Location = %q\n      want = %q", loc, want)
		}
	})
}

// ownAuthority is the open-redirect predicate: loc must be an absolute URL, on
// the wanted scheme, whose authority is exactly wantHost. A Location that a
// browser would resolve to any other origin is an error here. It is a plain
// function rather than an assertion so that it can be tested directly.
func ownAuthority(loc, wantScheme, wantHost string) error {
	if loc == "" {
		return fmt.Errorf("no Location header")
	}
	if strings.ContainsAny(loc, "\r\n\x00") {
		return fmt.Errorf("Location contains a control character: %q", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		return fmt.Errorf("Location %q does not parse: %v", loc, err)
	}
	if u.Scheme != wantScheme {
		return fmt.Errorf("Location %q has scheme %q, want %q", loc, u.Scheme, wantScheme)
	}
	if u.User != nil {
		return fmt.Errorf("Location %q carries userinfo %q, which shifts the authority", loc, u.User)
	}
	if u.Host != wantHost {
		return fmt.Errorf("Location %q points at host %q, want %q - open redirect", loc, u.Host, wantHost)
	}
	return nil
}

func assertOwnAuthority(t *testing.T, requestLine, loc, wantHost string) {
	t.Helper()
	if err := ownAuthority(loc, "http", wantHost); err != nil {
		t.Errorf("request %q: %v", requestLine, err)
	}
}

// TestOwnAuthorityBites keeps the predicate above honest. A check that only
// ever sees good input passes trivially, so these are the strings it must
// reject - and, below them, the awkward but legitimate Location it must not.
func TestOwnAuthorityBites(t *testing.T) {
	t.Parallel()
	const wantHost = "127.0.0.1:8090"
	bad := []string{
		"http://evil.com/",
		"//evil.com/",
		"https://127.0.0.1:8090/",
		"http://127.0.0.1:9999/",
		"http://127.0.0.1:8090@evil.com/",
		"http://127.0.0.1:8090/\r\nX-Injected: yes",
		"/relative/only",
		"",
	}
	for _, loc := range bad {
		if err := ownAuthority(loc, "http", wantHost); err == nil {
			t.Errorf("ownAuthority accepted %q; the predicate cannot bite", loc)
		}
	}

	good := []string{
		"http://" + wantHost + "/",
		"http://" + wantHost + "//not-an-authority/a%2fb?q=1#frag",
		"http://" + wantHost + "/a/%2e%2e/b?a=1?b=2",
	}
	for _, loc := range good {
		if err := ownAuthority(loc, "http", wantHost); err != nil {
			t.Errorf("ownAuthority rejected the legitimate Location %q: %v", loc, err)
		}
	}
}
