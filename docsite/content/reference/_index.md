---
title: "Reference"
weight: 30
description: "Exact answers about websocketd: every flag, every environment variable, every exit code, the dev console contract, supported platforms, and the shipped examples."
---

Exact answers about what websocketd does. Every command line flag and its
default, every environment variable a spawned process receives, every exit
code the binary returns, what `--devconsole` serves, which platforms are
built and released, and what ships in the repository's `examples/`
directory.

One rule applies to every flag rather than to any single one.

## Flag placement

Every websocketd flag must come before `COMMAND` on the command line, or
before `--dir`, `--staticdir`, or `--cgidir` when one of those is used
instead of a command.

```sh
websocketd --port=8080 --header=X-Trace:1 ./myscript.sh
```

websocketd parses its command line with Go's standard `flag` package,
which stops parsing at the first argument that is not a flag. That
argument is `COMMAND`. Everything after it is passed through to the
wrapped program as its own argument, unexamined.

So this passes `--header=X-Trace:1` to `myscript.sh` and sets no header at
all:

```sh
websocketd --port=8080 ./myscript.sh --header=X-Trace:1
```

websocketd reports no error in that case, because a wrapped program is
entitled to any arguments you give it. The flag is simply never applied.
This has been reported several times as `--header` not working (issues
#151, #155). It applies to every flag, not only the header flags.

## Pages

- [CLI flags](/reference/cli-flags/) lists every flag, its default, and
  its exact effect.
- [Environment variables](/reference/environment-variables/) lists every
  variable a spawned process receives, and which of them the client
  controls.
- [Exit codes](/reference/exit-codes/) gives every code websocketd returns
  and what triggers it.
- [Dev console](/reference/dev-console/) covers what `--devconsole` serves
  and the contract its response follows.
- [Platform support](/reference/platform-support/) names the released
  platforms, and what differs on Windows.
- [Examples](/reference/examples/) covers what ships in `examples/`, per
  language, and the command to run each one.

For the reasoning behind any of it, see [the process
model](/understanding/process-model/), [message
framing](/understanding/message-framing/), and [the security
model](/understanding/security-model/). To wrap a script for the first
time, start with the [tutorial](/start/tutorial/).
