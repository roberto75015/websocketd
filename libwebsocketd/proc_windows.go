// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build windows

package libwebsocketd

import (
	"os"
	"syscall"
)

// Windows has no Unix process groups; keep the previous behavior of
// signaling the direct child only (in practice only Kill works there, and
// the escalation sequence's errors are logged, not fatal).

func newChildSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

func signalChild(p *os.Process, sig os.Signal) error {
	return p.Signal(sig)
}

func killLeftoverGroup(pid int, log *LogScope) {
	// no process-group equivalent to sweep
}
