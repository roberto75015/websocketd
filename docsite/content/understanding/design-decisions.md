---
title: "Design decisions"
weight: 70
description: "Why websocketd has no broadcast, no pty, no built-in authentication and no compression, and what was built and then deliberately removed."
---

Most of what websocketd does not do, it does not do on purpose. This
page collects the reasoning, including the two features that were
built and then taken out again.

There is one criterion behind nearly all of it. websocketd's value comes
from the size of the contract it asks your program to honour: read
lines from stdin, write lines to stdout. Every feature
proposed here would have added something to that contract, or added a
concept your program would eventually have to know about. Staying small
is not modesty. It is the reason a shell script and a Haskell program
can both be WebSocket servers with no adapter in between.

## Why not broadcast

The most requested feature, over more than a decade, is a way to send
one message to every connected client.

The problem is not that it is hard. The problem is that "broadcast"
turns out to be at least four different features wearing one word. A
chat room wants messages fanned out to a named room with membership and
history. A monitoring dashboard wants the latest value pushed to
whoever happens to be watching, with no history at all and no need to
deliver anything to a client that missed it. A multiplayer game wants
ordered, low-latency delivery with per-client filtering. An
administrative tool wants one message to one other session.

Each of those needs different guarantees about ordering, durability,
membership, and what happens to a client that reconnects. Any built-in
mechanism would have to pick one set, and would then be wrong for the
other three while still being in everyone's way. Meanwhile websocketd
would have acquired a concept it currently does not have: a registry of
live connections, with a lifetime, a memory cost, and its own failure
modes.

The alternative is not a hardship, because the thing that fans messages
out is a thing you probably want to be able to restart independently of
your web tier anyway. A message bus, a shared file, or a long-lived
backend service that the per-connection scripts talk to: all
three are ordinary, and all three keep working when you add a second
host, which an in-process broadcast would not. [Sharing state across
connections](/how-to/patterns/share-state/) works through the choice.

## Why not a process pool

A related idea, and one that got further: rather than forking a new
process per connection, keep a pool of already-started processes and
hand each new connection an idle one. Forking is the expensive part of
[the process model](/understanding/process-model/), and pooling is the
standard way to amortise it.

It was implemented in 2014 and then explicitly removed.

Pooling requires the pooled thing to be reusable, and a program that
reads stdin until end of input is not. To hand a used process to a new
client you must be able to reset it: clear its accumulated state, rewind
whatever it read, discard whatever it half-wrote, and be sure the
previous connection's data cannot leak into the next one. None of that
is possible from outside the process. The only way to achieve it is for
the program itself to know it is being pooled and to reset on command,
which means a new obligation in the contract, and a leak of one
connection's data into another if any program gets that obligation
wrong.

That trade is a bad one. Fork cost is real but bounded, and it is paid
in exchange for the crash isolation and the zero-effort statefulness
that make the model worth having. A pool would have traded a guarantee
for a performance improvement, on a tool whose whole appeal is the
guarantee.

## Why no pty

websocketd hands your program three pipes. It never allocates a
pseudo-terminal, a pty, which is the kernel device that makes a program
believe a real terminal is attached.

A pty would fix one real problem. Runtimes check
whether stdout is a terminal and switch to block buffering when
it is not, which is the cause of nearly every "my script sends nothing"
report; see [output buffering](/understanding/output-buffering/). Under
a pty they would stay line-buffered and the problem would evaporate.

But a pty is not a buffering fix with no other effects. It is a terminal,
and a terminal brings a large surface with it: line discipline, echo,
canonical versus raw modes, window size and the SIGWINCH that reports
changes to it, job control, signal generation from control characters,
and a stream of escape sequences mixed into the output that a WebSocket
client would then have to interpret. A wrapped program under a pty
starts emitting cursor movement and colour codes. websocketd would go
from a pipe with a framing rule to a partial terminal emulator, and the
browser side would need a terminal emulator to match. That is a
different product, and good ones already exist.

There is also a plainer reason: pty allocation is a Unix concept, and
websocketd runs on Windows.

The permanent consequence is that programs requiring a real terminal do
not work through websocketd. `vim`, `less`, `screen`, `watch` and
`docker run -it` were each reported independently, and they are all the
same limitation. Non-newline-terminated interactive input, the
keystroke-at-a-time model those programs want, is not supported and is
not planned. [Message framing](/understanding/message-framing/) explains
why `--binary` does not change this, which is the usual next question.

## Why no built-in authentication

Authentication is the piece of a system most tightly coupled to the
organisation it lives in. A cookie from an existing session store, a
bearer token from an identity provider, mutual TLS, an internal service
mesh, HTTP basic auth behind a VPN: these are not variations on one
feature, they are entirely different systems, and the right one is
determined by infrastructure that websocketd cannot see.

Choosing one would serve a narrow slice of deployments. It would also
change what websocketd is. A tool that authenticates must store or
verify credentials, which brings key management, rotation, timing-safe
comparison, lockout policy, and a much less forgiving relationship with
its own bugs. websocketd would become a security product that happens to
pipe stdin, instead of a program that pipes stdin and delegates security
to things built for it.

The delegation is the point, and it is not a workaround: a reverse proxy
in front, or a token check inside your own script, both of which are
covered in [adding authentication](/how-to/patterns/add-auth/) and in
[the security model](/understanding/security-model/). Mutual TLS via
`--sslca` is the one exception, and it is there because it happens at the
transport layer where websocketd already sits, without requiring
websocketd to hold a credential store.

## Why no compression

The WebSocket protocol has a compression extension,
`permessage-deflate`, and websocketd does not implement it. Every
message goes over the wire uncompressed.

The traffic websocketd carries is usually small messages sent
frequently, which is the case where per-message compression pays worst:
the dictionary never warms up and the CPU cost is not repaid. Where
compression does pay, the payloads are large and usually already
compressed at the application layer, or the deployment already has a
reverse proxy in the path that can do it. Adding the extension would
mean negotiation state, a per-connection compression context with its
own memory cost, and a well-known class of memory-exhaustion concerns,
in exchange for a saving that the deployments most likely to want it can
already obtain elsewhere.

## Why multiple listen sockets look the way they do

websocketd can listen on several addresses, and the way you ask for it
is by repeating `--address`. That flat, repeatable flag is the second
design.

The first was a more elaborate multi-socket implementation, and it did
not work properly. It was replaced in 2014 by the repeated-flag model,
which has the advantage of being obvious: each occurrence adds one
listener, there is no configuration syntax to learn, and there is no
state shared between listeners to get wrong. It is a good illustration
of the general pattern on this page, which is that the simple version
survived and the clever version did not.

## Why version numbers are typed by hand

websocketd's version string is set manually in the source. It is not
derived from the git tag, the commit, or the build date.

This is the second time that decision was made. Automatic
per-build version numbers were introduced and then reverted, on the
grounds that a version number should mean something to a person
deciding whether to upgrade, and a number that increments on every build
does not. It marks a release, which is a deliberate act, not a
compilation, which is not.

The practical consequence is that the version you see from `--version`
identifies a release rather than a build. If you need to know exactly
which build you are running, the artefact you downloaded is what
identifies it. What changed between releases is in the
[changelog](/changelog/).

## Next

- [The process model](/understanding/process-model/) is the decision the
  rest of these follow from.
- [The security model](/understanding/security-model/) covers the
  posture the no-authentication decision produces.
- The [FAQ](/faq/) has the short forms of these answers.
