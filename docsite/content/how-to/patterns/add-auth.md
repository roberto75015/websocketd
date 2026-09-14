---
title: "Add authentication"
weight: 20
description: "websocketd authenticates nobody, so put the check in a reverse proxy in front of it, or validate a token inside the wrapped script."
---

websocketd has no built-in authentication, so the check goes either in a
reverse proxy in front of it or inside the script it wraps. This page
gives you both. [The security model](/understanding/security-model/)
covers why the choice is yours rather than a flag.

Your script cannot look up who is calling. `AUTH_TYPE`, `REMOTE_USER` and
`REMOTE_IDENT` are part of the CGI specification, and websocketd sets all
three to the empty string on every connection. Reading them tells you
nothing, ever.

## Which one to use

Put the check **in front** if a reverse proxy is already in your path, or
if the endpoint faces the public internet. An unauthenticated request
never reaches websocketd, so no process is started, no `--maxforks` slot
is taken, and one place in your stack owns the decision.

Put the check **in the script** if there is no proxy and you want to keep
the deployment to one moving part. The cost is that websocketd forks your
script before the check runs, and every script you write has to get the
check right on its own.

## In front: a reverse proxy

Bind websocketd to the loopback interface so the proxy is the only way
in, and use `--origin` rather than `--sameorigin`, because a
TLS-terminating proxy rewrites the `Host` that `--sameorigin` compares
against:

```sh
websocketd --port=8080 --address=127.0.0.1 \
  --origin=https://example.com ./myscript.sh
```

### A shared password

Enough for an internal tool. Create the password file once:

```sh
htpasswd -c /etc/nginx/websocketd.htpasswd someuser
```

Then gate the proxied location on it:

```nginx
location / {
    auth_basic           "Restricted";
    auth_basic_user_file /etc/nginx/websocketd.htpasswd;

    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
}
```

nginx answers an unauthenticated request with `401` before the upgrade is
attempted. Your script never runs. It also never learns which user
authenticated, only that somebody did.

### A token, checked by your own service

To validate a session cookie, a JWT, or an API key, hand the decision to
a service of your own with nginx's `auth_request`:

```nginx
location = /_auth_check {
    internal;
    proxy_pass              http://127.0.0.1:9000/verify;
    proxy_pass_request_body off;
    proxy_set_header        Content-Length "";
    proxy_set_header        Cookie $http_cookie;
}

location / {
    auth_request /_auth_check;
    auth_request_set $auth_user $upstream_http_x_auth_user;
    proxy_set_header X-Auth-User $auth_user;

    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
}
```

Your service at `127.0.0.1:9000/verify` answers `200` to allow and `401`
or `403` to deny. When it allows, it can return an `X-Auth-User` response
header naming the caller, which nginx forwards and websocketd turns into
an environment variable:

```
HTTP_X_AUTH_USER=[ada]
```

Every request header becomes `HTTP_` plus the header name uppercased with
dashes turned into underscores. That is how your script gets a per-user
identity without websocketd knowing anything about authentication. See
[the CGI environment](/understanding/cgi-environment/) for the full
mapping.

That identity is only as trustworthy as the path in front of it. It is a
header, so anything that can reach port 8080 directly can set it to
whatever it likes. Binding to `127.0.0.1` is what makes it mean anything.

The base proxy configuration, without the auth layer, is in [serving
behind nginx](/how-to/deploy/nginx/).

## In the script: check a token

The token has to reach your script through the request, which means the
[CGI environment](/understanding/cgi-environment/). Two variables carry
it, and which one you use is decided by the client:

- `QUERY_STRING` holds the raw query string, exactly as sent, with no
  flag needed. Use it for browser clients: the browser WebSocket API
  cannot set request headers, so the URL is the only channel it has.
- `HTTP_AUTHORIZATION`, or any other `HTTP_` variable, holds the
  corresponding request header. Use it for scripts and services, which
  can set headers freely.

Both are written by the client. Treat them as hostile input, compare them
in constant time, and never pass either to a shell.

Keep the expected secret out of the URL and out of the command line.
Put it in websocketd's own environment and name it in `--passenv`:

```sh
export APP_TOKEN=s3cret
websocketd --port=8080 --passenv=PATH,APP_TOKEN ./authed.py
```

`--passenv` replaces the default list rather than adding to it, which is
why `PATH` is named too. [Pass data into your
script](/how-to/patterns/pass-arguments/) has the details.

```python
#!/usr/bin/env python3
# authed.py
import hmac, os, sys
from urllib.parse import parse_qs

expected = os.environ.get("APP_TOKEN", "")
token = parse_qs(os.environ.get("QUERY_STRING", "")).get("token", [""])[0]

if not expected or not hmac.compare_digest(token, expected):
    print("unauthorized")
    sys.exit(1)

print("authorized")
for line in sys.stdin:
    print("echo: " + line.rstrip("\n"))
    sys.stdout.flush()
```

Connecting to `ws://localhost:8080/?token=s3cret` and sending `hi`:

```
authorized
echo: hi
```

Connecting with the wrong token, or none:

```
unauthorized
```

The script exits, websocketd closes the connection, and nothing else
runs.

### A token in a URL leaks

websocketd logs the full request URL, query string included, at access
level. A connection carrying `?token=s3cret` writes that secret into the
log:

```
ACCESS | session | url:'http://127.0.0.1:8080/?token=s3cret' ... | CONNECT
```

The same string also lands in browser history, in `Referer` headers, and
in the logs of every proxy in the path. Prefer a header where the client
can set one. Where it has to be the query string, use a short-lived token
your application issues per session, not a long-lived secret.

## What this does not cover

An origin policy is not authentication. `--sameorigin` and `--origin`
constrain which web pages a browser will let connect; they place no
constraint at all on a script with a WebSocket library, which sets
whatever origin it likes.

Mutual TLS is the one form of client authentication websocketd performs
itself. `--sslca` requires every client to present a certificate signed
by a named authority, verified before your program is launched. It suits
machine-to-machine deployments and not browsers. See [serving over
wss://](/how-to/deploy/tls/).

## Next

- [The security model](/understanding/security-model/) covers the origin
  policy, TLS, and why authentication is not built in.
- [The exposure checklist](/how-to/deploy/public-internet/) is what to
  work through before this matters.
- [Environment variables](/reference/environment-variables/) is the
  complete table your script can read from.
