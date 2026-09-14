---
title: "Platform support"
weight: 50
description: "The platforms websocketd is released for, and the exact behaviour differences on Windows."
---

websocketd ships released binaries for Linux, macOS, and Windows. It
builds from source on any platform Go supports. Three behaviours differ
on Windows.

## Released binaries

Each release ships a zip archive per platform, containing the binary,
`README.md`, `LICENSE`, and `CHANGES`.

| Operating system | Architecture | Archive name suffix |
|---|---|---|
| Linux | amd64 | `linux_amd64` |
| Linux | 386 | `linux_386` |
| Linux | arm | `linux_arm` |
| Linux | arm64 | `linux_arm64` |
| macOS | amd64 | `darwin_amd64` |
| macOS | arm64 (Apple silicon) | `darwin_arm64` |
| Windows | 386 | `windows_386` |
| Windows | amd64 | `windows_amd64` |

The `linux_arm` binary is built for ARMv5, so it runs on every Raspberry
Pi model.

Two Linux package formats are built alongside the archives: `.deb` for
i386 and amd64, and `.rpm` for i386 and x86_64.

FreeBSD, OpenBSD, and Solaris are not release targets. The source
compiles for those platforms with `go build`, and `--passenv`'s built-in
default carries entries for several of them, but no archive ships.

## Building from source

`go build` at the repository root produces a `websocketd` binary. The
minimum Go version is the one named in `go.mod`.

## What differs on Windows

Each entry below is a property of the Windows platform, not of
websocketd, and none of them has a fix pending.

### SIGINT and SIGTERM are never delivered

On Unix, websocketd tears a process down in four steps: it closes the
process's stdin, then sends SIGINT, then SIGTERM, then SIGKILL, pausing
between each. On Windows, only the last step has any effect.

Go's `os.Process.Signal` implements exactly one signal on Windows,
`os.Kill`, which calls the Win32 `TerminateProcess`. Every other signal
returns "not supported by windows". websocketd logs that error and moves
on to the next step, so a program running under websocketd on Windows
receives no SIGINT and no SIGTERM, and gets no chance to shut down
gracefully (issues #298, #362).

A process group is the second difference. On Unix, each wrapped process
gets its own process group, signals go to the whole group, and a final
SIGKILL sweeps whatever is left in it, so descendants the program started
are torn down with it. Windows has no process-group equivalent here, so
websocketd signals the direct child only. Any process the wrapped program
started outlives it unless the program terminates it.

Stdin closing works identically on both platforms. A program that needs a
teardown hook on Windows has stdin EOF and nothing else.

### Shebang lines have no effect

A shebang is the `#!/usr/bin/env python3` line at the top of a script. On
Unix the kernel reads it and runs the named interpreter. Windows has no
such mechanism; the line is inert text even when present.

For websocketd to launch a `.bat`, `.cmd`, or `.ps1` file directly, as
`COMMAND` or through `--dir` or `--cgidir`, that extension must be
associated with its interpreter at the operating-system level, the way
Windows normally runs `.bat` and `.cmd` through `cmd.exe` and `.ps1`
through PowerShell. A script that runs when double-clicked can still fail
under websocketd if the association is not present in the environment
websocketd itself runs under (issues #384, #454).

### PATH lookup can resolve a different binary

Given a bare command name rather than a full path, websocketd resolves it
against `PATH`. A shell may resolve the same name differently, through an
alias, a function, or a shim that the shell knows about and a plain `PATH`
lookup does not. websocketd then launches a different program than the one
tested in the shell (issue #372). Passing the full path to the executable
removes the lookup.

## See also

- [Run Windows scripts](/how-to/languages/windows-scripts/) gives the
  steps for `.bat`, `.cmd`, and `.ps1`.
- [Process lifecycle](/understanding/process-lifecycle/) has the full
  teardown sequence these signal differences apply to.
- [Exit codes](/reference/exit-codes/) are identical on every platform.
