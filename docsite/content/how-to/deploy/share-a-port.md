---
title: "Coexist with another web server"
weight: 80
description: "Serve websocketd and another application on one public port by putting a reverse proxy in front of both."
---

Two servers cannot bind the same port. Put a reverse proxy on the public port
and route by path: one prefix to websocketd, everything else to your
application. websocketd has no mechanism for sharing a port directly, and the
proxy is the answer every time.

Give each server its own loopback port:

```sh
websocketd --port=8080 --address=127.0.0.1 /opt/myapp/myscript.sh
# your application, separately, on 127.0.0.1:3000
```

Then configure the proxy you already run. Each of these pages gives you a
working WebSocket-capable configuration; add one path rule to it.

- [nginx](/how-to/deploy/nginx/): a second `location /ws/ { ... }` block
  pointing at 8080, with the existing `location / { ... }` pointing at 3000.
- [Apache](/how-to/deploy/apache/): a `ProxyPass "/ws/" "ws://127.0.0.1:8080/"`
  alongside a `ProxyPass "/" "http://127.0.0.1:3000/"`, longest prefix first.
- [HAProxy](/how-to/deploy/haproxy/): an `acl ... path_beg /ws/` selecting the
  websocketd backend, with the application as `default_backend`.

## Watch the path the prefix leaves behind

Decide whether the proxy strips `/ws/` before forwarding, then check that
against how you run websocketd.

If you run a single command, the path does not matter. websocketd serves that
one command on every path.

If you run `--dir`, the path selects the script, so a stripped prefix and a
preserved one reach different scripts. nginx strips the `location` prefix when
`proxy_pass` carries a path (`proxy_pass http://127.0.0.1:8080/;`) and
preserves it when it does not (`proxy_pass http://127.0.0.1:8080;`). Apache's
`ProxyPass` replaces the matched prefix with the backend path. HAProxy
forwards the path untouched unless you rewrite it.

## When the other server is the one that has to move

If the application on the public port is not something you can put behind a
proxy, the reverse works: leave it where it is and give websocketd its own
port. Nothing requires a WebSocket endpoint to share the application's port,
only the same host and, for `--sameorigin`, the same origin. See
[serving over wss://](/how-to/deploy/tls/) for what the origin policy
compares.

## Next

- [The exposure checklist](/how-to/deploy/public-internet/) before the shared
  port is public.
- [Add authentication](/how-to/patterns/add-auth/), which the same proxy
  can do.
