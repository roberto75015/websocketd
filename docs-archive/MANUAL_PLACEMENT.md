# Documentation placement guide

> **Archived 2026-09-07. A3 landed as `380a043`, and this document was its
> last live citation.** The surface-boundary rule it exists for is now in
> `CLAUDE.md`'s Documentation section, where a writer will actually meet it.
> Read the rest as a record rather than as guidance. Already overtaken: the
> docs site shipped at **`websocketd.com/docs`**, not the
> `docs.websocketd.com` subdomain this originally proposed (GitHub Pages
> allows one custom domain per repository); the wiki verdict below was
> executed in A4; and the "item 5 mockups" question was settled by
> `docsite/themes/blueprint/`. One recommendation here was never acted on and
> is tracked nowhere else: enabling GitHub Discussions, under "GitHub Issues
> / Discussions".

Where should each piece of documentation live? Every surface websocketd's
docs can appear on, what it's for, who reads it there, what belongs on it,
what explicitly does not — with the reasoning, so this can be revisited
later without re-litigating from scratch. Feeds the new manual (issue
#467) and the site plan (`MANUAL_PLAN.md`).

---

## The surfaces

| Surface | Audience | Reaches them when |
|---|---|---|
| `--help` (CLI output) | Anyone who already has the binary | They typed `websocketd` wrong, or `--help` |
| man page | Sysadmins, package maintainers | `man websocketd`, or packaging metadata |
| GitHub README | Evaluators, contributors | Landing on the repo itself |
| Website homepage (websocketd.com) | Evaluators, people who heard about it | A link from somewhere else on the internet |
| New docs site (websocketd.com/docs) | Everyone actually using it | Following any "docs" link, or a search engine |
| GitHub Wiki | Nobody, going forward | — see verdict below |
| GitHub Issues / Discussions | Bug reporters, people stuck | Filing something |
| CHANGES / DIARY.md / CLAUDE.md | Maintainers, agents working on the repo itself | Not user-facing at all |

## `--help`

**What goes here:** every flag, one line each, defaults included. A
one-line description of what websocketd is, for the person who typed the
command with no arguments and is now staring at a wall of text. A pointer
to the docs site for anything conceptual.

**What does not:** tutorials, cookbook recipes, deployment guidance,
security rationale beyond a one-line pointer.

**Opinion:** `--help` should be the **single source of truth for flag
text** — not the man page, not the website. It's currently the most
accurate of the three existing surfaces (`current-surface-and-drift.md`
confirms it already matches the code; only the man page has drifted), so
the new tooling should generate the man page and the website's CLI
reference *from* it, not maintain three hand-written copies (see
`MANUAL_TOOLING.md`).

## man page

**What goes here:** the same flag reference as `--help`, in man(1) format,
plus the handful of things Unix convention expects (`SYNOPSIS`, `SEE ALSO`,
exit codes, `BUGS`/security notes section pointing at the security-notes
page on the docs site).

**What does not:** anything that isn't reference material. No tutorial, no
cookbook, no FAQ — a man page is not read start-to-end and shouldn't try
to be.

**Opinion:** this should be **generated, not hand-maintained** (see
`MANUAL_TOOLING.md`) — it's the surface that's actually drifted (wrong
`--maxforks` default, missing two flags), precisely because it's the one
hand-copied from `--help` instead of generated from the same source.

## GitHub README

**What goes here:** the pitch (what it is, one sentence), a copy-pasteable
quickstart (the *fixed* version — see `MANUAL_INVENTORY.md` §11 on the
tutorial's known traps), install instructions, a link to the full docs
site, and — this is the part that's currently missing entirely — a section
for people who want to **work on websocketd itself**: build instructions,
test instructions, where CONTRIBUTING-style info lives, a link to
`CLAUDE.md`/`DIARY.md` for anyone curious about project conventions.

**What does not:** the full flag reference (link to `--help`/docs site
instead), deployment guides, cookbook recipes, FAQ. The README is
currently the de facto full manual by accretion — that's exactly the
"scrappy and distributed" problem issue #467 names. It should shrink once
the docs site exists, not grow.

**Opinion:** README's audience is **two different people wearing one
hat** — someone evaluating the project on GitHub, and someone about to
send a PR to it. Both need a *short* page with clear exits: "want to use
this? → docs site. want to build it? → keep reading." Don't try to serve
end-user reference material here.

## Website homepage

**What goes here:** what websocketd is and why it's interesting, at the
level of someone who has never heard of it — the pitch, the dev console
demo (screenshot/video, already produced for A2), 3-4 headline features,
a "get started" button that goes to the docs site's tutorial, not a wall
of its own content.

**What does not:** the manual itself. No flag reference, no deployment
guides. If a visitor scrolls past the fold looking for "how do I actually
use this," that's a sign content leaked onto the wrong surface.

**Opinion:** matches the model you sketched — homepage is marketing and
orientation, not documentation. It should look and feel like the *front
door* to the docs site, sharing enough visual language to feel like the
same project (this is what item 5's mockups explore), but it is not
where reference content lives.

## New docs site (websocketd.com/docs)

**What goes here:** everything issue #467 actually asked for — tutorial,
install, full reference, platform notes, deployment guides, best
practices, FAQ, cookbook. This is where the entire content of
`MANUAL_INVENTORY.md` lands. Full structure in `MANUAL_PLAN.md`.

**What does not:** project-governance content (contributing, how CI works,
`DIARY.md`-style engineering rationale) — that belongs on GitHub, next to
the code it explains.

**Opinion:** this is the one surface that gets to be comprehensive. Every
other surface above should be *short specifically because* this one
exists — a `--help`/README/homepage that tries to be complete duplicates
this site badly. The dev console has already established a visual
language (Option D) worth extending or deliberately departing from here —
see the item 5 mockups for both directions.

## GitHub Wiki

**Verdict: retire it.** Not "clean it up" — retire it. Three independent
findings converge on this:

1. **It's the thing issue #467 is complaining about.** README and the
   website's "Docs" nav link both point straight at the wiki today — it
   *is* the current manual, and it's the scrappy, distributed one.
2. **It's comprehensively stale and, in places, actively wrong.** No page
   has been edited since 2022; most substantive content is 2013-2019. Its
   CLI reference is a pasted `0.3.0 --help` dump missing at least 8 flags
   and stating the wrong `--maxforks` default. Its Go-embedding example
   doesn't compile. One page documents a bug as current behavior that's
   since been fixed.
3. **It's a standing security/integrity risk, not just a quality one.**
   It's publicly editable with no review gate (`_Footer.md` invites
   exactly that), and it was previously vandalized with malware/ransomware
   links (issues #432, #433, 2022) — cleaned up since, confirmed by this
   research, but the open-write door that allowed it hasn't been closed.

**What to do with it:** a handful of pages have salvageable bones —
`Environment-variables.md`'s structure, the Nginx/Apache reverse-proxy
recipes, the per-language example snippets. Hand-port the *content* of
those into the new docs site (verify each against current code first;
none of them can be trusted as-is), then take the wiki itself offline —
either disable the GitHub wiki feature for the repo, or at minimum unlink
it from README and the website nav and leave a single redirect page.
Don't leave it live-but-unlinked; a publicly-writable page with no
incoming links is still a publicly-writable page.

## GitHub Issues / Discussions

**What goes here:** actual bugs, actual feature requests.

**What does not, ideally:** "how do I..." questions that a real manual
would answer. The issues-mining research found that a large fraction of
all 339 closed issues were really support questions — stdout buffering,
one-process-per-connection, no built-in auth, the `file://`-origin
tutorial trap, and Windows signal handling account for a striking share of
the total on their own.

**Opinion:** this isn't purely a docs-placement decision, so treat it as a
secondary recommendation rather than part of the core plan — but consider
enabling **GitHub Discussions** and pointing "how do I" questions there (a
user proposed exactly this in 2019, #407, and it was never acted on). The
real fix is still the FAQ/cookbook existing at all; Discussions is a
release valve for whatever a written FAQ doesn't anticipate, not a
replacement for writing one.

## CHANGES / DIARY.md / CLAUDE.md

**What goes here (unchanged):** exactly what's there today. `CHANGES` is a
user-facing changelog (worth *linking* from the docs site, not
duplicating). `DIARY.md` is engineering rationale for maintainers —
genuinely useful raw material when *writing* the manual's "why does it
work this way" asides, but shouldn't be linked *from* user docs; a reader
following a link from the manual into `DIARY.md` lands in a different
register entirely (terse, dated, written for someone already holding the
whole codebase in their head). `CLAUDE.md` is agent/contributor operating
instructions, not documentation at all.

**Opinion:** no change needed here beyond fixing the drift already found
(`CLAUDE.md`'s wrong linter claim) — these are working correctly as
maintainer-facing surfaces and shouldn't be pulled toward a user audience.

---

## The one-sentence version of this whole document

**`--help`** is the seed. **Man page** and **docs site reference pages**
are generated from it. **README** and **homepage** are both short and
exist to point somewhere else — GitHub visitors toward the docs site or
the code, website visitors toward the docs site. **The docs site** is the
only surface allowed to be long. **The wiki** goes away.
