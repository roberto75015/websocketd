// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !windows

package libwebsocketd

import (
	"os"
	"syscall"
)

// newChildSysProcAttr puts each wrapped process in its own process group, so
// teardown can reach the whole tree: signals sent to the group hit the direct
// child and any descendants that did not start process groups of their own
// (a child that wants to outlive the session must setsid, which is the
// standard Unix opt-out). It also keeps terminal-directed signals aimed at
// websocketd's own group away from the child.
func newChildSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

// signalChild sends sig to the child's whole process group (negative pid),
// so grandchildren that stayed in the group are torn down alongside it.
func signalChild(p *os.Process, sig os.Signal) error {
	if sysSig, ok := sig.(syscall.Signal); ok {
		return syscall.Kill(-p.Pid, sysSig)
	}
	return p.Signal(sig)
}

// killLeftoverGroup delivers a final SIGKILL to whatever still remains in the
// child's process group once the direct child is gone. The group id is the
// (now reaped or exiting) child's pid, which the kernel will not hand to a new
// process group leader in this window in practice; an ESRCH just means the
// group is already empty.
func killLeftoverGroup(pid int, log *LogScope) {
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		log.Error("process", "SIGKILL of leftover process group %v failed: %s", pid, err)
	}
}
