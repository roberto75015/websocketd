---
title: "Exit codes"
weight: 30
description: "Every exit code websocketd returns, what triggers it, and which stream carries the message."
---

A running websocketd server has no finished state: it exits only when
something outside it stops it, or when it never got started. Every code
below is returned before or during startup.

| Code | Meaning |
|---|---|
| `0` | `--version`, `--license`, or `--help` was given. The output is printed and the server never starts. |
| `1` | A flag value or a combination of flags was rejected. |
| `2` | The command line could not be parsed at all, for example an undefined flag. |
| `3` | Configuration was accepted, but a listener could not be started. |
| `4` | `--devconsole` was combined with `--staticdir` or with `--cgidir`. |

## Which stream carries the message

| Trigger | Stream |
|---|---|
| `--version` and `--license` output | stdout |
| `--help` output | stderr |
| `Command line arguments are missing.` (no arguments at all) | stdout |
| `Incorrect loglevel flag ...` | stdout |
| Every other exit code 1 message | stderr |
| The usage summary printed after most exit code 1 messages | stderr |
| The exit code 2 parse error and usage summary | stderr |
| The exit code 3 and exit code 4 messages | stdout, as log lines |

The exit code 3 and 4 messages go to stdout because they are written
through websocketd's log, which writes every line to stdout. Being log
lines makes them the only messages on this page that `--loglevel=none`
silences. The exit code is still 3 or 4, but nothing is printed on either
stream. Every other message here is written directly rather than logged, so
`--loglevel` does not affect it. The origin-policy security warning printed
at startup is the exception: it goes to stderr, whatever `--loglevel` is
set to.

## What triggers exit code 0

- `--version` prints the process name and version.
- `--license` prints the process name, version, and the BSD licence text.
- `--help` prints the extended help.

## What triggers exit code 1

Each of these is caught before any listener is opened.

| Trigger | Message |
|---|---|
| No command line arguments at all | `Command line arguments are missing.` |
| No `COMMAND` and none of `--dir`, `--staticdir`, `--cgidir` | `Please specify COMMAND or provide --dir, --staticdir or --cgidir argument.` |
| `--loglevel` is not one of `debug`, `trace`, `access`, `info`, `error`, `fatal`, `none` | `Incorrect loglevel flag ...` |
| `--ssl` without both `--sslcert` and `--sslkey` | `please specify both --sslcert and --sslkey when requesting --ssl` |
| `--sslcert` or `--sslkey` without `--ssl` | `you should not be using --ssl* flags when there is no --ssl option` |
| `--sslca` without `--ssl` | `--sslca requires --ssl (mutual TLS has no effect without a TLS listener); add --ssl with --sslcert and --sslkey, or drop --sslca` |
| `--binary` together with `--passstderr` | `please only specify one of --binary and --passstderr` |
| A negative `--maxframesize` | `--maxframesize must not be negative; use 0 for unlimited` |
| `--socketmode=0` | `--socketmode 0 would make the socket unusable; pick a mode like 0700` |
| `--socketmode` above `0777` | `--socketmode "1000" has bits beyond permission bits (keep it within 0777)` |
| `--socketmode` that is not octal | `--socketmode "abc" is not an octal permission mode (e.g. 0700)` |
| `--anyorigin` together with `--sameorigin` or `--origin` | `--anyorigin means 'accept any origin' and cannot be combined with --sameorigin or --origin, which restrict it` |
| `COMMAND` not found on `PATH` | `unable to locate specified COMMAND '...' in OS path` |
| Both `COMMAND` and `--dir` given | `ambiguous: provided COMMAND and --dir argument, please only specify one` |
| `--dir` does not exist | `could not find your script dir '...'` |
| `--dir` exists but is not a directory | `did you mean to specify COMMAND instead of --dir '...'?` |
| `--dir` cannot be resolved to an absolute path | `could not resolve absolute path to dir '...'` |
| `--cgidir` is not an accessible directory | `your CGI dir '...' is not pointing to an accessible directory` |
| `--staticdir` is not an accessible directory | `your static dir '...' is not pointing to an accessible directory` |

## What triggers exit code 2

Go's `flag` package rejected the command line before websocketd looked at
it: an undefined flag, or a value the flag's type cannot parse. The
message is Go's, for example `flag provided but not defined: -nosuchflag`.

## What triggers exit code 3

A listener failed to start after configuration was accepted. websocketd
exits on the first listener that fails, so one failing address stops the
whole server. Causes include:

- The TCP address is already in use.
- The `--redirport` listener could not bind.
- The `--unixsocket` path already has a socket file with a live server
  behind it. A socket file with nothing listening is treated as stale,
  removed, and rebound.
- The `--unixsocket` file could not be chmodded to `--socketmode`.
- The `--sslca` file could not be read, or held no parseable certificate.

## What triggers exit code 4

`--devconsole` with `--staticdir`, or `--devconsole` with `--cgidir`.
All three serve the same non-WebSocket HTTP surface, so only one of them
may be given. See [the dev console](/reference/dev-console/).

## See also

- [CLI flags](/reference/cli-flags/) describes what each flag above does.
- [Platform support](/reference/platform-support/) confirms these codes
  are the same on every platform.
