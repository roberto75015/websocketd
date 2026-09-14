---
title: "Dev console"
weight: 40
description: "What --devconsole serves: the page, its mutual exclusions, and the response contract it follows."
---

`--devconsole` makes websocketd answer every non-WebSocket HTTP request
with one built-in HTML page: a WebSocket client that connects to an
endpoint you type, sends messages, and shows every frame in both
directions.

```sh
websocketd --port=8080 --devconsole ./myscript.sh
```

![The websocketd dev console in light mode: a frame list on the left, an inspector pane on the right, and a message composer along the bottom](/img/console/console-light.png)

The same page in dark mode:

![The websocketd dev console in dark mode, showing the same session](/img/console/console-dark.png)

A short screen recording of a session is at
[console-demo.mp4](/img/console/console-demo.mp4).

## What the page contains

| Element | Behaviour |
|---|---|
| Address bar | The WebSocket URL to connect to, prefilled from the page's own `location.href`. Pressing Enter connects. |
| Connect button and status | Connects and disconnects; the status reads `DISCONNECTED`, `CONNECTING`, or `CONNECTED`. |
| Frame list | One row per frame, with timestamp, direction, size, and a preview of the message. Selecting a row opens it in the inspector. |
| Inspector | The selected frame's opcode, size, and timing, with `Pretty`, `Raw`, and `Hex` views and a `Copy` button. |
| Composer | A text area. Enter sends, Shift+Enter inserts a newline, and Up recalls what you sent before. |
| Counters | Frames sent, frames received, total traffic, and connection duration. |
| Auto-reconnect | A checkbox. When it is ticked, a closed connection is reopened one second later. |
| Theme button | Cycles auto, light, and dark. The choice is remembered in the browser. |
| Clear button | Empties the frame list. |

Nothing about the page is configurable. It is a single HTML file with its
CSS and JavaScript inline, compiled into the binary with `//go:embed`.

## Mutual exclusions

`--devconsole` cannot be combined with `--staticdir` or with `--cgidir`.
All three answer the same non-WebSocket HTTP surface. websocketd exits
with [code 4](/reference/exit-codes/) if two of them are given, whichever
pair it is.

## Response contract

| Property | Value |
|---|---|
| Request path | Every path. The console answers `/` and any other non-WebSocket request. |
| Body | A constant. No part of the request is interpolated into it; the console derives its WebSocket URL in the browser from `location.href`. |
| `Content-Type` | `text/html; charset=utf-8` |
| `X-Content-Type-Options` | `nosniff` |
| `ETag` | A strong validator: the first 16 bytes of the SHA-256 of the response body, hex-encoded and quoted. A conditional request carrying it is answered `304 Not Modified`. |
| `Last-Modified` | The server's startup time. |
| `Content-Security-Policy` | See below. |

The policy is computed at process start from the page that is actually
served, so the hashes cannot go stale:

```
default-src 'none'; script-src 'sha256-...'; style-src 'sha256-...'; connect-src ws: wss:; frame-ancestors 'none'
```

| Directive | Effect |
|---|---|
| `default-src 'none'` | Nothing loads unless a directive below allows it. |
| `script-src` | One `sha256-` hash per inline `<script>` block, over that block's exact bytes. There is no `'unsafe-inline'`, so a script whose bytes changed does not run. |
| `style-src` | The same, one hash per inline `<style>` block. |
| `connect-src ws: wss:` | WebSocket connections to any host, which is what the console exists to make. |
| `frame-ancestors 'none'` | The page cannot be embedded in another page's frame. |

If the embedded page ever contained no inline `<script>` or no inline
`<style>`, websocketd panics at startup rather than serving a console
whose policy would block it.

## See also

- [Debug a script with the dev console](/how-to/patterns/debug-a-script/)
  drives an endpoint before you write any client code.
- [CLI flags](/reference/cli-flags/) covers `--devconsole`, `--staticdir`,
  and `--cgidir`.
- [Exit codes](/reference/exit-codes/) says what code 4 means.
