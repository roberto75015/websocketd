---
title: "Examples"
weight: 60
description: "Every example program shipped in the repository's examples/ directory, what it does, and the command that runs it."
---

The repository's
[`examples/`](https://github.com/joewalnes/websocketd/tree/main/examples)
directory holds small programs in twenty subdirectories, one per language
or platform. Most of them carry the same three programs, so the same idea
can be compared across languages.

| Program | Behaviour |
|---|---|
| `greeter` | Reads a line from stdin and writes one line back. Most implementations answer `Hello <line>!`. |
| `count` | Writes the numbers 1 to 10 to stdout, half a second apart, then exits. |
| `dump-env` | Writes the CGI environment variables websocketd set for the connection, then exits. |

Every command below is run from inside the example's own directory, and
assumes `websocketd` is on your `PATH`. `--devconsole` serves the
[dev console](/reference/dev-console/) at `http://localhost:8080/`, so
you can drive the program without writing a client.

## Unix scripting languages

| Directory | Programs | Command |
|---|---|---|
| `bash` | `greeter.sh`, `count.sh`, `dump-env.sh`, `chat.sh`, `send-receive.sh` | `websocketd --port=8080 --devconsole ./greeter.sh` |
| `python` | `greeter.py`, `count.py`, `dump-env.py` | `websocketd --port=8080 --devconsole python3 -u ./greeter.py` |
| `ruby` | `greeter.rb`, `count.rb`, `dump-env.rb` | `websocketd --port=8080 --devconsole ruby ./greeter.rb` |
| `perl` | `greeter.pl`, `count.pl`, `dump-env.pl` | `websocketd --port=8080 --devconsole perl ./greeter.pl` |
| `php` | `greeter.php`, `count.php`, `dump-env.php` | `websocketd --port=8080 --devconsole php ./greeter.php` |
| `lua` | `greeter.lua`, `json_ws.lua`, `json.lua` | `websocketd --port=8080 --devconsole lua ./greeter.lua` |

Two of the bash programs have no counterpart elsewhere:

- `chat.sh` is a multi-user chat server. Each connection appends to a
  shared `chat.log` in the working directory, and every other connection
  tails it. It requires GNU `tail`, which is not the `tail` macOS ships.
- `send-receive.sh` reads input on a short timeout while emitting a line
  every few seconds, so both directions are active at once.

The Python programs carry a `#!/usr/bin/python` shebang, a path that many
systems no longer have. Running them as `python3 -u ./greeter.py` uses
the interpreter you have and turns off output buffering; see [Python
scripts](/how-to/languages/python/).

`lua/greeter.lua` echoes each line back unchanged. `lua/json_ws.lua`
answers with a JSON object wrapping the line, and needs `json.lua` in the
same directory.

## Compiled and runtime languages

| Directory | Programs | Command |
|---|---|---|
| `nodejs` | `greeter.js`, `count.js` | `websocketd --port=8080 --devconsole node greeter.js` |
| `rust` | `greeter.rs`, `count.rs`, `dump-env.rs` | `rustc greeter.rs` then `websocketd --port=8080 --devconsole ./greeter` |
| `haskell` | `greeter.hs`, `count.hs` | `websocketd --port=8080 --devconsole ./greeter.hs` |
| `swift` | `greeter.swift`, `count.swift` | `websocketd --port=8080 --devconsole ./greeter.swift` |
| `java` | `Echo/Echo.java`, `Count/Count.java`, each with a launcher script | `cd Echo` then `websocketd --port=8080 --devconsole ./echo.sh` |
| `hack` | `greeter.hh`, `count.hh`, `dump-env.hh` | `websocketd --port=8080 --devconsole hhvm greeter.hh` |
| `qjs` | `request-reply.js` | `websocketd --port=8080 --devconsole qjs --module request-reply.js` |

`nodejs/greeter.js` answers each line with `data: <line>` rather than a
greeting. `qjs/request-reply.js` answers with `RCVD: <line>`.

The Java launcher scripts compile before running: `echo.sh` runs `javac
Echo.java` and then `java Echo`, so a JDK must be on the `PATH`. The
`.java` files and their launchers live in the `Echo/` and `Count/`
subdirectories, not in `examples/java/` itself.

The Haskell scripts have a `#!/usr/bin/env runhaskell` shebang and the
executable bit, so websocketd can launch them directly. The Swift scripts
do the same through `xcrun`, which exists on macOS only.

## Windows

Each of these programs has a `.cmd` launcher next to it, and the `.cmd`
file is what websocketd runs, because [Windows does not read shebang
lines](/reference/platform-support/).

| Directory | Programs | Command |
|---|---|---|
| `powershell` | `greeter.ps1`, `count.ps1`, `dump-env.ps1`, each with a `.cmd` launcher | `websocketd --port=8080 --devconsole greeter.cmd` |
| `windows-jscript` | `greeter.js`, `count.js`, `dump-env.js`, run by Windows Script Host, each with a `.cmd` launcher | `websocketd --port=8080 --devconsole greeter.cmd` |
| `windows-vbscript` | `greeter.vbs`, `count.vbs`, `dump-env.vbs`, run by Windows Script Host, each with a `.cmd` launcher | `websocketd --port=8080 --devconsole greeter.cmd` |
| `c#` | `Echo` and `Count` Visual Studio projects, `Examples.sln`, `run_echo.cmd`, `run_count.cmd` | Build `Examples.sln`, then run `run_echo.cmd` |
| `f#` | `Echo` and `Count` Visual Studio projects, `Examples.sln`, `run_echo.cmd`, `run_count.cmd` | Build `Examples.sln`, then run `run_echo.cmd` |

The PowerShell `.ps1` files also carry a `#!/usr/bin/env pwsh` shebang,
so on Linux and macOS they can be run as `websocketd --port=8080
--devconsole pwsh ./greeter.ps1`.

The `run_echo.cmd` and `run_count.cmd` launchers already contain the full
websocketd command line, including `--port=8080` and `--devconsole`.

## Not WebSocket programs

| Directory | Contents | Command |
|---|---|---|
| `cgi-bin` | `dump-env.sh`, served over plain HTTP as CGI | `websocketd --port=8080 --cgidir=.`, then `curl http://localhost:8080/dump-env.sh` |
| `html` | `count.html`, a browser client for the `count` programs | Start a `count` program on port 8080, then open the file in a browser |

`cgi-bin/dump-env.sh` prints the variables a CGI request receives, which
differ from the WebSocket set; see [environment
variables](/reference/environment-variables/).

## See also

- [CLI flags](/reference/cli-flags/) covers `--devconsole`, `--dir`, and
  `--cgidir`.
- [The tutorial](/start/tutorial/) walks through wrapping your own script
  for the first time.
- [Python scripts](/how-to/languages/python/) fixes output that does not
  appear.
