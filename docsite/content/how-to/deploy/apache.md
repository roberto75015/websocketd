---
title: "Serve behind Apache"
weight: 20
description: "Proxy WebSocket upgrades to websocketd through Apache httpd using mod_proxy_wstunnel."
---

Apache carries the upgrade for you through `mod_proxy_wstunnel`. You name a
`ws://` backend and the module handles the handshake, so unlike nginx there
are no `Upgrade` or `Connection` headers to restore by hand.

Run websocketd bound to loopback:

```sh
websocketd --port=8080 --address=127.0.0.1 --origin=https://example.com /opt/myapp/myscript.sh
```

Then configure Apache:

```apache
LoadModule proxy_module           modules/mod_proxy.so
LoadModule proxy_http_module      modules/mod_proxy_http.so
LoadModule proxy_wstunnel_module  modules/mod_proxy_wstunnel.so

<VirtualHost *:80>
    ServerName example.com

    ProxyPass        "/ws/" "ws://127.0.0.1:8080/"
    ProxyPassReverse "/ws/" "ws://127.0.0.1:8080/"

    ProxyTimeout 3600
</VirtualHost>
```

Restart Apache, and `ws://example.com/ws/` reaches websocketd.

## The modules

Three must be loaded. `mod_proxy` is the proxy core and does nothing on its
own. `mod_proxy_wstunnel` is what understands a `ws://` or `wss://` backend
URL and hands the connection over as a bidirectional tunnel once the upgrade
succeeds. `mod_proxy_http` is needed for any ordinary HTTP you also proxy to
the same websocketd, such as a `--staticdir` file or a `--cgidir` script.

On Debian and Ubuntu, enable them with `a2enmod proxy proxy_http
proxy_wstunnel` instead of writing `LoadModule` lines by hand.

## Serving WebSocket and plain HTTP on the same path

`ProxyPass` commits a path to one backend scheme. If the same URL prefix must
answer both a WebSocket upgrade and an ordinary HTTP request, route on the
`Upgrade` header with `mod_rewrite`:

```apache
LoadModule rewrite_module modules/mod_rewrite.so

RewriteEngine On

RewriteCond %{HTTP:Upgrade} =websocket [NC]
RewriteRule ^/?(.*) "ws://127.0.0.1:8080/$1" [P,L]

RewriteCond %{HTTP:Upgrade} !=websocket [NC]
RewriteRule ^/?(.*) "http://127.0.0.1:8080/$1" [P,L]
```

Both rules point at the same websocketd on the same port. Only the scheme
differs, and the scheme is what tells Apache whether to tunnel the connection
or proxy it as ordinary HTTP. `[P]` sends the request through the proxy;
`[L]` stops rewrite processing for that request.

## Timeouts, and how they interact with `--pingms`

`ProxyTimeout` is an idle timer on the backend connection, and it applies to a
tunnelled WebSocket the same as to an HTTP response. Unset, it inherits the
server-wide `Timeout`, which is 60 seconds in a stock configuration. An idle
WebSocket connection dies at that mark.

Raise `ProxyTimeout` above the longest silence you expect, as above. Or start
websocketd with `--pingms` set below it so the connection is never idle:

```sh
websocketd --port=8080 --address=127.0.0.1 --pingms=30000 /opt/myapp/myscript.sh
```

websocketd sends a ping frame at that interval, which is traffic on the
tunnel, and drops a connection whose pongs stop arriving for twice the
interval.

## The `Host` header Apache forwards

By default Apache sends the backend a `Host` header naming the backend
itself, `127.0.0.1:8080` in the configuration above, not `example.com`.
`ProxyPreserveHost On` forwards the client's `Host` instead. This matters if
you set an origin policy on websocketd. See
[serving over wss://](/how-to/deploy/tls/) for what `--sameorigin` compares
and why it is the wrong flag behind a proxy.

## Next

- [Coexist with another web server](/how-to/deploy/share-a-port/) if Apache
  already serves an application on this domain.
- [The exposure checklist](/how-to/deploy/public-internet/) before this goes
  on a public address.
- [Run it under systemd](/how-to/deploy/systemd/) so websocketd starts at boot.
- [The security model](/understanding/security-model/) for what the origin
  policy protects.
