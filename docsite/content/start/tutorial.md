---
title: "Tutorial"
weight: 20
description: "Wrap a shell script with websocketd, drive it from the built-in dev console, then serve your own web page that talks to it."
---

By the end of this page a web page in your browser will count to five,
one number a second, driven by a shell script on your machine.

Here is what `websocketd` does between the two. It runs your script and
connects it to the browser: every line the script prints to stdout is
sent to the browser as one WebSocket message, at the moment it is
printed, and anything the browser sends back arrives on the script's
stdin. The script contains no networking code and never knows a browser
is there.

You need `websocketd` on your `PATH`. If `websocketd --version` does not
print a version, work through the [install](/start/install/) first.

## 1. Write a script

Make a directory to work in, and go into it:

```sh
mkdir counter
cd counter
```

Create a file called `count.sh` containing exactly this:

```sh
#!/bin/bash
for COUNT in 1 2 3 4 5; do
  echo $COUNT
  sleep 1
done
```

Make it executable:

```sh
chmod +x count.sh
```

Run it on its own first, with no `websocketd` involved:

```sh
./count.sh
```

```
1
2
3
4
5
```

The numbers appear one a second. Nothing about this script knows what a
WebSocket is, and nothing about it is going to change.

## 2. Wrap it with websocketd

```sh
websocketd --port=8080 --sameorigin ./count.sh
```

The server starts and prints two lines:

```
Sun, 06 Sep 2026 19:41:15 -0700 | INFO   | server     |  | Serving using application   : ./count.sh
Sun, 06 Sep 2026 19:41:15 -0700 | INFO   | server     |  | Starting WebSocket server   : ws://example-host.local:8080/
```

Your second line will name your own machine rather than
`example-host.local`, which stands in for it throughout this page.

`--sameorigin` allows a WebSocket connection only from a page served by
this same host and port. Leave it out and `websocketd` accepts one from
any page in any browser that can reach the port, and prints a long
warning at startup saying so. Keep it on for the rest of this tutorial.
[The security model](/understanding/security-model/) covers the other
policies and when to reach for them.

Now open **`http://localhost:8080/`** in your browser. You get this:

```
404 page not found
```

This server speaks WebSocket and nothing else so far, and you have not
given it a web page to hand out. Step 4 does that. The server agrees with
the browser, in its own log:

```
Sun, 06 Sep 2026 19:41:16 -0700 | ACCESS | http       | url:'http://localhost:8080/' | NOT FOUND
```

### The banner says a different hostname than you typed

The startup line above says `ws://example-host.local:8080/`, but you
opened `http://localhost:8080/`. Both reach the same server.

`websocketd` looks up your machine's own network name to build that
banner. It never prints `example-host.local`: that name is this page's
stand-in for whatever your machine is called, and yours will read
something else entirely.

This tutorial says `localhost` everywhere, because `localhost` always
resolves to your own machine no matter what network you are on. When the
banner and this page disagree about the hostname, they are not in
conflict. Use `localhost`.

### Flags go before the command, always

`websocketd --port=8080 ./count.sh` works. `websocketd ./count.sh
--port=8080` does not, and it does not tell you so. Go's flag parser stops
looking for flags at the first argument that is not one, so everything
after `./count.sh` is handed to `count.sh` as an argument instead:

```
Sun, 06 Sep 2026 19:36:12 -0700 | INFO   | server     |  | Serving using application   : ./count.sh --port=9999
Sun, 06 Sep 2026 19:36:12 -0700 | INFO   | server     |  | Starting WebSocket server   : ws://example-host.local:80/
```

Look at the first line. If a flag you typed shows up after your script
name on the `Serving using application` line, it was never read as a flag,
and the server fell back to its default of port 80. Move the flag before
the command.

## 3. Drive it from the dev console

You have a WebSocket server, but nothing to talk to it with yet.
`websocketd` ships with a page for exactly this. Stop the server with
**Ctrl+C** and start it again with `--devconsole`:

```sh
websocketd --port=8080 --sameorigin --devconsole ./count.sh
```

There is a third startup line now:

```
Sun, 06 Sep 2026 19:38:40 -0700 | INFO   | server     |  | Serving using application   : ./count.sh
Sun, 06 Sep 2026 19:38:40 -0700 | INFO   | server     |  | Starting WebSocket server   : ws://example-host.local:8080/
Sun, 06 Sep 2026 19:38:40 -0700 | INFO   | server     |  | Developer console enabled   : http://example-host.local:8080/
```

Open **`http://localhost:8080/`** again. Instead of the 404 you get the
console: a connect button, a box to send messages from, and a running log
of everything your script prints.

Connect, and watch `1` through `5` arrive one a second. Each number is a
separate WebSocket message: `count.sh` wrote a line to stdout, and
`websocketd` sent that line on as it appeared. The server logs the
session as it opens:

```
Sun, 06 Sep 2026 19:38:40 -0700 | ACCESS | http       | url:'http://localhost:8080/' | DEVCONSOLE
Sun, 06 Sep 2026 19:38:40 -0700 | ACCESS | session    | url:'http://localhost:8080/' id:'4f43ed0dfd946de9' remote:'127.0.0.1' command:'./count.sh' origin:'http://localhost:8080' | CONNECT
```

Every connection you make starts its own fresh copy of `count.sh`. Open a
second browser tab and it counts from `1` again, independently, in its own
process.

### Numbers arriving one at a time is not free

You saw the numbers stream because `count.sh` is a bash script, and bash
writes each `echo` out immediately. Most other languages do not. Python,
Ruby, PHP, C, and others switch to holding output in a buffer when stdout
is a pipe rather than a terminal, and a pipe is exactly what your program
gets here. The symptom is all five numbers landing at once when the script
exits, instead of one a second.

The fix is one flag or one line per language, and it is on that language's
page: [Python](/how-to/languages/python/),
[Ruby](/how-to/languages/ruby/), [PHP](/how-to/languages/php/),
[C](/how-to/languages/c/), [Node.js](/how-to/languages/nodejs/). The
reason it happens is in [output
buffering](/understanding/output-buffering/).

### Do not add `--staticdir` to this command

`--devconsole` serves its own page at `/`, so it cannot share the server
with `--staticdir` or `--cgidir`, which also want to serve `/`. Combining
them is not ignored or quietly resolved. The server refuses to start and
exits with code 4:

```
Sun, 06 Sep 2026 19:38:22 -0700 | FATAL  | server     |  | Invalid parameters: --devconsole cannot be used with --staticdir. Pick one.
```

These are two ways to run the server, not two flags to combine. Use
`--devconsole` while you are poking at the script. Use `--staticdir`, as
you are about to, once you have a front end of your own.

## 4. Serve your own page

Stop the server with **Ctrl+C**.

Your HTML goes in a directory of its own. `websocketd` serves everything
in the directory you point `--staticdir` at, and `count.sh` is not
something you want handed out:

```sh
mkdir public
```

Create `public/count.html`:

```html
<!DOCTYPE html>
<title>count</title>
<pre id="log"></pre>
<script>
  const log = document.getElementById('log');
  const ws = new WebSocket('ws://' + location.host + '/');
  ws.onopen    = () => { log.textContent += 'CONNECT\n'; };
  ws.onmessage = (e) => { log.textContent += e.data + '\n'; };
  ws.onclose   = () => { log.textContent += 'DISCONNECT\n'; };
</script>
```

Start the server pointing at that directory:

```sh
websocketd --port=8080 --sameorigin --staticdir=public ./count.sh
```

Four startup lines this time, including an `http://` one:

```
Sun, 06 Sep 2026 19:39:14 -0700 | INFO   | server     |  | Serving using application   : ./count.sh
Sun, 06 Sep 2026 19:39:14 -0700 | INFO   | server     |  | Serving static content from : public
Sun, 06 Sep 2026 19:39:14 -0700 | INFO   | server     |  | Starting WebSocket server   : ws://example-host.local:8080/
Sun, 06 Sep 2026 19:39:14 -0700 | INFO   | server     |  | Serving CGI or static files : http://example-host.local:8080/
```

Open **`http://localhost:8080/count.html`**. The page shows `CONNECT`,
then `1` through `5` one a second, then `DISCONNECT` when the script
finishes and its process exits. The server logs the same story:

```
Sun, 06 Sep 2026 19:39:14 -0700 | ACCESS | http       | url:'http://localhost:8080/count.html' | STATIC
Sun, 06 Sep 2026 19:39:15 -0700 | ACCESS | session    | url:'http://localhost:8080/' id:'12d9958a57d99ae6' remote:'127.0.0.1' command:'./count.sh' origin:'http://localhost:8080' | CONNECT
Sun, 06 Sep 2026 19:39:21 -0700 | ACCESS | session    | url:'http://localhost:8080/' id:'12d9958a57d99ae6' remote:'127.0.0.1' command:'./count.sh' origin:'http://localhost:8080' pid:'1965' | DISCONNECT
```

That is the whole thing working.

### Open the page through the server, not off the disk

Do not double-click `count.html`, and do not open it as a `file://` URL
straight from the disk. A page loaded that way has no
origin. Browsers send `Origin: null` for it, `--sameorigin` rejects the
upgrade, and the page sits there having never connected:

```
Sun, 06 Sep 2026 19:38:58 -0700 | ACCESS | session    | url:'http://localhost:8080/' id:'4f6bfb8c6295f37b' remote:'127.0.0.1' command:'./count.sh' origin:'file:' | Same origin policy mismatch
Sun, 06 Sep 2026 19:38:58 -0700 | ACCESS | session    | url:'http://localhost:8080/' id:'4f6bfb8c6295f37b' remote:'127.0.0.1' command:'./count.sh' origin:'file:' | Unable to Upgrade: websocket: request origin not allowed by Upgrader.CheckOrigin
```

The browser gets a `403 Forbidden` for the upgrade, which is visible only
in its developer tools. On the page itself nothing happens at all. Always
reach the page at `http://localhost:8080/count.html`, so the page and the
socket share an origin.

### Never type the port into the JavaScript

The snippet above builds the socket URL from `location.host`, which is the
host and port the page itself was loaded from. Change `--port` and it
follows, with nothing to keep in sync.

Had it said `new WebSocket('ws://localhost:8080/')` and you were running on
8081, the page would load perfectly and simply never connect: no server
log line, no error on the page, nothing. That silent mismatch is easy to
miss and slow to diagnose, and `location.host` removes it entirely.

## What you know now

- Any program that reads stdin and writes stdout is a WebSocket backend.
  `count.sh` was never modified.
- Every connection gets its own process. Two tabs are two independent runs
  of your script.
- Flags go before the command name. If a flag appears on the `Serving
  using application` line, it was swallowed as an argument.
- Without an origin policy the server accepts connections from anywhere
  and prints a warning saying so. `--sameorigin` is the right answer while
  you are developing.
- The startup banner names your machine's hostname. `localhost` reaches
  the same server.
- `--devconsole` and `--staticdir` are two ways to run the server, chosen
  per run. Together they exit with code 4.
- Serve your page over `http://` and derive the socket URL from
  `location.host`. A `file://` page is rejected, and a hardcoded port
  fails silently.

## Where to go next

- [The process model](/understanding/process-model/) covers what one
  process per connection means once your script does something real, and
  what it rules out.
- [Output buffering](/understanding/output-buffering/) explains why the
  output stops appearing when you rewrite `count.sh` in another language.
- [Pass data into your script](/how-to/patterns/pass-arguments/) gets
  query-string and per-connection data into your program.
- [Debug a script](/how-to/patterns/debug-a-script/) gets more out of the
  dev console than the connect button.
- [CLI flags](/reference/cli-flags/) lists everything `websocketd` takes,
  with its default.
