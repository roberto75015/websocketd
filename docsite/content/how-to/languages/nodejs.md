---
title: "Read input in a Node.js script"
weight: 30
description: "Read standard input line by line with readline. Node's output needs no flush, so the Node trap is input: a script that waits for end-of-file never responds at all."
---

Read stdin with the `readline` module and handle each line as
it arrives. Node is the exception among the languages here: its output
needs no flush call, and what goes wrong is the reading side.

```sh
websocketd --port=8080 node ./echo.js
```

```js
const readline = require('node:readline');

const rl = readline.createInterface({ input: process.stdin });

rl.on('line', (line) => {
  process.stdout.write(`echo: ${line}\n`);
});

rl.on('close', () => {
  process.exit(0);
});
```

Send `hello world` from a client and `echo: hello world` comes back
immediately. Every message you send fires `line` once, with the trailing
newline already stripped.

## Why waiting for end-of-file produces nothing

The pattern that fails is any variation on "read all of stdin,
then start work". It looks like this, and under websocketd it produces
no output at all, ever:

```js
// Do not do this.
function readAll() {
  return new Promise((resolve) => {
    let data = '';
    process.stdin.on('data', (chunk) => { data += chunk; });
    process.stdin.on('end', () => resolve(data));
  });
}

(async function () {
  const input = await readAll();
  process.stdout.write('got: ' + input.trim() + '\n');
})();
```

The `end` event fires when stdin reaches end-of-file, meaning
the writing end of the pipe has closed and no more bytes will ever
arrive. Under `echo 'hello' | node script.js` that happens the instant
`echo` finishes, so the script works. Under websocketd it happens only
when the client disconnects, because until then the connection is still
open and more messages may still come. The promise never resolves, the
script never reaches its first `write`, and the browser sees silence.

Treat stdin as a stream that stays open for the life of the
connection and is consumed a line at a time. That is what `readline`
gives you.

## Do not read raw data events yourself

websocketd writes one line to your script's stdin per message
it receives, but a pipe makes no promise that one `data` event
corresponds to one line. A single event can carry a partial line,
several whole lines at once, or a line split across two events,
depending on timing and how the operating system happened to fill the
buffer. Reassembling those correctly is exactly what `readline` already
does. Attach a raw `data` handler only when you need bytes before a
newline has arrived, which under the default line framing is rare.

## Output needs no flush call

`process.stdout.write` does not accumulate output the way Python, Ruby,
PHP and C do. There is no block buffer to empty, and no Node equivalent
of Python's `-u` to set:

```js
let n = 1;
const timer = setInterval(() => {
  process.stdout.write(`${n}\n`);
  if (n++ === 5) { clearInterval(timer); process.exit(0); }
}, 500);
```

Those five numbers arrive half a second apart with nothing else done to
the script. [Output buffering](/understanding/output-buffering/)
explains what the other runtimes do; Node does not do it.

Remember the trailing `\n` regardless. websocketd sends a WebSocket
message when it reads a newline, so a write without one is held, not
sent. `console.log` appends the newline for you;
`process.stdout.write` does not.

## Your script's stdin closes before any signal

When a client disconnects, the first thing websocketd does is close your
script's stdin. Only after that does it escalate to signals. So
`rl.on('close', ...)` fires before any `SIGINT` or `SIGTERM` handler
would, and it is the earlier and more portable place to put your
cleanup. Keep a signal handler if you have one, but do not rely on it as
the first notification. A short script may exit before any signal is
sent. The full sequence is in [process
lifecycle](/understanding/process-lifecycle/).

## Next

- [Process lifecycle](/understanding/process-lifecycle/) has the
  teardown sequence and its timings.
- [Message framing](/understanding/message-framing/) explains the
  newline rule in both directions.
- [Process model](/understanding/process-model/) explains why your
  script has exactly one client and never needs to tell them apart.
