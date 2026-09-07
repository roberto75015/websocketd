# Docs tooling recommendations

> **Archived 2026-09-07. Superseded — do not act on this document.**
>
> Everything below shipped *except its lead recommendation*. Hugo, the
> `tools/gendocs` flag generator with a CI drift check, Mermaid via
> shortcode, `llms.txt`/`llms-full.txt`, and the lychee link check are all
> live. **The site location is wrong:** this document recommends the
> `docs.websocketd.com` subdomain and rejects the `websocketd.com/docs`
> path; the project shipped the path, because GitHub Pages allows one
> custom-domain deployment per repository. The DNS `CNAME` action item
> below is therefore void, and so is the separate
> `.github/workflows/docs.yml` — the docs build lives in `pages.yml`
> alongside the homepage. Current truth: `docsite/PLAN.md`,
> `docsite/STYLE.md`, `docsite/hugo.toml`.

Concrete technical choices for building `docs-archive/MANUAL_PLAN.md`'s site: static
site generator, hosting/location, single-sourcing between `--help`/man
page/website, diagrams, and screenshots/recordings.

---

## Site location: subdomain, in-repo

**Recommendation: `docs.websocketd.com`, built from a `docsite/` directory
in this same repo, with its own GitHub Actions workflow.**

Considered three shapes:

| Option | Verdict |
|---|---|
| Path (`websocketd.com/docs`) | Couples the docs build to the homepage's existing `pages.yml` — every docs change risks the homepage deploy and vice versa. Rejected. |
| Separate repo (`websocketd-docs`, litestream/k6 pattern) | Justified at a scale websocketd doesn't have yet — those projects' docs churn independently of the binary's release cadence. Here, a new flag and its reference-page update should land in the *same* PR, which argues for one repo. Rejected for now; revisit if the docs site outgrows this. |
| **Subdomain, in-repo (`docsite/`)** | **Recommended.** Own deploy pipeline (so a broken docs build never takes down the homepage), own `CNAME` file, but changes to flags/behavior and their docs stay atomic in one PR. |

**Action only you can take:** this needs one DNS `CNAME` record
(`docs.websocketd.com` → GitHub Pages) added wherever `websocketd.com`'s
DNS is managed — outside repo/tool reach, flagging it now so it's not a
surprise when the site is ready to go live.

## Static site generator: Hugo

**Recommendation: Hugo**, with a custom theme (not a stock one) built
around whichever direction comes out of the item 5 visual mockups.

Why, against the alternatives actually considered:

- **Hugo** — single Go binary, no Node/Python toolchain to maintain,
  matches this project's own "small binary, minimal dependencies" ethos,
  and has direct precedent at a comparable scale: litestream.io runs
  Hugo+Doks. GitHub Actions setup is a one-line `peaceiris/actions-hugo`
  step, no `node_modules` cache to manage. **Chosen.**
- MkDocs Material — best out-of-box search and navigation UX of anything
  considered, but pulls in a Python toolchain this repo has none of
  today. Worth reconsidering only if the team wants zero custom
  templating effort and doesn't mind the dependency.
- Zola — same single-binary appeal as Hugo, Rust instead of Go; smaller
  theme/plugin ecosystem. No specific advantage over Hugo for this
  project.
- Docusaurus / Astro Starlight — most flexible for bespoke interactive
  components (React/MDX), but a full Node toolchain for a site this size
  is more machinery than the content needs. **Fallback option** if the
  chosen visual design (item 5) turns out to need interactive components
  Hugo templates can't express cleanly.

Deploy: `.github/workflows/docs.yml`, triggered on push to `main` touching
`docsite/**`, pinned Hugo version (matching the existing CI convention of
pinning linter versions with a comment explaining the pin), deploying to
a separate GitHub Pages target — mirrors the structure of the existing
`.github/workflows/pages.yml` for the homepage rather than replacing it.

## Single-sourcing: `--help`, man page, and the CLI reference page

**Can flag docs be extracted dynamically? Practically, no — extract them
at build time, not view time.** Running the binary client-side (e.g. via
WASM) to regenerate `--help` output live on every docs-site page load
would be real engineering effort spent solving a problem a build step
already solves for free. The right answer is generation, checked into
git, drift-checked in CI.

**Recommendation:** add a small generator (e.g. `tools/gendocs/main.go`)
that walks the existing `flag.FlagSet` via `VisitAll(func(f *flag.Flag))`
— every flag's name, default, and usage string, live from the same
in-memory definitions `--help` itself reads — and emits:
1. `release/websocketd.man`'s flag table section.
2. `docsite/content/reference/cli-flags.md`.

This is the same pattern `cobra`'s `doc.GenManTree`/`GenMarkdownTree` use
(confirmed as the mainstream approach across the Go CLI ecosystem, and
independently mirrored by ripgrep's `build.rs`-driven man-page generation
in Rust) — websocketd doesn't use cobra, but `flag.FlagSet.VisitAll`
exposes exactly the same data, so the pattern ports directly without
adding a framework dependency.

**Commit the generated output**, don't generate it only at build time —
then add a CI check that re-runs the generator and diffs against the
committed files, failing the build on drift. This is the same
integrity pattern already used for the dev console's CSP hashes
(computed from actual served content, never hand-copied) — applied here
to close the exact gap the 2026-08-28 scorecard flagged ("help text
duplicates the flag table with no parity check"), structurally instead of
by discipline.

**Environment variables:** `env.go` doesn't currently expose its variable
list as a walkable structure the way `flag.FlagSet` does. Either (a)
introduce a small ordered table in `env.go` the generator can also read
(same pattern, slightly more refactoring), or (b) keep the env var
reference hand-written and add a lighter parity test — grep `env.go` for
every `os.Setenv`/map-key write and assert each has a corresponding
entry in the reference doc, the same hand-rolled-checker philosophy
already used for the console's `checkHTMLWellformed` (no new dependency
for one check). Recommend starting with (b); it's cheap and catches the
same class of drift without a refactor.

**CHANGES:** already exists and is user-facing — render it as
`docsite/content/changelog` directly (a Hugo shortcode or simple copy
step at build time), don't hand-write a second "what's new" page.

## Diagrams: Mermaid

**Recommendation: Mermaid**, embedded via Hugo shortcode. Text-based
(diffable in PRs, editable by an agent without image tools), renders
natively in GitHub markdown too (so the same diagram source works in
`docs-archive/MANUAL_PLAN.md`-style planning docs and the published site), and needs
no new binary dependency. Concrete diagrams worth building:
- Sequence diagram: browser ↔ websocketd ↔ spawned process, showing
  stdin/stdout as the wire (for `guide/the-process-model`).
- State diagram: the process teardown ladder (stdin close → SIGINT →
  SIGTERM → SIGKILL, with the timing) (for `guide/process-lifecycle`).
- Flow diagram: a request through the CGI env var contract (for
  `guide/cgi-environment`).

## Screenshots, recordings, and GIFs

**Dev console:** already solved. Reuse and extend
`qa/capture/capture-console.sh`, which already produces PNG/MP4 via
scripted headless Chrome. Same convention already established in
`CLAUDE.md`'s asset-locations rule: store under `website/img/console/`,
reference from both the homepage and the new `reference/dev-console`
page rather than duplicating assets.

**Terminal/CLI recordings for the tutorial:** recommend
[`vhs`](https://github.com/charmbracelet/vhs) (Go-based, scriptable
terminal-GIF/MP4 recorder driven by a small checked-in `.tape` text
file) — deterministic and diffable, the same "reproducible from a
checked-in script, not a manually captured artifact" pattern already
established for the console. Alternative worth considering specifically
for the tutorial page: **asciinema**, which embeds as a replayable
terminal session rather than a video — a visitor can copy commands
directly out of it, which they can't from a GIF/MP4. Recommendation:
`vhs` for short illustrative clips (e.g. the homepage teaser), asciinema
for the tutorial's actual step-by-step terminal walkthrough where
copy-paste matters more than visual polish.

## LLM-readable output

Generate `docsite/static/llms.txt` (curated index, hand-written) and
`docsite/static/llms-full.txt` (every content page concatenated in
site-map order) as a Hugo build step — a small template/shortcode that
walks the content collection, not a hand-maintained file. Same
single-source-of-truth rule as the CLI reference: one content tree, three
renderings (HTML pages, `llms.txt`, `llms-full.txt`).

## Docs CI

Two checks, mirroring how this repo already treats code:
1. **Drift check** — re-run the flag-doc generator, fail if output
   differs from committed files (see single-sourcing above).
2. **Link check** — a link-checker action (e.g. `lycheeverse/lychee`)
   over the built site, catching dead internal links before they ship —
   same "don't let a green build hide a real break" discipline
   `CLAUDE.md` already applies to `qa/integration`.

## Summary of open action items this plan surfaces (not docs work itself)

- A real 0.5.0 release needs to be cut before `start/install`'s download
  links can be honest (`MANUAL_INVENTORY.md` §6).
- One DNS `CNAME` record for `docs.websocketd.com`, outside repo reach.
- Filed already: issue #476 (`--staticdir` dotfile/listing gap), should
  land before or alongside the security page ships, or be documented as
  an explicit known caveat if it doesn't.
