---
title: "websocketd"
description: "websocketd wraps any program that reads stdin and writes stdout and serves it over WebSocket, with no networking code in the program itself."
---

```sh
websocketd --port=8080 --sameorigin ./count.sh
```

`count.sh` is an ordinary script. It reads lines from stdin, writes lines
to stdout, and has no idea a WebSocket exists.

Each browser that connects gets its own copy of the script running. What
the browser sends arrives on the script's stdin. Every line the script
prints goes back to that browser as a WebSocket message. That is the whole
integration: no library to import, no framework, no change to the program.

websocketd starts one process per connection. Connections are fully
isolated from each other and share nothing, so there is no built-in
broadcast. See [the process model](/understanding/process-model/) for
what that buys and what it rules out.

Next: [install the binary](/start/install/), or read the
[CLI flag reference](/reference/cli-flags/) if you already have it.
