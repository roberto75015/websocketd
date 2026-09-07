---
title: "Run a program once, or keep it running"
weight: 30
description: "Enforce a single instance with a lock-file wrapper, or keep a long-running program alive across a connection by handling SIGINT and SIGTERM."
---

websocketd starts one fresh process per connection and starts nothing at
all until the first client arrives, so neither "run this once" nor "keep
this running" happens on its own. Both are things you arrange around it.

Two opposite problems live on this page. Go to the one you have:

- **Only one instance may exist at a time**, because it holds a device, a
  lock, or a resource that does not tolerate a second copy. Use a
  [wrapper that refuses to start twice](#enforce-a-single-instance).
- **The program must survive for the whole connection**, or longer,
  without being cut off mid-write. Give it [signal
  handling](#keep-a-program-running).

## Enforce a single instance

Wrap the real program in a script that takes a lock before handing over.
Every connection still gets its own wrapper process. Only one wrapper
ever reaches the program.

```sh
#!/bin/sh
# run-once.sh
LOCKDIR=/tmp/control-motor.lock

if ! mkdir "$LOCKDIR" 2>/dev/null; then
  echo "busy: another connection holds the motor"
  exit 0
fi
trap 'rmdir "$LOCKDIR"' EXIT INT TERM

exec ./control-motor.sh
```

```sh
websocketd --port=8080 ./run-once.sh
```

Creating a directory is atomic on every POSIX filesystem, which is what
makes this a lock rather than a race. A second connection arriving while
the first holds it gets a message and nothing else:

```
busy: another connection holds the motor
```

When the first connection closes, the wrapper's `trap` removes the lock
directory, and the next connection acquires it normally.

On Linux you can use `flock` instead, which releases the lock when the
file descriptor closes and so survives a wrapper that dies without
running its trap:

```sh
#!/bin/bash
exec 9>/tmp/control-motor.lock
flock -n 9 || { echo "busy"; exit 0; }
exec ./control-motor.sh
```

`flock` ships with util-linux and is not present on macOS or the BSDs,
which is why the portable version above uses `mkdir`.

### Let the newest connection win instead

If the right answer is "the latest connection takes over" rather than
"the second connection is refused", kill the incumbent instead of
refusing the newcomer:

```sh
#!/bin/sh
# takeover.sh
PIDFILE=/tmp/control-motor.pid

if [ -f "$PIDFILE" ]; then
  kill -TERM "$(cat "$PIDFILE")" 2>/dev/null
  sleep 1
fi

echo $$ > "$PIDFILE"
exec ./control-motor.sh
```

Pick whichever matches what should happen when two people reach for the
same resource. websocketd has no opinion, because a chat room and a motor
controller need opposite answers.

## Keep a program running

A program that loops rather than exiting when its input runs out has to
handle the signals websocketd sends when the connection closes. This is
the whole contract: **handle SIGINT and SIGTERM, and exit promptly on
either.**

```python
#!/usr/bin/env python3
import signal, sys, time

def shutdown(signum, frame):
    flush_everything()
    sys.exit(0)

signal.signal(signal.SIGINT, shutdown)
signal.signal(signal.SIGTERM, shutdown)

while True:
    do_work()
    sys.stdout.flush()
    time.sleep(1)
```

SIGINT arrives first, so a program that handles only SIGTERM gets one
fewer chance to clean up than it thinks. Handle both.

A program that ignores both, and does not read stdin either,
does not survive: websocketd escalates to SIGKILL, and the process is
gone under a second after the connection closes, having run no cleanup at
all. It rides websocketd's teardown ladder to the bottom on every
disconnect. [Process lifecycle](/understanding/process-lifecycle/) has
the ladder and the `--closems` flag that lengthens it, which is what you
want if your cleanup needs a network round trip.

Reading stdin is the other way to notice. websocketd closes your
program's stdin first, so a loop of the form `while read -r line` ends by
itself when the connection does, with no signal handling at all.

### Keep it running past the connection

Anything still in the process group when the connection ends is killed.
If a program has to outlive the connection that started it,
do not start it from websocketd at all. Run it as a service in its own
right, and let each connection bridge to it:

```sh
#!/bin/sh
exec socat - UNIX-CONNECT:/var/run/hub.sock
```

That is the third pattern in [share state across
connections](/how-to/patterns/share-state/), and it is the answer to
"one process for the whole server" as well as to "shared state". Use
[systemd](/how-to/deploy/systemd/), or whatever supervises services on
your host, to start and restart it.

## Next

- [Process lifecycle](/understanding/process-lifecycle/) covers exactly
  when a process starts, the teardown ladder, and process groups.
- [The process model](/understanding/process-model/) covers why there is
  one process per connection and no pool.
- [Share state across connections](/how-to/patterns/share-state/) covers
  the long-lived backend this page points at.
