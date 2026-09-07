---
title: "Check before exposing it publicly"
weight: 90
description: "A checklist of decisions to settle before websocketd is reachable from anywhere but localhost."
---

Work through these before websocketd answers on a public address. A
websocketd endpoint runs a program for whoever connects, and every item
below follows from that.

## 1. Decide what binds the public address

With no `--address`, websocketd listens on every interface. That is right
only when websocketd itself is the public server.

If a reverse proxy fronts it, which is the usual arrangement, bind loopback so
nothing else can reach it:

```sh
websocketd --port=8080 --address=127.0.0.1 /opt/myapp/myscript.sh
```

See [nginx](/how-to/deploy/nginx/), [Apache](/how-to/deploy/apache/) or
[HAProxy](/how-to/deploy/haproxy/).

## 2. Confirm the network in front forwards

Binding a public interface is not the same as being reachable, and the two
failures look identical from outside.

Behind NAT on a home or office network, the router needs an explicit
port-forwarding rule to the machine's private address.

On a cloud provider, the network-level firewall is separate from the host's:
a security group on AWS, a firewall rule on GCP, a network security group on
Azure. An open host firewall alone is not enough. This is the usual cause of
"it works when I curl it on the box, not from outside."

## 3. Set an origin policy

The default accepts upgrades from any origin, and websocketd prints a warning
about it on every start. Browsers do not apply the same-origin policy to
WebSocket connections, so any page in any browser that can reach your server
can drive the commands it serves.

Pick one:

- `--origin=https://example.com` names the origins you accept. Use this
  behind a proxy.
- `--sameorigin` requires the `Origin` to match the `Host` websocketd itself
  received. Use this only when the browser reaches websocketd directly; it
  rejects every upgrade behind a TLS-terminating proxy. See
  [serving over wss://](/how-to/deploy/tls/).
- `--anyorigin` keeps the permissive behaviour deliberately and silences the
  warning.

Why the default is permissive, and what an origin check is worth, is in
[the security model](/understanding/security-model/).

## 4. Serve it over TLS

Terminate TLS at the proxy, or in websocketd with `--ssl`. Either way the
browser should be connecting to `wss://`, not `ws://`. See
[serving over wss://](/how-to/deploy/tls/).

## 5. Decide who is allowed to connect

websocketd has no authentication of any kind. It does not read credentials,
has no user model, and does not gate connections on anything but the origin
policy and, with `--sslca`, a client certificate.

Anyone who can complete a WebSocket upgrade can run the program you are
serving. If that is not acceptable, put authentication in front of it. See
[authenticate in front of websocketd](/how-to/patterns/add-auth/), and
[why there is no built-in authentication](/understanding/design-decisions/).

## 6. Cap concurrent processes

`--maxforks` limits how many processes websocketd will have running at once,
and defaults to 1024. Past the limit, upgrades and CGI requests are refused
rather than queued; static files and redirects are unaffected.

The default is a runaway backstop, not a capacity plan. Set it against what
your machine can run at once, given what your script costs. `0`
means unlimited, which on a public address means one client can fork until
the machine stops.

## 7. Cap inbound message size

`--maxframesize` rejects inbound WebSocket messages larger than its value and
closes the connection. It defaults to 1048576 bytes, one mebibyte, which
bounds how much a single client can make websocketd buffer.

Raise it only if your protocol needs larger messages. `0` disables
the limit.

## 8. Detect clients that vanish

`--pingms` sets a ping interval in milliseconds and is off by default. With it
set, websocketd pings each client at that interval and drops a connection
whose pongs stop arriving for twice as long.

Without it, a client that loses power rather than closing cleanly leaves its
process running until something else notices. On a public address, set it. It
also keeps proxy idle timeouts from cutting healthy connections.

## 9. Audit what `--staticdir` exposes

If you serve files with `--staticdir`, point it at a directory containing
only what is meant to be public.

websocketd refuses any path with a segment beginning with `.`, so a `.git`
directory or an `.env` file under `--staticdir` cannot be fetched by name.
It also refuses to list a directory that has no index file, rather than
generating one. Symlinks that point outside the directory are also blocked.
So are files inside a `--dir` or `--cgidir` tree, so a script that also sits
under `--staticdir` is not handed back as source.

None of that vets what you put in the directory: a secret file with an
ordinary name is served like anything else, so the directory itself still
needs to hold only what you mean to publish.

## 10. Know where the logs go

websocketd writes to stdout and stderr and never to a file. Under
[systemd](/how-to/deploy/systemd/) that is the journal; in
[a container](/how-to/deploy/docker/) it is `docker logs`. Confirm something
is capturing and retaining them before you need them.

## Next

- [Run it under systemd](/how-to/deploy/systemd/) so it survives a reboot.
- [The security model](/understanding/security-model/) for the reasoning
  behind items 3, 5 and 9.
- [CLI flags](/reference/cli-flags/) for the exact form of every flag above.
