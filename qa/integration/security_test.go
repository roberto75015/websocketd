package integration

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestSEC001_SameOriginAccepted(t *testing.T) {
	t.Parallel()
	s := startServerOpts(t, []string{"--sameorigin"}, "echo")

	headers := http.Header{}
	headers.Set("Origin", "http://127.0.0.1:"+itoa(s.Port))
	ws, _, err := s.TryConnect("/", headers)
	if err != nil {
		t.Fatalf("same-origin connection should succeed: %v", err)
	}
	defer ws.Close()
	ws.Send("hello")
	ws.ExpectMessage("hello")
}

func TestSEC002_SameOriginRejected(t *testing.T) {
	t.Parallel()
	s := startServerOpts(t, []string{"--sameorigin"}, "echo")

	headers := http.Header{}
	headers.Set("Origin", "http://evil.com")
	_, resp, err := s.TryConnect("/", headers)
	if err == nil {
		t.Fatal("cross-origin connection should be rejected")
	}
	if resp != nil && resp.StatusCode != 403 {
		t.Logf("rejection status: %d (expected 403)", resp.StatusCode)
	}
}

func TestSEC003_OriginWhitelistAccepted(t *testing.T) {
	t.Parallel()
	s := startServerOpts(t, []string{"--origin=trusted.com"}, "echo")

	headers := http.Header{}
	headers.Set("Origin", "http://trusted.com")
	ws, _, err := s.TryConnect("/", headers)
	if err != nil {
		t.Fatalf("whitelisted origin should be accepted: %v", err)
	}
	defer ws.Close()
	ws.Send("hello")
	ws.ExpectMessage("hello")
}

func TestSEC004_OriginWhitelistRejected(t *testing.T) {
	t.Parallel()
	s := startServerOpts(t, []string{"--origin=trusted.com"}, "echo")

	headers := http.Header{}
	headers.Set("Origin", "http://untrusted.com")
	_, _, err := s.TryConnect("/", headers)
	if err == nil {
		t.Fatal("non-whitelisted origin should be rejected")
	}
}

func TestSEC005_OriginWhitelistWithPort(t *testing.T) {
	t.Parallel()
	s := startServerOpts(t, []string{"--origin=trusted.com:3000"}, "echo")

	// Correct port should work
	headers := http.Header{}
	headers.Set("Origin", "http://trusted.com:3000")
	ws, _, err := s.TryConnect("/", headers)
	if err != nil {
		t.Fatalf("correct port origin should be accepted: %v", err)
	}
	ws.Close()

	// Wrong port should be rejected
	headers = http.Header{}
	headers.Set("Origin", "http://trusted.com:4000")
	_, _, err = s.TryConnect("/", headers)
	if err == nil {
		t.Error("wrong port origin should be rejected")
	}
}

func TestSEC006_MultipleOrigins(t *testing.T) {
	t.Parallel()
	s := startServerOpts(t, []string{"--origin=a.com,b.com"}, "echo")

	for _, origin := range []string{"http://a.com", "http://b.com"} {
		headers := http.Header{}
		headers.Set("Origin", origin)
		ws, _, err := s.TryConnect("/", headers)
		if err != nil {
			t.Errorf("origin %s should be accepted: %v", origin, err)
			continue
		}
		ws.Close()
	}

	// Unlisted origin should be rejected
	headers := http.Header{}
	headers.Set("Origin", "http://c.com")
	_, _, err := s.TryConnect("/", headers)
	if err == nil {
		t.Error("unlisted origin should be rejected")
	}
}

func TestSEC007_NullOrigin(t *testing.T) {
	// Regression: v0.2.10 fixed null origin handling
	t.Parallel()
	s := startServerOpts(t, []string{"--sameorigin"}, "echo")

	headers := http.Header{}
	headers.Set("Origin", "null")
	_, _, err := s.TryConnect("/", headers)
	if err == nil {
		t.Error("null origin should be rejected with --sameorigin")
	}
}

func TestSEC008_NoOriginRestrictionDefault(t *testing.T) {
	t.Parallel()
	s := startServer(t, "echo")

	// Any origin should work when no restriction is set
	headers := http.Header{}
	headers.Set("Origin", "http://any-domain.com")
	ws, _, err := s.TryConnect("/", headers)
	if err != nil {
		t.Fatalf("should accept any origin by default: %v", err)
	}
	defer ws.Close()
	ws.Send("hello")
	ws.ExpectMessage("hello")
}

func TestSEC009_EnvironmentIsolation(t *testing.T) {
	t.Parallel()
	s := startServer(t, "env")
	ws := s.Connect("/")
	defer ws.Close()

	output := strings.Join(collectMessages(ws, 3*time.Second), "\n")

	// Standard CGI variables should be present
	if _, ok := findEnvValue(output, "SERVER_SOFTWARE"); !ok {
		t.Error("SERVER_SOFTWARE not found")
	}
	if _, ok := findEnvValue(output, "REMOTE_ADDR"); !ok {
		t.Error("REMOTE_ADDR not found")
	}

	// Parent shell variables should NOT be leaked
	sensitiveVars := []string{"HOME", "USER", "SHELL", "TERM", "LANG"}
	for _, v := range sensitiveVars {
		if _, ok := findEnvValue(output, v); ok {
			t.Errorf("parent variable %s leaked to child (env isolation failure)", v)
		}
	}
}

func TestSEC010_SSLConnection(t *testing.T) {
	t.Parallel()
	s := startServerSSL(t, nil, "echo")
	ws := s.ConnectTLS("/")
	defer ws.Close()
	ws.Send("encrypted")
	ws.ExpectMessage("encrypted")
}

func TestSEC011_CommandInjectionViaURL(t *testing.T) {
	t.Parallel()
	s := startServer(t, "env")

	// These should not execute any shell commands
	dangerousPaths := []string{
		"/;ls",
		"/$(whoami)",
		"/`id`",
		"/|cat",
	}
	// The backend is `testcmd env`, so every output line must be an
	// environment assignment. Any other shape means something else ran
	// (`ls` prints filenames, `whoami` a bare username, `id` "uid=...").
	// Checking line shape rather than substrings like "root" keeps the
	// test independent of the user account running it.
	envLine := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_()]*=`)
	for _, path := range dangerousPaths {
		ws, _, err := s.TryConnect(path, nil)
		if err != nil {
			continue // rejected is fine
		}
		// If connected, just verify no command injection happened
		for _, line := range collectMessages(ws, 2*time.Second) {
			if line == "" {
				continue
			}
			if !envLine.MatchString(line) || strings.HasPrefix(line, "uid=") {
				t.Errorf("SECURITY: possible command injection via path %q: unexpected output line %q", path, line)
			}
		}
	}
}

func TestSEC012_CommandInjectionViaQueryString(t *testing.T) {
	t.Parallel()
	s := startServer(t, "env")

	ws := s.Connect("/?$(whoami)")
	defer ws.Close()

	output := strings.Join(collectMessages(ws, 2*time.Second), "\n")
	// QUERY_STRING should contain the raw text, not executed
	if v, ok := findEnvValue(output, "QUERY_STRING"); ok {
		if !strings.Contains(v, "$(whoami)") {
			t.Errorf("QUERY_STRING should contain raw text, got %q", v)
		}
	}
}

// TestSEC013_HttpoxyProxyHeaderStripped verifies that a client-supplied "Proxy"
// request header is NOT propagated to the child process as HTTP_PROXY
// (httpoxy, CVE-2016-5385). Otherwise an attacker could redirect the backend's
// outbound HTTP traffic through a proxy they control with a single request.
func TestSEC013_HttpoxyProxyHeaderStripped(t *testing.T) {
	t.Parallel()
	s := startServer(t, "env")

	headers := http.Header{}
	headers["Proxy"] = []string{"http://attacker.example:8080/"}
	// Control header: proves the header->env path is live, so a pass on the
	// assertion below can't be vacuous (e.g. if headers stopped flowing at all).
	headers["X-Custom-Test"] = []string{"custom-value"}

	ws, _, err := s.TryConnect("/", headers)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer ws.Close()

	output := strings.Join(collectMessages(ws, 3*time.Second), "\n")

	if v, ok := findEnvValue(output, "HTTP_PROXY"); ok {
		t.Errorf("Proxy header leaked to child as HTTP_PROXY=%q (httpoxy / CVE-2016-5385)", v)
	}
	if v, ok := findEnvValue(output, "HTTP_X_CUSTOM_TEST"); !ok || v != "custom-value" {
		t.Fatalf("control header missing: HTTP_X_CUSTOM_TEST=%q (ok=%v); test cannot validate the Proxy assertion", v, ok)
	}
}

// Helper
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// TestSEC014_OriginPortMatching verifies the port semantics of --origin
// entries (issue #473). A portless entry used to match ANY port, so
// --origin=trusted.example accepted http://trusted.example:1337 — any service
// listening on any port of the trusted host could produce a matching origin.
// Portless entries now match only the scheme's default port; an explicit
// ":*" wildcard restores any-port matching.
func TestSEC014_OriginPortMatching(t *testing.T) {
	t.Parallel()

	// Portless entry: non-default port must be rejected.
	s := startServerOpts(t, []string{"--origin=trusted.com"}, "echo")
	headers := http.Header{}
	headers.Set("Origin", "http://trusted.com:1337")
	if _, _, err := s.TryConnect("/", headers); err == nil {
		t.Error("portless --origin entry must not match a non-default port")
	}

	// Explicit wildcard: any port must be accepted.
	s2 := startServerOpts(t, []string{"--origin=trusted.com:*"}, "echo")
	if _, _, err := s2.TryConnect("/", headers); err != nil {
		t.Errorf("--origin=trusted.com:* must accept any port: %v", err)
	}
}

// TestSEC015_OriginPolicyWarning verifies the hard-to-miss startup warning:
// with no origin policy configured, websocketd must say so loudly, explain the
// options, and announce the future --sameorigin default (audit finding A2).
func TestSEC015_OriginPolicyWarning(t *testing.T) {
	t.Parallel()
	s := startServer(t, "echo") // no --sameorigin/--origin/--anyorigin
	out := s.Stdout()
	for _, want := range []string{
		"SECURITY WARNING",
		"--sameorigin",
		"--origin=",
		"--anyorigin",
		"future version of websocketd will default to --sameorigin",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("origin-policy warning missing %q in server stdout:\n%s", want, out)
		}
	}
}

// TestSEC016_OriginPolicyWarningSilenced verifies the warning appears only
// when no policy is configured: --sameorigin, --origin, and an explicit
// --anyorigin all silence it.
func TestSEC016_OriginPolicyWarningSilenced(t *testing.T) {
	t.Parallel()
	for _, flags := range [][]string{
		{"--sameorigin"},
		{"--origin=example.com"},
		{"--anyorigin"},
	} {
		s := startServerOpts(t, flags, "echo")
		if out := s.Stdout(); strings.Contains(out, "SECURITY WARNING") {
			t.Errorf("warning printed despite %v", flags)
		}
	}
}

// TestSEC017_AnyOriginKeepsPermissiveDefault verifies --anyorigin explicitly
// retains the accept-any-origin behavior it promises.
func TestSEC017_AnyOriginKeepsPermissiveDefault(t *testing.T) {
	t.Parallel()
	s := startServerOpts(t, []string{"--anyorigin"}, "echo")
	headers := http.Header{}
	headers.Set("Origin", "http://evil.example")
	ws, _, err := s.TryConnect("/", headers)
	if err != nil {
		t.Fatalf("--anyorigin must accept any origin: %v", err)
	}
	defer ws.Close()
	ws.Send("hello")
	ws.ExpectMessage("hello")
}

// TestSEC018_AnyOriginConflictsWithPolicies verifies --anyorigin cannot be
// combined with --sameorigin or --origin: the flags say opposite things, and
// a silent precedence rule would hide operator confusion.
func TestSEC018_AnyOriginConflictsWithPolicies(t *testing.T) {
	t.Parallel()
	for _, flags := range []string{"--sameorigin", "--origin=example.com"} {
		_, stderr, code := runWebsocketd(t, append([]string{"--anyorigin"}, flags)...)
		if code == 0 {
			t.Errorf("--anyorigin combined with %s was accepted (exit 0)", flags)
		}
		if !strings.Contains(stderr, "anyorigin") {
			t.Errorf("expected an error naming the conflict with %s, got: %q", flags, stderr)
		}
	}
}
