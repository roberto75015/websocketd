---
title: "Message framing"
weight: 20
description: "A newline is what turns your program's output into a WebSocket message, and --binary changes the message type rather than turning websocketd into a terminal."
---

A WebSocket connection carries discrete messages. A pipe carries an
undifferentiated stream of bytes. Something has to decide where one
message ends and the next begins, and in websocketd that something is
the newline character.

## The newline is the boundary

By default, websocketd frames in both directions on `\n`:

Going in, each WebSocket message the client sends is written to your
program's stdin with a newline appended. Your program can read
it with an ordinary line read. It never sees WebSocket framing, message
types, masking, or any other protocol detail.

Coming out, websocketd reads your program's stdout until it
sees a newline, then sends everything up to that point as one WebSocket
message. The newline itself is stripped. A trailing `\r\n` is stripped
as a unit, so a program written on Windows does not leak carriage
returns into the message.

The second half is a hard rule, not an optimisation. Bytes your program
has written but not yet terminated with a newline are not sent. They sit
in websocketd's reader until a newline arrives, and then they go out as
one message together with whatever followed. A program that writes a
progress bar by printing characters without newlines will appear to send
nothing at all, then send the entire bar at once when it finally prints
one.

This trap has a twin, and the two compound. The framing rule is about
whether websocketd has seen a newline; [output
buffering](/understanding/output-buffering/) is about whether your
program's runtime has handed websocketd the bytes at all. A line that is
buffered inside your program's runtime is invisible to websocketd no
matter how many newlines it contains, and a flushed line with no newline
is invisible for the other reason. When output is not arriving, rule out
both, starting with buffering, which is by far the likelier of the two.

## What `--binary` changes

`--binary` changes two mechanical things, and nothing else.

It changes the WebSocket message type. Messages are sent and received as
binary frames rather than text frames. Your client code can see this
directly: in a browser, `event.data` arrives as a `Blob` or
`ArrayBuffer` rather than a string.

It removes the newline rule. Outbound, websocketd forwards whatever
bytes it reads from your program's stdout as soon as it reads
them, in chunks of up to 64KB, which is the most a single read from a
pipe can return. Inbound, the bytes a client sent are written to
stdin exactly as they arrived, with no newline appended.

So `--binary` is not cosmetic. If you need to move images, audio, or
protocol buffers through websocketd, it is the flag that lets you do it
without base64 in the middle.

## What `--binary` does not change

It does not give your program a terminal.

This is the misreading people arrive with. `--binary` looks like a
"raw mode" switch, and raw mode in a terminal emulator means
keystroke-at-a-time input with
character-by-character echo. People reach for `--binary` expecting to
pipe individual keypresses into a shell or a curses application and get
a live terminal in the browser. That is not what happens, and no
combination of flags makes it happen.

websocketd never allocates a pty. A pty, short for pseudo-terminal, is a
kernel device pair that pretends to be a terminal: it is what makes a
program believe a human with a keyboard is on the other end. websocketd
gives your program three ordinary pipes instead, one each for stdin,
stdout, and stderr, in binary mode exactly as in text mode.

Programs notice the difference, because they are designed to. A program
that checks whether its output is a terminal and finds a pipe will
usually change behaviour: it will switch to block-buffered output, drop
colour, refuse to run interactively, or abandon the
keystroke-at-a-time input model it would use on a real terminal. That
check is made by the program, inside its own process, and `--binary`
changes nothing it can see.

The permanent consequence is that programs which require a real terminal
do not work through websocketd. `vim`, `less`, `screen`, `watch`, and
`docker run -it` are all in this category. Keystroke-at-a-time input,
with no newline to end each message, is not supported and is not
planned. The reasoning is in [design
decisions](/understanding/design-decisions/).

## The two modes do not mix

websocketd reads only the message type it is configured for. A binary
frame sent to a text-mode server is discarded, and so is a text frame
sent to a `--binary` server. Nothing is reported to the client, and the
discard is recorded only at debug log level.

That silence is easy to misread. If a client's messages
are vanishing with no error anywhere, check that client and server agree
on the message type before investigating anything else. The [dev
console](/reference/dev-console/) shows you the frames as they arrive,
which settles the question quickly.

## Message size, and what happens at the limit

An inbound message larger than `--maxframesize`, which defaults to 1 MiB,
is not truncated and not ignored. websocketd closes the connection with
WebSocket close code 1009, "message too big". Your program then sees its
stdin close and is shut down through the usual [teardown
ladder](/understanding/process-lifecycle/).

The limit exists because websocketd has to hold an inbound message in
memory before it can write it to a pipe, and an unbounded message is an
unbounded allocation driven by whoever is connecting. It is a backstop,
not a sizing decision. Setting it to zero removes the limit, and removes
that memory bound along with it.

There is no compression. `permessage-deflate` is not implemented, so
every message goes over the wire uncompressed.

## Next

- [Output buffering](/understanding/output-buffering/) explains the
  other half of the "nothing is arriving" problem.
- The [flag reference](/reference/cli-flags/) has the exact defaults for
  `--binary` and `--maxframesize`.
- [Debug a script](/how-to/patterns/debug-a-script/) shows how to watch
  real frames instead of guessing.
