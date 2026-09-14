# Docs site plan

The actual shape of the new manual (docs.websocketd.com, per
`MANUAL_TOOLING.md`'s recommendation). Built from `MANUAL_INVENTORY.md`
(what needs documenting) and `MANUAL_PLACEMENT.md` (what belongs here vs.
elsewhere). Organized around what a reader is trying to *do*, not around
the code's structure — a flag reference is one page among many, not the
spine of the site.

---

## Who reads this site

Five people, in the order they actually show up:

1. **The evaluator.** Never used it. Wants to know what it is and whether
   it's worth ten more minutes. Bounces off the homepage if unconvinced;
   the docs site meets them at the tutorial.
2. **The newcomer.** Decided to try it. Following the tutorial right now,
   in a terminal, with a browser tab open next to it. Every trap in
   `MANUAL_INVENTORY.md` §11 is aimed at this person specifically.
3. **The implementer.** Writing a real backend script. Needs the CGI/env
   var contract, the framing rules, and per-language buffering guidance —
   precisely, not conversationally.
4. **The operator.** Deploying this somewhere real. Needs the flag
   reference, deployment recipes, and the security posture stated
   plainly enough to make decisions from.
5. **The stuck person.** Something isn't working. Needs the FAQ and
   cookbook to be findable in under thirty seconds, or they file issue
   #340-something and the cycle continues.

(A sixth person — someone wanting to modify websocketd itself — is
explicitly *not* served here; per `MANUAL_PLACEMENT.md` they're routed to
the GitHub README instead.)

## Design principle: the five FAQ answers are load-bearing, not buried

`MANUAL_INVENTORY.md` §7's ranked FAQ list isn't FAQ material by accident
— the issues research found that a handful of these questions
(buffering, no-broadcast, no-auth, the `file://` origin trap, Windows
signals) plausibly account for a large fraction of the entire 339-issue
history. This site plan puts the four highest-value ones **on the page a
newcomer reads first**, not three clicks deep in an FAQ they don't know to
visit yet.

---

## Site map

```
docs.websocketd.com/
├── (home)                     — thin: pitch + big "Start here" button
├── start/
│   ├── what-is-this            — the process model, up front (see below)
│   ├── install                 — per-platform, real working download links
│   └── tutorial                — the rewritten 10-minute tutorial
├── guide/
│   ├── the-process-model        — one process per connection, no broadcast
│   ├── message-framing          — newline text vs --binary, precisely
│   ├── streaming-output         — the buffering page (highest-leverage single page)
│   ├── cgi-environment           — full env var contract + trust boundaries
│   ├── origins-and-security      — origin policy, TLS/mTLS, "no auth" answer
│   └── process-lifecycle         — signals, teardown ladder, Windows caveat
├── reference/
│   ├── cli-flags                 — generated from --help (MANUAL_TOOLING.md)
│   ├── environment-variables      — generated/parity-checked from env.go
│   ├── exit-codes
│   └── dev-console                — DOM/CSP contract, screenshots, how to use it
├── cookbook/
│   ├── languages-and-platforms/     — recipes keyed to a specific language or OS
│   │   ├── python-unbuffered-output
│   │   ├── ruby-unbuffered-output
│   │   ├── node-stdin-patterns
│   │   ├── php-flushing
│   │   ├── c-unbuffered-output
│   │   └── windows-scripts             — running .bat/.cmd/.ps1, PATH lookup
│   ├── infrastructure-and-deployment/ — recipes keyed to where it runs
│   │   ├── nginx-reverse-proxy
│   │   ├── apache-reverse-proxy
│   │   ├── haproxy
│   │   ├── systemd-unit
│   │   ├── docker                      — with the no-pty caveat stated up front
│   │   ├── kubernetes
│   │   ├── sharing-a-port
│   │   └── exposing-to-the-internet
│   └── design-patterns/               — recipes keyed to a problem shape, not a tool
│       ├── sharing-state-across-connections   — message bus / FIFO / fan-out process
│       ├── auth-via-reverse-proxy
│       ├── run-once-vs-long-running            — the shell-wrapper pattern
│       └── passing-arguments-to-a-script       — query string vs env vars
├── platform-notes/
│   ├── windows
│   └── building-from-source
├── security/
│   └── (single page — the "one coherent pass" narrative + accepted limitations
│         + issue #476's staticdir gap, stated as a known caveat)
├── faq/
│   └── (the ranked list, each answer 2-4 sentences, linking into guide/cookbook
│        pages for the full version rather than duplicating them)
├── examples/
│   └── (real-world use cases, pending verification with each project — see
│        MANUAL_INVENTORY.md §10)
└── changelog                     — rendered from CHANGES, not duplicated
```

## Page-by-page content outline

### `start/what-is-this`
One paragraph pitch, then immediately: **"websocketd starts one instance
of your program per WebSocket connection. Programs share nothing with
each other — there's no built-in broadcast or shared state."** This is
FAQ #2 from the inventory, moved to the very first substantive page
instead of waiting for someone to hit the wall and go searching. Follow
with a 4-line ASCII/diagram sketch of the model (browser ↔ websocketd ↔
process, stdin/stdout as the wire).

### `start/install`
Per-platform (Homebrew for Mac, direct binary download for Linux/Windows,
`go build` for from-source). **Every download link must resolve to an
actual current release** — this page cannot ship until the version/release
drift in `MANUAL_INVENTORY.md` §6 is fixed; it's the one page where stale
content isn't just embarrassing, it's a broken product experience.

### `start/tutorial`
The rewritten 10-minute tutorial. Must not reproduce the traps issue #438
documented: avoid the `file://` origin-rejection trap (serve the test page
via `--staticdir`/`--devconsole` or explicitly warn about opening HTML
directly from disk), put flags before the command in every example (the
Go-`flag`-package ordering trap bites here worst of all, since it's a
newcomer's very first command), and use a language/example whose stdout is
unbuffered by default or explicitly shows the flush call — the first
example a newcomer runs should never be the one that silently no-ops due
to buffering.

### `guide/the-process-model`
Expands on `what-is-this` with the actual workaround patterns for people
who want cross-connection state: external message bus, file/FIFO
coordination, a single long-lived backend process the per-connection
scripts fan out to. Directly answers "why not add broadcast" preemptively.

### `guide/message-framing`
Newline-delimited text is the default and the whole story unless
`--binary` is set. Explicit statement of what `--binary` does and doesn't
change (message type flag, not framing) — this single paragraph would
have resolved issue #443.

### `guide/streaming-output`
The buffering page. One short section per language actually seen in
issues: Python (`-u` or `flush()`), C (`setbuf(stdout, NULL)`), Ruby
(`STDOUT.sync = true`), PHP (explicit `flush()`/`ob_flush()`, plus the
`<?` vs `<?php` short-tag trap), Node.js (readline/async stdin gotchas).
Cross-linked from the tutorial, the FAQ, and every per-language example.

### `guide/cgi-environment`
The full env var table from `MANUAL_INVENTORY.md` §2, plus the two worked
examples (plain WS script vs `--cgidir` script) that would have preempted
four separate issues about `QUERY_STRING`. Trust-boundary notes
(`SERVER_NAME`/`SERVER_PORT` Host-derived, httpoxy mitigation, `PATH`
passthrough) live here, with a pointer from `security/`.

### `guide/origins-and-security`
Origin policy default and the three flags, TLS/mTLS setup with a worked
`--sslca` example (the original 2026 feature request, #413, had a good
real scenario worth reusing), and the confident "no built-in
authentication — front it with a proxy" answer, stated once here and
linked from the FAQ rather than re-answered.

### `guide/process-lifecycle`
The teardown ladder, stated precisely (users have had to reverse-engineer
the exact timing). The SIGTERM/SIGINT handling requirement for long-running
processes (#71). The Windows caveat (no SIGINT/SIGTERM support) as a
forward pointer to `platform-notes/windows`, not repeated in full here.

### `reference/*`
Generated content — see `MANUAL_TOOLING.md`. These pages should look and
feel hand-written (good prose per flag, not a raw dump) but be
mechanically incapable of drifting from `--help`.

### `cookbook/*`
Format: short recipe title, the command/config, 2-3 sentences of why —
directly modeled on fzf's `ADVANCED.md`, which the external research
identified as the closest scale-appropriate precedent for what issue #467
calls a "cookbook." Three categories, chosen so a reader can find a recipe
by whichever thing they already know — their language, their infra, or
their problem — rather than hunting one flat list:

- **Languages & platforms** — "I'm writing this in Python / on Windows,
  what do I need to know." One recipe per language/OS.
- **Infrastructure & deployment** — "I'm running this somewhere, how do I
  wire it up." The reverse-proxy recipes (`nginx`, `apache`, `haproxy`)
  should be hand-ported from the wiki's existing pages and re-tested
  against current software before publishing, not trusted as-is.
  `kubernetes` has no prior art to inherit — build it fresh, keep it
  minimal (Deployment + Service + a websocket-aware Ingress annotation
  example).
- **Design patterns** — "I have a problem shape, not a tool in mind."
  This is where the no-broadcast workaround, the auth pattern, and the
  run-once-vs-long-running pattern live — recipes that answer a FAQ
  entry with a fuller worked example than the FAQ page itself carries.

### LLM-readable docs

Assume a large fraction of readers are an LLM (a coding agent looking up
a flag, a user pasting the manual into a chat). Two additions, following
the emerging `llms.txt` convention:
- `/llms.txt` at the site root — a short, curated index: what
  websocketd is, and a linked list of every page, grouped the same way
  as the site map above. This is the front door for an agent deciding
  what to fetch.
- `/llms-full.txt` — every page concatenated into one plain-text/markdown
  document, in site-map order, generated at build time (not
  hand-maintained — it's a render step, the same way the CLI reference is
  generated from `--help`). This is the "just give me everything" option
  for a model with room in its context window.

Both are static files Hugo can emit from the same content collection that
renders the HTML pages, so there's one source of truth, not a fourth copy
to keep in sync.

### `platform-notes/windows`
No SIGINT/SIGTERM (permanent Go platform limitation). `.bat`/`.cmd`/`.ps1`
need direct executable associations — shebangs are ignored. `PATH`
resolution can pick the wrong binary for aliased commands (`nodejs` vs
`node`). State all three as known, permanent facts, not bugs to watch for
a fix on.

### `security/`
One page. The "coherent hardening pass" narrative from
`MANUAL_INVENTORY.md` §5, the standing-defaults list (verbatim from
`CLAUDE.md`'s do-not-touch, since that list is the actual ground truth),
and issue #476 (`--staticdir` dotfile/listing gap) stated as a known,
tracked caveat rather than omitted.

### `faq/`
The ranked ten from `MANUAL_INVENTORY.md` §7. Each answer is short and
links into the guide/cookbook page that owns the full answer — the FAQ
page is a directory, not a duplicate manual.

### `examples/`
Hold until each project is confirmed willing to be named (see inventory
§10) — this page can ship later than the rest without blocking anything.

### `changelog`
Render `CHANGES` directly (single-sourced, not rewritten).

---

## Rollout priority

If this ships incrementally rather than all at once, sequence it by how
much existing pain it retires, per the issues research:

1. `guide/streaming-output` + `start/tutorial` fix — these two pages alone
   address the largest and second-most-damaging recurring problems
   (buffering, and the tutorial's own `file://`/flag-ordering traps).
2. `guide/the-process-model` + `faq/` — cheap to write, headline value,
   directly preempts the "is this abandoned/not production ready" pattern.
3. `reference/cli-flags` — once the generator exists (`MANUAL_TOOLING.md`),
   this page and the man page ship together and never drift again.
4. `cookbook/*` deployment recipes — highest total effort (each needs
   re-testing against current software), lower urgency since these
   questions get answered ad hoc in issues today rather than causing
   confusion about whether the tool works at all.
5. `examples/` — last, gated on external confirmation.

Retiring the wiki (`MANUAL_PLACEMENT.md`) should happen no later than step
2 — every day it stays linked as "the manual" is a day new visitors get
the 2013-era, partly-wrong version instead of whatever's shipped so far.
