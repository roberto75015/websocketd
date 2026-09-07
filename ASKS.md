# Asks

Joe's own requests, in priority order. These outrank anything an agent
finds or invents for itself — one agent must always be working the top
open item.

<!-- Status: [ ] open, [~] in progress, [x] done, [-] dropped -->

## Open

## Done

- [x] **A6** Enforce the writing-style rules you gave, everywhere, with a check
  - 2026-09-07, merged as f476614
  Your instruction, given twice this session, had been applied to the docs
  at the time and never written down, so the homepage rebuilt hours earlier
  reproduced it with four `&mdash;` entities. `docsite/STYLE.md` gains rules
  2.11 (all four em-dash spellings, with the reason) and 2.12 (the wider
  list), in the same shape as 2.7-2.10. 50 dashes across 13 files rewritten
  rather than re-punctuated: 24 were one construction, a "See also" bullet
  where a dash separated a link from its gloss, and seven pages did that
  while thirty-four already made the link the subject of a sentence. Guard
  in `docs_voice_test.go`, proven three ways: it fails on an untouched copy
  of the base tree naming all 50, it still bites when each of the four
  spellings is injected into the fixed tree, and it refuses to pass when it
  cannot reach the docs at all.

  Two judgements to overrule if you disagree: the wider word list ships in
  STYLE.md but deliberately NOT in the test, because "load-bearing" appears
  three times in this repo in its correct technical sense and a checker that
  reddened the build on it would be switched off within a week. And 2.12
  ships with zero current instances, labelled forward-looking rather than
  dressed up as a measurement. Blockquotes are exempt, which is what lets
  STYLE.md print its own wrong examples and is also an evasion route.

- [x] **A3** Website redesign, following the console's new aesthetic
  - 2026-09-07, merged as 380a043
  `website/index.html` and `home.css` rebuilt against `docs-archive/REDESIGN_PLAN.md`,
  which a research round wrote first. Net -640 lines; the page drops from
  6,829px to 2,209px at 1440x900 and takes `home.js`, `prism.*`,
  `bluebg.jpg` and `icons/` with it. It links the docs theme's
  `blueprint.css` and `fonts.css` rather than copying them, so `home.css`
  holds no hex colour and no `:root` at all -- verified by computed
  padding (28.96px at 1440, 24.64px at 390) being blueprint's own
  `lh - cap x f` calc re-running, not a number typed twice. Gated in a
  real browser across five viewports with `prefers-color-scheme` emulated
  explicitly: 21/21 links 200, `scrollWidth` exactly 390 at phone width,
  zero elements escaping the viewport, both CTAs inside the fold
  everywhere, and the identical assertions failing every clause against
  the old page as a control. The dead Universal Analytics tag
  UA-38494812-1 is gone. Also widened the Pages link check to cover the
  homepage, a gap proven real: with a broken `/docs/` link injected, the
  old docs-only recipe exits 0.

  Left for Joe, not blockers: the spec claimed the hero command matches
  the tutorial's step one and it does not (the tutorial uses
  `--sameorigin`); and the demo video's `poster` is a light screenshot
  that sits on a dark page in dark mode, which cannot be media-queried
  without JS the spec forbids. Not proven: production, which still 404s
  on `/docs/` until someone pushes.

- [x] **A5** Rewrite the docs site from scratch on Diataxis (issue #467)
  - 2026-09-06
  Not a patch of the previous site: `docsite/content/` replaced entirely
  against two binding specs written first, `docsite/STYLE.md` (voice, the
  four modes, and a hard rule against writing about the documentation
  itself) and `docsite/PLAN.md` (the map, and the single objective each
  page must deliver). 44 pages across start/, how-to/ (languages, deploy,
  patterns), reference/, understanding/, faq/ and changelog/; guide/,
  cookbook/, platform-notes/, security/ and examples/ are gone. Six
  parallel workers wrote the sections, then one reader went through every
  page in sequence to unify the voice. Facts were carried forward from the
  old pages and re-verified against source; prose was written fresh.

  Writing against source rather than against the old docs is what found
  the real problems: two CI jobs red (docsdrift, and a link check that
  could never pass), llms-full.txt never ordered by the site map, section
  indexes sorting alphabetically with no descriptions, three false
  statements in --help (--closems, --passenv, --devconsole), and
  --sslca silently serving plain HTTP with no client verification, filed
  as issue #477 rather than fixed because the fix is a behaviour change.

  Still open, out of agent reach: cutting an actual 0.5.0 release. The
  install page now sends readers to the releases page rather than naming
  archives that do not exist, so nothing is broken meanwhile, but the
  newest tag is still v0.4.1. Not independently verified: TLS/mutual-TLS
  claims (source-read only, no handshake driven) and Windows claims
  (source-read only, no Windows machine).


- [x] **A4** Full manual and cookbook (GitHub issue #467) — 2026-09-06,
  QA pass 2026-09-06
  Hugo site under `docsite/` (Engineering Blueprint theme, baseline-grid
  typography), every page in `docs-archive/MANUAL_PLAN.md`'s site map with real
  content from `docs-archive/MANUAL_INVENTORY.md`, the flag-doc generator
  (`tools/gendocs`) single-sourcing the man page and `reference/cli-flags`
  with a CI drift check proven to actually fail on a hand-edited man
  page, `/llms.txt` + `/llms-full.txt`, the docsite build in
  `.github/workflows/pages.yml` + a lychee link-check job (proven to fail on an injected broken link),
  the cheap doc-drift fixes from `docs-archive/MANUAL_INVENTORY.md` §6, the wiki
  retired (unlinked from README/website/examples, not touched itself),
  and a final Fable 5.1 copy-edit pass over all prose (verified
  byte-identical code/links/facts, then five real factual issues it
  correctly flagged rather than silently "fixing" were investigated
  against source and fixed). Site now serves at `websocketd.com/docs`
  (not a subdomain — GitHub Pages allows one custom domain per repo),
  which also surfaced and fixed a real bug: Hugo's `relURL`/`absURL`
  don't prepend baseURL's path segment to an input already starting
  with `/`, breaking every nav/footer/CSS/font link in the theme.

  A follow-up adversarial QA pass (three parallel tracks, each verified
  independently before merging) found and fixed real problems, not just
  polish: **the theme had never actually rendered** — a CSS comment in
  `blueprint.css` (`--pt-*/--pb-*`) closed early and silently deleted
  the entire `:root` design-token block, so the deployed site served
  unstyled Times in every browser since it went live; fixing it also
  surfaced 404ing self-hosted fonts under the `/docs` prefix, unstyled
  markdown images/tables, mermaid diagrams cropped to a sliver by
  `text-box-trim`, a header height 6px off the baseline grid, a
  2.23:1-contrast CTA in dark mode, and an unopenable (keyboard-wise)
  mobile nav drawer — all confirmed with real headless-Chrome
  screenshots, before and after. A reference-accuracy pass found
  `--sameorigin` silently rejecting every upgrade behind a typical
  TLS-terminating reverse proxy (the cookbook recipe recommended that
  exact broken combination), `--cgidir` not sharing the WebSocket CGI
  environment contract as documented, a second mermaid diagram
  (process-lifecycle) that also never rendered, and the flag-doc
  generator's `--passenv` default being OS-dependent (so the docsdrift
  CI job could never pass on Linux after being generated on macOS). A
  new-user pass walked the tutorial cold against a real binary and
  fixed the hostname-vs-localhost mismatch, the `--devconsole`/
  `--staticdir` transition, and a hardcoded port in the example client
  JS — all issues raised in GitHub issue #438. `start/install.md` is
  rewritten to assume 0.5.0 is shipped, pointing plainly at the GitHub
  releases page with no version-hedging language. Full history in
  DIARY.md and `CHANGES`.

  Open follow-ups, out of repo/agent reach: cutting an actual 0.5.0
  release so `start/install`'s download links resolve to real assets.
  Not independently re-verified in this pass: TLS/mutual-TLS claims
  (source-read only, no real handshake driven) and Windows-specific
  claims (source-read only, no Windows machine available).

- [x] **A1** Rebuild the dev console (GitHub issue #466) — 2026-09-03
  Split-inspector layout, Option D palette, strict CSP/ETag/{{addr}}
  removal, qa/browser chromedp suite passing. Full history in DIARY.md
  (2026-09-02, 2026-09-03 entries) and CHANGES.

- [x] **A2** Screenshots and demo recording of the new console — 2026-09-03
  website/img/console/*, generated by qa/capture/capture-console.sh
  against the real console. Re-run the script whenever the console
  changes.

<!-- Move completed items here with a date and a one-line resolution. -->
