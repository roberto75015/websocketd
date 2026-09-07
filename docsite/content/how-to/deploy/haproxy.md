---
title: "Serve behind HAProxy"
weight: 30
description: "Carry WebSocket upgrades to websocketd through an HTTP-mode HAProxy backend, with the timeouts long-lived connections need."
---

HAProxy needs no WebSocket-specific directive. In HTTP mode the upgrade
handshake is an ordinary request and response, and HAProxy switches the
connection to a tunnel once it sees the `101`. What you must set explicitly
are the timeouts, because HAProxy's defaults assume short HTTP requests.

Run websocketd bound to loopback:

```sh
websocketd --port=8080 --address=127.0.0.1 --origin=https://example.com /opt/myapp/myscript.sh
```

Then configure HAProxy:

```haproxy
defaults
    mode    http
    timeout connect  5s
    timeout client   30s
    timeout server   30s
    timeout tunnel   3600s

frontend public
    bind *:80

    acl is_upgrade hdr(Upgrade) -i websocket
    use_backend websocketd if is_upgrade
    default_backend app

backend websocketd
    server ws1 127.0.0.1:8080 check

backend app
    server app1 127.0.0.1:3000 check
```

Reload HAProxy, and `ws://example.com/` reaches websocketd.

## The timeouts

`timeout tunnel` is the one that matters, and the easiest to miss. Once the
upgrade succeeds the connection stops being a request and response and
becomes a raw bidirectional tunnel. HAProxy then stops applying `timeout
client` and `timeout server` to it and applies `timeout tunnel` instead. If
you never set `timeout tunnel`, HAProxy falls back to the client and server
timeouts, and a healthy but idle WebSocket connection is cut at whichever is
shorter.

Set `timeout tunnel` above the longest silence you expect. Or keep the
connection from ever falling silent by starting websocketd with `--pingms`
below the tunnel timeout:

```sh
websocketd --port=8080 --address=127.0.0.1 --pingms=30000 /opt/myapp/myscript.sh
```

websocketd then sends a WebSocket ping at that interval, which is traffic
through the tunnel. It also sets its own read deadline at twice the interval
and drops a client whose pongs stop arriving.

`timeout connect` covers reaching websocketd in the first place and can stay
short. Loopback either connects immediately or refuses immediately.

## Routing by `Upgrade` versus routing by path

The `acl is_upgrade` above sends anything carrying `Upgrade: websocket` to
websocketd and everything else to another application. That works when the
two share a hostname but not a path.

If you would rather split on the URL, replace the ACL with a path match:

```haproxy
    acl is_ws path_beg /ws/
    use_backend websocketd if is_ws
```

websocketd receives the path as sent, `/ws/...` included. That matters when
you run it with `--dir`, where the path selects which script runs. See
[coexisting with another web server](/how-to/deploy/share-a-port/).

## Terminating TLS at HAProxy

Add `bind *:443 ssl crt /etc/haproxy/certs/example.com.pem` to the frontend.
The ACLs, the backends and the timeouts are unchanged, and websocketd keeps
speaking plain HTTP on loopback. See [serving over wss://](/how-to/deploy/tls/)
for the origin-policy trap that arrangement creates.

## Next

- [The exposure checklist](/how-to/deploy/public-internet/) before this goes
  on a public address.
- [Run it under systemd](/how-to/deploy/systemd/) so websocketd starts at boot.
- [The security model](/understanding/security-model/) for what the origin
  policy protects.
