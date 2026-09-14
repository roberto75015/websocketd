// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Test for #477: --sslca without --ssl used to start successfully and serve
// plain HTTP with no TLS and no client-certificate verification, despite the
// flag naming a CA for mutual TLS. websocketd must now refuse to start that
// combination, on stderr, with exit code 1.

func TestIssue477_SslcaWithoutSslRejected(t *testing.T) {
	t.Parallel()
	port := freePort(t)
	stdout, stderr, code := runWebsocketd(t,
		"--port="+strconv.Itoa(port),
		"--sslca=/nonexistent-ca.pem",
		testcmdBin, "echo")
	if code == 0 {
		t.Fatal("websocketd accepted --sslca without --ssl and exited 0")
	}
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "sslca") || !strings.Contains(stderr, "ssl") {
		t.Errorf("expected an error on stderr naming --sslca and --ssl, got stderr=%q", stderr)
	}
	if stdout != "" {
		t.Errorf("expected nothing on stdout, got %q", stdout)
	}
}

// TestIssue477_SslcaWithSslCertKeyStillWorks guards against a startup
// validation regression: the legitimate mutual-TLS combination must still
// bind and serve, not just avoid a validation error in isolation. The full
// handshake (valid client cert accepted, missing client cert rejected) is
// covered end to end by TestIssue413_MutualTLS and
// TestIssue413_MutualTLSRejectsNoClientCert.
func TestIssue477_SslcaWithSslCertKeyStillWorks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	caCertFile, caKeyFile := generateCA(t, dir)
	serverCertFile, serverKeyFile := generateSignedCert(t, dir, "server", caCertFile, caKeyFile)

	port := freePort(t)
	cmd := exec.Command(websocketdBin,
		"--port="+strconv.Itoa(port),
		"--address=127.0.0.1",
		"--loglevel=error",
		"--ssl",
		"--sslcert="+serverCertFile,
		"--sslkey="+serverKeyFile,
		"--sslca="+caCertFile,
		testcmdBin, "echo",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start: %v", err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	waitForPort(t, port, 5*time.Second)
	// waitForPort succeeding proves the combination was accepted at
	// startup and the listener bound; if the process had already exited
	// (rejected), the port would never open and waitForPort would fail
	// the test via its own timeout.
}
