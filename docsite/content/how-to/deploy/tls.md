---
title: "Serve over wss://"
weight: 70
description: "Terminate TLS in websocketd with --ssl, require client certificates with --sslca, or terminate at a proxy and keep the origin policy working."
---

Pass `--ssl` with a certificate and a key, and websocketd serves `wss://`
instead of `ws://`:

```sh
websocketd --port=443 --ssl \
    --sslcert=/etc/ssl/certs/example.com.crt \
    --sslkey=/etc/ssl/private/example.com.key \
    /opt/myapp/myscript.sh
```

Clients connect to `wss://example.com/`. All three flags go together: give
all of them or none. `--sslcert` and `--sslkey` without `--ssl` are rejected
at startup.

The certificate file must contain the server certificate followed by any
intermediates, in PEM form. Browsers reject a chain they cannot complete.

websocketd negotiates TLS 1.2 at the lowest. A client that offers nothing
above TLS 1.1 fails the handshake.

## Requiring client certificates

`--sslca` names a CA certificate file, and clients must then present a
certificate that CA signed:

```sh
websocketd --port=443 --ssl \
    --sslcert=/etc/ssl/certs/example.com.crt \
    --sslkey=/etc/ssl/private/example.com.key \
    --sslca=/etc/ssl/certs/client-ca.crt \
    /opt/myapp/myscript.sh
```

Every connection is now verified against that CA, and an unverified client is
refused during the handshake, before any HTTP request exists.

**`--sslca` requires `--ssl`.** Drop `--ssl` from that command and
websocketd refuses to start, exiting with code 1 and a message naming
`--sslca` and `--ssl` on stderr, because mutual TLS has no handshake to
verify a client certificate in without a TLS listener.

The file `--sslca` points at holds the CA certificates in PEM form. If it
contains no parseable certificate, websocketd fails to start rather than
starting without verification.

## Terminating TLS at a proxy instead

Most deployments do this. The proxy holds the certificate on 443, and
websocketd runs plain on loopback behind it:

```sh
websocketd --port=8080 --address=127.0.0.1 /opt/myapp/myscript.sh
```

You get certificate renewal, HTTP/2 to the browser, and one place to manage
TLS for every service on the host. See [nginx](/how-to/deploy/nginx/),
[Apache](/how-to/deploy/apache/) and [HAProxy](/how-to/deploy/haproxy/) for
the proxy side.

### The trap: `--sameorigin` behind a TLS-terminating proxy

If you terminate TLS at a proxy, do not use `--sameorigin`. It rejects every
upgrade with `403 Forbidden`.

`--sameorigin` compares the browser's `Origin` header against the `Host`
header of the request websocketd itself received, filling in a default port
on each side from the scheme that side arrived under. Behind a
TLS-terminating proxy the two sides disagree:

- The browser is on `https://example.com`, so the origin side resolves to
  host `example.com`, port **443**.
- websocketd was reached over plain HTTP. Even with `proxy_set_header Host
  $host;` preserving the name, the `Host` it sees carries no port, so the
  request side resolves to host `example.com`, port **80**.

Ports differ, so the check fails. Preserving the original `Host` name is not
enough on its own, because it is the port that mismatches.

Use `--origin` instead, naming the public origin exactly:

```sh
websocketd --port=8080 --address=127.0.0.1 --origin=https://example.com /opt/myapp/myscript.sh
```

`--origin` is matched against the `Origin` header alone. It does not care
what `Host` the proxy forwards, and it stays strict about scheme, host and
port. `https://example.com` with no port matches only port 443.

If you would rather keep `--sameorigin`, make the proxy forward a `Host` that
carries the public port, for example nginx's `proxy_set_header Host
$host:$server_port;` on a `listen 443 ssl` server. Both sides then resolve to
443 and the check passes. `--origin` is the simpler answer.

`--sameorigin` is a good fit when the browser talks to websocketd directly:
local development, or serving the client page from the same websocketd with
`--staticdir`.

## Redirecting plain HTTP

`--redirport` opens a second, plain-HTTP port that answers every request with
a permanent redirect to the canonical address:

```sh
websocketd --port=443 --ssl --sslcert=... --sslkey=... --redirport=80 /opt/myapp/myscript.sh
```

A browser sent to `http://example.com/` is redirected to
`https://example.com/`. It is only useful when websocketd itself terminates
TLS; a proxy in front already handles this.

## Next

- [The exposure checklist](/how-to/deploy/public-internet/) before this goes
  on a public address.
- [The security model](/understanding/security-model/) for why the
  origin policy defaults to permissive and what it does not protect.
- [Add authentication](/how-to/patterns/add-auth/), which TLS does not
  give you.
- [CLI flags](/reference/cli-flags/) for the exact form of each flag above.
