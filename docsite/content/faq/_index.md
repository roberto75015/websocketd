---
title: "FAQ"
weight: 50
description: "The ten questions readers ask most, each answered in a sentence with a link to the page that covers it properly."
---

Ordered by how often people hit them. Every answer here is a pointer to
the page that covers the subject.

## Why doesn't my script send output in real time?

Your language's runtime is holding the output in a buffer, because stdout
is a pipe rather than a terminal. The fix is one flag or one line, and it
is on your language's page: [Python](/how-to/languages/python/),
[Ruby](/how-to/languages/ruby/), [PHP](/how-to/languages/php/),
[C](/how-to/languages/c/), [Node.js](/how-to/languages/nodejs/).

## How do I share or broadcast data across connections?

Not through websocketd: each connection gets its own process, and those
processes share nothing. Put the shared part outside them, as described in
[share state across connections](/how-to/patterns/share-state/).

## Does this support authentication?

No, websocketd has no built-in authentication and sets `AUTH_TYPE` and
`REMOTE_USER` empty for every request. Authenticate in front of it or
inside your script, both covered in
[add authentication](/how-to/patterns/add-auth/).

## How do I pass URL or query-string data to my script?

The query string arrives in your script's environment as `QUERY_STRING`,
along with the rest of the CGI variables, with no flag needed. See
[pass data into your script](/how-to/patterns/pass-arguments/), which also
untangles this from `--passenv`.

## Why did my connection drop after a few minutes?

`--pingms` defaults to `0`, meaning websocketd sends no keepalive pings,
so an idle connection can be dropped by a proxy, a load balancer, or the
operating system without anything noticing. Set it to a non-zero value;
[the CLI flag reference](/reference/cli-flags/) has the details.

## Is this production-ready? Isn't a process per connection wasteful?

It is a deliberate trade: a process per connection costs more than a
thread, and in return connections are completely isolated and your backend
can be written in anything. [The process model](/understanding/process-model/)
sets out what that buys and where it stops making sense.

## How do I run a Windows `.bat`, `.cmd`, or `.ps1` file?

Windows ignores the `#!` line at the top of a script, so name the
interpreter yourself and pass the script to it as an argument. See
[Windows scripts](/how-to/languages/windows-scripts/).

## How do I keep a process running, or run it exactly once?

websocketd starts a fresh process for every connection and has no mode
that changes this, so the run-once behaviour goes in a small wrapper
around your program. See [run a program once, or keep it
running](/how-to/patterns/run-once/).

## Can I run this behind nginx or Apache?

Yes, provided the proxy is configured to pass the WebSocket upgrade
through rather than terminating it. Working configurations are on the
[nginx](/how-to/deploy/nginx/), [Apache](/how-to/deploy/apache/), and
[HAProxy](/how-to/deploy/haproxy/) pages.

## Why can't I use `screen`, `watch`, or `docker run -it`?

websocketd gives your program plain pipes and never a pty, so anything
that checks for a terminal before behaving interactively will not work
through it. This is permanent, and the reasoning is in
[design decisions](/understanding/design-decisions/).
