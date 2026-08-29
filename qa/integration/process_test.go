package integration

import (
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPROC001_ProcessPerConnection(t *testing.T) {
	t.Parallel()
	s := startServer(t, "pid-echo")

	ws1 := s.Connect("/")
	pid1 := ws1.Recv()

	ws2 := s.Connect("/")
	pid2 := ws2.Recv()

	ws1.Close()
	ws2.Close()

	if pid1 == pid2 {
		t.Errorf("expected different PIDs, both got %s", pid1)
	}
	// Verify PIDs are actual numbers
	if _, err := strconv.Atoi(strings.TrimSpace(pid1)); err != nil {
		t.Errorf("pid1 is not a number: %q", pid1)
	}
	if _, err := strconv.Atoi(strings.TrimSpace(pid2)); err != nil {
		t.Errorf("pid2 is not a number: %q", pid2)
	}
}

// TestPROC013_SessionTeardownKillsProcessGroup verifies that when a client
// disconnects, websocketd tears down the whole process group — not just the
// direct child. Previously a script could spawn a background child, exit,
// and leave that child running (and, by holding the inherited pipes, the
// session and its --maxforks slot occupied) long after the connection was
// gone, because signals were sent to the direct child only.
func TestPROC013_SessionTeardownKillsProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process-group teardown is Unix-only")
	}
	t.Parallel()

	// The grandchild prints its pid, then the script serves stdin so the
	// session stays alive until we close it. sleep 60 makes it unmistakable
	// if teardown fails to reach it.
	script := `sleep 60 & echo GRANDCHILD=$!; while IFS= read -r line; do echo "R:$line"; done`
	s := startServerRaw(t, nil, "/bin/sh", "-c", script)

	ws := s.Connect("/")
	bg := strings.TrimPrefix(ws.Recv(), "GRANDCHILD=")
	pid, err := strconv.Atoi(strings.TrimSpace(bg))
	if err != nil {
		t.Fatalf("grandchild pid %q is not a number: %v", bg, err)
	}
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) }) // don't leak it if the test fails

	ws.Send("hello")
	ws.ExpectMessage("R:hello")

	// Close the session; the grandchild must not survive it.
	ws.Close()

	// Signal(0) succeeds while the process exists (or is an unreaped zombie).
	// The grandchild's parent (the script) has exited, so it is reparented to
	// init and gets reaped quickly once killed — a short poll is enough.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if proc, err := os.FindProcess(pid); err == nil {
			if err := proc.Signal(syscall.Signal(0)); err != nil {
				return // gone — teardown reached it
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("grandchild %d still alive %v after the session closed", pid, 10*time.Second)
}

func TestPROC002_StdoutToWebSocket(t *testing.T) {
	t.Parallel()
	s := startServer(t, "welcome", "ready")
	ws := s.Connect("/")
	defer ws.Close()
	// Should receive "ready" without sending anything first
	ws.ExpectMessage("ready")
}

func TestPROC003_StderrToLogs(t *testing.T) {
	t.Parallel()
	s := startServer(t, "stderr")
	ws := s.Connect("/")
	defer ws.Close()

	// Should only receive stdout, not stderr
	ws.ExpectMessage("stdout line")

	// stderr goes to websocketd logs, not to the client
	ws.ExpectClosed()

	// Poll for stderr content (process may still be flushing). The relay
	// goes through websocketd's logger, which writes to its stdout.
	var stdout string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stdout = s.Stdout()
		if strings.Contains(stdout, "stderr line") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(stdout, "stderr line") {
		t.Error("child stderr was not relayed to websocketd's log on stdout")
	}
	if strings.Contains(s.Stderr(), "stderr line") {
		t.Error("child stderr unexpectedly appeared on websocketd's own stderr")
	}
}

func TestPROC004_MaxforksLimit(t *testing.T) {
	t.Parallel()
	s := startServerOpts(t, []string{"--maxforks=2"}, "echo")

	ws1 := s.Connect("/")
	defer ws1.Close()
	ws2 := s.Connect("/")
	defer ws2.Close()

	// Third connection should be rejected
	_, resp, err := s.TryConnect("/", nil)
	if err == nil {
		t.Fatal("expected third connection to be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected HTTP 429, got %d", resp.StatusCode)
	}
}

func TestPROC005_MaxforksRecovery(t *testing.T) {
	t.Parallel()
	s := startServerOpts(t, []string{"--maxforks=1"}, "echo")

	ws1 := s.Connect("/")
	ws1.Send("hello")
	ws1.ExpectMessage("hello")
	ws1.Close()

	// Should be able to connect again after fork is released
	ws2 := s.retryConnect(t, "/", 5*time.Second)
	defer ws2.Close()
	ws2.Send("world")
	ws2.ExpectMessage("world")
}

func TestPROC006_ProcessExitNonZero(t *testing.T) {
	t.Parallel()
	s := startServer(t, "exit", "42", "about to fail")
	ws := s.Connect("/")
	defer ws.Close()
	ws.ExpectMessage("about to fail")
	ws.ExpectClosed()
}

func TestPROC007_ScriptDirectoryMode(t *testing.T) {
	t.Parallel()
	// Create a "script directory" — but since we use testcmd, we use --dir isn't
	// directly testable with testcmd. Instead test URL routing with single command.
	// The handler_test.go unit tests cover --dir path resolution.
	// Here we verify that PATH_INFO and SCRIPT_NAME are set correctly
	// when connecting to different paths.
	s := startServer(t, "env")
	ws := s.Connect("/some/path")
	defer ws.Close()
	output := strings.Join(collectMessages(ws, 2*time.Second), "\n")
	if v, ok := findEnvValue(output, "PATH_INFO"); ok {
		if v != "/some/path" {
			t.Errorf("PATH_INFO: expected /some/path, got %s", v)
		}
	}
}

func TestPROC008_CommandArguments(t *testing.T) {
	t.Parallel()
	s := startServer(t, "output", "arg1", "arg2", "arg3")
	ws := s.Connect("/")
	defer ws.Close()
	ws.ExpectMessages("arg1", "arg2", "arg3")
}

func TestPROC009_RapidProcessExit(t *testing.T) {
	t.Parallel()
	s := startServer(t, "exit", "0", "quick")

	// Rapid connect/disconnect shouldn't crash the server
	for i := 0; i < 5; i++ {
		ws := s.Connect("/")
		ws.ExpectMessage("quick")
		ws.ExpectClosed()
	}
}

func TestPROC010_ProcessExitNoOutput(t *testing.T) {
	t.Parallel()
	s := startServer(t, "exit", "0")
	ws := s.Connect("/")
	defer ws.Close()
	ws.ExpectClosed()
}

func TestPROC011_SlowStartProcess(t *testing.T) {
	t.Parallel()
	s := startServer(t, "slow-start", "500")
	ws := s.Connect("/")
	defer ws.Close()

	// Should wait for the process to be ready
	msg, err := ws.RecvTimeout(10 * time.Second)
	if err != nil {
		t.Fatalf("failed to receive after slow start: %v", err)
	}
	if msg != "ready" {
		t.Errorf("expected 'ready', got %q", msg)
	}

	ws.Send("hello")
	ws.ExpectMessage("hello")
}

func TestPROC012_InfiniteOutputProcess(t *testing.T) {
	t.Parallel()
	s := startServer(t, "infinite", "50")
	ws := s.Connect("/")
	defer ws.Close()

	// Should receive several ticks
	for i := 0; i < 5; i++ {
		msg, err := ws.RecvTimeout(2 * time.Second)
		if err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
		if msg != "tick" {
			t.Errorf("tick %d: expected 'tick', got %q", i, msg)
		}
	}
}
