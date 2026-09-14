---
title: "Pass data into your script"
weight: 40
description: "The query string carries per-connection data from the client with no flag; --passenv forwards named variables from websocketd's own environment to every connection."
---

Two different mechanisms put data into a wrapped script, and telling them
apart is most of the job:

- **The query string** carries data from the client and is different for
  every connection. Your script reads it from `QUERY_STRING`. No flag
  turns it on.
- **`--passenv`** copies named variables out of websocketd's own process
  environment into every connection's script. Same value every time. It
  has nothing to do with the request.

If the value should vary per connection, it belongs in the URL. If it
should be identical for every connection, it belongs in websocketd's
environment.

## Per-connection data: the query string

Every connection's script gets the query string it was opened with, in
`QUERY_STRING`.

```sh
#!/bin/bash
# show-query.sh
echo "QUERY_STRING is: $QUERY_STRING"
```

```sh
websocketd --port=8080 ./show-query.sh
```

Connect to `ws://localhost:8080/?name=Ada&room=general` and the script
prints:

```
QUERY_STRING is: name=Ada&room=general
```

`QUERY_STRING` is the raw string. Splitting it on `&` and `=`, and
percent-decoding the parts, is your script's job, exactly as it is for
any CGI script:

```python
#!/usr/bin/env python3
import os
from urllib.parse import parse_qs

params = parse_qs(os.environ.get("QUERY_STRING", ""))
room = params.get("room", ["general"])[0]
```

The client writes that string, so anything in it is attacker-controlled.
Validate it, and keep it away from a shell.

The query string is one of a couple of dozen request variables websocketd
builds for each connection, alongside `REMOTE_ADDR`, `REQUEST_URI`, and
an `HTTP_` variable per request header. [Environment
variables](/reference/environment-variables/) is the full table.

## Fixed configuration: `--passenv`

`--passenv` takes a comma-separated list of variable *names*. websocketd
looks each one up in its own environment at startup and copies the value
into every child process.

```sh
export APP_TOKEN=s3cret
websocketd --port=8080 --passenv=PATH,APP_TOKEN ./show-config.sh
```

```sh
#!/bin/bash
# show-config.sh
echo "APP_TOKEN is: ${APP_TOKEN:-<unset>}"
```

```
APP_TOKEN is: s3cret
```

Every connection sees the same value, because it came from the shell that
started websocketd rather than from the client.

A variable that is unset or empty in websocketd's environment is dropped
rather than forwarded as an empty string.

### `--passenv` replaces the default, it does not extend it

The default is `PATH` plus your platform's library search path, and
naming your own variable throws that default away:

```sh
websocketd --port=8080 --passenv=APP_TOKEN ./show-config.sh
```

The child's environment now has no `PATH` entry at all. Name `PATH`
yourself whenever you use the flag:

```sh
--passenv=PATH,APP_TOKEN
```

The missing `PATH` can be slow to notice, because some shells invent one.
Run the command above with a `bash` script and it reports a `PATH` that
websocketd never set:

```
PATH is: /usr/gnu/bin:/usr/local/bin:/bin:/usr/bin:.
```

That is bash's own compiled-in fallback for a missing `PATH`, not the one
you were running with, and it will not find anything you installed. A
program that reads the environment directly sees the truth: no `PATH`
key.

Everything websocketd does not carry across is gone. On every platform
except Windows, websocketd clears its own environment after reading
`--passenv`, so nothing reaches your script by accident. [The CGI
environment](/understanding/cgi-environment/) covers why it works that
way.

## Side by side

| | Query string | `--passenv` |
|---|---|---|
| Variable your script reads | `QUERY_STRING` | The name you listed |
| Where the value comes from | The client's URL | websocketd's own environment |
| Varies per connection | Yes | No |
| Trust | Client-controlled | Operator-controlled |
| Flag needed | None | `--passenv` |
| Typical use | Room name, user id, a per-connection setting | API key, `PATH`, deployment config |

The two do not meet. `--passenv=QUERY_STRING` does nothing useful:
websocketd's own environment has no `QUERY_STRING` to copy, and the real
one is already in your script's environment with no flag.

## The URL path can select the script

With `--dir`, the path picks which script runs and the query string still
arrives as usual.

```sh
websocketd --port=8080 --dir=./scripts
```

Connecting to `ws://localhost:8080/greet.sh?name=Ada` runs
`./scripts/greet.sh` with `QUERY_STRING` set to `name=Ada`, and tells the
script where it sits in the URL:

```
SCRIPT_NAME=[/greet.sh]
PATH_INFO=[]
```

Anything after the script's own path is handed over in `PATH_INFO`, which
is how a script routes on the URL. Connecting to
`ws://localhost:8080/greet.sh/extra/path?x=1`:

```
SCRIPT_NAME=[/greet.sh]
PATH_INFO=[/extra/path]
```

`--cgidir` builds these two variables by different rules. If you route on
them, check the [environment variable
reference](/reference/environment-variables/) for the mode you are using.

## Fixed command-line arguments

Anything after the command on the websocketd command line is passed
straight through to your program, identically for every connection:

```sh
websocketd --port=8080 ./myscript.sh --verbose /var/data
```

This is a third fixed channel, alongside `--passenv`, and like it, it
cannot vary by connection.

## Next

- [The CGI environment](/understanding/cgi-environment/) explains the
  request-to-environment contract and where the trust boundaries are.
- [Environment variables](/reference/environment-variables/) is the
  complete table, including the `--cgidir` differences.
- [Add authentication](/how-to/patterns/add-auth/) uses both mechanisms
  together: a per-connection token in the URL, checked against a secret
  from `--passenv`.
