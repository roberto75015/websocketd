---
title: "Share state across connections"
weight: 10
description: "Put the shared part outside the per-connection processes: a message bus, a shared file, or one long-lived backend that every connection's script bridges to."
---

To share state between connections, put the state outside the connection
processes and make each connection's script a thin, disposable client of
it. Every connection gets its own process with nothing between the
processes, so there is nowhere inside websocketd for shared state to
live. [The process model](/understanding/process-model/) covers why.

Three places to put it, in increasing order of what they cost you.

## Pick one

| Put it in | When | Cost |
|---|---|---|
| A shared file on disk | One host, one writer, a feed every connection reads | Nothing to install; no back pressure, no multi-host |
| A message bus | Topics with many subscribers, or you already run one | Another service to operate |
| A long-lived backend process | The shared state has behaviour attached to it | You write and run the backend |

The first two suit data that flows one way. Once connections need to
affect each other, or the shared state needs rules, you want the third.

## A shared file on disk

One writer appends lines to a file. Every connection tails it.

```sh
#!/bin/sh
# feed.sh
exec tail -n 0 -f /var/log/sensor-feed.log
```

```sh
websocketd --port=8080 ./feed.sh
```

Whatever appends to `/var/log/sensor-feed.log` now reaches every
connected client. With two clients connected and two lines appended,
each client receives both:

```
reading 21.5
reading 21.7
```

`-n 0` starts each connection at the end of the file, so a client that
connects late gets what happens next rather than the whole history. Drop
it if you want the backlog.

**Do not reach for a named pipe here.** A FIFO does not fan out. Every
line goes to exactly one reader, chosen by the kernel, so two connections
reading one FIFO steal lines from each other. Two `cat` processes on one
FIFO, six lines written:

```
reader A got: line 3, line 6
reader B got: line 1, line 2, line 4, line 5
```

A FIFO is the right tool for handing a stream to one consumer, and the
wrong tool for handing it to all of them.

Appending is the other thing to be careful about. Two writers appending
to the same file at the same moment can interleave a long line. Keep the
lines short, or serialise the writes through a single writer process.

## A message bus

Each connection's script subscribes to a topic and publishes to it, using
the bus's own command-line client. Redis, NATS and MQTT all work the same
way here.

```sh
#!/bin/bash
# chat.sh
CHANNEL=chatroom

# Subscribe in the background and forward payloads to the browser.
# redis-cli prints each delivery as three lines: the word "message",
# the channel name, then the payload. Only the payload is wanted.
redis-cli subscribe "$CHANNEL" | while read -r kind; do
  read -r channel
  read -r payload
  [ "$kind" = "message" ] && printf '%s\n' "$payload"
done &
SUBSCRIBER=$!
trap 'kill $SUBSCRIBER' EXIT

# Forward everything the browser sends into the channel.
while read -r message; do
  redis-cli publish "$CHANNEL" "$message" >/dev/null
done
```

```sh
websocketd --port=8080 ./chat.sh
```

The bus owns the one-message-many-subscribers relationship. Your script
never does, and neither does websocketd.

This costs you a service to run and a second process per connection, and
it buys you fan-out that already works across more than one host. If you
run a bus already, this is usually the cheapest of the three.

## A long-lived backend process

Run one process yourself, outside websocketd, holding the shared state in
ordinary memory. Each connection's script does nothing but relay bytes
between the WebSocket and that process.

Start the backend however you normally run a service, listening on a Unix
domain socket:

```sh
./hub --socket=/var/run/hub.sock
```

The bridge script needs no custom code, because `socat` already relays
stdin and stdout to a socket:

```sh
#!/bin/sh
# bridge.sh
exec socat - UNIX-CONNECT:/var/run/hub.sock
```

```sh
websocketd --port=8080 ./bridge.sh
```

Every connection gets its own throwaway `bridge.sh`, and all of them are
clients of the same backend. With a hub that echoes each line to every
connected client, a message sent by one browser arrives at another that
sent nothing:

```
client A received: hello from B
client B received: hello from B
```

This is the most flexible of the three and the only one where the shared
state can have logic attached to it: a game board, a session registry, a
rate limiter, an authority on who is allowed to say what. The cost is
that you write the backend, and that it is now a service you have to
keep running. Give it a supervisor, as in [running under
systemd](/how-to/deploy/systemd/).

If the backend must be started once and only once, [run a program once,
or keep it running](/how-to/patterns/run-once/) covers how to enforce
that.

## Next

- [The process model](/understanding/process-model/) explains why there
  is no shared state to begin with, and what the model buys in exchange.
- [Design decisions](/understanding/design-decisions/) covers why
  broadcast was never built in.
- [Pass data into your script](/how-to/patterns/pass-arguments/) covers
  getting a room or topic name into each connection.
