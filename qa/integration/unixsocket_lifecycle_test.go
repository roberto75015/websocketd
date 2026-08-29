package integration

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// Lifecycle tests for --unixsocket: refusing to take over a socket that
// another server is still listening on, which mirrors how a TCP listener
// already refuses a port that is in use.
//
// The stale-socket recovery path (a leftover file from an unclean exit) is
// covered by TestIssue435_StaleSocketCleanup. websocketd installs no signal
// handlers, so a socket file outliving its server is expected; startup
// recovery is what clears it.

// TestUnixSocket_RefusesLiveSocket verifies that starting a second server on a
// socket path that a healthy server is already listening on fails loudly,
// instead of unlinking the incumbent's socket and binding over it. Binding
// over it leaves the first process running but permanently unreachable, with
// no error reported on either side.
func TestUnixSocket_RefusesLiveSocket(t *testing.T) {
	skipUnixSocketOnWindows(t)
	t.Parallel()

	sockPath := shortSocketPath(t)

	first := startServerRawArgs(t, []string{"--unixsocket=" + sockPath, testcmdBin, "echo"})
	waitForSocket(t, sockPath, 10*time.Second)

	second := startServerRawArgs(t, []string{"--unixsocket=" + sockPath, testcmdBin, "echo"})
	if !second.WaitExit(10 * time.Second) {
		t.Fatal("second server should have exited rather than bound over a live socket")
	}
	if code := second.ExitCode(); code == 0 {
		t.Errorf("second server exited 0, want non-zero")
	}
	// log.Fatal routes through logfunc, which writes to stdout.
	if out := second.Stdout(); !strings.Contains(out, "already in use") {
		t.Errorf("expected an 'already in use' error on stdout, got:\n%s", out)
	}

	// The incumbent must still own the socket and still serve.
	if _, err := os.Stat(sockPath); err != nil {
		t.Fatalf("incumbent's socket file was removed: %v", err)
	}
	ws := dialUnixSocket(t, sockPath, "/")
	ws.Send("still mine")
	ws.ExpectMessage("still mine")

	if strings.Contains(first.Stdout(), "already in use") {
		t.Error("incumbent should not have logged a bind conflict")
	}
}

// TestUnixSocket_RefusesLiveSocketLeavesItServing is the regression guard for
// the specific silent-failure mode: after a losing start attempt, the socket
// must still route to the *original* process. Checking only that the path is
// dialable is not enough — a bind-over leaves a perfectly serviceable socket
// that happens to belong to the wrong server. The two servers therefore greet
// with distinct messages, so the greeting identifies which one answered.
func TestUnixSocket_RefusesLiveSocketLeavesItServing(t *testing.T) {
	skipUnixSocketOnWindows(t)
	t.Parallel()

	sockPath := shortSocketPath(t)

	startServerRawArgs(t, []string{"--unixsocket=" + sockPath, testcmdBin, "welcome", "incumbent"})
	waitForSocket(t, sockPath, 10*time.Second)

	second := startServerRawArgs(t, []string{"--unixsocket=" + sockPath, testcmdBin, "welcome", "usurper"})
	second.WaitExit(10 * time.Second)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("socket unreachable after a rejected second start: %v", err)
	}
	conn.Close()

	ws := dialUnixSocket(t, sockPath, "/")
	ws.ExpectMessage("incumbent")
	ws.Send("round trip")
	ws.ExpectMessage("round trip")
}

// TestUnixSocket_SocketMode verifies --socketmode: the socket file's
// permissions must follow the flag instead of the process umask (issue #474).
// Without the flag they follow umask, which is world-connectable under the
// permissive umasks common in daemon contexts.
func TestUnixSocket_SocketMode(t *testing.T) {
	skipUnixSocketOnWindows(t)
	t.Parallel()

	sockPath := shortSocketPath(t)

	startServerRawArgs(t, []string{
		"--unixsocket=" + sockPath,
		"--socketmode=0754",
		testcmdBin, "echo",
	})
	waitForSocket(t, sockPath, 10*time.Second)

	fi, err := os.Stat(sockPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if fi.Mode()&os.ModeSocket == 0 {
		t.Fatalf("%s is not a socket", sockPath)
	}
	if got := fi.Mode().Perm(); got != 0o754 {
		t.Errorf("socket mode = %o, want 754", got)
	}

	// The server must still serve normally on the restricted socket.
	ws := dialUnixSocket(t, sockPath, "/")
	ws.Send("still serving")
	ws.ExpectMessage("still serving")
}

// TestUnixSocket_SocketModeInvalid verifies that a malformed --socketmode is
// rejected at startup rather than silently ignored.
func TestUnixSocket_SocketModeInvalid(t *testing.T) {
	skipUnixSocketOnWindows(t)
	t.Parallel()

	sockPath := shortSocketPath(t)
	s := startServerRawArgs(t, []string{
		"--unixsocket=" + sockPath,
		"--socketmode=0999", // 9 is not an octal digit
		testcmdBin, "echo",
	})
	if !s.WaitExit(10 * time.Second) {
		t.Fatal("websocketd accepted an invalid --socketmode and kept running")
	}
	if code := s.ExitCode(); code == 0 {
		t.Errorf("exit code = 0, want non-zero")
	}
	if out := s.Stdout() + s.Stderr(); !strings.Contains(out, "socketmode") {
		t.Errorf("expected an error naming socketmode, got:\n%s", out)
	}
}
