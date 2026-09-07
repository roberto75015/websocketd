# websocketd.com homepage: redesign spec

> **Archived 2026-09-07.** The rebuild this specifies shipped as `380a043`
> (ASKS.md A3), and §6's verification procedure has been run by the gate.
> It leaves `website/` because `pages.yml` assembles the site with
> `cp -R website/. _site/`, so every file in that directory is served from
> the root of websocketd.com: this spec was one deploy away from being
> published at `https://websocketd.com/REDESIGN_PLAN.md`.
>
> **§0 is no longer a to-do list.** §0.4, drop the dead Universal Analytics
> tag `UA-38494812-1`, was carried out by the build. §0.1's ruling (name no
> version number and no archive filename) was followed, §0.3's copy shipped,
> and §0.6 was a statement of fact about assets rather than an action. Still
> open, and tracked in `ASKS.md` rather than here: cutting a 0.5.0 release,
> and pushing to origin, without which every `/docs/` link on the live site
> still 404s. Only §0.5 is genuinely unanswered: the footer's two
> `twitter.com` credits were kept as they were, pending Joe's call.
>
> §1 and §2 are why this is archived rather than deleted. They hold rendered
> measurements of the old homepage, and of three comparable project sites on
> 2026-09-06, which nobody can reproduce from this checkout. §3 is derivable
> from the docs theme's `blueprint.css`; §4 and §5 are better read as
> `website/index.html` itself.

Binding spec for the rebuild of `website/index.html` (ASKS.md **A3**),
written before the work, in the same spirit as `docsite/PLAN.md` and
`docsite/STYLE.md`. This is a research round's output. Nothing in
`website/` was changed to produce it.

**Scope.** The marketing homepage at `/` only. Not the docs site at
`/docs` (that is A5, landed). Marketing and orientation only — no
reference content, per `MANUAL_PLACEMENT.md`.

**How the measurements here were taken.** Every rendered claim comes from
headless Chrome 
(`/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`) driven by
chromedp with `Emulation.setDeviceMetricsOverride` (viewport emulation) and
`Page.captureScreenshot` with `captureBeyondViewport`, against a local
`python3 -m http.server` on a port allocated from the OS — never a
`file://` URL. Link status codes come from `curl`. Where a claim is read
out of source rather than observed rendered, it says so.

> **Instrument warning, learned the hard way in this round.** The obvious
> recipe — `Google Chrome --headless --screenshot --window-size=390,844` —
> produced a screenshot of the current homepage that appeared to overflow
> its viewport badly: logo clipped, nav clipped, hero text running off the
> right edge. That was the instrument, not the page. Re-measured through
> chromedp's viewport emulation, `document.documentElement.scrollWidth` at
> a 390px viewport is exactly **390** and nothing overflows the document.
> The old-headless `--window-size` flag does not reliably establish the
> layout viewport before first paint. **Do not use it for this page.** The
> procedure in §6 is the one to run.

---

## 0. Actions only Joe can take

Listed first because the build round should not block on any of them, and
two of them are already settled.

1. **The download CTA — settled, not a blocker.** Joe's ruling: *"That's
   fine for now. Make the site look perfect and we'll fix up the release
   downloads last."* What the CTA can honestly point at **today**,
   measured against the GitHub API on 2026-09-06:
   - Newest tag and newest release: **v0.4.1**, published **2021-01-25**.
     No 0.5.0 exists, draft or otherwise.
   - `https://github.com/joewalnes/websocketd/releases/latest` → **200**,
     resolving to `/releases/tag/v0.4.1`.
   - All seven `v0.4.1` archive URLs currently hard-coded into
     `website/index.html` → **200**. They work; they are just five years
     old and there is no `darwin_arm64` asset.

   **Therefore:** the rebuilt page names no archive and no version number.
   It links to the docs install page and to the releases page, both of
   which stay correct the day 0.5.0 is cut. Cutting 0.5.0 is a known
   follow-up on Joe's plate and is explicitly *not* a prerequisite for
   this build round.

2. **Push to origin — the only thing that makes the page's own links
   work in production.** `https://websocketd.com/docs/` returns **404**
   right now, and the live homepage is a revision from **2026-08-20**
   (18,989 bytes; the repo's is 20,408 and contains a dev-console section
   the live page does not). The fleet merges to local `main` and never
   pushes, so the docs site A5 built has never been deployed. Every
   `/docs/` deep link this spec specifies is verified against a locally
   assembled `_site` (§6), which is byte-for-byte the tree
   `.github/workflows/pages.yml` publishes. They will 404 in production
   until Joe pushes. **This is the one item that gates the page actually
   working for a real visitor.**

3. **Hero copy approval.** §4 writes the headline and subhead out in
   full. They are the one part of this spec that is a matter of taste
   rather than measurement. Joe should read those four lines and say yes
   or rewrite them; the rest of the build does not depend on which.

4. **Analytics decision.** The current page loads **two** analytics
   systems: the 2013-era Google Analytics `ga.js` snippet (property
   `UA-38494812-1` — Universal Analytics, a product Google shut down in
   2023; the script still returns 200 but the property cannot be
   receiving data) and a current Plausible tag. The build round's default
   is **drop `ga.js`, keep Plausible**. Joe can overrule.

5. **Two credits to confirm.** The footer credits `@joewalnes` and names
   Alex Sergeyev as project lead, both via `twitter.com` URLs that now
   redirect to `x.com`. Keep, update to `x.com`, or drop — Joe's call, not
   an agent's.

6. **No asset regeneration is needed.** `website/img/console/` already
   holds everything §4 uses. `qa/capture/capture-console.sh` does not need
   re-running for this work.

---

## 1. What exists today

### 1.1 The page, section by section

`website/index.html` is 456 lines. Rendered at a 1440x900 viewport the
document is **6,829px tall — 7.6 screens**. In source order, with the
measured vertical offset of each section's own heading:

| y (px) | Section (`class`) | Content | Verdict |
|---|---|---|---|
| 0 | `.header` | Fixed 99px bar: wordmark + 5 nav links | Nav's "Tutorial" and "Download" are in-page `#anchors`, not docs links |
| 197 | `.section-hero` | `h1` "WebSockets / the UNIX way", `h2` "Full duplex messaging between web browsers and servers", on `bluebg.jpg` | Marketing. Says nothing about what websocketd *is* |
| 601 | `.section-summary` | "websocketd is the WebSocket daemon" + 3-line prose + "It's like CGI, twenty years later" + GitHub star button | Marketing. This is where the actual explanation hides — 601px down |
| 985 | `.section-benefits` | 3 columns: Language agnostic / No libraries required / Avoid threading headaches | Marketing. Keep the substance |
| 1298 | `.section-philosophy` | Doug McIlroy UNIX quote in italic 34px | Marketing, but pure atmosphere |
| 1742 | `.section-tutorial` | "10 second tutorial": an 11-tab language switcher (Bash/Java/Python/Ruby/PHP/Perl/C/C#/Swift/Dart/"anything"), each with a full counter program, then the `websocketd` command, then a JS snippet | **Reference/tutorial content on a marketing surface.** ~858px. `MANUAL_PLACEMENT.md` puts this on the docs site, and `/docs/start/tutorial/` now covers it properly |
| 2600 | `.section-languages` | "Whichever way inclined you are" + an animated `I ♥ <language>` ticker + a 34-item language list | Marketing, but it is 400px to make one point |
| 2999 | `.section-features` | "Oh, and..." + 6 icon cards: Static server, Program routing, CGI server, SSL, Origin checking, Development console | Marketing. Six is too many; the icons are stock SVGs at 30% opacity |
| 3761 | `.section-console` | "Try it in the console" + prose + `console-light.png` + `console-dark.png` + `console-demo.mp4`, stacked full-width | **2,217px from this heading to the next — 2.5 screens on one feature.** Shows both themes *and* a video of the same thing |
| 5978 | `.section-install` | "Get it" + 3 platform columns + 7 hard-coded v0.4.1 archive links + "Current version: 0.4.1" | **Reference content**: a platform/architecture matrix belongs on `/docs/start/install/` |
| ~6600 | `.section-footer` | Credits, Go link, GitHub link | Fine |

### 1.2 What it actually looks like

Observed, not inferred:

- **Desktop, 1440x900, above the fold:** grey wordmark and nav on white;
  a full-bleed low-poly blue-grey photograph (`bluebg.jpg`, 114KB) with
  70px Lato-200 white type reading "WebSockets / the UNIX way"; below it
  the subhead. **A visitor who has never heard of websocketd learns
  nothing in the first screen.** "WebSockets the UNIX way" is a slogan
  about a philosophy; "Full duplex messaging between web browsers and
  servers" is a definition of WebSockets, not of websocketd.
- **The fold contains no call to action.** Measured: every `<a>` with
  `y < 900` is one of the five nav links (`#home`, `#tutorial`,
  `https://websocketd.com/docs/`, GitHub, `#download`). The first
  in-content link is the Wikipedia CGI link at **y=738**; the first
  button is "Star on GitHub" at **y=793**. There is no route to the
  tutorial or to a download in the first screen at any width.
- **Phone, 390x844:** the page does **not** overflow (`scrollWidth` 390).
  The layout collapses to one centred column; nav wraps under the
  wordmark. The document is **7,699px** tall. The fold shows wordmark,
  five nav links, and the hero — again no CTA.
- **Mojibake, live-latent.** `website/index.html` contains **zero**
  `charset` declarations. Its em-dashes are correct UTF-8 bytes
  (`e2 80 94`), so under any server that does not send a charset they
  render as `â€"` — reproduced in the browser on the dev-console
  section's two paragraphs. GitHub Pages happens to send
  `content-type: text/html; charset=utf-8`, so production is currently
  saved by the server. The page is not self-describing and any other
  host, or a `file://` open, breaks it.
- **The video renders as a white box in a screenshot.** `console-demo.mp4`
  is `autoplay muted loop playsinline controls` with no `poster`, so
  before it paints its first frame it is a blank rectangle with browser
  chrome across the bottom.

### 1.3 Link and asset audit

Every row was fetched. Codes are from `curl -sSL -o /dev/null -w '%{http_code}'`
on 2026-09-06.

| Target | Code | Notes |
|---|---|---|
| `https://websocketd.com/docs/` | **404** | Nav "Docs". The docs site is built but never deployed — see §0.2 |
| `https://github.com/joewalnes/websocketd` | 200 | ×3 uses |
| `https://github.com/joewalnes/websocketd/releases/latest` | 200 | → `/releases/tag/v0.4.1` |
| 7 × `.../releases/download/v0.4.1/websocketd-0.4.1-<plat>.zip` | 200 | linux amd64/386/arm/arm64, darwin_amd64, windows amd64/386 — all live |
| `https://en.wikipedia.org/wiki/Common_Gateway_Interface` | 200 | |
| `https://go.dev/` | 200 | |
| `https://twitter.com/joewalnes` | 200 | → `x.com/joewalnes` |
| `https://twitter.com/alexsergeyev` | 200 | → `x.com/alexsergeyev` |
| `//fonts.googleapis.com/css?family=Lato:200,300,300italic` | 200 | Third-party runtime dependency; protocol-relative |
| `//cdnjs.cloudflare.com/.../normalize/3.0.1/normalize.min.css` | 200 | pinned at 3.x; long superseded |
| `//cdnjs.cloudflare.com/.../font-awesome/4.2.0/css/font-awesome.css` | 200 | pinned at 4.x; long superseded |
| `https://plausible.io/js/pa-JbFVA5-K477wlxHERE0J0.js` | 200 | |
| `https://www.google-analytics.com/ga.js` | 200 | Universal Analytics, sunset 2023 — see §0.4 |
| `#home` `#tutorial` `#download` | n/a | In-page anchors; all three targets exist |
| local: `home.css` `home.js` `prism.css` `prism.js` `bluebg.jpg` | 200 | served locally |
| local: `icons/icon_{16662,29158,65754,18460,4921,59654}/*.svg` | 200 | all 6 icon directories under `website/icons/` are referenced |
| local: `img/console/console-{light,dark}.png`, `console-demo.mp4` | 200 | |

**Three findings from the audit.**

1. The single most important outbound link on the page — "Docs" — is the
   only one that is broken, and it is broken because of a deploy, not a
   typo.
2. The page pulls three files from two third-party CDNs at runtime
   (Google Fonts, cdnjs ×2), each pinned to a long-superseded major
   version. The docs site deliberately vendored its fonts to remove
   exactly this dependency
   (`docsite/static/fonts/fonts.css` says so in a header comment).
3. Nothing is orphaned: all six directories under `website/icons/` are
   referenced (checked by diffing the listing against the `src`
   attributes; the seventh entry is a `notes.txt`). Every asset in
   `website/` is used by the current page. The deletions in §4.8 are
   therefore deletions of *live* assets, made only because §4 removes
   their last consumer.

### 1.4 Asset inventory

| Asset | Size | Used by the current page? | Keep? |
|---|---|---|---|
| `img/console/console-light.png` | 74KB | yes | **yes** — §4 |
| `img/console/console-dark.png` | 73KB | yes | **yes** — §4 |
| `img/console/console-demo.mp4` | 19KB | yes | **yes** — §4 |
| `bluebg.jpg` | 114KB | yes (2 sections) | **no** — the blueprint language has no photographic backgrounds |
| `prism.js` + `prism.css` | 20KB | yes (tutorial tabs) | **no** — the tutorial section goes to the docs site, which uses Hugo's Chroma |
| `home.js` | 3.7KB | yes (tab switcher, language ticker, GitHub star count) | **no** — all three features are cut |
| `icons/**` (6 dirs + `notes.txt`) | — | all 6 used | **no** — the six-icon feature grid is cut |
| `home.css` | 6.1KB | yes | **replaced** — see §3 |
| `CNAME` | 14B | — | **UNTOUCHED. Do not delete, rename or rewrite.** It carries the custom domain and with it the TLS certificate |

**Note on a duplicate that already exists.** `docsite/static/img/console/`
holds byte-identical copies of the three console assets that live in
`website/img/console/`. `CLAUDE.md` says these assets have one home in
`website/img/console/` and should be referenced, not duplicated. They have
already been duplicated once. This is the concrete precedent behind §3's
refusal to copy CSS or font files into `website/`.

---

## 2. Comparables

Three single-binary developer tools with a marketing homepage and a
separate docs site, each rendered here in headless Chrome at 1440x900 and
390x844 and read at both widths. Everything below was seen rendered
unless marked *(from source)*.

### 2.1 Caddy — caddyserver.com

*Why comparable:* a single Go binary web server, configured from a CLI and
a plain-text file, whose docs live on the same domain under `/docs` —
exactly websocketd's shape and exactly its deploy topology.

- **Above the fold (1440x900), left to right, top to bottom:** a thin
  utility strip (parent org, Store, Forum, GitHub, theme switch); a nav
  bar (logo, Documentation, Features, Account, Support | **Download**
  button, **Sponsor** button); then the hero split in two — on the left a
  three-line display headline "THE ULTIMATE SERVER" in heavy uppercase
  gradient type, a subhead "makes your sites more **secure**, more
  **reliable**, and more **scalable** than any other solution", and a
  button row: **Download** (solid, primary), **Docs** (outlined,
  secondary), **Star / 75,530** (GitHub widget); on the right an autoplaying
  screen recording captioned "Watch in real-time as Caddy serves HTTPS in
  < 1 minute."
- **How "what is this" is made:** not by prose and not by a command. The
  headline is a claim, the subhead is three adjectives, and the *video*
  carries the entire explanation — it shows a person editing a Caddyfile
  in nano and a site coming up on HTTPS. A visitor who does not play the
  video does not learn what Caddy is from the first screen.
- **Docs handoff:** three separate routes, and they do not compete with
  install. "Documentation" in the nav opens a full mega-menu of **~26
  deep links** (Install, Caddyfile, JSON, Getting started, Static files,
  Reverse proxy, Troubleshooting, Command line, API, Auto HTTPS,
  Architecture, Logging, Monitoring, …) — measured. "Docs" is also the
  secondary hero button, landing on the docs *index* at `/docs/`.
  Download is its own button in both the nav and the hero.
- **Below the fold:** an enormous amount. `scrollHeight` **20,463px at
  1440 — 23 screens**, with 40 headings: sponsor strips, automatic-HTTPS
  explainers, a PKI section, testimonials, reverse-proxy features, static
  file server, config formats, extensibility, PHP benchmarks. This is a
  product marketing site, not a project homepage.
- **At 390px:** `scrollHeight` grows to **33,710px**. The nav does *not*
  collapse to a hamburger — it wraps to two rows and the docs mega-menu
  degrades to a plain "Documentation" link. The hero's two columns stack
  with the **copy first and the video pushed below the fold**. The two
  hero buttons stack vertically, Download above Docs. 25 elements
  overflow their container at this width (mostly inside horizontal
  scrollers).
- **Take:** the button pair *Download* + *Docs*, in that visual weight
  order, at the end of the hero, survives to phone width intact. The
  20,000px of page below it does not transfer to a project this size.

### 2.2 mitmproxy — mitmproxy.org

*Why comparable:* a single-binary interactive CLI tool with a terminal UI,
a developer audience arriving cold from a link, and docs on a separate
host (`docs.mitmproxy.org`).

- **Above the fold (1440x900):** a dark bar (logo, Blog, **Docs**,
  Publications | Sponsor, Star 44,934). Then a two-column hero: on the
  left a single sentence at large weight — "**mitmproxy** is a free and
  open source interactive HTTPS proxy." — and directly under it one dark
  code block containing `brew install mitmproxy` with a **copy** affordance,
  and one line of small text: "Release Notes (v12.2) – Other Downloads".
  On the right, a screenshot of the terminal UI mid-session.
- **How "what is this" is made:** an **is-sentence plus one command plus
  one screenshot**, and nothing else. The sentence names the category
  ("HTTPS proxy"), the licence ("free and open source") and the
  distinguishing property ("interactive") in eleven words. This is the
  tightest first screen of the three and the one closest to websocketd's
  problem.
- **Docs handoff:** a single "Docs" link in the top nav, going to
  `https://docs.mitmproxy.org/stable/` — the docs *index*, not a deep
  link. Install does **not** compete with it: install is the code block,
  docs is the nav. They occupy different slots, so neither is a choice
  the visitor has to make.
- **Below the fold:** modest. `scrollHeight` **2,911px — 3.2 screens**.
  Three feature blocks (Command Line, Web Interface, Python API), each a
  heading and a screenshot; then "Powerful Ecosystem", "Open Source",
  "Sponsored By". Every block is one idea and one image.
- **At 390px:** `scrollHeight` **4,403px**. The nav collapses to logo +
  hamburger. The hero **reorders**: the terminal screenshot moves
  **above** the sentence, then the sentence, then the install command,
  then the release-notes line. The install command block stays full
  width and legible.
- **Take:** the strongest model for websocketd. One sentence, one
  command, one picture, docs in the nav, install in the body — and the
  reorder-image-first move at phone width is worth copying only if the
  image is the fastest explanation, which for websocketd it is not (see
  §4).

### 2.3 Syncthing — syncthing.net

*Why comparable:* a single Go binary, no hosted service, run from a
terminal, with docs on a separate host (`docs.syncthing.net`) and a
homepage that is unambiguously marketing.

- **Above the fold (1440x900):** logo; a nav bar split left (Project,
  Downloads, Security, Foundation) and right (**Docs**, Forum, Code,
  Donations); a site-wide notice banner; then a hero panel over a starfield
  image containing a four-sentence paragraph beginning "Syncthing is a
  **continuous file synchronization** program…", and beneath it a
  centred "**Get Started**" heading with two inline text links: "Grab one
  of the **downloads** and start syncing! Check out the **getting started
  guide** for some tips along the way."
- **How "what is this" is made:** prose only. No command, no screenshot,
  no diagram in the first screen. Four sentences, ~60 words, of which
  only the first eight words say what it is; the rest is positioning.
- **Docs handoff:** two routes. "Docs" in the nav goes to the docs index.
  The hero's second text link is a **deep link straight into the tutorial**
  (`docs.syncthing.net/intro/getting-started.html`). Install and docs
  sit side by side in the same sentence and compete directly — measured,
  they are 31px apart vertically and both rendered as plain blue inline
  links, so neither reads as primary.
- **Below the fold:** short. `scrollHeight` **2,117px — 2.4 screens**:
  "Private & Secure", "Open", "Easy to Use" as bulleted columns, then a
  footer.
- **At 390px:** `scrollHeight` **3,382px**, no overflow. Nav collapses to
  a hamburger; the two-column feature blocks stack; the hero paragraph
  keeps all four sentences and consumes the entire first screen, pushing
  "Get Started" and both links **below the fold**.
- **Take:** the negative lesson is the clearest one here. Making both
  calls to action plain inline links inside a paragraph means that at
  phone width the visitor's first screen is pure prose with nothing to
  press. Buttons that survive the fold are worth more than another
  sentence of positioning.

### 2.4 A candidate measured and rejected: restic — restic.net

Rendered at 1440x900 for completeness. restic is otherwise an excellent
match (single Go binary, CLI, separate docs on Read the Docs), but its
"homepage" is a docs page: a dark sidebar carrying **Home / Blog / Forum /
Docs / GitHub** plus a 10-entry "Table of Contents", and a content column
that opens with an `<h1>` reading "Introduction" and a seven-bullet
feature list, then "Quickstart". `scrollHeight` 3,091px. It has no hero,
no CTA button, and no separation between marketing and manual — it is
precisely the failure mode `MANUAL_PLACEMENT.md` is written to prevent,
so it is recorded here as a control rather than a model.

### 2.5 What the three agree on

| | Caddy | mitmproxy | Syncthing |
|---|---|---|---|
| First-screen explanation | video | sentence + command + screenshot | prose |
| Primary CTA | Download (button) | `brew install` (code block) | "downloads" (inline link) |
| Secondary CTA | Docs (button) | Docs (nav only) | getting-started (inline link) |
| Docs link position | nav **and** hero button | nav only | nav **and** hero link |
| Deep link into a tutorial? | yes, via mega-menu | no | yes |
| Page height @1440 | 20,463px | 2,911px | 2,117px |
| Phone: what changes | copy first, media below | media first, copy below | stacks, CTAs fall below fold |

Three agreements worth acting on:

1. **All three put the docs link in the top nav**, and it is never the
   only route.
2. **All three lead with either a sentence that names the category or a
   moving picture — never a slogan.** websocketd today leads with a
   slogan.
3. **Two of three keep the whole page under 3,100px.** websocketd today
   is 6,829px. Caddy is the outlier and Caddy is a commercial product.

---

## 3. The shared visual language, and the exact reuse mechanism

### 3.1 What the docs site actually defines

Read in full from `docsite/themes/blueprint/static/css/blueprint.css`
(665 lines), including its 95-line header comment.

**Design tokens — the complete `:root` block, verbatim values:**

| Token | Light (`:root`) | Dark (`prefers-color-scheme: dark` and `[data-theme=dark]`) |
|---|---|---|
| `--u` | `8px` (baseline grid unit; not themed) | — |
| `--bg` | `#f7f5ef` | `#14181c` |
| `--bg-panel` | `#ffffff` | `#1b2126` |
| `--fg` | `#1b1f23` | `#e8e6df` |
| `--fg-muted` | `#565b61` | `#a2a9ad` |
| `--accent` | `#1d5fae` | `#6fb1ff` |
| `--accent-soft` | `#dbe8fb` | `#1e2c3d` |
| `--grid-line` | `rgba(29,95,174,0.10)` | `rgba(111,177,255,0.08)` |
| `--grid-line-strong` | `rgba(29,95,174,0.18)` | `rgba(111,177,255,0.16)` |
| `--border` | `#d8d3c4` | `#2c343a` |
| `--code-bg` | `#eef0e6` | `#1f2731` |

Plus six font-metric tokens that are **not** colours and drive all the
spacing arithmetic: `--mono-cap: 0.730`, `--mono-asc: 1.020`,
`--mono-desc: 0.300`, `--sans-cap: 0.698`, `--sans-asc: 1.025`,
`--sans-desc: 0.275`. These are stated in the header comment to be
measured from the actual font files, not approximated.

**Fonts, and which role gets which:**

- `--font-mono`: `"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace`
  — used for **h1, h2, h3, `pre`, `code`, the site logo, and the CTA
  button label**. Headings are mono. That single decision is most of the
  theme's character.
- `--font-sans`: `"IBM Plex Sans", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`
  — used for **body copy, list items, `small`, nav, table cells**.

**Type scale**, each line-height a whole multiple of `--u`:

| Role | Font | `--f` | `--lh` |
|---|---|---|---|
| h1 | mono | 32px | 5U = 40px |
| h2 | mono | 24px | 4U = 32px |
| h3 | mono | 18px | 3U = 24px |
| body (`p`, `li`) | sans | 16px | 3U = 24px |
| small / nav | sans | 12px | 2U = 16px |
| code (`pre`, `code`) | mono | 14px | 3U = 24px |

**The 8px derivation.** `--u: 8px`. Every line-height above is `n * --u`.
Every block gap is a whole multiple (`2U` between siblings, `4U` before an
h2, `3U` before an h3). Padding is *not* typed by hand — it is a `calc()`
of the font metrics, in one of two branches selected by `@supports`:

- **Modern** (`text-box-trim: trim-both; text-box-edge: cap alphabetic`):
  `padding-top: calc(var(--lh) - var(--cap) * var(--f))`, `padding-bottom: 0`.
- **Classic fallback**:
  `padding-top: calc(var(--lh)/2 - (var(--asc) - var(--desc))/2 * var(--f))`
  and `padding-bottom: calc(var(--lh) - var(--f) - <that same shift>)`,
  with `line-height: 1` to pin a known box.

**Light/dark mechanism.** Three blocks, in this order: bare `:root`
carries the complete light palette; `@media (prefers-color-scheme: dark)`
guarded as `:root:not([data-theme="light"])` redefines the same names;
`:root[data-theme="dark"]` redefines them again so an explicit choice
wins either way. `html { color-scheme: light dark; }`. **No colour is
defined only inside a media query.** *(Verified rendered in both schemes
by emulating `prefers-color-scheme` — screenshots taken in this round.)*

**Fonts are self-hosted.** `docsite/static/fonts/` holds four variable
`.woff2` subsets (109KB total: JetBrains Mono latin + latin-ext, IBM Plex
Sans latin + latin-ext) and a `fonts.css` whose `src: url(...)` values are
**relative to the stylesheet**, deliberately — its header comment records
that an absolute `/fonts/x.woff2` 404s under the `/docs` path prefix and
silently drops every face back to a system font, which breaks the very
metrics the grid is derived from. `head.html` links both stylesheets
through Hugo's `relURL`, and `nav.html` carries a long comment explaining
that every internal href must be `(strings.TrimPrefix "/" "/path/") | relURL`
because `relURL` will not prefix a root-absolute input.

**How the two sites are assembled** (`.github/workflows/pages.yml`, read
in full): `cp -R website/. _site/` then `cp -R docsite/public/. _site/docs/`.
One artifact, one domain. The homepage is at the artifact root and the
docs are one directory down. That is what makes §3.2 possible.

### 3.2 The mechanism the homepage should use — recommendation

**The homepage links the docs site's own stylesheet and font files by
root-absolute path, and adds one small homepage-only stylesheet after
them.**

```html
<meta charset="utf-8">
<link rel="stylesheet" href="/docs/fonts/fonts.css">
<link rel="stylesheet" href="/docs/css/blueprint.css">
<link rel="stylesheet" href="home.css">
```

**Verified, not assumed.** Against a locally assembled `_site` (Hugo
0.165.0 extended, the version pinned in CI, then the workflow's two `cp`
commands), served over HTTP:

- `/docs/css/blueprint.css` → **200**
- `/docs/fonts/fonts.css` → **200**
- `/docs/fonts/jetbrains-mono-var-latin.woff2` → **200**
- `blueprint.css` contains **zero** `url(` references (`grep -c` = 0), so
  it has no path-prefix exposure of its own; the grid paper is
  `repeating-linear-gradient`, not an image.
- `fonts.css`'s relative `src` URLs resolve against `/docs/fonts/`, which
  is where the files are. The bug its comment warns about does not
  recur when the *consumer* is at `/`, because the URLs are relative to
  the stylesheet, not to the page.

**And the part that makes this "sharing" rather than "resembling".**
The homepage needs two type roles the docs site does not have: a display
headline and a lede. It must not invent numbers for them and it must not
re-type the padding formulas. It does not have to. blueprint.css applies
the padding `calc()` to the `h1` selector using the custom properties
`--f`, `--lh`, `--cap`; a more specific homepage rule that redefines
`--f` and `--lh` on that same element re-runs the docs' own arithmetic
with the new values.

Measured in the browser on a probe page linking exactly the three
stylesheets above:

| Element | Declared | Computed `font-size` / `line-height` | Computed `padding-top` | Rendered height |
|---|---|---|---|---|
| `.home-hero h1` with `--f:48px; --lh:calc(var(--u)*8)` | — | 48px / 64px | **28.96px** = `64 − 0.730×48` | **64.00px** = 8U ✓ |
| `.home-hero p.lede` with `--f:20px; --lh:calc(var(--u)*4)` | — | 20px / 32px | **18.04px** = `32 − 0.698×20` | 32.13px ≈ 4U |
| untouched `h1` (docs default) | — | 32px / 40px | 16.64px | 39.98px = 5U ✓ |

Font family resolved to `"JetBrains Mono"` and `"IBM Plex Sans"` from
`/docs/fonts/`, and `text-box-trim` computed to `trim-both`, so the
modern branch was the one exercised. The homepage therefore inherits the
grid *machinery*, not a copy of the docs' numbers.

**Grid check for the two new roles, using the header comment's own rule**
(`(asc + desc) × F ≤ n × U`):

| New role | Font | F | (asc+desc)×F | chosen n·U | ok? |
|---|---|---|---|---|---|
| hero headline, ≥700px wide | mono | 48px | 1.320 × 48 = 63.36 | **8U = 64** | ✓ |
| hero headline, <700px wide | mono | 32px | 1.320 × 32 = 42.24 | **6U = 48** | ✓ |
| hero lede | sans | 20px | 1.300 × 20 = 26.00 | **4U = 32** | ✓ |

*Observation, recorded and deliberately not acted on:* the docs' own `h1`
takes 5U = 40px at 32px mono, where the rule as written would demand
48px (42.24 > 40). Under the modern `text-box-trim` branch the visible
box is `cap × F` = 23.36px so nothing clips, and the site has shipped
that way; the phone hero role above takes 6U rather than copy the
discrepancy into a new role. **Do not "fix" the docs' h1 as part of A3.**

**Risks of the recommended mechanism, each with its mitigation:**

| # | Risk | Mitigation |
|---|---|---|
| R1 | The homepage renders unstyled when `website/` is served on its own — the `/docs/` paths 404 | The verification procedure in §6 always serves the **assembled** `_site`, which is the tree that actually deploys. Never serve `website/` alone and never open `file://` |
| R2 | A future docs-theme change silently restyles the homepage | That is the intent of "shares … rather than merely resembling", but it must be *visible*: §6's browser check covers the homepage, and `pages.yml`'s `pull_request` path filter should be widened to `website/**` (see §6.4) |
| R3 | Class-name collision: blueprint.css already owns `.layout`, `.site-header`, `.site-nav`, `.site-main`, `.content`, `.hero`, `.cta`, `.section-list`, `.site-footer` | Every homepage-only class is prefixed **`home-`**. In particular do **not** reuse `.hero` or `.cta` — the docs site styles both |
| R4 | Cascade order — `home.css` overriding the wrong things | `home.css` loads last, may set `--f`/`--lh`/`--cap` on its own selectors, and must **not** redefine any `:root` colour token. If the homepage needs a colour, it uses the existing token or adds a new `--home-*` name |
| R5 | `blueprint.css` paints the grid-paper `body` background and a `.content`/`.hero` max-width | The homepage inherits the grid paper deliberately — it is the strongest single cue that the two surfaces are one project. It must not reuse `.content` or `.hero` for its own widths (see R3) |
| R6 | A broken docs build takes the homepage's styling with it | Real, and acceptable: the workflow already fails the whole deploy if `hugo` fails, so the two ship or fail together anyway |

**Alternatives considered and rejected:**

- **Copy the `:root` token block into `home.css`.** Rejected: two sources
  of truth for eleven colours. This is the definition of "merely
  resembling", and the acceptance clause rules it out.
- **Copy the four `.woff2` files into `website/fonts/`.** Rejected: 109KB
  duplicated, and a subset change would update one copy. The repo already
  has a live example of exactly this drift risk —
  `docsite/static/img/console/` duplicating `website/img/console/`.
- **Hoist `blueprint.css` to a shared top-level directory both sites
  reference.** Architecturally cleanest and worth revisiting if a third
  surface ever appears, but today it means a new Hugo mount, a new copy
  step in the workflow, and a moved file, for no behaviour the linked
  path does not already give. Rejected for A3.
- **Inline the whole theme into the homepage's `<style>`.** Rejected: a
  665-line copy that starts drifting the moment it lands.

---

## 4. The recommendation

One page. Three screens at desktop. Target `scrollHeight` **under
3,200px at 1440x900** — in the band mitmproxy and Syncthing occupy, an
almost exactly halved page.

### 4.0 The job, in one line

A visitor who has never heard of websocketd must, **in the first screen
and without scrolling**, learn (a) that it turns an ordinary program into
a WebSocket server, (b) what the command looks like, and (c) where to go
next — tutorial or download — with a button, not an inline link.

### 4.1 Header

Sticky or static, one row: the wordmark `websocketd` in mono (matching
`.site-logo`), and on the right exactly three links — **Docs**
(`/docs/`), **GitHub**, **Download** (`/docs/start/install/`).

- "About" and "Tutorial" are dropped from the nav: "About" pointed at the
  page you are already on, and "Tutorial" pointed at an in-page anchor
  that will no longer exist.
- All three comparables put Docs in the top nav (§2.5, agreement 1). This
  does the same, and it is the *only* nav item that is not also a hero
  button, so the nav is never the sole route to anything.

### 4.2 Hero — the whole point of the page

Two columns above 900px; one column below. Left column, in order:

**Headline** (mono, `--f:48px`, `--lh:8U`; 32px/6U below 700px):

> Turn any program into a WebSocket server

**Lede** (sans, `--f:20px`, `--lh:4U`, two sentences):

> websocketd is a single binary that wraps a command-line program and
> serves it over a WebSocket. Your program reads lines from stdin and
> writes lines to stdout — no library, no framework, no change to your
> code.

**One code block** (the docs' `pre` role, `--code-bg`, mono 14px/3U):

```
$ cat count.sh
#!/bin/bash
for COUNT in 1 2 3 4 5; do echo $COUNT; sleep 1; done

$ websocketd --port=8080 ./count.sh
```

**Button row:**

| | Label | Destination | Style |
|---|---|---|---|
| Primary | **Start the tutorial →** | `/docs/start/tutorial/` | solid `--accent`, label in `--bg` |
| Secondary | **Download** | `/docs/start/install/` | outlined, `--border` + `--accent` label |

**One small line under the buttons** (sans 12px/2U, `--fg-muted`):

> All releases on GitHub → (`https://github.com/joewalnes/websocketd/releases/latest`)

Right column: **`console-light.png` / `console-dark.png`**, theme-matched:

```html
<picture>
  <source srcset="img/console/console-dark.png"
          media="(prefers-color-scheme: dark)">
  <img src="img/console/console-light.png" width="1200" height="..."
       alt="The websocketd dev console: a list of WebSocket frames beside a detail pane showing the selected frame.">
</picture>
```

**Why each choice:**

- **The headline is a verb phrase, not a slogan.** "WebSockets the UNIX
  way" fails the cold-visitor test measured in §1.2. Every comparable
  either names the category in a sentence (mitmproxy, Syncthing) or shows
  it moving (Caddy); none of them ships a philosophy as a headline.
- **The lede is an is-sentence, then a mechanism sentence.** Modelled on
  mitmproxy's eleven-word category sentence, extended by one sentence
  because "reads stdin, writes stdout" *is* the product and eleven words
  cannot carry it. It obeys `docsite/STYLE.md` §2: front-loaded, one idea
  per sentence, no abstract-noun stance.
- **The command is `websocketd --port=8080 ./count.sh`** — byte-identical
  to `docsite/content/start/tutorial.md` line 59, the first command the
  tutorial has you run, and to `README.md`. The docs *landing* page uses
  `--sameorigin`, which the tutorial introduces later; the homepage
  matches the tutorial's step one because that is where its primary
  button goes. *(Minor inconsistency between the docs landing page and
  the tutorial's first step is noted, not resolved here.)*
- **The code block shows the script and the command, not an install
  line.** mitmproxy's `brew install mitmproxy` works because the
  surprising thing about mitmproxy is what it *does*; the surprising
  thing about websocketd is that *your existing script is the server*.
  A `brew install` line would spend the hero's one code block on the
  least interesting fact.
- **A still, not the video, in the hero.** Caddy's autoplaying hero video
  is its differentiator and it still pushes its own CTAs around at phone
  width. A still paints instantly, needs no `poster`, and — via
  `<picture>` — matches the reader's own colour scheme, which is a
  visible demonstration that the console has both themes. The video does
  a different job further down (§4.4).
- **Buttons, not inline links.** Syncthing's inline-link CTAs fall below
  the fold at 390px and read as body text (§2.3). Both routes named in
  the `Done:` line — tutorial and download — are one click from the first
  screen, at both widths.

### 4.3 "How it works" — one screen, three items

Heading (`h2`, mono 24px/4U): **How it works**

Three items, three columns above 900px, stacked below. Each is an `h3`
plus two sentences of body copy. No icons.

1. **Your program stays a program.** It reads lines from stdin and writes
   lines to stdout. It does not import anything and it does not know a
   WebSocket exists.
2. **One process per connection.** Every browser that connects gets its
   own copy of your program. Connections are fully isolated and share
   nothing.
3. **Any language.** Bash, Python, Ruby, Node, PHP, Perl, C, Go — if you
   can run it from a shell, you can serve it. *(→ `/docs/how-to/languages/`)*

Then a single closing line, sans 16px:

> It is CGI's idea, applied to WebSockets. → `/docs/understanding/process-model/`

**Why:** these are the three claims from the current
`.section-benefits` block, rewritten to STYLE.md's voice and cut from six
feature cards to three ideas. Item 3 replaces both the 11-tab language
switcher (858px) and the 34-item animated ticker (400px) with one
sentence and one link — 1,258px of page for eight words and a deep link.
The CGI line preserves the current page's best sentence ("It's like CGI,
twenty years later") as a pointer into the explanation that now exists.

### 4.4 The dev console — one screen

Heading: **See every frame while you build**

Two sentences of body copy: start the server with `--devconsole` and open
the root URL; connect, send a frame, and read back what your script wrote
to stdout, with no client code.

Then **`console-demo.mp4`**, at most 900px wide, centred, with
`autoplay muted loop playsinline` and — the fix for §1.2's white box —
`poster="img/console/console-light.png"`. Under it, one link:
**Dev console reference →** `/docs/reference/dev-console/`.

**Why:** this replaces 2,217px (2.5 screens, measured heading-to-heading)
of stacked light PNG + dark PNG + video with one moving picture. All three assets are still used, each
once, each doing a different job: the stills are the hero's theme-matched
image, the video is the demonstration. `MANUAL_PLACEMENT.md` names the
dev console demo as belonging on this page; it does not name three copies
of it.

### 4.5 Also in the box — one short strip

Heading: **Also in the box**

One line each, sans 16px, no icons, no flags named:

- Serves your static files alongside your program
- Routes different URLs to different programs
- CGI for ordinary HTTP responses
- HTTPS and `wss://` out of the box
- Origin checking, so only your pages can connect

Closing link: **All CLI flags →** `/docs/reference/cli-flags/`

**Why:** the current six-icon grid says the same things with six stock
SVGs at 30% opacity and 760px of vertical space. Naming the capability
and linking the reference is marketing; listing the flags would be
reference content and is forbidden here.

### 4.6 Footer

`--border` rule; sans 12px/2U in `--fg-muted`. Three groups:
**Docs** (`/docs/`) · **GitHub** · **Changelog** (`/docs/changelog/`);
credits (Joe Walnes, Alex Sergeyev — see §0.5); "Written in Go".

### 4.7 What the phone page does (390px)

Explicit, because it is an acceptance clause.

1. **One column throughout.** Hero left/right columns stack; "How it
   works" and "Also in the box" stack.
2. **Hero order does not change:** headline → lede → code block → both
   buttons → the small releases line → console image. The image goes
   *below* the buttons, unlike mitmproxy's reorder, because for
   websocketd the sentence and the command explain faster than the
   picture does, and because the `Done:` line requires the tutorial and
   the download to be reachable — they must not be pushed off the first
   screen by a 1200px-wide screenshot. Caddy does exactly this and it is
   the right call.
3. **Headline drops to 32px / 6U** below 700px (§3.2's table).
4. **Nav does not become a hamburger.** Three links fit on one row at
   390px in 12px sans. A drawer for three links is machinery for its own
   sake; the docs site needs one because it has 24 nav entries.
5. **The code block scrolls inside itself** (`overflow-x: auto`, which
   blueprint.css already sets on `pre`) and never widens the document.
   The longest line, `for COUNT in 1 2 3 4 5; do echo $COUNT; sleep 1; done`,
   is 54 characters and will not fit at 390px — that is expected and
   handled, not a defect.
6. **The video is `width: 100%`.** `blueprint.css` already caps
   `img, video, svg { max-width: 100%; height: auto; }`.
7. **`document.documentElement.scrollWidth` must equal exactly 390** and
   no element may extend past the viewport except inside an
   `overflow-x: auto` scroller. This is checkable (§6.2) and it is the
   one thing that must not regress: the current page passes it and a
   rebuild is a chance to break it.

### 4.8 Files after the rebuild

| File | Action |
|---|---|
| `website/index.html` | Rewritten |
| `website/home.css` | Rewritten small — homepage-only layout, `home-`-prefixed classes, the two new type roles, no `:root` redefinitions |
| `website/home.js` | **Deleted** — tabs, ticker and star-count are all cut |
| `website/prism.js`, `website/prism.css` | **Deleted** — no client-side highlighting; the hero block is plain `pre` |
| `website/bluebg.jpg` | **Deleted** |
| `website/icons/` | **Deleted** |
| `website/img/console/*` | Unchanged, all three used |
| `website/CNAME` | **UNTOUCHED** |
| `website/REDESIGN_PLAN.md` | This file; stays as the record |

Deletions are ~140KB of assets and 20KB of vendored JS/CSS. Each is
deleted only because §4 removes its last consumer; the build round must
`grep` for each filename across the repo before removing it.

---

## 5. What we are deliberately NOT doing

1. **Not touching the docs site.** Not its theme, not its content, not
   its `h1` line-height discrepancy (§3.2). A3 is the homepage.
2. **Not touching `website/CNAME`.** Named in `CLAUDE.md`'s do-not-touch
   list. Deleting or renaming it drops the custom domain and its TLS
   certificate.
3. **Not putting reference content on the homepage.** No flag table, no
   platform/architecture matrix, no environment variables, no deployment
   recipes. `MANUAL_PLACEMENT.md` is explicit, and the current install
   section is the live violation.
4. **Not keeping the 11-language tutorial tab box.** It is the tutorial,
   on the wrong surface, in eleven copies. `/docs/start/tutorial/` and
   `/docs/how-to/languages/` exist now.
5. **Not naming a version number or an archive filename anywhere.**
   "0.4.1" and seven `.zip` URLs are how the current page will go stale
   the day 0.5.0 ships (§0.1).
6. **Not building a theme toggle.** The docs site has none either; both
   follow `prefers-color-scheme`, and the `<picture>` element matches the
   screenshot to it.
7. **Not adding a framework, a bundler, a build step, or a JS
   dependency.** `website/` is copied verbatim by the workflow. The
   rebuilt page should need zero JavaScript; if it ends up needing any,
   that is a signal to cut a feature, not to add a script.
8. **Not adding a GitHub star counter.** It needs JS and a live API call
   to render a number that is not a reason to use the tool.
9. **Not keeping the runtime CDN dependencies.** Google Fonts, normalize
   3.0.1 and Font Awesome 4.2.0 all go; the docs site's self-hosted
   fonts replace the first and nothing needs the other two.
10. **Not redesigning the README.** A3 is `website/`. The README's
    "Download" link already points at `/docs/start/install/` and is
    consistent with this plan.
11. **Not re-running `qa/capture/capture-console.sh`.** The committed
    assets are current (§0.6).
12. **Not adding a blog, a sponsors strip, testimonials, or a feature
    matrix.** That is the Caddy shape, and Caddy is a company.

---

## 6. How the build round proves each clause of the `Done:` line

The gate will re-run this procedure independently. It is written to be
executable by someone who was not the author of the page.

### 6.1 Build the artifact the way the deploy does

Never verify by opening `website/index.html` from disk, and never serve
`website/` on its own — the `/docs/*` stylesheet, font and deep-link
paths only exist in the assembled tree.

```bash
# In an isolated worktree. Requires: hugo 0.165.0 extended (the version
# pinned in .github/workflows/pages.yml), python3, Google Chrome.
cd "$WT/docsite" && hugo --minify

cd "$WT"
rm -rf _site && mkdir -p _site
cp -R website/. _site/
mkdir -p _site/docs && cp -R docsite/public/. _site/docs/

# Never a fixed port -- sibling agents are live.
PORT=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
(cd _site && python3 -m http.server "$PORT" --bind 127.0.0.1) &
```

Everything below runs against `http://127.0.0.1:$PORT/`.

### 6.2 The browser verification procedure

**Instrument.** Headless Chrome at
`/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`, driven
through chromedp so the layout viewport is established by
`Emulation.setDeviceMetricsOverride` before paint, with
`Page.captureScreenshot(captureBeyondViewport: true)`. Each Chrome run
gets its **own `--user-data-dir`** in the agent's own temp directory —
two chromedp sessions sharing a profile crash each other.

**Do not** use `Google Chrome --headless --screenshot --window-size=W,H`.
It misreported this exact page in this round (see the warning at the top
of this document).

**Viewports, and what a passing screenshot must show at each:**

| # | Viewport | `prefers-color-scheme` | A passing capture shows |
|---|---|---|---|
| 1 | **1440 × 900** | light | Header with Docs/GitHub/Download. Headline, lede, the code block, **both buttons**, and the console still — all within the 900px capture. Grid-paper background visible. Headings in JetBrains Mono, body in IBM Plex Sans |
| 2 | **1440 × 900** | dark | Same layout on `#14181c`; the `<picture>` has switched to `console-dark.png`; the primary button's label is `--bg`-coloured, not white |
| 3 | **390 × 844** | light | One column. Headline, lede, code block and **both buttons** all inside the 844px capture. Nothing clipped at the right edge |
| 4 | **390 × 844** | dark | As #3, dark palette, dark console still |
| 5 | **768 × 1024** | light | The breakpoint between one and two columns behaves — no half-collapsed row, no orphaned column |

Emulate the colour scheme explicitly (`Emulation.setEmulatedMedia` with
feature `prefers-color-scheme`). Do **not** rely on the host's setting:
this machine is in dark mode, and an unemulated capture of the docs site
in this round came back dark by default. A capture that does not state
which scheme it emulated is not evidence.

**Two numeric assertions, read out of the live DOM at 390 × 844:**

```js
document.documentElement.scrollWidth === 390          // exactly
// and: no element's bounding rect extends past the viewport
//      except inside an overflow-x:auto ancestor
```

**One typographic assertion, at 1440 × 900**, which is what distinguishes
"shares spacing" from "resembles it": for the hero headline and the lede,
`getComputedStyle(el).paddingTop` must equal
`lh − cap × f` from §3.2's table (28.96px and 18.04px respectively), and
`getBoundingClientRect().height` must be a whole multiple of 8 (±0.2px for
sub-pixel rounding of the trimmed cap box). If `home.css` has re-typed a
number instead of overriding `--f`/`--lh`, this is where it shows.

### 6.3 Clause-by-clause

| `Done:` clause | Proof |
|---|---|
| "a visitor who has never heard of websocketd understands what it is" | Captures 1 and 3 contain the headline and the two-sentence lede in full, above the fold, at both widths |
| "reaches either the tutorial or the download within one click" | Captures 1 and 3 show both buttons inside the fold. `curl` each `href`: `/docs/start/tutorial/` → 200, `/docs/start/install/` → 200 against the assembled `_site` |
| "shares type, palette and spacing with the docs site rather than merely resembling it" | `index.html` links `/docs/css/blueprint.css` and `/docs/fonts/fonts.css` (grep the file). `home.css` contains **no** hex colour and **no** `:root` block (grep). The computed-padding assertion in §6.2 passes. Computed `font-family` on a heading resolves to `JetBrains Mono` |
| "the dev console demo assets in `website/img/console/` are used" | grep `index.html` for all three filenames; capture 2 shows the dark still, capture 1 the light one, and the video element is present with `poster` set |
| "every link resolves (including the `/docs/` deep links)" | Extract every `href`/`src` from the built `index.html` and `curl -sSL -o /dev/null -w '%{http_code}'` each. Local and `/docs/*` targets go through the local server; external ones go to the network. **Every row must be 200.** Known and expected: the same `/docs/*` URLs return 404 on production until §0.2 is done — verify against `_site`, and say so in the report |
| "`website/CNAME` is untouched" | `git diff --stat main -- website/CNAME` is empty, and `test -f _site/CNAME` after assembly (the workflow's own guard) |
| "renders correctly at a phone width in a real browser, evidenced by a screenshot" | Capture 3 (and 4), with the two numeric assertions |
| "Marketing and orientation only — no reference content" | `index.html` contains no flag table, no platform/architecture matrix, no environment variable names, and names no release version or archive filename |

### 6.4 Two repo-level checks worth adding in the same round

- `.github/workflows/pages.yml`'s `pull_request` trigger filters on
  `docsite/**` only, and the `docs-link-check` job assembles **only**
  `_site/docs`. A homepage-only change therefore gets **no CI at all**
  today, and the homepage's links are not link-checked even on a push.
  Widening the path filter to `website/**` and pointing lychee at the
  full `_site` would make "every link resolves" a standing guarantee
  rather than a one-off manual pass. *(Recommended, not required for A3.)*
- The rebuilt `index.html` must open with `<meta charset="utf-8">`.
  §1.2 shows what its absence costs the moment the page is served by
  anything other than GitHub Pages.

### 6.5 What this procedure does not prove

- **The classic `@supports not (text-box-trim)` fallback branch.** The
  Chrome used here supports `text-box-trim`, so only the modern branch was
  exercised. A browser without it takes the other formula; that path is
  unverified here and on the docs site alike.
- **Production.** Until §0.2 happens, no procedure run on this machine
  says anything about what `websocketd.com` serves.
- **Real devices.** 390 × 844 emulated in Chrome is not an iPhone. It is
  the same instrument the docs site was verified with, and it catches
  layout overflow, which is the failure that actually happens.
