// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package browser is a behavioural test suite for the --devconsole page: it
// drives a real, headless Google Chrome (via chromedp) against a real
// websocketd process, built from source, talking to a real backend command.
// It is deliberately separate from the Go-side unit/serving guards in
// libwebsocketd/console_serving_test.go, which never open a socket, send a
// frame, or read one back — this package exists because none of that proves
// the console actually works for a user.
//
// This mirrors qa/integration/harness_test.go's approach (build the binary
// fresh in TestMain rather than trust a stale one; go test's own cache
// still applies the same way — see CLAUDE.md's note on re-running with
// -count=1 after changing libwebsocketd/, main.go or config.go).
package browser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

// chromeExecPath is fixed per this agent's brief rather than auto-detected:
// a missing browser must fail the whole suite loudly (see TestMain), not
// silently fall back to a different executable that may not exist either.
const chromeExecPath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

var (
	websocketdBin string
	testcmdBin    string
)

func TestMain(m *testing.M) {
	// "Guard the suite so it FAILS LOUDLY if Chrome is missing ... It must
	// never silently skip into a green." os.Exit(1) from TestMain, not
	// t.Skip from an individual test, so a missing browser fails the whole
	// binary rather than reporting a deceptive partial pass.
	if _, err := os.Stat(chromeExecPath); err != nil {
		fmt.Fprintf(os.Stderr,
			"BLOCKER: headless Chrome not found at %q: %v\n"+
				"qa/browser requires a real browser and does not substitute a fake driver.\n",
			chromeExecPath, err)
		os.Exit(1)
	}

	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find project root: %v\n", err)
		os.Exit(1)
	}

	tmpDir, err := os.MkdirTemp("", "wsconsole-browser-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	websocketdBin = filepath.Join(tmpDir, "websocketd"+ext)
	cmd := exec.Command("go", "build", "-o", websocketdBin, ".")
	cmd.Dir = projectRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "BLOCKER: failed to build websocketd: %v\n%s\n", err, out)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	testcmdBin = filepath.Join(tmpDir, "testcmd"+ext)
	cmd = exec.Command("go", "build", "-o", testcmdBin, "./qa/integration/testcmd")
	cmd.Dir = projectRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "BLOCKER: failed to build testcmd: %v\n%s\n", err, out)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// issuedPorts records every port freePort has handed out in this test
// binary, mirroring qa/integration/helpers_test.go's freePort: the probe
// listener must close before the port can be reused, which leaves a window
// for the OS to reissue it to a concurrent probe.
var (
	issuedPortsMu sync.Mutex
	issuedPorts   = map[int]bool{}
)

func freePort(t *testing.T) int {
	t.Helper()
	for attempt := 0; attempt < 20; attempt++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to find free port: %v", err)
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()

		issuedPortsMu.Lock()
		reissued := issuedPorts[port]
		issuedPorts[port] = true
		issuedPortsMu.Unlock()

		if !reissued {
			return port
		}
	}
	t.Fatal("failed to find a free port not already issued to another test")
	return 0
}

// syncBuffer is a goroutine-safe bytes.Buffer for capturing subprocess output.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// devConsole wraps a running `websocketd --devconsole` process.
type devConsole struct {
	t      *testing.T
	Port   int
	cmd    *exec.Cmd
	stdout syncBuffer
	stderr syncBuffer
	exited chan struct{}
}

// URL is the page the browser should navigate to.
func (dc *devConsole) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/", dc.Port)
}

var probeCounter atomic.Int64

// startDevConsole starts `websocketd --devconsole <testcmdBin> echo` on a
// free port with an independent Unix-domain-free TCP listener (no
// --unixsocket path is used, so there is nothing shared across agents to
// collide on there), and blocks until it is provably serving.
//
// Dialing the port only proves *something* is listening, not that our
// process is (see qa/integration/harness_test.go's waitReady for the full
// argument: the probe listener used to pick a free port has to close before
// websocketd can bind it, leaving a gap another process can win). So this
// requests a path unique to this run and waits for it to appear in the
// captured stdout access log, which only our own websocketd could have
// written. If the process exits first, or the timeout elapses, this fails
// the test loudly with the captured output rather than leaving a later
// chromedp call to time out against a dead port with no explanation.
func startDevConsole(t *testing.T) *devConsole {
	t.Helper()
	port := freePort(t)
	args := []string{
		"--port=" + strconv.Itoa(port),
		"--address=127.0.0.1",
		"--loglevel=access",
		"--devconsole",
		testcmdBin, "echo",
	}
	cmd := exec.Command(websocketdBin, args...)
	dc := &devConsole{t: t, Port: port, cmd: cmd, exited: make(chan struct{})}
	cmd.Stdout = &dc.stdout
	cmd.Stderr = &dc.stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start websocketd: %v", err)
	}
	go func() {
		cmd.Wait()
		close(dc.exited)
	}()
	t.Cleanup(func() {
		dc.cmd.Process.Kill()
		<-dc.exited
		if t.Failed() {
			t.Logf("websocketd stdout:\n%s", dc.stdout.String())
			t.Logf("websocketd stderr:\n%s", dc.stderr.String())
		}
	})

	probePath := fmt.Sprintf("/__browser_ready_%d__", probeCounter.Add(1))
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(dc.stdout.String(), probePath) {
			return dc
		}
		select {
		case <-dc.exited:
			out := strings.TrimSpace(dc.stdout.String() + dc.stderr.String())
			t.Fatalf("websocketd exited during startup:\n%s", out)
		default:
		}
		probeDevConsole(port, probePath)
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("websocketd did not answer on port %d within 10s", port)
	return nil
}

// probeDevConsole sends one plain HTTP request for probePath. Any reply is
// fine (the dev console answers everything with the same 200); the point is
// the access-log line it leaves in the captured stdout for startDevConsole
// to find. Errors are ignored — the caller just tries again.
func probeDevConsole(port int, probePath string) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
	if _, err := fmt.Fprintf(conn, "GET %s HTTP/1.0\r\nHost: 127.0.0.1\r\n\r\n", probePath); err != nil {
		return
	}
	io.Copy(io.Discard, conn)
}

// clickSafe and typeSafe click/type without the NodeVisible-gated node
// lookup that chromedp.Click and chromedp.SendKeys use internally
// (query.go: both append the NodeVisible QueryOption, which polls
// dom.GetBoxModel on every node matching the selector until all of them
// report a valid box model).
//
// chromedp v0.16.0 has an internal DOM-node-tracking cache bug that this
// trips on every time: right around a WebSocket connecting, the console
// fires several DOM mutations in quick succession (state() flips #status
// and #connect's dataset and textContent, note() clones and appends a
// template row). Confirmed by direct measurement, not suspicion: after that
// churn, document.querySelectorAll('#send').length in the real browser
// stays at 1 (checked from live JS), but chromedp's own Nodes() query for
// the same selector returns 2 - a stale cached node alongside the live one.
// NodeVisible's poller waits for the box model of every returned node, so
// with a phantom entry that will never resolve, it spins until the test's
// context deadline. A raw box-model probe on the FIRST of those two nodes
// (the same one Click/SendKeys would themselves act on - see below)
// resolves in under a millisecond, so the element itself is not the
// problem; the second, extra node is.
//
// The workaround is to never let NodeVisible run after a connection: look
// the node up with a plain Nodes() (no visibility gate) and act on it
// directly. Both chromedp.Click and chromedp.SendKeys act on nodes[0] when
// a selector matches more than one node (see their source), so clickSafe
// and typeSafe do the same, to keep selecting "the first match" whenever a
// selector is not unique (e.g. #frames .frame[data-dir="in"] before only
// one such row exists).
func clickSafe(t *testing.T, ctx context.Context, sel string) {
	t.Helper()
	var nodes []*cdp.Node
	if err := chromedp.Run(ctx, chromedp.Nodes(sel, &nodes)); err != nil {
		t.Fatalf("resolving %q for click: %v", sel, err)
	}
	if len(nodes) == 0 {
		t.Fatalf("clickSafe: selector %q matched no nodes", sel)
	}
	if err := chromedp.Run(ctx, chromedp.MouseClickNode(nodes[0])); err != nil {
		t.Fatalf("clicking %q: %v", sel, err)
	}
}

// typeSafe focuses sel (chromedp.Focus does not use NodeVisible, so this
// step alone is not subject to the bug above) and then dispatches text as
// raw key events with no selector re-query - chromedp.KeyEvent with no
// selector argument sends to whatever element is currently focused.
func typeSafe(t *testing.T, ctx context.Context, sel, text string) {
	t.Helper()
	if err := chromedp.Run(ctx, chromedp.Focus(sel)); err != nil {
		t.Fatalf("focusing %q to type %q: %v", sel, text, err)
	}
	if err := chromedp.Run(ctx, chromedp.KeyEvent(text)); err != nil {
		t.Fatalf("typing %q into %q: %v", text, sel, err)
	}
}
