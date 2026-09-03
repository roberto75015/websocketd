// Copyright 2013 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libwebsocketd

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

type ProcessEndpoint struct {
	process    *LaunchedProcess
	closetime  time.Duration
	output     chan []byte
	done       chan struct{}
	doneOnce   sync.Once
	log        *LogScope
	bin        bool
	raw        bool	
	passStderr bool
	wg         sync.WaitGroup
}

func NewProcessEndpoint(process *LaunchedProcess, raw bool, bin bool, log *LogScope, passStderr bool) *ProcessEndpoint {
	return &ProcessEndpoint{
		process:    process,
		output:     make(chan []byte),
		done:       make(chan struct{}),
		log:        log,
		bin:        bin,
		raw:		raw,
		passStderr: passStderr,
	}
}

func (pe *ProcessEndpoint) Terminate() {
	// Unblock a stdout reader parked on the output channel send: killing the
	// process only unblocks reads, so without this the reader goroutine (and
	// its buffer) leaks whenever the relay stopped draining Output().
	pe.doneOnce.Do(func() { close(pe.done) })

	// Buffered so the waiter goroutine can exit even if the process never
	// gets reaped and this method gives up after SIGKILL.
	terminated := make(chan struct{}, 1)
	go func() {
		if err := pe.process.cmd.Wait(); err != nil {
			pe.log.Debug("process", "Process exit: %s", err)
		}
		terminated <- struct{}{}
	}()

	// for some processes this is enough to finish them...
	if err := pe.process.stdin.Close(); err != nil {
		pe.log.Debug("process", "STDIN close: %s", err)
	}

	pid := pe.process.cmd.Process.Pid

	// Escalating termination: stdin close → SIGINT → SIGTERM → SIGKILL.
	// Signals go to the child's whole process group, and a final SIGKILL
	// sweep (killLeftoverGroup) ensures nothing is left in it once the direct
	// child is gone — previously grandchildren survived the session.
	signals := []struct {
		signal  os.Signal
		name    string
		timeout time.Duration
	}{
		{nil, "stdin was closed", 100*time.Millisecond + pe.closetime},
		{syscall.SIGINT, "SIGINT", 250*time.Millisecond + pe.closetime},
		{syscall.SIGTERM, "SIGTERM", 500*time.Millisecond + pe.closetime},
		{syscall.SIGKILL, "SIGKILL", 1000 * time.Millisecond},
	}

	for _, step := range signals {
		if step.signal != nil {
			if err := signalChild(pe.process.cmd.Process, step.signal); err != nil {
				pe.log.Error("process", "%s unsuccessful to %v: %s", step.name, pid, err)
			}
		}
		select {
		case <-terminated:
			pe.log.Debug("process", "Process %v terminated after %s", pid, step.name)
			killLeftoverGroup(pid, pe.log)
			return
		case <-time.After(step.timeout):
		}
	}

	pe.log.Error("process", "SIGKILL did not terminate %v!", pid)
	killLeftoverGroup(pid, pe.log)
}

func (pe *ProcessEndpoint) Output() chan []byte {
	return pe.output
}

func (pe *ProcessEndpoint) Send(msg []byte) bool {
	_, err := pe.process.stdin.Write(msg)
	if err != nil {
		pe.log.Debug("process", "Cannot write to STDIN: %s", err)
		return false
	}
	return true
}

func (pe *ProcessEndpoint) StartReading() {
	if pe.passStderr {
		// Both streams feed the same output channel, tagged by source, so
		// it must close only once both readers are done - never while
		// either might still send. (--binary is rejected together with
		// --passstderr at config-validation time, so only line-based
		// reading is needed here.)
		pe.wg.Add(2)
		go pe.readStdoutTagged()
		go pe.readStderrTagged()
		go pe.closeOutputWhenDone()
		return
	}
	go pe.logStderr()
	if pe.bin {
		go pe.readBinaryOutput()
	} else {
		go pe.readTextOutput()
	}
}

func (pe *ProcessEndpoint) closeOutputWhenDone() {
	pe.wg.Wait()
	close(pe.output)
}

func (pe *ProcessEndpoint) readTextOutput() {
	defer close(pe.output)
	bufin := bufio.NewReader(pe.process.stdout)
	for {
		var buf []byte
		var err error
		if pe.raw {
			var b byte
			b, err = bufin.ReadByte()
			buf = []byte{b}
		} else {
			buf, err = bufin.ReadBytes('\n')
		}
		if err != nil {
			if err != io.EOF {
				pe.log.Error("process", "Unexpected error while reading STDOUT from process: %s", err)
			} else {
				pe.log.Debug("process", "Process STDOUT closed")
			}
			break
		}
		select {
		case pe.output <- pe.trimEOLifNotRaw(buf):
		case <-pe.done:
			return
		}
	}
}

func (pe *ProcessEndpoint) readBinaryOutput() {
	defer close(pe.output)
	// 64KB matches the largest chunk a pipe read can return (the kernel
	// hands out at most the pipe capacity per read), so anything larger is
	// virtual-memory pressure that can never be touched: 10MB x 1024 default
	// --maxforks looked like 10GB of RSS while measuring ~300KB per connection
	// (audit finding A10). Multi-chunk relaying is covered by
	// TestCLI016_BinaryModeLargePayload.
	buf := make([]byte, 64*1024)
	for {
		n, err := pe.process.stdout.Read(buf)
		if err != nil {
			if err != io.EOF {
				pe.log.Error("process", "Unexpected error while reading STDOUT from process: %s", err)
			} else {
				pe.log.Debug("process", "Process STDOUT closed")
			}
			break
		}
		select {
		case pe.output <- append(make([]byte, 0, n), buf[:n]...): // cloned buffer
		case <-pe.done:
			return
		}
	}
}

// taggedMessage is the JSON envelope sent to WebSocket clients when
// --passstderr is enabled, so they can distinguish the two streams.
type taggedMessage struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

func tagMessage(stream string, data []byte) []byte {
	// json.Marshal cannot fail here: the struct holds only plain strings
	// (invalid UTF-8 is replaced, not rejected).
	msg, _ := json.Marshal(taggedMessage{Stream: stream, Data: string(data)})
	return msg
}

func (pe *ProcessEndpoint) readStdoutTagged() {
	defer pe.wg.Done()
	bufin := bufio.NewReader(pe.process.stdout)
	for {
		var buf []byte
		var err error
		if pe.raw {
			var b byte
			b, err = bufin.ReadByte()
			buf = []byte{b}
		} else {
			buf, err = bufin.ReadBytes('\n')
		}
		if err != nil {
			if err != io.EOF {
				pe.log.Error("process", "Unexpected error while reading STDOUT from process: %s", err)
			} else {
				pe.log.Debug("process", "Process STDOUT closed")
			}
			break
		}
		select {
		case pe.output <- tagMessage("stdout", pe.trimEOLifNotRaw(buf)):
		case <-pe.done:
			return
		}
	}
}

// Stderr lines are read with a bounded reader and streamed in chunks: a
// stderr write larger than the reader's buffer (4KB) with no trailing newline
// would otherwise return bufio.ErrBufferFull, and treating that as fatal
// abandoned the pipe — the child then blocked forever on its next stderr
// write once the OS pipe filled (a remote-triggered hang, since stdin is
// attacker-driven). Long stderr lines are delivered (and logged) as
// consecutive partial chunks instead.
func (pe *ProcessEndpoint) readStderrTagged() {
	defer pe.wg.Done()
	bufstderr := bufio.NewReader(pe.process.stderr)
	for {
		buf, err := bufstderr.ReadSlice('\n')
		if len(buf) > 0 {
			line := trimEOL(buf)
			pe.log.Error("stderr", "%s", string(line)) // still logged server-side, same as without --passstderr
			select {
			case pe.output <- tagMessage("stderr", line):
			case <-pe.done:
				return
			}
		}
		if err == bufio.ErrBufferFull {
			continue // partial chunk emitted above; keep draining
		}
		if err != nil {
			if err != io.EOF {
				pe.log.Error("process", "Unexpected error while reading STDERR from process: %s", err)
			} else {
				pe.log.Debug("process", "Process STDERR closed")
			}
			return
		}
	}
}

// logStderr drains stderr line by line into the log, streaming partial
// chunks for lines longer than the reader's buffer (see readStderrTagged for
// why abandoning the pipe here would wedge the child).
func (pe *ProcessEndpoint) logStderr() {
	bufstderr := bufio.NewReader(pe.process.stderr)
	for {
		buf, err := bufstderr.ReadSlice('\n')
		if len(buf) > 0 {
			pe.log.Error("stderr", "%s", string(trimEOL(buf)))
		}
		if err == bufio.ErrBufferFull {
			continue // partial chunk logged above; keep draining
		}
		if err != nil {
			if err != io.EOF {
				pe.log.Error("process", "Unexpected error while reading STDERR from process: %s", err)
			} else {
				pe.log.Debug("process", "Process STDERR closed")
			}
			return
		}
	}
}

// trimEOL cuts unixy style \n and windowsy style \r\n suffix from the string (only if raw mode is not enabled)
func (pe *ProcessEndpoint) trimEOLifNotRaw(b []byte) []byte {
	if pe.raw {
		return b
	}
	return trimEOL(b)
}

// trimEOL cuts unixy style \n and windowsy style \r\n suffix from the string
func trimEOL(b []byte) []byte {
	lns := len(b)
	if lns > 0 && b[lns-1] == '\n' {
		lns--
		if lns > 0 && b[lns-1] == '\r' {
			lns--
		}
	}
	return b[:lns]
}