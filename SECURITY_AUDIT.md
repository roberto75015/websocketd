# Security Audit: websocketd

**Date**: 2026-08-17 | **Scope**: full source tree (`*.go`), dependency tree, existing security tests | **Method**: manual code review + `govulncheck` + empirical verification of the two most suspect findings.

`govulncheck ./...` reports **no known vulnerabilities** in the dependency tree (`gorilla/websocket v1.5.3`, pinned). `go vet` is clean. `go test ./...` passes.

## Summary

Posture is genuinely strong in the high-risk areas (path traversal, CGI RCE, symlink escape, env isolation, origin policy, command injection). The gaps that remain are **DoS hardening** and one **reflected XSS in the dev console**.

| Sev | Finding | Location |
|-----|---------|----------|
| MED | Unbounded WebSocket message size (memory DoS) | `libwebsocketd/websocket_endpoint.go` `readFrames` |
| MED | No HTTP server timeouts (slowloris) | `main.go` `serve` / redirect server |
| MED | Reflected XSS in dev console | `libwebsocketd/console.go` / `libwebsocketd/http.go` `serveDevConsole` |
| LOW | No explicit TLS minimum version | `main.go` `serveMutualTLS` |
| LOW | Default unlimited concurrent forks (no DoS cap) | `config.go` `--maxforks` default 0 |
| LOW | `--origin` without scheme skips scheme matching | `libwebsocketd/http.go` `matchOrigin` |

---

## Findings

### 1. MED — Unbounded WebSocket inbound message size

`libwebsocketd/websocket_endpoint.go`, `readFrames`:

```go
p, err := io.ReadAll(rd)   // no size cap
```

`SetReadLimit` is never called on the gorilla `*Conn`, and gorilla's default `readLimit` is `0` (meaning *unlimited*, gated by `if c.readLimit > 0`). A single client can stream an arbitrarily large text/binary frame; `io.ReadAll` buffers the whole thing in memory before forwarding to the process stdin. Combined with finding #5 (unlimited forks by default), this is a trivial memory-exhaustion DoS from one host.

**Fix:** call `we.ws.SetReadLimit(...)` in `NewWebSocketEndpoint` (configurable, e.g. a new `--maxframesize` flag, sane default like 1 MiB).

### 2. MED — No HTTP server timeouts (slowloris)

`main.go` `serve()` and the redirect server:

```go
return (&http.Server{}).ServeTLS(...)           // no Read/Write/Idle Timeout
return http.Serve(listener, nil)               // none
redir := &http.Server{Addr: rediraddr, Handler: ...}  // none
```

None of the three `http.Server` instances set `ReadTimeout`, `ReadHeaderTimeout`, `WriteTimeout`, or `IdleTimeout`, and `MaxHeaderBytes` is left at default. A client can open connections and dribble bytes to hold sockets and goroutines indefinitely. The WebSocket upgrader *does* set `HandshakeTimeout: 1500ms` (good), but the pre-upgrade HTTP read and the redirect server are exposed. CGI/static/devconsole paths inherit the no-timeout server.

**Fix:** set `ReadHeaderTimeout` (cheap, handshake-only) at minimum on all servers; consider `IdleTimeout` and a configurable `ReadTimeout`/`WriteTimeout`. Note: a blanket `WriteTimeout` would break long-lived WebSocket/CGI streams — apply only to the redirect server, and use `ReadHeaderTimeout` for the main server.

### 3. MED — Reflected XSS in the dev console

`libwebsocketd/http.go` `serveDevConsole` → `libwebsocketd/console.go`:

```go
content := strings.Replace(ConsoleContent, "{{addr}}",
    h.TellURL("ws", req.Host, req.RequestURI), -1)
```

`req.RequestURI` and `req.Host` are inserted into the page inside `value="{{addr}}"` with **no HTML escaping**. I verified empirically that Go's `net/http` *accepts* a raw `"` in the request target and surfaces it verbatim in `req.RequestURI`:

```
GET /"><script>alert(1)</script> HTTP/1.1   →  RequestURI="/\"><script>alert(1)</script>"
```

So a request to a `--devconsole` instance produces:
```html
<input class="url" ... value="ws://host/"><script>alert(1)</script>">
```
The `"` closes the attribute and the script runs on page load (no click needed). Severity is bounded because the dev console is opt-in (`--devconsole`), typically bound to localhost, and explicitly a development tool — but if it's ever exposed (e.g. behind a reverse proxy for testing), it's a reflected XSS in the developer's browser.

**Fix:** HTML-escape the substituted value (`html.EscapeString`) and/or build the URL from the cleaned `req.URL.Path` rather than `req.RequestURI`. Also escapes `req.Host`.

### 4. LOW — No explicit TLS minimum version

`main.go` `serveMutualTLS` builds `tls.Config{ClientAuth, ClientCAs}` but does not set `MinVersion`. It currently relies on Go's default (TLS 1.2 as of Go 1.18+), which is acceptable today but not pinned to the code. For a server that may be exposed publicly this should be explicit.

**Fix:** `MinVersion: tls.VersionTLS12` (and optionally `PreferServerCipherSuites: true`).

### 5. LOW — Default unlimited concurrent forks

`config.go`: `--maxforks` defaults to `0`; `NewWebsocketdServer` only allocates the semaphore when `maxforks > 0`, so `noteForkCreated` is a no-op → unlimited concurrent subprocesses/connections. The mechanism is correct and tested; the *default* is the gap. A public-facing deployment that forgets `--maxforks` is fork/memory-DoS-able (compounds findings #1 and #2).

**Fix:** pick a finite default (e.g. a few hundred) or at least document loudly that public deployments must set `--maxforks`. Note: the `noteForkCreated` counter *only* gates WebSocket upgrades and CGI execs, not static/devconsole/redirect requests, so it isn't a full connection cap regardless.

### 6. LOW — `--origin` without a scheme skips scheme matching

`libwebsocketd/http.go` `matchOrigin`: an entry with no `://` (e.g. `--origin=trusted.com`) skips the scheme comparison entirely, so it matches both `http://trusted.com` and `https://trusted.com`. This is arguably user intent, but it's worth documenting or defaulting to require-scheme so an operator doesn't accidentally allow an `http` origin when they meant `https`.

---

## Defenses already in place (verified)

- **Path traversal / CGI RCE**: `resolveCgiPath` normalizes via `path.Clean("/"+...)` so `../` folds back inside the dir; `containsPath` is a belt-and-suspenders lexical check; well-tested on POSIX and Windows backslash cases. (`libwebsocketd/http_security_test.go`)
- **Symlink escape**: `checkPathBoundary` uses `filepath.EvalSymlinks` on both path and boundary and is applied to scriptdir, cgidir, and the `boundedDir` static file FS. The static `boundedDir.Open` checks the symlink target *before* the file is read, so no content leaks. (`libwebsocketd/handler_security_test.go`, `TestCgiSymlinkEscape`)
- **Environment isolation**: child env is explicitly constructed (`cmd.Env = env`), never inherited; `--passenv` whitelist; `buildParentEnv` strips `\n`/`\r` (env-injection hardening) and always drops `HTTPS`; on non-Windows `os.Clearenv()` after capture. Integration test `TestSEC009` asserts `HOME/USER/SHELL/...` don't leak.
- **Command injection**: `exec.Command` is used (no shell), args pass through verbatim; URL path/query are data, not interpreted. `TestSEC011`/`TestSEC012` verify.
- **Origin policy**: same-origin and allow-list checks exist and are tested (`TestSEC001`–`TestSEC008`), including `null`-origin regression.
- **mTLS**: `tls.RequireAndVerifyClientCert` with a configured CA pool when `--sslca` is set.
- **Handshake timeout** and ping/pong keepalive with read-deadline reset prevent half-open WS sessions from piling up.
- **Open-redirect (redirect server)**: the `#nosec G710` is justified — the redirect only rewrites the port of the *client-supplied* host to the canonical `--port`; the client is sent to the same host it asked for, not an attacker-chosen one.
- **Backpressure**: `PipeEndpoints` runs each direction independently with no unbounded buffering (the previous 10 MB-per-connection leak noted in SCORECARD.md was fixed).
- **Dependency hygiene**: single pinned dep `gorilla/websocket v1.5.3`; `govulncheck` clean.
- **Process teardown**: escalating `stdin close → SIGINT → SIGTERM → SIGKILL` with timeouts; no orphan/leak.

## Recommended fix order (priority)

1. Add `SetReadLimit` to the WebSocket endpoint (fixes #1) — small, high value.
2. HTML-escape `{{addr}}` substitution in the dev console (fixes #3) — one line.
3. Set `ReadHeaderTimeout` on all `http.Server` instances, and full timeouts on the redirect server (fixes #2).
4. Set `MinVersion: tls.VersionTLS12` in the mTLS config (fixes #4).
5. Document/limit the default `--maxforks` for public deployments (fixes #5).

## Test-first guidance

Per the project's test-first convention (CLAUDE.md), findings #1 and #3 are good candidates for regression tests before implementing the fixes:

- **#1**: a test asserting `readFrames` / the connection rejects a frame larger than the configured limit (send an oversized text frame, expect the connection to close / `ErrReadLimit`).
- **#3**: a test asserting `serveDevConsole` output is escaped for a `"`-containing request path — assert the response body does not contain a raw `"><script` substring and does contain the HTML-escaped form.

---
---

# Security Audit: websocketd (second pass)

**Date**: 2026-08-28 | **Scope**: same tree, plus live exploitation of every suspect code path against a running binary | **Method**: manual review + `go vet`/`govulncheck`/`gosec` + raw-socket and hand-rolled RFC6455 client attacks.

The 2026-08-17 findings were all verified as fixed in code (read limit, timeouts, `--maxforks` default 1024, TLS 1.2 pin, HTML-escaped dev console). This pass hunted for what the first audit missed, and **verified every candidate live** rather than by reading alone. All findings below were reproduced against a running server on this machine.

## Summary

| Sev | Finding | Where | Live-verified |
|-----|---------|-------|---------------|
| HIGH | One 5-byte client message permanently wedges the wrapped process (stderr pump dies on >4096-byte un-newlined stderr) | `libwebsocketd/process_endpoint.go` `logStderr`/`readStderrTagged` | ✅ both default and `--passstderr` modes |
| MED | CSWSH: default config accepts any origin (incl. `null`) — any webpage can drive the wrapped command | `libwebsocketd/http.go` `checkOrigin` default | ✅ |
| MED | IPv6 `--address` + `--redirport`: whole daemon Fatal-exits (first-colon parse bug); any one failed listener kills all | `main.go` redirect addr construction + `rejects` fan-in | ✅ |
| MED-LOW | Grandchildren escape teardown: no process group, session+fork slot held by inherited pipes after parent exits | `libwebsocketd/process_endpoint.go` `Terminate`, `launcher.go` | ✅ |
| LOW-MED | Log injection: raw terminal control sequences from child stderr land verbatim in the log stream | `process_endpoint.go` stderr logging | ✅ (`ESC[2J`, OSC) |
| LOW | `--maxframesize` < 0 silently disables the inbound frame cap | `websocket_endpoint.go` `if maxFrameSize > 0` | ✅ 3MB frame accepted with `-1` |
| LOW | `--origin` schemeless entry matches any scheme AND any port (warning only covers scheme, only under `--ssl`) | `http.go` `matchOrigin` | ✅ `http://trusted:1337` accepted |
| LOW | Unix socket perms follow umask (world-connectable under umask 0); no `--socketmode`; stale-socket probe is TOCTOU-able locally | `main.go` `serveUnixSocket` | ✅ `srwxr-xr-x` under umask 022 |
| LOW | CGI env: `SERVER_NAME`/`SERVER_PORT` derived from spoofable `Host` header; operator's real `PATH` handed to remote-triggered CGI | `http.go` `serveCGI` + Go `cgi.Handler` | ✅ |
| INFO | Misc: superfluous-WriteHeader on rejected origins; `GetURLInfo` panics on empty `URL.Path` if used without ServeMux; 10MB/conn binary-mode buffer (virtual only, ~300KB RSS measured); unbounded stdout line buffering; `--reverselookup` sync DNS per conn; predictable `UNIQUE_ID` | various | ✅ where applicable |

## Findings

### A1. HIGH — Remote-triggered permanent process hang via stderr line > 4096 bytes

`process_endpoint.go`: both stderr readers use `bufio.ReadSlice('\n')` with the default 4096-byte buffer:

```go
buf, err := bufstderr.ReadSlice('\n')
if err != nil {
    if err != io.EOF {
        pe.log.Error("process", "Unexpected error while reading STDERR from process: %s", err)
    }
    break        // <-- ErrBufferFull lands here
}
```

A stderr write of more than 4096 bytes **without a trailing newline** (progress bars, stack traces, base64 blobs — very common) returns `bufio.ErrBufferFull`, which is treated as fatal: the pump goroutine quits and never drains the pipe again. The child then blocks forever on stderr writes once the OS pipe buffer (64KB) fills.

**Live repro** (script reading commands from stdin; `bomb` = 200KB to stderr, no newline; `ping` = stdout heartbeat):

```
baseline ping -> [b'alive']
(after 5-byte "bomb" message)
post-bomb ping -> []            # process wedged
server log: "Unexpected error while reading STDERR from process: bufio: buffer full"
ps: shell + head alive but unresponsive for as long as the client holds the session
```

Reproduced in **both** default mode (`logStderr`) and `--passstderr` mode (`readStderrTagged` — same pattern). If the wrapped program echoes attacker-influenced data on stderr (compilers, linters, REPLs), this is a one-packet remote DoS per connection; wedged sessions also occupy `--maxforks` slots until the client disconnects.

**Fix:** treat `ErrBufferFull` as a partial-line event and keep reading (loop `ReadSlice` until newline/EOF, emitting partials), or use `ReadBytes` like the stdout path, or size the stderr reader to `--maxframesize`.

### A2. MED — CSWSH: default accepts every origin

`checkOrigin` returns nil when neither `--sameorigin` nor `--origin` is configured. Verified live: `Origin: http://evil.example`, `https://attacker.test`, and `null` (sandboxed-iframe origin) all receive `101 Switching Protocols` plus a full duplex channel to the wrapped process. Browsers do **not** enforce same-origin on WebSocket handshakes — the server must. Any webpage visited by a developer running a local `websocketd` (the tool's flagship use-case) can silently spawn the wrapped command per connection, feed it stdin, and read stdout. `TestSEC008` codifies the default as intended, and the docs mention the flags — but the blast radius (local command execution server) argues for an unconditional startup warning, or defaulting `--sameorigin` on when nothing is configured.

### A3. MED — IPv6 + `--redirport`: entire daemon exits; one failed listener kills all

`main.go` builds the redirect address with `strings.IndexByte(addr, ':')` — the *first* colon, which for `[::1]:8194` is inside the brackets:

```
$ ./websocketd --address='[::1]' --port=8194 --redirport=8195 --dir=...
FATAL | Can't start server: listen tcp: address [:8195: missing ']' in address
```

Verified live: the primary listener was fine, but the malformed redirect address errored into the `rejects` channel and `log.Fatal` killed **every** listener. Two issues: (1) parse with `net.SplitHostPort` (or find the colon outside brackets); (2) architecturally, any single listener failure (e.g. `--redirport` already squatted by a local process) tears down the whole server — acceptable fail-fast at startup, but the misparse makes the IPv6 combination impossible to run, and the same first-colon bug produces malformed `Location` headers for IPv6 `Host` values in the redirect handler.

### A4. MED-LOW — Grandchildren escape teardown; sessions outlive the wrapped process

`launchCmd` doesn't use `Setpgid`, and `Terminate` signals only the direct child. Verified live with a script that runs `sleep 30 &` then exits:

- The WS session and its `--maxforks` slot stay occupied after the parent exits (the grandchild inherits the stdout pipe, so no EOF) — a daemonizing script exhausts all 1024 fork slots with sleeping grandchildren while "doing nothing".
- On disconnect, signal escalation hits only the (already dead) direct child; the grandchild survives, reparented to init.
- No zombie leak was observed (reaped on session teardown).

For a tool whose contract is "the process is the connection", process-group semantics (`Setpgid` + signal `-pid`) is the standard fix.

### A5. LOW-MED — Log injection of terminal control sequences via child stderr

stderr lines are logged verbatim (`log.Error("stderr", "%s", ...)`). Verified live: a client message echoed by the script to stderr delivered raw `ESC[2J` and `ESC]50;...BEL` sequences into websocketd's stdout log stream (`od` confirms raw `033`). Request-URL/headers are *not* injectable (Go's parser rejects control chars — verified, got 400), but the child process channel is. If the wrapped program echoes remote input on stderr, an attacker can forge/mask log lines or attack whatever terminal consumes the log. **Fix:** strip C0/C1 controls (or escape them) at the log-function boundary.

### A6. LOW — `--maxframesize` ≤ 0 silently means "unlimited"

`websocket_endpoint.go`: `if maxFrameSize > 0 { ws.SetReadLimit(...) }`. A negative flag value skips the guard entirely. Verified live: `--maxframesize=-1` accepted a 3MB message (default config resets the connection). Help documents `0 = unlimited` but not negatives. **Fix:** reject negative values in `parseCommandLine`.

### A7. LOW — schemeless `--origin` entry matches any scheme *and* any port

Verified live: `--origin=trusted.example` accepts `http://trusted.example:1337` (any port) and would accept `https://` too. The existing startup warning covers only the scheme half and only under `--ssl`. Empty entries from trailing commas (`--origin=foo,`) do **not** create a bypass — `url.ParseRequestURI` rejects empty-host origins (verified: `Origin: http://` → 403). Recommend extending the warning to ports, or strict matching with an explicit wildcard syntax.

### A8. LOW — Unix socket hardening gaps

Verified `srwxr-xr-x` under umask 022 (others can't connect — connect needs the write bit), but under umask 0 (common in daemon contexts) the socket is world-connectable, and there is no `--socketmode`/chmod option. The stale-socket probe→unlink→bind sequence in `serveUnixSocket` is also TOCTOU-able by a local attacker with write access to the socket directory (minor; matches typical local threat models).

### A9. LOW — CGI environment nits

Live dump of a CGI child's env shows:
- `SERVER_NAME=x`, `SERVER_PORT=80` derived from the attacker-controlled `Host` header (`Host: x`), not from the actual bind — spoofable channel to scripts that trust RFC 3875 authority values.
- The operator's real `PATH` is passed through (default darwin `passenv`) — by design, but worth documenting: remote requesters learn the operator's PATH layout. Go's `cgi.Handler` also injects its own `PATH` (fallback after `os.Clearenv`), but `removeLeadingDuplicates` lets the `passenv` value win — harmless but a fragile coupling. On Windows (where `Clearenv` is skipped) `cgi.Handler` additionally inherits `SystemRoot/COMSPEC/PATHEXT/WINDIR` — same set as the default Windows `passenv`, so benign.

### A10. INFO — smaller notes

- `serveWebSocket` calls `http.Error` after a failed `upgrader.Upgrade` → "superfluous response.WriteHeader" noise in the log (client correctly sees gorilla's 403). Cosmetic, but pollutes stderr.
- `GetURLInfo` does `path[1:]` and would panic on `req.URL.Path == ""`; unreachable through the production wiring (DefaultServeMux 301-redirects empty/`*`/absolute-form targets — verified live), but the exported lib handler is panic-able if embedded without the mux.
- Binary mode allocates a 10MB read buffer per connection — measured only ~300KB RSS/conn (pages lazily touched; pipe reads only ever touch the front), so it's virtual-memory pressure, not the 10GB RSS it looks like. Right-size to 64KB (pipe granularity) or `--maxframesize`.
- `readTextOutput` uses `ReadBytes` → unbounded buffering of a single stdout line (mitigated by backpressure; the wrapped program is semi-trusted).
- `--reverselookup` performs a synchronous DNS lookup per connection (slow-resolver DoS amplifier).
- `generateId()` is `UnixNano` — predictable `UNIQUE_ID` (informational).
- Static file serving enables directory listings (http.FileServer default) — informational.

## Defenses re-verified live this pass

- **Traversal** (`../`, `%2e%2e`, `....//`, `//`, absolute-form, `OPTIONS *`) against both scriptdir and cgidir: 301/404, nothing executed. Double defense (ServeMux normalization + handler re-clean).
- **Symlink escape** (cgidir symlink → `/bin/sh`): refused via `checkPathBoundary` (404).
- **Env isolation / header env injection**: header names can't carry `=`; values stripped of CR/LF; `HTTPS` dropped.
- **Default 1MB frame cap enforced** (3MB frame → connection reset).
- **Slowloris/timeout posture** and ping-pong dead-client detection unchanged and sound.
- **Open redirect** on the redirect server: still not exploitable (only rewrites port of client's own Host).
- **Dev console XSS**: escape now in place (verified in code; `html.EscapeString` on `{{addr}}`).
- **httpoxy (CVE-2016-5385)**: fixed *after* this audit's live pass, in commit `c7b13e1` — a client-supplied `Proxy` header is now dropped in `createEnv` so it never reaches the child as `HTTP_PROXY` (matching `net/http/cgi`, which already protected the `--cgidir` path). Regression-tested by `TestSEC013`.
- **Dependencies**: `govulncheck` clean, `go vet` clean, `gosec` only G104-style noise. CI supply chain (pinned k6 + checksums) sound.

## Remediation status (2026-08-28, same day)

- **A1 FIXED** — commit `c304bc4`: both stderr pumps now treat `ErrBufferFull` as a partial line and keep draining; long stderr lines are relayed/logged as consecutive chunks. Regression: `TestStderrLongLineKeepsProcessAlive{,_PassStderr}`.
- **A2 DEFERRED** — deliberately not changed: restricting the default origin policy is a breaking behavior change and needs a maintainer call (candidates: unconditional startup warning, or `--sameorigin` by default). Remains documented above.
- **A3 FIXED** — commit `f8e3cb3`: `redirectAddress`/`redirectLocation` parse with `net.SplitHostPort`; IPv6 `--address` + `--redirport` works, Location headers correctly bracketed. Regression: `TestRedirect{Address,Location}`, `TestCLI014`.
- **A4 FIXED (teardown half)** — commit `14374dc`: children run in their own process group; teardown signals the group and finishes with a SIGKILL sweep; survivors must `setsid`. Regression: `TestPROC013`. The *session-linger* half (a live client keeps a session open while a grandchild holds the pipes) is intentionally unchanged — closing on direct-child exit would break the legitimate fork-a-worker-then-exit pattern.
- **A5 FIXED** — commit `2e963cf`: the log function escapes control bytes as `\xNN` in messages and associated values. Regression: `TestLogfuncEscapesControls`, `TestEscapeControls`.
- **A6, A7, A8, A9 — filed for triage** as issues #472, #473, #474, #475.

## Recommended fix order

1. **A1** stderr pump (`ErrBufferFull` handling) — small fix, highest impact.
2. **A2** unconditional CSWSH warning (or same-origin default) — one log line, changes the default risk story.
3. **A3** `net.SplitHostPort` for the redirect address (+ decide listener-failure policy).
4. **A6** reject negative `--maxframesize`.
5. **A4** process-group teardown; **A5** control-char stripping in logs; **A7/A8/A9** docs + hardening.

## Test-first guidance

- **A1**: integration test — serve a script that writes >4KB un-newlined to stderr then a stdout heartbeat; assert the heartbeat still arrives (fails today).
- **A3**: unit test on the redirect-address construction for `[::1]` input.
- **A6**: config test — `--maxframesize=-1` should exit(1) with a message.
- **A2**: assertion-free (behavior already codified); the fix is a warning, testable via log output.
