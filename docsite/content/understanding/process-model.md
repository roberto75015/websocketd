---
title: "The process model"
weight: 10
description: "One process per WebSocket connection is the decision everything else follows from: it lets websocketd wrap any program, and it rules out shared state between connections."
---

websocketd starts one fresh instance of your program for every WebSocket
connection, and tears that instance down when the connection closes. Two
browser tabs mean two processes. A thousand connections mean a thousand
processes. Nothing is shared between them.

Almost every other question about websocketd resolves to this one fact.
Understand it before you design anything on top of it: the model is
unusually generous in one direction and completely closed in the other.

## What the model gives you

Your program does not have to know that websocketd exists. It reads
lines from stdin and writes lines to stdout, which is
the oldest and most portable interface in computing. That is the whole
contract.

The consequences are larger than they first look:

**Any language works, with no library.** There is no websocketd client
library for Python, no gem for Ruby, no npm package, and there never
needs to be. If the language can print a line, it can serve a WebSocket
connection. That is why the [examples](/reference/examples/) span a
dozen languages with no shared code between them.

**You can test without a browser.** A program that talks over stdin and
stdout can be run directly in a terminal and driven by typing at it. If
it behaves there, it will behave under websocketd. Debugging a
misbehaving endpoint usually means running the program by hand first,
which is what makes [debugging a
script](/how-to/patterns/debug-a-script/) tractable.

**One connection cannot corrupt another.** A crash, a memory leak, an
infinite loop, or a wild pointer is contained inside one operating
system process. The other connections do not notice. You get this
isolation for free, without writing a single line of defensive code, and
without the careful state hygiene a shared-process server demands.

**Your program can be stateful without being careful.** Inside a single
connection, your script owns the world. Global variables, open files,
accumulated buffers: none of it needs locking, because nothing else is
looking at it. Concurrency bugs are the class of bug this model deletes
outright.

## What the model takes away

There is no broadcast. There is no shared state. There is no
cross-connection anything.

Concretely: a variable your script sets while serving one connection
does not exist in any other connection's process. There is no built-in
publish/subscribe, no "send this message to every connected client", no
registry of who is currently connected, and no shared in-memory store.
websocketd does not keep a list of live connections that your program
can reach, because your program is not the kind of thing that could
reach one.

This is the most common surprise websocketd springs on people, and it
is not a gap waiting to be filled. The reasons are in [design
decisions](/understanding/design-decisions/).

There is also no singleton. No process is running when websocketd
starts; the first one appears when the first client connects. If you
were hoping for one long-lived process that all clients talk to, that is
the opposite of what websocketd does. See [process
lifecycle](/understanding/process-lifecycle/) for exactly when processes
appear and disappear.

## The cost of a process

A process is heavier than a thread and much heavier than an async task.
Every connection costs a fork, an exec, three pipes, and whatever memory
your program's runtime demands at startup. A Python interpreter is tens
of megabytes; a small C program is a rounding error. The model's cost is
therefore mostly your program's cost, not websocketd's.

That cost is bounded on purpose. websocketd refuses to fork past
`--maxforks`, which defaults to 1024 concurrent processes, and answers
further upgrade requests with `429 Too Many Requests` until one
finishes. The default is a backstop against a client that opens
connections in a loop, not a capacity recommendation. If you need more,
raise it deliberately, having first worked out what a thousand copies of
your program cost on that host. The exact behaviour is in the
[flag reference](/reference/cli-flags/).

This model suits tens or hundreds of concurrent
connections running programs you wrote, on a host you control. It is not
built to hold a hundred thousand idle sockets. If that is your problem,
you want a purpose-built server, and websocketd will tell you so by
running out of processes rather than by degrading quietly.

## Where shared state goes

The model does not stop you from building a chat room or a live
dashboard. It moves the shared part somewhere else, which is where it
was always going to have to live once you had more than one host
anyway.

The usual shapes are an external message bus that already knows how to
fan one message out to many subscribers, a shared file that several
processes on the same host append to and follow, or a single
long-lived backend service that the per-connection scripts act as thin
clients for. Choosing between them is a real decision with real
trade-offs, worked through in [sharing state across
connections](/how-to/patterns/share-state/).

## Next

- [Message framing](/understanding/message-framing/) covers what
  crosses the stdin and stdout boundary.
- [Process lifecycle](/understanding/process-lifecycle/) covers when the
  process starts and how it is shut down.
- [Design decisions](/understanding/design-decisions/) covers why this
  model was kept, including the process pool that was built and then
  removed.
