---
title: "Serve behind nginx"
weight: 10
description: "Proxy WebSocket upgrades to websocketd through nginx, with the two headers nginx drops and the timeout that kills idle connections."
---

Put nginx on the public port and websocketd on a local one, and forward the
upgrade with an explicit `Upgrade` and `Connection` header pair. nginx does
not pass those two through on its own, so a `location` block that works for
ordinary HTTP fails the WebSocket handshake.

Run websocketd bound to loopback:

```sh
websocketd --port=8080 --address=127.0.0.1 --origin=https://example.com /opt/myapp/myscript.sh
```

Then configure nginx:

```nginx
http {
    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      close;
    }

    server {
        listen 80;
        server_name example.com;

        location / {
            proxy_pass http://127.0.0.1:8080;
            proxy_http_version 1.1;

            proxy_set_header Upgrade    $http_upgrade;
            proxy_set_header Connection $connection_upgrade;
            proxy_set_header Host       $host;
            proxy_set_header X-Real-IP  $remote_addr;

            proxy_read_timeout 3600s;
            proxy_send_timeout 3600s;
        }
    }
}
```

Reload nginx, and `ws://example.com/` reaches websocketd.

## Why each of those lines is there

`proxy_http_version 1.1` sets the protocol nginx speaks to the backend.
The default is HTTP/1.0, which has no `Upgrade` mechanism at all. Leave this
out and the handshake cannot succeed no matter what headers you set.

`proxy_set_header Upgrade` and `proxy_set_header Connection` restore the two
hop-by-hop headers nginx strips before forwarding a request. websocketd
answers with `101 Switching Protocols` only when it sees both.

The `map` block computes the right `Connection` value per request. Send
`Connection: upgrade` only when the client asked to upgrade;
everything else on the same `location` (a `--cgidir` script, a `--staticdir`
file) needs `Connection: close`. One `location` then serves both kinds of
traffic.

## Timeouts, and how they interact with `--pingms`

`proxy_read_timeout` and `proxy_send_timeout` both default to 60 seconds.
Neither one is a connection lifetime. Each is an idle timer: `proxy_read_timeout`
counts from the last byte nginx read from websocketd, and `proxy_send_timeout`
from the last byte nginx wrote to it. A WebSocket connection where nobody
types for a minute trips them, and nginx closes it while both ends still
believe it is healthy.

There are two ways to stop that, and they combine.

Raise the timeouts, as above, above the longest silence you expect.

Or make the connection never fall silent. Start websocketd with `--pingms`
set below the proxy timeout:

```sh
websocketd --port=8080 --address=127.0.0.1 --pingms=30000 /opt/myapp/myscript.sh
```

websocketd then sends a WebSocket ping frame every 30 seconds. The ping is
traffic from the backend, so it resets nginx's read timer; the client's pong
is traffic toward the backend, so it resets the send timer. `--pingms` also
gives websocketd its own liveness check: it sets a read deadline of twice the
ping interval and drops a connection that misses its pongs for that long.

Pick a ping interval below the proxy timeout with room to spare, so a single
lost frame does not trip it. Half the timeout, as above, is a reasonable
starting point.

## Terminating TLS at nginx

Add the usual `listen 443 ssl` server block with `ssl_certificate` and
`ssl_certificate_key`. The `location` block does not change, and websocketd
keeps speaking plain HTTP on loopback. See [serving over wss://](/how-to/deploy/tls/)
for the origin-policy trap that this arrangement creates.

## Next

- [Coexist with another web server](/how-to/deploy/share-a-port/) if nginx is
  already serving an application on this domain.
- [The exposure checklist](/how-to/deploy/public-internet/) before this goes
  on a public address.
- [Run it under systemd](/how-to/deploy/systemd/) so websocketd starts at boot.
- [The security model](/understanding/security-model/) for what the origin
  policy protects.
