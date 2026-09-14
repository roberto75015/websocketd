---
title: "Run a .bat, .cmd, or PowerShell script on Windows"
weight: 60
description: "Name the interpreter yourself rather than pointing websocketd at the script file, give it a full path, and expect no signal-based cleanup when a client disconnects."
---

Name the interpreter explicitly and pass your script to it as an
argument. Do not point websocketd at the script file on its own.

For a `.bat` or `.cmd` batch file, the interpreter is `cmd.exe`, and
`/c` tells it to run the file and then exit:

```bat
websocketd.exe --port=8080 cmd.exe /c C:\scripts\count.bat
```

For a `.ps1` PowerShell script, the interpreter is `powershell.exe`, and
`-File` tells it to run the file:

```bat
websocketd.exe --port=8080 powershell.exe -NoProfile -ExecutionPolicy Bypass -File C:\scripts\count.ps1
```

Use `pwsh.exe` in place of `powershell.exe` for PowerShell 7 and later.

`-ExecutionPolicy Bypass` is needed because a default PowerShell
installation refuses to run unsigned local script files. `-NoProfile`
skips the user's profile script, which would otherwise run first and
can write its own text to stdout, mixing startup noise into
your script's messages.

A batch file to try it with:

```bat
@echo off
for /L %%i in (1,1,5) do (
  echo %%i
  timeout /t 1 /nobreak > nul
)
```

## Why the interpreter has to be named

On Unix, a script file can say which interpreter runs it. The first line
holds a **shebang**, written `#!` followed by a path, as in
`#!/usr/bin/env python3`. The Unix kernel reads those two characters
when it is asked to execute the file, and launches the named interpreter
with the file as its argument.

Windows has no equivalent. The shebang is a kernel feature, and the
Windows kernel does not implement it. A `#!` line at the top of a file
on Windows is a comment, or a syntax error, depending on the
language.

Windows decides what runs a file from its extension instead, through a
registry association. That association is a property of the machine's
configuration, not of the file, and it is not guaranteed to be present
or correct in the account and session that websocketd is running under.
This is a recurring cause of a script that runs perfectly when
double-clicked and fails to start under websocketd.

Naming `cmd.exe` or `powershell.exe` yourself removes the question. You
are no longer asking Windows to work out what to run; you are telling
it.

## Give the interpreter a full path

websocketd resolves the command you give it by searching `PATH`, the
list of directories the operating system looks in for an executable.
Two things make that search less predictable on Windows than on Unix.

The first is that names differ between installations. Node.js, for
example, installs its executable as `node.exe` under some methods, while
documentation and scripts written elsewhere refer to it as `nodejs`. A
name that works on a colleague's machine can find nothing on yours, and
websocketd reports only that it could not locate the command.

The second is that a different program with the same name can be earlier
in `PATH` and win the search. Then websocketd starts successfully and
runs the wrong program, which is the harder failure to diagnose because
nothing reports an error.

Give the full path to the interpreter and neither can happen:

```bat
websocketd.exe --port=8080 "C:\Program Files\nodejs\node.exe" C:\scripts\echo.js
```

## Your script gets no signal on disconnect

On Unix, websocketd tears a process down in stages when a client
disconnects. It closes the process's stdin, then sends
`SIGINT`, then `SIGTERM`, then `SIGKILL`, pausing between each. A
**signal** is a Unix notification that a process can catch and act on,
so those middle stages give a well-behaved program a window to save
state and exit cleanly.

Windows has no equivalent mechanism for one process to send `SIGINT` or
`SIGTERM` to another. websocketd still attempts both stages, and both
fail; you will see the failures logged as errors. The process is then
terminated forcibly. Your script gets no window in which to clean up,
and there is no fix for this, because there is no Windows facility to
use instead.

Closing stdin is the one part of the sequence that works everywhere,
and it happens first. If your script needs to do anything on
disconnect, have it watch for end-of-file on stdin rather than
for a signal. That approach behaves the same on every platform.

Windows also has no process groups in the Unix sense, so websocketd
cannot sweep up processes that your script started. Anything your script
launches is left running unless your script stops it itself.

## Next

- [Platform support](/reference/platform-support/) is the factual list
  of what differs on Windows.
- [Process lifecycle](/understanding/process-lifecycle/) has the full
  teardown sequence and its timings, and what a long-running program
  should do about it.
- [Output buffering](/understanding/output-buffering/) applies on
  Windows exactly as it does elsewhere. If your script runs but sends
  nothing, that page and the language page for whatever the script is
  written in are the place to look.
