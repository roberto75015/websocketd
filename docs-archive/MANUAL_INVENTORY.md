# Manual content inventory

> **Archived 2026-09-07. Worked through — not a to-do list any more.**
>
> This was the content checklist for the docs rewrite. The site shipped
> (`docsite/`, 44 pages), every fact in it was re-verified against source
> while writing rather than copied from here, and §6's documentation-debt
> punch list is done bar the 0.5.0 release, which `ASKS.md` tracks. It is
> kept for provenance only: §1 and §7 carry the issue numbers behind each
> flag note, which `tools/gendocs` still cites so a reader can find the
> original report. For what the docs *are*, read `docsite/PLAN.md` and the
> pages themselves; §6's drift list describes a state of the repo that no
> longer exists.

Everything documentable about websocketd, built from a full scan of the
codebase, all 470 commits (2013-02-14 → 2026-09-05), and all 339 closed
GitHub issues. Feeds the new manual/cookbook (issue #467). This is a content
checklist for whoever writes the manual, not the manual itself — each item
below should end up as one fact, one sentence, or one page, not be copied
verbatim.

Full research trails (raw, unedited fork output): `ISSUES_RESEARCH.md` and
`EXTERNAL_RESEARCH.md` alongside this file. The other two trails were
deleted rather than archived — one was a repository-state snapshot that had
gone wrong, the other was regenerable from this repository's own git
history. See DIARY.md, 2026-09-07.

---

## 1. CLI flags (current, authoritative — 28 total)

The single most important fact about this list: **`--help` is already
correct and should be treated as the seed text**, not the man page (which
has drifted — see §6). That split no longer exists: `--help`, the man page
and the site's flag reference are generated from one definition
(`internal/cliflags`, `tools/gendocs`), so the current table with defaults
is `docsite/content/reference/cli-flags.md`. Per-flag
documentation notes worth calling out beyond a bare reference entry:

| Flag | Doc note beyond the bare reference |
|---|---|
| `--maxforks` (default 1024) | State plainly it's a backstop, not a capacity plan; explain the 150-1024 concurrent-connection ceiling users hit and ask about (issues #226, #228, #356). |
| `--pingms` (default 0/disabled) | Recommend a non-zero value as a matter of course — default-off is a known DoS vector (idle connections never time out) and the #1 cause of mystery disconnects users spent years asking about (#37, #209, #260, #275, #439). |
| `--maxframesize` | Missing from the man page entirely; also invisible enough that a user asked how to find it (#445) — needs prominent placement, not just a table row. |
| `--binary` | Widely misunderstood as "disable newline framing for raw keystrokes" — it doesn't (#443). Needs a precise, mechanism-level explanation: it changes the WS message type flag, not the framing rule. |
| `--origin`/`--sameorigin`/`--anyorigin` | Default is permissive by design (do-not-touch); scheme-less `--origin` matches any port (#473) and negative `--maxframesize` disables the cap (#472) are both accepted-but-easy-to-misread behaviors — document the implication, not just the flag. |
| `--sslca` | Silently no-ops without `--ssl` — no validation ties them together. Common "why didn't mTLS turn on" trap. |
| `--passenv` | Users confuse this with query-string/CGI variable passthrough (#202, #223, #312, #391) — it's neither; needs one worked example per mode. |
| `--header`/`--header-ws`/`--header-http` | Reported as "not working" when actually just placed after the command instead of before it (#151, #155) — this Go-`flag`-package ordering trap likely bites many flags; needs one prominent callout in the reference intro, not a per-flag note. |
| `--staticdir` | Serves dotfiles and directory listings with no exclusion — confirmed still true, filed as **issue #476**. Document as an operator responsibility until fixed: point it at a directory containing only what's meant to be public. |
| `--dir`/`--cgidir` | Chronically under-documented as a *pair* — deserves a flagship cookbook page, not scattered flag descriptions (§7 below). |

## 2. Environment variables (CGI contract)

Full list in `current-surface-and-drift.md` §Part 1. Trust-boundary facts
that MUST be stated plainly, not implied:
- `SERVER_NAME`/`SERVER_PORT` are **Host-header derived**, spoofable by
  anything that controls the client's Host header (issue #475, accepted).
- httpoxy (CVE-2016-5385): `Proxy` header is dropped before the
  `HTTP_<NAME>` mapping — **fixed in code, undocumented everywhere else**.
  Worth its own short security-notes paragraph; this exact bug class hit
  `net/http/cgi` industry-wide in 2016, which is why it's worth naming.
- `PATH` passthrough is default-on — leaks the operator's directory layout
  to any requester by design (documented, not "fixed").
- `AUTH_TYPE`/`REMOTE_USER`/`REMOTE_IDENT` always blank — no built-in auth,
  say so here too, not just in the security FAQ.
- `QUERY_STRING` availability is the #1 source of "am I doing something
  wrong" reports (#202, #223, #312, #391) — needs one worked example per
  mode (plain WS script vs `--cgidir` script), not a table row.

## 3. Behavior not gated by a flag

- One process **per WebSocket connection**, no shared state, no built-in
  broadcast — see §7, this is the single highest-value fact in the whole
  manual.
- Message framing: newline-delimited text by default; a script's output
  is only forwarded **when a newline is seen** (#271) — state as a hard
  framing rule, not an implementation detail.
- Non-newline-terminated interactive input (`vim`, `less`) is not
  supported and never will be (#9) — say so plainly rather than leaving it
  silent.
- Process teardown ladder: stdin close → 100ms+closems → SIGINT →
  250ms+closems → SIGTERM → 500ms+closems → SIGKILL → 1000ms → SIGKILL
  sweep of the whole process group (Unix only — Windows signals only the
  direct child). Long-running processes **must handle SIGTERM/SIGINT** to
  exit cleanly, or they never terminate (#71, requested as a doc addition
  in 2014 and apparently never added — cheap, high-value fix).
- Subprocesses of the wrapped process are not killed on disconnect, only
  the direct child (#191).
- Process starts lazily on first connection, not at websocketd startup —
  intended, not a bug (#34).
- Origin default is permissive (loud stderr banner) unless
  `--sameorigin`/`--origin`/`--anyorigin` given.
- TLS floor pinned at 1.2. No `ReadTimeout`/`WriteTimeout` on the main
  server (deliberate — would kill long streams); `ReadHeaderTimeout: 10s`
  only.
- No compression (`permessage-deflate`) — say "not supported" once, in the
  reference (#174).
- No pty allocation — anything requiring a real terminal (`docker run -it`,
  `watch`, `screen`) does not work through websocketd. Three issues (#182,
  #339, #368) independently hit this from different angles; it's one
  limitation, document it once.

## 4. Removed / superseded — do NOT document as current, but good FAQ fodder

- `--verbose` (removed 2013), `--basepath` (removed 2014, superseded by
  `--dir`), a reusable process pool (built then explicitly reverted, 2014
  — "why not pool processes" is a legitimate FAQ entry), the original
  multi-socket implementation (buggy, replaced by today's repeated
  `--address`), FreeBSD/OpenBSD/Solaris/MIPS release targets (dropped
  2026-08-20 — don't imply official support even though `defaultPassEnv`
  still harmlessly lists them), a proposed built-in `stdbuf`/LD_PRELOAD
  unbuffering feature (never built — document the manual workaround
  instead: `stdbuf -oL your-program`), a proposed 4-byte length-prefixed
  framing mode built by a third party and never merged (#377 — confirm
  current status before writing this up as "not supported").
- Version numbering is **manually set, not build-derived** — this was
  tried, then deliberately reverted back ("was right originally," 2019).
  Counter-intuitive enough to state explicitly rather than let someone
  rediscover it.

## 5. Security & hardening — write as one coherent pass, not scattered history

The busiest single day in the project's history, 2026-08-17, landed roughly
a dozen independent hardening fixes (symlink jails, XSS escape, TLS floor,
slowloris timeouts, `--maxforks` default flip to 1024, `--maxframesize`
added). A second wave (2026-08-20 → 09-01) added the httpoxy fix, strict
origin port matching, crypto-random connection IDs, and the
loud-banner-plus-`--anyorigin` origin policy. **The manual's security
chapter should be dated/framed as documenting one coherent posture, not a
changelog.**

Standing, deliberate defaults a manual must state as *intentional*, not
bugs (verbatim from `CLAUDE.md`'s do-not-touch list — do not relitigate):
no global SIGTERM handler for unix-socket cleanup; no read/write timeouts
on the main server; `--maxforks`/`--maxframesize` defaults are backstops,
not capacity plans; `--origin` port matching is strict; `--socketmode`
doesn't change default permissions; `SERVER_NAME`/`SERVER_PORT` are
Host-derived; the permissive origin default stays until an announced flip
— say a stricter default is coming, without promising a date.

**Known, currently-unfixed gap** (now tracked, not just noted): `--staticdir`
serves dotfiles and directory listings — issue #476, filed alongside this
inventory. Document as an operator responsibility until it's fixed.

**No built-in authentication, ever, by design.** Asked repeatedly for a
decade (#172, #272, #334) and reads as "abandoned" to newcomers if left
unanswered (#226, #228). Needs a confident, one-paragraph answer: put it
behind a reverse proxy that does auth, or validate a token in the wrapped
script itself.

## 6. Documentation debt to fix as part of the rewrite (not new content — corrections)

Confirmed still live at current HEAD (`current-surface-and-drift.md` Part 2):
1. Man page: `--maxforks` says default 0 (unlimited); real default is 1024.
2. Man page: missing `--maxframesize` and `--anyorigin` entirely.
3. Website: "Current version: 0.4.1," download links point at that stale
   tag — **no 0.5.0 release has actually been cut**, so this isn't a copy
   fix, it needs a release (see `MANUAL_TOOLING.md`).
4. `CLAUDE.md`: "No linter is currently configured" — CI runs four.
5. `CHANGES` header date ("Apr 26, 2026") predates the commits it
   describes by five months; three version sources disagree (git tag
   0.4.1, CHANGES 0.5.0, website 0.4.1).
6. `examples/java/README.md` tells readers to run scripts one directory
   level up from where they actually live.
7. The wiki's `Command-line-options.md` is a pasted `0.3.0` `--help` dump,
   missing at least 8 flags — moot once the wiki is retired (`MANUAL_PLACEMENT.md`).
8. The wiki's `Embedding-in-go-apps.md` example does not compile — verify
   the real `libwebsocketd` embedding API before writing any "embed
   websocketd in your own Go program" page.

## 7. FAQ candidates, ranked by how often independent users hit the same wall

1. Why doesn't my script send output in real time? → per-language buffering
   (Python needs `-u`/`flush()`, C needs `setbuf`, Ruby needs
   `STDOUT.sync=true`, PHP needs explicit flush + `<?php` not `<?`). **The
   single highest-leverage page in the whole manual** — a large fraction of
   all 339 closed issues trace back to this one gap.
2. How do I share/broadcast data across connections? → you can't, one
   process per connection by design; document the real workarounds
   (external message bus, FIFO/file coordination, a long-lived backend
   process the per-connection scripts fan out to).
3. Does this support authentication? → no, front it with a proxy.
4. How do I pass URL/query-string data to my script? → CGI env vars,
   worked example per mode.
5. Why did my connection drop after N minutes? → `--pingms`/keepalive,
   recommend non-zero.
6. Is this production-ready? Isn't a process per connection wasteful? →
   answer directly in the manual instead of re-litigating in issue threads
   every year.
7. How do I run a Windows `.bat`/`.cmd`/`.ps1` file? → platform notes (§8).
8. How do I keep a process running / run it exactly once? → shell-wrapper
   pattern (self-solved by a user in #317, worth documenting officially).
9. Can I run this behind nginx/Apache? → deployment guide.
10. Why can't I use `screen`/`watch`/`docker -it`? → no pty, permanent
    limitation.

## 8. Cookbook / deployment content (issue #467 explicitly asks for this)

- Reverse proxy fronting: nginx, Apache (`mod_proxy_wstunnel`), HAProxy —
  real user-contributed configs exist in the wiki (#109, #248, #399) and
  are worth recovering and re-testing before the wiki is retired.
- systemd unit file — asked for repeatedly (#222, #329), **never actually
  produced**; genuinely new content, not a port from anywhere.
- Docker — works, but no pty (see §3); document the limitation plus the
  working non-interactive pattern.
- Kubernetes — issue #467 names this explicitly; no prior art in the issue
  history at all, plan a minimal Deployment+Service+websocket-aware-Ingress
  example from scratch.
- Sharing a port with another web server — always "reverse proxy, route by
  path" (#165, #299); point at the reverse-proxy guide rather than
  re-answering.
- Exposing to the internet — a short checklist for `--address` binding vs.
  router port-forwarding vs. cloud security groups (#297, #349); not
  websocketd-specific, but asked often enough to earn a page.
- `--dir`/`--cgidir` routing — flagship cookbook page (chronic confusion:
  #102, #192, #233, #245, #246, #247, #294, #310, #353, #391).

## 9. Platform notes

- Windows: no SIGINT/SIGTERM support (Go platform limitation, permanent —
  #298, #362); `.bat`/`.cmd`/`.ps1` CGI execution needs direct executable
  associations, shebangs are ignored (#384, #454); `PATH` lookup can
  resolve the wrong binary for aliased commands like `nodejs`/`node`
  (#372).
- Build from source: already solved by Go modules (`go build`) — a short,
  current page retires a whole historical category of pre-modules
  GOPATH/Makefile confusion (#76, #175, #176, #225, #295, #328, #124,
  #434) in one shot.

## 10. Real-world examples (candidate content, verify before publishing)

codebender.cc (#36), a cable-modem monitoring dashboard (#202),
industrial/IoT sensor dashboards (#196, #420), a Raspberry-Pi robotics
counter (#211), gphoto2 photo-dump tooling (#191), a terminal-in-browser
game (#443). **Ask/verify with each project before naming them publicly**
— these were mentioned in old support issues, not offered as case studies.

## 11. The tutorial itself needs a rewrite, not just new surrounding pages

Issue #438 (2025) is a detailed, good-faith walkthrough of everything
confusing about the *current* Ten-minute tutorial, written by someone who
tried hard to follow it and hit the `file://`-origin-rejection trap, the
flags-must-come-before-the-command trap, and undocumented
`--staticdir`/`--devconsole` mutual exclusivity, all in one sitting. Read
it directly when drafting the new tutorial — it's close to a ready-made
punch list.

---

**Counts:** ~90 documentable items from commit history (12 categories),
~120 issue-derived items (14 categories), 34 current flags/behaviors
catalogued, 9 confirmed documentation-drift items.
