---
title: "The security model"
weight: 60
description: "websocketd executes a program on behalf of whoever connects, so its security posture is about who may connect, what the connection can reach, and which parts of the job belong to you."
---

websocketd runs a program for anyone who can open a connection to it.
That is the feature. It is also the entire security model in one
sentence, and everything below is a consequence of taking it seriously.

The right way to think about a websocketd endpoint is as a remotely
callable program, not as a web page. If the program can delete files,
then whoever can connect can delete files. Every question worth asking
reduces to who is allowed to connect, and what the program they reach
is allowed to do.

## Who may connect: the origin policy

The most important thing to understand about WebSocket connections is
that browsers do not protect you from them.

The same-origin policy that stops a page on one site from reading
another site's data does not apply to WebSocket. A page on any site,
open in any browser on any machine that can route to your server, may
open a WebSocket connection to it. The browser will send an `Origin`
header saying where the page came from, and will then connect
regardless of what your server thinks of the answer. Deciding what to do
with that header is entirely the server's job.

By default, websocketd accepts every origin. That default exists to
make local development frictionless, and it is fully permissive: a
websocketd you started to try something out on your laptop can be driven
by any web page you happen to visit while it is running. The page cannot
see your filesystem, but it can talk to the program you wrapped, and
that program can.

Because that is easy to miss, websocketd prints a warning about it to
stderr every time it starts without an origin policy. The
warning goes to stderr rather than into the log stream on
stdout, so that redirecting the log to a file still leaves the
notice visible on the console. It names the three ways to resolve it.

`--sameorigin` accepts an upgrade only when the `Origin` header matches
the `Host` of the same request. It is the natural choice when websocketd
is the thing the browser talks to directly, including when websocketd
serves the client page itself. It is a poor fit behind a
TLS-terminating reverse proxy, where the browser's origin implies port
443 and the proxied request implies port 80, so every upgrade is
rejected.

`--origin` takes an explicit list of acceptable origins and is matched
against the `Origin` header alone. That makes it the right tool behind a
proxy, and the right tool whenever you know the public origin your
clients will come from.

`--anyorigin` says you have considered the question and chosen the
permissive answer, and silences the warning. It is meaningful precisely
because it is a decision. Reach for it when authentication happens in
front of websocketd, or when nothing but localhost can reach the port,
and not merely to quiet the console.

The three are not interchangeable. `--anyorigin` contradicts the other
two and websocketd refuses to start if you combine them. `--sameorigin`
and `--origin` compose, and an upgrade must then satisfy both.

Origin matching is strict about ports, deliberately. An entry naming a
port matches that port only. An entry with no port matches only the
default port for its scheme. A host is not a blanket endorsement of
everything listening on it, because in practice a host runs services of
very different sensitivities, and allowlisting a web app should not
also allowlist whatever is on the port next door. Where a host really
is uniformly trusted, an explicit `:*` suffix says so.

A future release will default to `--sameorigin`. No date is set. If you
depend on today's permissive default, pass `--anyorigin` now and the
change will not move under you.

The origin policy is a control on browsers, and a good one, because a
browser reliably tells the truth in the `Origin` header. It is not a control on anything else. A script
with a WebSocket library sets whatever origin it likes. Origin policy
raises the floor against hostile web pages; it is not authentication.

## What the connection can reach

Once a connection is accepted, it reaches a process, and that process is
where your real security boundary lives.

That process should run as a user with only the privileges the job
needs. Everything arriving on its stdin is attacker-controlled by
construction, and so is every request-derived value in the [CGI
environment](/understanding/cgi-environment/), where `SERVER_NAME` and
`SERVER_PORT` are client-controlled despite their names. Passing any of
it to a shell is the classic mistake.

websocketd puts backstops around the process rather than around your
program's logic, because it cannot know your logic. `--maxforks`, at
1024 by default, stops a client opening connections in a loop until the
host runs out of processes. `--maxframesize`, at 1 MiB by default, stops
a client sending one enormous message that websocketd must hold in
memory. Both are limits on abuse, not capacity plans, and neither should
be read as advice about how much traffic to expect.

When you listen on a Unix domain socket rather than a TCP port, the
access control moves to the filesystem, and the socket file's permissions
are what decide which local users can connect. websocketd leaves those
permissions to your umask unless you say otherwise, which under a
permissive umask can leave the socket open to every account on the
machine. `--socketmode` pins them, applied the moment the socket is
bound. It changes nothing else, and nothing beyond that file.

The main server sets no read or write timeout, on purpose. A timeout on
the body of a request would kill exactly the long-lived streaming
connections websocketd exists to carry. What it does set is a header
timeout, bounding how long a client may take to send its request
headers, which is what closes off a slowloris-style attack that opens
many connections and dribbles bytes into them forever. Idle connections
are a separate matter, handled by `--pingms` keepalives rather than by a
blunt timeout.

## Transport: TLS

`--ssl`, with `--sslcert` and `--sslkey`, serves `https://` and
`wss://`. The minimum protocol version is pinned at TLS 1.2 explicitly,
rather than inheriting whatever the Go runtime happens to default to
this year.

`--sslca` additionally requires every client to present a certificate
signed by the named authority, verified during the handshake, before
your program is ever launched. That is mutual TLS, and it is the one
form of client authentication websocketd does have. It suits
machine-to-machine deployments where you control both ends. It does not
suit browsers, where certificate provisioning is a real burden.

`--sslca` requires `--ssl`: websocketd rejects `--sslca` given without
`--ssl` at startup, with exit code 1, because mutual TLS has no
handshake to verify a client certificate in without a TLS listener.
Setting up either is covered in [serving over
wss://](/how-to/deploy/tls/).

`--redirport` opens a second plain HTTP listener whose only response is a
301 to the canonical address, so that a visitor who typed `http://` still
arrives. The `Location` it sends keeps the host the client itself named
and the path and query it asked for, and rewrites only the scheme and the
port. That is what stops it being an open redirect: the path and query
are resolved against the canonical origin as a relative reference, and a
reference cannot introduce a host of its own, so a request for
`//evil.com/` redirects to `//evil.com/` as a path on your server rather
than to another site.

The host comes from the client's request, which means a client that names
a different host in its `Host` header, or writes an absolute URL in its
request line, is redirected to the host it named. It is redirecting
itself, and it could have gone there without asking you, so this is not a
way to redirect anybody else. It does mean the redirect listener is a poor
place to look for evidence of who visited your canonical address.

## There is no built-in authentication

websocketd authenticates nobody. `AUTH_TYPE`, `REMOTE_USER`, and
`REMOTE_IDENT` are always blank for that reason.

This is settled, not pending. Authentication is the part of a system
most tightly bound to the organisation around it: a session cookie, a
bearer token, mutual TLS, an SSO provider, an internal service mesh.
Building one of those in would serve a narrow slice of deployments and
would put websocketd in the business of storing credentials, which is a
different and much less forgiving product than a program that pipes
stdin to a socket. The reasoning is expanded in [design
decisions](/understanding/design-decisions/).

There are two good places to put it instead, and which one fits depends
on whether you already have a reverse proxy in the path.

Put it in front. A reverse proxy that authenticates before forwarding
means websocketd only ever sees requests that already passed, and your
script does not have to think about identity at all. This is the usual
answer for anything facing the public internet.

Or put it inside. Your script can read a token from the query string or
a request header through the CGI environment, check it, and refuse to do
anything useful otherwise. This suits deployments with no proxy, and it
has the advantage that the check lives next to the thing it protects.
Both are worked through in [adding
authentication](/how-to/patterns/add-auth/).

## What `--staticdir` refuses

Serving files is the one thing websocketd does that is not running a
program, and it comes with its own refusals. They all follow from one
rule: a request is answered only with a plain file that really is inside
the directory you named. Each refusal below is a way a file can fail
that test.

A path with a segment beginning with `.` is refused, so a `.git`
directory or an `.env` file sitting in the tree cannot be fetched by
name. A directory with no `index.html` is refused rather than listed, so
a directory nobody wrote a page for does not become a public index of
its own contents. A file reached through a symlink that leaves the
directory is refused, because what decides the answer is where the file
really lives, not the path that named it.

The one exception is `/.well-known/`. [RFC
8615](https://www.rfc-editor.org/rfc/rfc8615) reserves that exact
directory name for URIs meant to be served publicly, such as an ACME
client's `/.well-known/acme-challenge/<token>` or a `security.txt`.
Refusing it would break a standardised convention with no flag to opt
back in, so it is matched on that exact first path segment and served. A
dotfile nested deeper inside it, or a directory whose name merely starts
with `.well-known`, is refused like any other.

### Files you configured to be executed are never handed back as source

`--dir` and `--cgidir` name directories of programs. What a client is
meant to see is a program's output; its source is a different thing, and
it routinely carries credentials, internal hostnames, and the shape of
the system behind it. So the static handler refuses any file living in
either tree and answers 404 instead.

That refusal has to be about the file rather than about the URL, because
a URL has many more spellings than the layout has directories. A symlink
elsewhere in the static tree pointing into the script directory names
the same file. So does a differently-cased path on a filesystem that
folds case, and so does a path that walks up out of a directory and back
down into it. websocketd resolves the file and compares directory
identity, so every one of those spellings gets the same answer.

With `--staticdir=/PAGE --dir=/PAGE/scripts`, a browser asking for
`/scripts/hello.sh` sends no `Upgrade` header, so the request never
reaches the WebSocket handler at all. Without this refusal it falls
through to the static handler and comes back as the text of the script.

A `--dir` script keeps the URL `--dir` gives it and gains no second one
from where the directory sits. In that same layout `/hello.sh` reaches
the script and `/scripts/hello.sh` reaches nothing.

One layout is exempt: a `--staticdir` naming one of those directories
itself, or a directory inside it. `--dir=. --staticdir=.` is the oldest
demo layout in the project, and excluding the script tree there would
leave the static handler with nothing whatsoever to serve. Scripts are
still served as source in that layout. If that is not what you want, put
the static files and the scripts in separate directories.

### Where the CGI directory sits decides its URL

Nesting the other way round is the natural layout for a self-contained
site: `--cgidir` pointing at a `cgi-bin` inside the `--staticdir` root.
The URL a browser forms for a script there is `/cgi-bin/hello.sh`, and
websocketd runs the script for it, alongside the `/hello.sh` that
`--cgidir` has always answered.

It works that position out by resolving both directories and asking
which directory each flag names, not by comparing the two strings you
typed. A release symlink named by one flag and by its real path in the
other is one pair of directories however it is spelled, and it routes as
one pair. Identity constrains the answer in the other direction too: a
`--cgidir` genuinely outside the static root is not brought inside it by
a symlink there. Only the two directories you named are resolved, so a
URL reaching the scripts through such a link stays refused.

The position is settled once and then reused, which matters if you
deploy by swapping a symlink. Flipping it under a running websocketd
does not re-route requests. Restart to pick up the new layout.

## What is still yours to get right

None of this vets what you put in the directory in the first place. A
secret checked in under a name that does not start with a dot
(`credentials.txt`, say) is served like any other file. The refusals
above are about how a file can be reached, not about whether it belonged
in a published tree, so `--staticdir` is not something to point at a
home directory and rely on. Point it at a tree that holds only what you
mean to publish.

The exec-directory exclusion covers the directories you named on the
command line, and only those. A second copy of the same scripts
elsewhere under `--staticdir` is an ordinary file as far as websocketd
is concerned.

The one thing that remains entirely the operator's responsibility is
exposure itself. websocketd binds where you tell it to, and binding to a
public interface is a decision to accept connections from the internet.
If you are about to do that, [the exposure
checklist](/how-to/deploy/public-internet/) is what to work through
first.

## Next

- [The CGI environment](/understanding/cgi-environment/) has the trust
  boundaries in the request data your script reads.
- [Adding authentication](/how-to/patterns/add-auth/) has the two
  working patterns.
- The [flag reference](/reference/cli-flags/) has every default named
  above, exactly.
