# External research for websocketd docs rewrite (issue #467)

> **Archived 2026-09-07. Spent — every recommendation in it shipped.**
>
> Track 2's single-sourcing recommendation is live: one flag definition
> drives `--help`, the man page and the reference, via
> `internal/cliflags` + `tools/gendocs`, with a CI drift check
> (`.github/workflows/docsdrift.yml`). Hugo shipped. Retiring the wiki,
> which Track 1 argued for, shipped.
>
> It is kept, rather than deleted with its two sibling research files,
> because Track 1 is the only local record of what the wiki contained and
> why it was retired — the wiki is a separate repository that was never
> mirrored into this one. Its page-by-page notes describe pages nobody is
> expected to read again, and the `scratchpad/` clone path it names was
> never committed anywhere.
>
> Nothing here describes current behaviour. For that, read `docsite/content/`.

## Track 1 — GitHub wiki inventory

Cloned read-only from `https://github.com/joewalnes/websocketd.wiki.git` into
`scratchpad/docs-research/wiki-clone/` (not committed anywhere, not part of
the main repo).

**44 markdown pages + 2 images.** Roughly half are Spanish translations of
the other half (`Bash.md`/`Bash-(ES).md`, `FAQ.md`/`Preguntas-frecuentes.md`,
etc.) — translation coverage is inconsistent (e.g. Rust, C99, Program-Flow,
"Run a CGI script" have no Spanish counterpart).

Linked from the main repo: `README.md` links to
`wiki/Download-and-install`, `wiki/Environment-variables`,
`wiki/Developer-console`, and a bare link to `wiki` itself ("More
documentation in the user manual"). `website/index.html`'s nav "Docs" link
also points straight at the wiki root. So today the wiki *is* the manual —
which is exactly the gap issue #467 is naming.

**Last-edit dates per page** (full list gathered via `git log -1 --date`
per file) — oldest is 2013-02-22 (`Environment-variables.md`), newest is
2022-11-20 (`Download-and-install.md`, a cosmetic edit). No page has been
touched since 2022. Most substantive pages (tutorials, command-line
reference, environment variables, program flow) are 2013-2019.

**Staleness verdict: comprehensively stale, spot-checked directly against
current code:**

- `Command-line-options.md` is a pasted `--help` dump for **websocketd
  0.3.0 (go1.9.2)**. Missing entirely: `--unixsocket`, `--maxframesize`,
  `--anyorigin`, `--pingms`, `--socketmode`, `--passstderr`, `--sslca`/mTLS
  flags, `--header-http`/`--header-ws` split (partially present),
  `--closems` default text disagrees with current default. States
  `--maxforks` default is "0/unlimited" — current default is 1024. (This
  independently reproduces a finding the 2026-08-28 codebase scorecard
  already made about the *man page* — the wiki has the same drift, worse,
  since it's 7+ years older.)
- `Embedding-in-go-apps.md` — the example **does not compile**: calls
  `os.ClearEnv()` (should be `os.Clearenv()`), uses `time.Now()` and `*port`
  with no imports/declarations shown, and calls
  `wsd.NewWebsocketdServer(config, logScope, MAXFORKS)` — worth verifying
  against the current `libwebsocketd` embedding API before reusing any of
  this content in the new docs (flagging for whoever builds the new
  "embedding" page: the constructor signature needs re-checking against
  current `libwebsocketd` source, not copied from here).
- `Program-Flow.md` documents a known bug ("QUERY_STRING is always blank")
  as current behavior, with a client-side JS workaround. Worth a fresh spot
  check but this reads like a since-fixed issue being preserved as if it's
  still true.
- `FAQ.md` is written in first-person forum-answer voice ("I'm not the
  developer, but AFAIK...", "I connected to the websocket, and put my phone
  in airplane mode...") — informative in spots but reads as a preserved
  mailing-list thread, not authored reference material. Not reusable
  verbatim; the underlying questions (scaling/concurrency limits, latency,
  multi-line messages, process signals on disconnect, idle timeouts) are
  legitimately good FAQ material and should be answered fresh against
  current behavior (idle timeout answer in particular has changed given
  the `--pingms`/read-deadline work referenced in DIARY.md).
- `Environment-variables.md` is the one page that holds up best: a clean,
  RFC-3875-cited, one-section-per-variable reference. Structure is worth
  keeping; content needs a pass against the current `env.go` variable list
  (at minimum check for anything added since 2013 — `UNIQUE_ID`,
  `REMOTE_PORT` etc. are marked "non-standard" additions already, there may
  be more since).
- Nginx/Apache reverse-proxy pages (`Websocketd-behind-Nginx.md`,
  `-Apache-(2.4.x).md`) are config-recipe style — closest thing the wiki
  has to the "cookbook" the issue asks for, and Nginx/Apache in front of a
  WebSocket backend hasn't changed much structurally, so these are
  probably the most salvageable pages in the whole wiki content-wise, pending
  someone actually testing the configs against a current Nginx/Apache.
- Per-language example pages (Bash, PHP, Perl, Python, Ruby, Node.js, Dart,
  Rust, C99, C++) are short, single-file snippets. Reasonable raw material
  for an "examples" section but should be diffed against what's already in
  `examples/` in the main repo — some of this may be pure duplication.
- `_Footer.md`: "The websocketd user guide is a publicly editable wiki.
  Please contribute!" — i.e. this is a public-write wiki with no review
  gate, on a document that (per the above) is the *only* linked manual.
  That's a real risk surface (drive-by edits, spam, or just well-meaning
  but wrong contributions with no CI/review), independent of the
  staleness problem.

**Bottom line for whoever writes the placement recommendation (item 2):**
this isn't a wiki that needs light editing — it's fully superseded content
in an unreviewed, publicly-writable format that is currently the *primary*
documentation entry point from both README and the website nav. Retiring
it (redirecting its URLs, or at minimum unlinking it once equivalents exist
on the new site) looks well justified, not just a style preference. A few
individual pages (env vars structure, Nginx/Apache recipes, per-language
snippets) have salvageable bones worth hand-porting rather than
regenerating from scratch.

---

## Track 2 — comparable small CLI tool documentation sites

Picked tools genuinely close to websocketd's shape: small, single-binary,
technical-developer audience — deliberately skipped huge
framework-scale docs (e.g. skipped fully investigating Caddy, which has a
much bigger config-language surface than websocketd's flag list).

### ripgrep (BurntSushi/ripgrep) — no dedicated docs site

Repo root has no docs/website tooling at all: documentation is
`README.md` + `GUIDE.md` (long-form usage guide) + `FAQ.md`, all plain
markdown, all in-repo. No subdomain, no SSG, no separate repo. The man
page is generated at build time (repo has a `build.rs` build script, the
standard Rust mechanism for shelling out to `clap`'s man-page/completion
generators at compile time) — so CLI help text and the man page trace back
to one flag-definition source, not two hand-maintained copies.

### fzf (junegunn/fzf) — no dedicated docs site, cookbook-as-markdown

Also no separate site: `README.md`, `ADVANCED.md` (a genuine cookbook —
numbered real-world recipes, e.g. multi-select pipelines, preview-window
tricks), `BUILD.md`, `man/man1/` for hand-authored man pages. This is the
closest thing to a scale-appropriate precedent for websocketd's
"cookbook" ask: `ADVANCED.md`'s format (short recipe title, the command,
2-3 sentences of why) is directly liftable as a pattern.

### litestream (benbjohnson/litestream.io) — dedicated site, Hugo + Doks

Docs site is its **own separate repo** (`benbjohnson/litestream.io`, not
inside the main `litestream` repo), built with **Hugo** using the **Doks**
theme (confirmed via repo layout: `hugo.yaml`, `archetypes/`, `layouts/`,
`content/`, plus a Node toolchain for linting/CSS — Doks is Tailwind-based
on top of Hugo). Versioned docs (v0.5.x / v0.3.x) live as separate content
trees. This is a reasonable model for "small tool, but the maintainer
wanted a real marketing-plus-docs site distinct from the README."

### k6 (grafana/k6-docs) — dedicated subdomain, docs.k6.io, separate repo

`docs.k6.io` is a separate repo (`grafana/k6-docs`) from the `grafana/k6`
binary's repo, with content under `docs/sources/<version>/`, so the URL
structure is generated from the folder tree (this is a heavier,
Grafana-ecosystem-scale setup than websocketd needs, but two details are
worth stealing regardless of scale): it runs **Vale** (a prose linter,
config at `.vale.ini`) as a CI check on the docs content itself — catching
style/terminology drift the way `gofmt`/`vet` catch code drift — and it
ships an `AGENTS.md` at the repo root, i.e. the docs repo is written to be
edited by AI agents as a first-class contributor, not just humans.

### Single-sourcing `--help` / man page / website: current 2026 patterns

The general pattern across Go CLI ecosystems (cobra, kong, urfave/cli) is:
**one struct/table of flag definitions drives every surface.** Concretely,
`spf13/cobra`'s `doc` subpackage (`cobra/doc`) ships
`GenMarkdownTree`/`GenManTree`/`GenReSTTree`/`GenYamlTree` functions that
walk a already-defined command tree and emit man pages and markdown
reference pages directly from each flag's registered name, shorthand,
default, and usage string — so the reference docs cannot drift from
`--help`, because they're rendered from the exact same in-memory
definitions `--help` itself reads. `kong` has an equivalent
(`kong.Vars`+doc generation via reflection over struct tags). websocketd
uses the stdlib `flag` package directly rather than a framework, but the
*pattern* is what's portable: `flag.FlagSet` already exposes exactly this
data via `VisitAll(func(f *flag.Flag))` — name, default value, usage
string per flag, all live. **This directly matches an already-flagged
problem in this repo**: the 2026-08-28 scorecard noted "help text
duplicates the flag table with no parity check" as a specific drift risk
in `help.go`/`config.go` — a `VisitAll`-driven generator (or even just a
parity test asserting every `flag.` registration has a matching line in
the hand-written usage text) would close exactly that gap, and the same
generated table could become the CLI-reference *content* on the new docs
site and/or the man page body, rather than a third hand-copied version.

Ripgrep's `build.rs` man-page generation is the same idea in the Rust
ecosystem (`clap`'s `Command` definition → man page at build time), cited
above as independent confirmation this is a mainstream, not exotic,
pattern in 2026.
