---
title: "Process lifecycle"
weight: 40
description: "A wrapped program starts when the first client connects, and is shut down by an escalating ladder of stdin close, SIGINT, SIGTERM and SIGKILL that a long-running program has to cooperate with."
---

A wrapped program's life is bounded exactly by one WebSocket connection.
It does not exist before the connection opens, and websocketd works
hard to make sure it does not exist after the connection closes. The
second end is where a long-running program can go wrong.

## Nothing runs until someone connects

When websocketd starts, it binds a socket and waits. It does not launch
your program. Run `ps` immediately after starting websocketd and you
will find websocketd and nothing else.

The first instance of your program appears when the first client
completes a WebSocket handshake, and it is created for that client
alone. This is deliberate, not a scheduling delay, and it follows
directly from [one process per
connection](/understanding/process-model/): there is no such thing as a
warm process waiting for a client, because a process only means anything
in the context of the connection it serves.

Two practical consequences follow. Startup errors in your program are
not startup errors in websocketd, so a script with a broken shebang line
or a missing interpreter will start websocketd cleanly and fail on first
connect. And any expensive initialisation your program does at startup
is paid once per connection, not once per server.

## The teardown ladder

When the connection closes, websocketd does not immediately kill your
program. Killing a program outright denies it the chance to flush a
file, commit a transaction, or release a lock. Instead websocketd asks
progressively less politely, waiting after each request, and stops the
moment the process is reaped.

It begins by closing your program's stdin. For a great many
programs this is sufficient on its own: a script whose main loop reads
lines until end of input will simply run off the end and exit. If that
happens within about 100 milliseconds, nothing further is sent.

If the program is still running after that wait, websocketd sends
**SIGINT**, the signal a terminal delivers on Ctrl+C, and waits about
250 milliseconds.

If it is still running, websocketd sends **SIGTERM**, the conventional
"please shut down" signal, and waits about 500 milliseconds.

If it is still running, websocketd sends **SIGKILL**, which cannot be
caught, blocked, or ignored, and waits up to 1000 milliseconds for the
kernel to finish the job. If even that does not produce a reaped
process, websocketd logs an error and stops waiting.

Whichever rung it reaches, and including the fast path where the program
exits immediately on end of input, websocketd finishes by sending a
final SIGKILL to the whole process group. Nothing is left behind because
teardown ended early.

`--closems` lengthens the first three waits, and only those three. It
adds its value to the 100, 250, and 500 millisecond waits, and does not
touch the final 1000 millisecond wait for SIGKILL to take effect. Raise
it when your program needs longer than a fifth of a second to notice a
signal and clean up after itself, which is common for anything that has
to finish a network round trip on the way out.

## Why a long-running program must handle signals

If your program is a short script that exits when its input runs out,
none of the ladder above will ever be visible to you.

If your program loops forever, ignores signals, and does not read
stdin, the picture is different. Closing stdin tells
it nothing, because it is not reading. SIGINT and SIGTERM both have a
default action of terminating the process, so an unhandled signal will
still stop it. The trouble is the program that installs a handler and
then does nothing useful in it, or that blocks the signal, or that is
stuck in a call that will not return. Such a program rides the ladder to
the bottom on every single disconnect and is killed with SIGKILL,
which means it never gets to flush anything, ever.

So if your program is long-running rather than a script that finishes on
its own, handle SIGINT and SIGTERM and exit promptly on either. That is
the entire contract. It costs a few lines, and it is the difference
between a clean shutdown and a program the operating system kills a
fraction of a second later, mid-write.

## What happens to the children your program spawns

On Unix, websocketd puts each wrapped program into its own process
group, using `setpgid`. A process group is a set of processes the kernel
can signal as a unit. Children your program spawns inherit its group,
which means the SIGINT, SIGTERM and SIGKILL of the ladder reach them
too, and the final sweep catches anything still alive in the group when
the direct child is gone. Your program does not have to forward signals
for this to work.

Only the stdin close is specific to the direct child. It is a
pipe, and only the program on the other end of it can notice.

The consequence to plan for is the reverse one. A process that is
supposed to **outlive** the connection has to leave the group
deliberately, by starting a new session with `setsid` or an equivalent.
Anything still in the group when the connection ends is killed. This is
the standard Unix opt-out and it is intentional: the default is that a
connection cleans up completely after itself.

On Windows the picture is smaller. Windows has no process groups to
signal, and no mechanism for delivering SIGINT or SIGTERM to another
process at all, so those rungs of the ladder fail and are logged.
Windows teardown is effectively "close stdin, wait, then
terminate the direct child", and there is no group sweep. A child your
program started on Windows keeps running unless your program stops it
itself. The details are in [platform
support](/reference/platform-support/).

## When the connection is closed by websocketd

Not every teardown starts with the client. websocketd closes the
connection itself when an inbound message exceeds `--maxframesize`, when
`--pingms` keepalives go unanswered for twice the ping interval, and
when your program's stdout reaches end of file, which usually
means the program exited on its own. The ladder is the same in all of
those cases: from the program's point of view, teardown always looks
like stdin closing, followed by escalating signals.

## Next

- [The process model](/understanding/process-model/) explains why there
  is one process per connection in the first place.
- The [flag reference](/reference/cli-flags/) has `--closems`,
  `--maxframesize`, and `--pingms` with their exact defaults.
- [Run a program once, or keep it running](/how-to/patterns/run-once/)
  covers the shapes a wrapped program can take.
