---
title: "The CGI environment"
weight: 50
description: "How an HTTP request becomes a child process's environment, which parts of it the client controls, and why the Proxy header is the one exception to the rule."
---

Your program has no way to ask websocketd about the request that started
it. It has stdin, stdout, and command-line arguments
that are the same for every connection. So websocketd puts the request
where any program in any language can already read it without a library:
the environment.

The convention it follows is the Common Gateway Interface,
[RFC 3875](https://tools.ietf.org/html/rfc3875), which web servers have
used to talk to child processes since 1993. CGI is not fashionable, but
it is the only request-passing convention that every language can
already read, because every language can read an environment variable.
`os.environ`,
`ENV`, `getenv`, `$_SERVER`, `process.env`: the same variables, no
adapter. It is the same reasoning that put stdin and stdout at the
centre of [the process model](/understanding/process-model/).

## The shape of the contract

The environment is built once, when the connection is accepted, and
handed to the child at exec time. It is a snapshot, not a channel.
Nothing that happens later in the connection changes it, and your
program cannot signal back through it. Everything dynamic goes over
stdin and stdout.

Three kinds of thing end up in it. There are facts about websocketd
itself, such as its version. There are facts about the request, derived
from the URL, the connection, and the headers. And there are variables
inherited from the environment websocketd was started in, but only the
ones you named.

That last category is an allowlist, not a passthrough, and it is the
most important structural fact on this page. On every platform except
Windows, websocketd wipes its own environment at startup, having first
copied out the variables `--passenv` names. Nothing your program sees
was inherited by accident. If you start websocketd from a shell holding
a database password in an environment variable, that password does not
reach your script unless you asked for it by name.

The exact list of variables, what populates each one, and how the
`--cgidir` set differs, is in the [environment variable
reference](/reference/environment-variables/).

## What the client controls

Some of these variables describe the server. Others describe the
client. The distinction matters because the client writes its own half.

`SERVER_NAME` and `SERVER_PORT` sound like server facts and are not.
They are derived from the request's `Host` header, which is a string the
client chose. Anything that can open a connection can put anything it
likes there. A request arriving on port 8080 can claim `Host:
admin.internal:443`, and your script will see exactly that in
`SERVER_NAME` and `SERVER_PORT`.

This is not a defect, and it is not going to be changed. Host-derived
values are what virtual hosting requires and what every CGI server does,
including Go's own `net/http/cgi`. It is documented and accepted
behaviour. What it means for you is a rule with no exceptions: never
make a trust or access-control decision from `SERVER_NAME` or
`SERVER_PORT`. If your script needs to know which interface or port it
is really serving, that comes from your deployment configuration, which
you control, not from the request, which you do not.

The same caution applies more obviously to `QUERY_STRING` and to the
`HTTP_` variables built from request headers. Those are unambiguously
client input. Treat them the way you would treat anything arriving from
the network, because that is what they are. Header values are
lightly normalised on the way in, with newlines and carriage returns
replaced by spaces so that a header cannot forge a second environment
entry, but their content is otherwise the client's.

`REMOTE_ADDR` and `REMOTE_PORT` are the exception in the other
direction. They come from the socket rather than from anything the
client wrote, so they are as trustworthy as your network path. Behind a
reverse proxy they describe the proxy.

## The Proxy header, and why it is special

One request header never becomes an environment variable. websocketd
drops `Proxy` before mapping headers into `HTTP_` variables, so a client
cannot cause `HTTP_PROXY` to appear in your program's environment.

The reason has a name: httpoxy, CVE-2016-5385, disclosed in 2016. The
vulnerability sits at the intersection of two conventions that were
individually reasonable and catastrophic together. CGI says that a
request header `Foo` becomes the environment variable `HTTP_FOO`. Quite
separately, a long Unix tradition says that a program wanting to make
outbound HTTP requests should read the variable `HTTP_PROXY` to find out
which proxy to use, and most HTTP client libraries do exactly that.

Put them together and a remote client can send a header called `Proxy`,
which becomes `HTTP_PROXY` in a CGI script's environment, which the
script's own HTTP library then obeys. Every outbound request the script
makes, including ones carrying credentials, is routed through a server
the attacker chose. The script does nothing wrong; it is asked to by an
environment variable it had no reason to distrust.

The whole CGI ecosystem was affected in 2016, and the fix everywhere was
the same: refuse to map that one header. Go's `net/http/cgi` carries
the same guard, and so does websocketd. The exception matters because
it is the one place where the "every header becomes a variable" rule is
not true.

## There is no authenticated user

`AUTH_TYPE`, `REMOTE_USER`, and `REMOTE_IDENT` are part of the CGI
specification and describe an authenticated caller. In websocketd they
are always empty strings.

They are set to empty deliberately rather than left out. If they were
simply absent, a variable of that name inherited from the parent
environment could survive into your script and be mistaken for an
authenticated identity. Blanking them means the answer to "who is this
user" is always, unambiguously, "websocketd does not know".

It does not know because it never asks. websocketd has no built-in
authentication at all, by design; that decision, and what to do instead,
is covered in [the security
model](/understanding/security-model/) and worked through in [adding
authentication](/how-to/patterns/add-auth/).

## PATH crosses the boundary on purpose

`--passenv` defaults to passing `PATH`, along with the platform's shared
library search path. Almost every script needs it, because without a
`PATH` even `ls` is unfindable and most interpreters cannot locate their
own helpers.

That has a cost. Your `PATH` describes your
machine's directory layout: where you keep binaries, which language
version managers you use, sometimes your username in a home directory
path. Any code running inside your script can read it. If your script
executes anything derived from client input, that includes the client's
code.

This is a considered default, and you can change it. `--passenv`
replaces the default list rather than adding to it, so naming your own
variable drops `PATH` unless you name `PATH` too.

## Two implementations, not one

websocketd builds the environment above for WebSocket connections
itself. It does not do so for `--cgidir`, which hands the request to
Go's `net/http/cgi`, and that package builds its own environment to its
own rules.

Most variables agree. Several do not, `SCRIPT_NAME` and `PATH_INFO`
among them, and a script that routes on those will behave differently
under the two modes. The differences are tabulated in the [environment
variable reference](/reference/environment-variables/). These are two
separate code paths, so "websocketd sets X" is a claim that needs a mode
attached to it.

## Next

- [Environment variables](/reference/environment-variables/) is the
  complete table, including the `--cgidir` differences.
- [The security model](/understanding/security-model/) covers the trust
  boundaries this page's warnings imply.
- [Passing data into your script](/how-to/patterns/pass-arguments/)
  shows how to read the query string.
