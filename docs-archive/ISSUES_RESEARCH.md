# Documentation-worthy content mined from 339 closed GitHub issues

> **Archived 2026-09-07. Consumed — not a to-do list any more.**
>
> Raw research: all 339 closed GitHub issues, read in full and grouped by
> theme. It fed `MANUAL_INVENTORY.md` and through that the shipped site
> (`docsite/`, 44 pages). It is kept, rather than deleted with its two
> sibling research files, because it is the only local record of an
> *external* system: reproducing it needs network access, `gh` auth, and a
> re-read of 339 issues, and issue bodies can be edited or deleted upstream.
>
> **§0's two "act now, independent of the docs rewrite" items are both
> done. Do not act on them again.** The wiki is no longer linked from
> `README.md`, `website/index.html` or any example — retiring it was
> executed as part of the docs project. httpoxy (CVE-2016-5385) was already
> fixed when this was written: `libwebsocketd/env.go` skips a client-supplied
> `Proxy` header before the `HTTP_<NAME>` mapping, with the CVE cited in the
> comment. This document asserting there is "no fix" was wrong on the day,
> and its sibling drift audit said so.
>
> Nothing here describes current behaviour. For that, read `docsite/content/`.

Source: `gh issue list --state closed --limit 400`, all 339 closed issues (2013-2026),
read in full (titles + bodies, comments pulled for ambiguous/high-value ones).
Feeds the docs rewrite for GitHub issue #467. Grouped by theme; each item gives
the issue(s), the documentation-worthy fact, and a relevance call informed by a
quick cross-check against current code/CHANGES/CLAUDE.md (not a full audit —
verify before publishing).

---

## 0. Urgent, non-docs finding surfaced by this scan

- **The GitHub wiki has been compromised with malware/ransomware links** (#432,
  #433, both 2022). The wiki is still linked from README.md ("Download for
  Linux, macOS and Windows" → wiki page) and is the historical home for
  install instructions, platform guides (Raspberry Pi, Apache, PHP/C
  examples), and the Ten-minute tutorial. **This should not wait for the docs
  rewrite** — recommend checking the current wiki state and taking it down or
  locking it immediately regardless of when the new manual ships. Strong
  independent corroboration for "wiki should be deleted" as a placement
  decision (item 2), separate from the security concern.

- **httpoxy (CVE-2016-5385) was flagged by a user in 2021** (#418, Gosec G504
  on `net/http/cgi` import) and closed/dismissed at the time. The 2026-08-28
  internal codebase scorecard **independently re-found this as a live,
  unauthenticated HIGH-severity vulnerability** (`env.go:101-105` — no `Proxy`
  header exclusion) that as of this scan has **no corresponding open GitHub
  issue and no fix**. This is a real gap between an internal audit and the
  public tracker. Recommend filing a real issue and fixing it (one-line fix
  per the scorecard) independent of the docs project.

---

## 1. Stream buffering — the single most common support request

By far the largest recurring category. A user picks a language, output
doesn't appear in the browser until the process exits or a buffer fills, and
they file an issue thinking websocketd is broken.

- **Python**: needs `-u` (unbuffered) flag or `sys.stdout.flush()` after every
  `print` — #85, #98, #199, #211, #354, #355, #388 (regression between two PHP
  point releases — actually python), #442. This is the most repeated issue in
  the entire tracker.
- **C**: needs `setbuf(stdout, NULL)` or explicit `fflush` — #117, #122.
- **PHP**: short-tag `<?` vs `<?php` breaks silently (outputs source instead
  of running) — #97. Also PHP-specific "no output" reports that turn out to
  be missing `flush()`/`ob_flush()` — #388, #406.
- **Ruby**: needs `STDOUT.sync = true` — implied pattern, not spelled out
  anywhere in current docs.
- **Node.js**: stdin read patterns are non-obvious (readline/async patterns
  fail silently) — #442.
- A generic **`stdbuf`/`unbuffer` workaround** was proposed as a built-in
  feature (LD_PRELOAD libstdbuf) and never implemented — #122, #400. Still
  relevant as a documented workaround (`stdbuf -oL your-program`), not a
  feature gap.
- **Manual's job**: a single prominent "Why isn't my program sending data in
  real time?" page covering the buffering story per-language, cross-linked
  from the tutorial, the FAQ, and every language example. This one page would
  have preempted a double-digit fraction of the entire issue tracker.

## 2. The wire protocol contract (newline-delimited text)

- Messages are framed by `\n` on both directions; a script's output is only
  forwarded to the client **when a newline is seen** — surprised multiple
  users badly enough to file issues after "struggling a lot": #271. This is
  presently under-documented as a hard framing rule (vs. an implementation
  detail).
- Sending non-newline-terminated interactive input (for programs like `vim`,
  `less`) was requested and never implemented — #9. Still a known limitation,
  worth stating explicitly as "not supported" rather than leaving silent.
- Binary/raw-frame mode (RFC6455 binary frames instead of newline-text) was
  proposed multiple times and never implemented — #38, #171 (fragmentation),
  #232, #258, #267, #429. **`--binary=true`/`--binary` flag exists but is
  widely misunderstood** — #443 shows a user expecting it to disable the
  newline-splitting behavior for raw keystroke-at-a-time terminal use; it
  doesn't. Needs a precise, mechanism-level explanation of what `--binary`
  actually changes (message type flag) vs. what it doesn't (framing is still
  newline-based).
- Compression (`permessage-deflate`) unsupported — #174. Still true, worth a
  one-line "not supported" in the reference rather than silence.
- `--maxframesize` exists but multiple users didn't know how to find/use it
  — #445 (asked "how to change framesize" with no awareness of the flag),
  ties into the man-page gap the scorecard already found (missing entirely
  from `release/websocketd.man`).

## 3. Process lifecycle, signals, and the one-process-per-connection model

- **The most consequential architectural fact users don't discover from docs
  until it bites them**: websocketd spawns a **new process per WebSocket
  connection**; there is no built-in way to share state or broadcast between
  connections. Requested repeatedly as "add multiplexing/broadcast" and
  declined/deferred every time: #50, #84, #95, #164, #224, #226, #284, #289,
  #345, #374, #416. This needs to be a first-class, early, unmissable
  statement in the manual — "how websocketd works" — with the recommended
  workaround patterns (external message bus, file/FIFO-based coordination,
  a single long-lived backend process the per-connection scripts fan out to)
  spelled out, not left to Stack-Overflow-style GitHub comments.
- **Long-running/daemon processes must handle SIGTERM/SIGINT to exit** or the
  websocketd child never terminates after disconnect — explicitly requested
  as a doc addition back in 2014 (#71) and apparently never added. High-value,
  cheap fix for the manual.
- **Windows does not support SIGINT/SIGTERM** — real, permanent platform
  limitation (Go's `os.Process.Signal` on Windows), confirmed by multiple
  independent reports across years: #298, #362. Needs to be a documented
  platform-notes item, not something users keep rediscovering as a bug.
- Subprocesses of the wrapped process are **not** killed on disconnect, only
  the direct child — #191 (gphoto2 continuing after browser tab closed).
  Worth a platform-notes/gotchas callout.
- Process starts lazily on first connection, not at websocketd startup —
  asked about as if it were a bug — #34. Worth stating as intended behavior.
- `--pingms` / keepalive: connections appear to die after ~90s–2min or after
  hours with no traffic; ping/pong control frames now exist but were, for a
  long time, entirely undiscovered by users hitting timeouts — #37, #209,
  #260, #275, #439. Given the scorecard's finding that `--pingms=0` (still
  the default) creates a DoS vector, the manual should recommend a non-zero
  `--pingms` as a matter of course, not just document the flag.
- "Run once and exit" / don't respawn on reconnect was requested repeatedly
  and never built — #132, #300, #317 (user self-solved with shell wrapper +
  `pkill`), #414. Worth documenting the shell-wrapper pattern as the
  recommended answer.
- Graceful shutdown hook (`--no-kill-process` type option, let a script finish
  cleanup before SIGKILL) requested, never built — #207. websocketd already
  waits some fixed grace period between SIGINT/SIGTERM/SIGKILL — worth
  documenting the actual current timing precisely (it's in `libwebsocketd/`
  process teardown code) since users have had to reverse-engineer it.

## 4. CGI / environment variables — recurring confusion, some real bugs

- `QUERY_STRING` / URL query parameters not reaching the script is one of the
  most repeated "am I doing something wrong" reports: #202, #223, #312, #391.
  Root cause is almost always **`--passenv` not doing what people assume**
  (it passes *environment* variables through, `QUERY_STRING` is already a
  synthesized CGI variable available regardless) or forgetting `--cgidir` vs
  plain WebSocket mode have different variable sets. Needs one clear
  worked example per mode (WS script vs `--cgidir` script) showing exactly
  which variables are populated.
- `PATH` not passed to child processes by default was a years-long complaint
  (#144, opened 2015) — **now resolved**: current default `--passenv` includes
  `PATH` on darwin/linux per the 2026-08-28 scorecard (finding A9, #475),
  which also flags that this leaks the operator's directory layout to any
  requester. Worth a one-line security-conscious mention in the reference
  rather than silence.
- `REMOTE_ADDR`/`REMOTE_HOST` historically included the port
  (`127.0.0.1:35947` instead of `127.0.0.1`) — #60 (2014). **Needs a live
  spot-check against current `env.go`** before deciding whether this is
  still true or was fixed silently; not verified in this pass.
- `SERVER_NAME`/`SERVER_PORT` are derived from the client-controlled `Host`
  header, not the server's actual bind address — flagged again in the
  2026-08-28 scorecard (#475) and tracked as accepted/documented behavior in
  `CLAUDE.md`'s do-not-touch list. Manual should state this plainly as a
  CGI environment quirk with a security note (don't trust these values for
  access decisions in your script).
- Custom headers (`--header`, `--header-ws`, `--header-http`) exist but users
  couldn't find documentation for them and reported them as "not working"
  when they were actually just placed after the command instead of before it
  (flag-ordering gotcha, Go's `flag` package stops parsing at the first
  non-flag argument) — #151, #155. This flag-ordering trap likely bites many
  flags, not just headers — worth a single prominent "flags go before the
  command" callout in the reference intro.
- `--sizeheader` (4-byte length-prefixed framing) was built by a third party
  as a fork feature and offered upstream, apparently never merged — #377.
  Confirm current status; if truly absent, it's a legitimate "not supported"
  entry, not a doc gap.

## 5. Origin / security / auth — the recurring "does it have X" FAQ

- **No built-in authentication.** Asked repeatedly across a decade — #172
  (basic auth), #272, #334 (intercept upgrade to return 401). Manual needs an
  explicit, confident answer: websocketd deliberately has none; put it behind
  a reverse proxy that does auth (nginx/Apache/Caddy with `auth_request` or
  similar), or have the wrapped script itself validate a token from
  `QUERY_STRING`/a header. This is exactly the kind of thing that reads as
  "abandoned/incomplete" to a newcomer if left unanswered (see #226, #228,
  the harshest "not production ready" complaints) but is a one-paragraph fix
  once written.
- **No CORS support** beyond the origin-checking mechanism — #302. Worth
  clarifying that `--origin`/`--anyorigin` is the whole story; there's no
  separate CORS header mechanism because WebSocket doesn't use CORS the way
  XHR does — this distinction itself is a common misunderstanding worth a
  short explainer.
- Origin rejection is the single most confusing default for newcomers
  following the tutorial: "rejected null origin" hitting anyone who opens
  their test HTML via `file://` instead of via the server — #75, #96, #148.
  The Ten-minute tutorial itself was reported as broken by this exact trap
  (#75, #438 much more recent and detailed, 2025). **This needs a fix in the
  tutorial's flow itself**, not just an FAQ entry — #438 is essentially a
  full rewrite review of the tutorial from a confused-but-diligent
  newcomer's perspective, worth reading directly when writing the new
  intro tutorial.
- Two currently-open, security-scorecard-confirmed origin/frame-limit
  behaviors are already tracked as accepted/intentional in `CLAUDE.md`'s
  do-not-touch list and correspond to real issues: **schemeless `--origin`
  matches any port** (#473) and **negative `--maxframesize` silently
  disables the frame cap** (#472). Both need clear, prominent documentation
  in the flag reference (not just a warning banner at startup) since the
  whole problem is operators not realizing the implication.
- SSL/TLS: disabling SSLv3/forcing TLS-only asked (#185, predates the current
  `MinVersion: tls.VersionTLS12` fix — now resolved, confirm doc reflects
  it), `--sslca` for custom client-CA validation requested with a detailed
  real use case (mTLS with self-signed client certs) — #413 — **this flag now
  exists** per `CLAUDE.md`'s do-not-touch list ("`--sslca`-without-`--ssl`" is
  called out as an untested combination), so the request was fulfilled;
  worth an example in the deployment/mTLS docs since the original ask
  included a good worked scenario.

## 6. Deployment patterns (issue #467 explicitly asks for these)

- **Reverse proxy fronting** (nginx, Apache, HAProxy) for SSL offload, path
  sharing, load balancing — requested as a doc addition in 2014 (#27) and
  apparently only ever answered ad hoc in wiki comments/pages (#109 Docker +
  nginx + basic auth, #248 Apache mod_proxy_wstunnel, referenced again in
  #399's Apache config for a real production bug report). These are proven,
  user-contributed, working configs — worth recovering the actual config
  snippets from the wiki (before it's taken down, see §0) and turning them
  into first-class deployment guide pages for nginx, Apache, and HAProxy.
- **Same-port coexistence with another web server** (e.g., Express/Node app
  and websocketd sharing port 80/443) — asked repeatedly, answer is always
  "reverse proxy in front, route by path" — #165, #299. Worth a explicit
  "can I share a port" FAQ entry pointing at the reverse-proxy guide.
- **systemd / start at boot** — asked whether special options are needed
  since systemd redirects stdin/stderr to the journal — #222, #329. Worth a
  minimal systemd unit file example in the deployment guide (a real gap:
  nobody ever produced one, per #329's own request).
- **Docker**: interactive TTY-requiring commands (`docker run -it`) don't
  work through websocketd because websocketd doesn't allocate a pty — #368.
  This is a real, permanent limitation (websocketd talks stdin/stdout pipes,
  not a pty) that deserves a clear "why this doesn't work and what to do
  instead" note, since multiple issues hit the same root cause from
  different angles (#182 `watch`, #339 `screen`, #368 `docker -it`) — all are
  really "programs that require a real terminal don't work" and should be
  one documented limitation, not three separate confused threads.
- **Cloud/NAT/public exposure confusion** — users repeatedly not
  understanding `--address` binding vs. router port-forwarding vs. cloud
  security groups: #297, #349. Not websocketd-specific, but worth a short
  "exposing this to the internet" checklist since it's asked often enough.
- **Kubernetes** — issue #467 explicitly names this; no existing issue
  history to mine (nobody's filed a k8s-specific bug), meaning this section
  of the manual has no prior art to inherit — plan it as new-from-scratch
  content, likely a minimal Deployment+Service+Ingress(websocket-aware)
  example.

## 7. `--dir` / `--cgidir` routing — chronically under-documented

This whole area recurs so often it should be one of the manual's flagship
"cookbook" pages, not scattered flag descriptions.

- Basic confusion about how `--dir` maps URLs to scripts and whether/how a
  matching `.html` gets served alongside each script — #102, #192 (detailed,
  offered to contribute a wiki write-up once someone explained it), #245,
  #246, #310.
- Subdirectories under `--dir`/`--cgidir` reportedly didn't resolve
  (`/scripts/dir1/event.sh` → "no such file or directory" even though the
  file exists) — #233 (2016). **Needs a live spot-check**; not verified here.
- Passing arguments/parameters to a script invoked via `--dir` — asked
  repeatedly with no clean answer beyond `QUERY_STRING`: #247, #294 (tried
  URL-encoding a literal argument into the path, doesn't work), #391, #353
  (asked specifically about `--devconsole` + args).
- A removed, poorly-specified `--basepath` option (#53, #87) illustrates that
  this area has a history of confusing flags being added and later ripped
  out — worth being extra precise in the new reference about exactly what
  `--dir`/`--cgidir`/`--staticdir` each do and don't do, since imprecision
  here has cost a real feature revert before.
- URL rewriting / path-based routing to a single script (Nginx-style rewrite
  rules) requested multiple times, never built: #251, #447. Legitimate
  "not supported, use a reverse proxy for this" answer, same shape as the
  auth question.

## 8. Windows-specific gotchas (recurring platform-notes material)

- `.cmd`/`.bat` files fail with cryptic `fork/exec` errors depending on how
  they're invoked — #129, #293 ("not a valid Win32 application").
- Command-name aliasing failures — e.g. `nodejs` vs `node` resolves to the
  wrong/no binary via Windows `PATH` lookup rules — #372.
- CGI on Windows: no shebang support, `.pl`/`.ps1` scripts don't "just run"
  the way they do on Unix — #384, #454. Needs an explicit "on Windows, CGI
  scripts must be directly executable (.exe/.bat with correct associations),
  shebangs are ignored" statement.
- SSL on Windows: `CryptAcquireContext: Invalid Signature` with certain
  certificate formats — #181. Possibly stale (old Windows crypto stack
  issue), worth a spot-check but likely fine as a historical footnote only.
- Non-ASCII output (Chinese characters) breaking things, tangled up with the
  separate "SIGINT unsuccessful… not supported by windows" message appearing
  for unrelated reasons — #298. The two problems are easy to conflate;
  worth disentangling clearly in platform notes.
- QUERY_STRING and other CGI vars not working correctly specifically on the
  64-bit Windows executable in one very old report — #63. Likely stale
  (pre-Go-modules build), not worth carrying forward without re-verification.

## 9. Build-from-source — resolved by Go modules, but confirm docs catch up

A long tail of confusion from the pre-Go-modules Makefile/GOPATH era: #76,
#175, #176, #225, #295, #328, #124 (IBM s390 cross-compile), #434 (MIPS
cross-compile). The underlying pain point — "there is no simple `go build`
story" — is very likely already resolved by the current `go.mod`-based
build (per `CLAUDE.md`: `go build`). **Action for the manual**: a short,
current, accurate "Building from source" page (just `go build`, plus how to
cross-compile with `GOOS`/`GOARCH` for the platforms people actually asked
about) would retire this entire historical category in one page, and is
worth writing precisely because so many now-stale issues show how much pain
existed here before.

## 10. Release/distribution gaps

- Missing platform binaries reported repeatedly: ARM64 shipped as x86-64 by
  mistake, twice (#200, #217, #280 — same bug recurring across releases,
  suggests the release pipeline itself needs a "does the binary match its
  label" check, not just a docs fix), 32-bit macOS (#36), BSD/Solaris/MIPS
  (#26, #107, #124, #419, #434).
- No way to verify a downloaded binary (no checksums/signatures published)
  — #101. Worth checking if this is still true and, if so, whether it's now
  addressed by GitHub's own release-asset checksums or GoReleaser-style
  provenance — relevant to the "download/install" section of the manual.
- Package manager requests: Homebrew (fulfilled — README already documents
  `brew install websocketd`), npm framed as "why not" (#11, fundamentally
  wrong tool for a Go binary, worth a one-line "why not npm" FAQ entry since
  it recurs), deb/rpm/pkg via `fpm` (#28) — confirm current release process
  status for the manual's install section.

## 11. Devconsole-specific documentation gaps

- Old browser incompatibilities (Firefox 21/26/35) — all stale, Firefox has
  long since fixed the underlying WebSocket issues; not worth carrying
  forward except as a reminder that the new devconsole (already rebuilt this
  cycle, see A1) should get real cross-browser testing in CI/QA rather than
  relying on bug reports.
- Console URL defaults to `ws://:PORT/` (empty host) when `--address` isn't
  specified — confusing/invalid-looking URL — #143. **Spot-check against
  current devconsole** (rebuilt this cycle) — may already be moot.
- Devconsole should default to `wss://` when the page itself is loaded over
  HTTPS — #142. Same — spot-check against the rebuilt console.
- Positional-argument confusion: `websocketd bash --devconsole` 404s, but
  `websocketd --devconsole bash` (flags-before-command) works — #189, and
  more recently the very detailed #438 (2025) walks through the exact same
  trap in the Ten-minute tutorial context, compounded with the `file://`
  origin-rejection issue (§5) and a `--staticdir`/`--devconsole` mutual
  exclusivity that isn't called out anywhere. **#438 is close to a ready-made
  spec for what's wrong with the current tutorial** — read it directly when
  drafting the new intro tutorial.
- `close` event's `clean`/`code`/`reason` fields shown in the devconsole are
  undocumented and never explained — #405. Small, cheap doc fix: a short
  glossary entry.

## 12. FAQ candidates (cross-cutting, asked independently many times)

Ranked roughly by how often independent users hit the same wall:

1. "Why doesn't my script send output in real time?" → buffering (§1).
2. "How do I share/broadcast data across multiple connected clients?" → you
   can't directly, one process per connection by design (§3).
3. "Does this support authentication?" → no, front it with a proxy (§5).
4. "How do I pass URL parameters / query string data to my script?" → CGI
   env vars, `--passenv`, and their limits (§4, §7).
5. "Why did my connection drop after N minutes/hours?" → `--pingms`/keepalive
   (§3).
6. "Is this production-ready? Isn't spawning a process per connection
   wasteful?" → a direct, honest answer belongs in the manual instead of
   being litigated in issue comments every year (#226, #228, #356 hitting a
   concurrent-connection ceiling around 150-1024 depending on `ulimit`/
   `--maxforks`).
7. "How do I run a Windows .bat/.cmd/.ps1 file?" → platform notes (§8).
8. "How do I keep the process running after the script naturally exits /
   run it once only?" → shell-wrapper pattern (§3).
9. "Can I run this behind nginx/Apache?" → deployment guide (§6).
10. "Why can't I use `screen`/`watch`/`docker -it`?" → no pty allocation,
    permanent limitation (§6).

## 13. Website/meta issues (relevant to the placement guide, item 2)

- Newsletter/email signup on websocketd.com is broken (#330, #343 — mailchimp
  404). If the new site keeps any kind of mailing list, verify it actually
  works, or drop it rather than ship a second broken version.
- Custom domain TLS cert mismatch (`SSL_ERROR_BAD_CERT_DOMAIN`) reported
  2026 (#449) — worth verifying resolved (relevant to the `website/CNAME`
  file CLAUDE.md flags as must-not-touch).
- Domain resolution outage reported historically (#313).
- A user explicitly proposed GitHub Discussions to separate genuine bugs from
  "questions that aren't issues" (#407, 2019) — directly relevant input for
  the docs-placement recommendation guide: a large fraction of these 339
  issues are really support questions that a good manual (plus, perhaps,
  Discussions) would have deflected.

## 14. Positive/community signal worth preserving

Not documentation gaps, but worth knowing before writing a "who uses this /
testimonials" or examples section: several real production users showed up
in issues describing their setups — codebender.cc (#36), a CMTS/cable-modem
monitoring dashboard (#202), industrial/IoT sensor dashboards (#196, #420),
a Raspberry-Pi robotics counter (#211), an Arduino/OpenWrt chat idea (#262),
gphoto2-based photo-dump tooling (#191), a terminal-in-browser game/MUD-style
project (#443). These are candidate "real-world examples" content for the
cookbook, several with enough detail in the issue to reconstruct a minimal
version.

---

## Summary

- **14 categories**, roughly **120 individual issue references** extracted
  and grouped (of 339 total closed issues scanned — the remainder were
  duplicates, one-line non-answers, pure "thank you" comments, or personal
  debugging sessions with no generalizable content).
- **Most surprising finding**: the wiki currently hosts ransomware/malware
  links (#432, #433) and is still linked from the live README — independent
  of the docs rewrite's timeline, this seems worth flagging for immediate
  action.
- **Most valuable single insight for scoping the manual**: an enormous
  fraction of this tracker's 339 issues are the same handful of questions
  asked over and over for a decade — stdout buffering, one-process-per-
  connection/no-broadcast, no built-in auth, origin rejection surprising
  `file://` testers, and Windows signal handling. A manual that puts these
  five things front-and-center (not buried in a flag reference) would have
  plausibly prevented dozens of these issues from ever being filed.
- **Best FAQ-candidate raw material**: issue #438 (2025) is a detailed,
  good-faith walkthrough of everything confusing about the *current*
  Ten-minute tutorial, written by someone who tried hard to follow it. Read
  it directly when drafting the new intro tutorial — it's close to a ready
  punch list.
