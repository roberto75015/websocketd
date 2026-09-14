# websocketd docs: writing standard

Binding rules for every page under `docsite/content/`. If a page breaks a
rule here, the page is wrong.

Sources this is built on: [Diátaxis](https://diataxis.fr/) for structure,
the [Google developer documentation style
guide](https://developers.google.com/style) for voice and mechanics.

---

## 1. The four modes, and why a page may only be one

Diátaxis: documentation serves four distinct needs, and a page that
serves two serves neither well. Every page belongs to exactly one mode,
and its mode determines its shape.

| Mode | Section | Reader's state | Page answers |
|---|---|---|---|
| Tutorial | `start/` | "I've never used this" | "Take me through it once, successfully" |
| How-to | `how-to/` | "I have a specific job to do" | "Give me the steps for this exact task" |
| Reference | `reference/` | "I need a precise fact" | "What exactly does X do?" |
| Explanation | `understanding/` | "I want to understand why" | "Why does it work this way?" |

The commonest failure is a how-to that teaches, or an explanation that
drifts into steps. When a page starts doing both, split it and link.

**Worked example, the one that matters most here.** Output buffering is
the single largest source of support requests in this project's history.
It splits across two modes and must stay split:

- `how-to/languages/python.md` answers "Your Python script's output
  isn't appearing. Add `-u`." Fix first, no theory.
- `understanding/output-buffering.md` answers "Why runtimes block-buffer
  when stdout is a pipe." Theory, no steps.

Each links to the other. Neither repeats the other.

## 2. Voice

1. **Second person, active voice, present tense.** "You pass `--port`",
   not "the port may be passed".
2. **Front-load the answer.** The first sentence says what the reader
   gets or what is true. Justification comes after. A reader who stops
   after one sentence should still have the answer.
3. **One idea per sentence.** Three technical facts stacked into one
   sentence is the house defect. Split them.
4. **Conditions before instructions.** Write "If you're behind a proxy,
   set X". Do not write "Set X if you're behind a proxy."
5. **Sentence case headings.**
6. **Define jargon on first use, or link it.** Terms like *shebang*,
   *block-buffered*, *pty*, and *origin* each need a clause of definition
   the first time a page uses them. Do not assume the reader arrived from
   another page. They arrived from a search engine.
7. **State the fact, not your assessment of it.** The landing page once
   said "The trade is one process per connection." A trade has two sides
   and that names one, and it puts an abstraction where the fact belongs.
   Write "websocketd starts one process per connection", then say what it
   costs. Watch for the pattern `The <abstract noun> is <fact>`: the
   trade, the point, the tell, the catch. It is nearly always a fact
   with a stance bolted to the front. Delete the stance. Naming a cost is fine when you are genuinely
   weighing two options and the cost is the information the reader needs.
8. **One rhetorical figure per page, and count only the decorative
   ones.** Antithesis ("not a bug, a permanent limitation"), the short
   sentence held back for emphasis, the rule of three. Each works once.
   At one per paragraph they stop being emphasis and become the voice,
   and the reader loses the ability to tell which sentence you meant them
   to slow down for. If a sentence carries no fact and exists for rhythm,
   cut it.

   The exception, which matters because this rule invites mechanical
   misapplication: a contrast that corrects a **specific wrong model the
   reader is likely to hold** is information, not decoration. "It is a
   snapshot, not a channel" and "an allowlist, not a passthrough" each
   name the mistaken belief and replace it, and a page explaining a
   contract may need several. Count the ones that would survive being
   rewritten as a plain statement without loss; those are the decorative
   ones. Do not strip a page to satisfy a number.
9. **Never narrate the reader's feelings.** The tutorial once printed a
   forty-line security banner under the words "looks much more alarming",
   then followed it with "Nothing has gone wrong." Both halves are the
   same mistake: the text managing an emotional reaction instead of
   conveying information. If output matters, say what it means and what
   to do about it. If it does not matter, do not print it and then
   apologise for printing it. Showing something and telling the reader to
   ignore it costs the page twice: the space it takes, and the habit it
   teaches of ignoring warnings.
10. **State the mechanism, not just the steps.** A reader could once
    finish the whole tutorial without being told the one thing
    `websocketd` does: that each line a program prints to stdout is sent
    to the browser as one WebSocket message. The page described the
    script, the server and the browser, and left the causal link between
    them implied. Every page that shows a thing working owes the reader
    one plain sentence saying what is actually happening, placed where
    they first watch it happen.
11. **No em dashes.** Not the character `—`, and not the HTML forms
    `&mdash;`, `&#8212;` or `&#x2014;` that render as it. The dash is the
    clearest signal that a page was generated rather than written, and it
    is almost never carrying weight a better sentence could not carry: it
    is holding together two clauses that wanted to be two sentences, or
    fencing an aside that wanted to be cut.

    Fix it by rewriting the sentence, not by substituting punctuation.
    Swapping the dash for a comma, a colon, or a pair of brackets leaves
    the same sentence standing and satisfies nothing but the checker.

    > ❌ websocketd refuses any path with a segment beginning with `.`
    > — so a `.git` directory cannot be fetched by name — and refuses
    > to list a directory that has no index file.
    > ✅ websocketd refuses any path with a segment beginning with `.`,
    > so a `.git` directory or an `.env` file cannot be fetched by name.
    > It also refuses to list a directory that has no index file.

    In a link list the dash is separating a term from its definition, and
    the fix is to make the link the subject of a sentence. Most of the
    site already does this, so it is a convergence rather than a new
    convention:

    > ❌ `- [CLI flags](/reference/cli-flags/) — what each flag does.`
    > ✅ `- [CLI flags](/reference/cli-flags/) describes what each flag does.`

    A colon is right where the second half genuinely is a list or a
    definition, as in the sentence you are reading. It is not a
    general-purpose stand-in for the dash.

    Enforced by `TestNoEmDashInLiveDocs` in the root Go package, which
    checks all four spellings. Checking only the character is not enough:
    searching `website/index.html` for it finds nothing while the file
    holds four `&mdash;` entities, one of them in the `<title>`. Code
    blocks, inline code spans, URLs, HTML `<pre>` and `<code>` elements,
    and blockquotes are exempt, because a code sample and a quotation have
    to match their source exactly.

    The en dash `–` is not banned. It has a legitimate mechanical use in
    numeric ranges (`#472–#475`), it is not a marker of register, and no
    page under `docsite/content/` contains one.
12. **Don't reach for the vocabulary that marks generated prose.** The em
    dash is the loudest of these but not the only one. Treat as suspect:
    "the honest truth", "load-bearing", "it's worth noting", "that said",
    "delve", "crucial", "seamless", "robust", "leverage" used as a verb,
    "in today's world", "at the end of the day", "out of the box", and the
    "it's not just X, it's Y" swivel. Each arrives with the draft rather
    than being chosen, and each can be deleted or replaced with the plain
    thing it is standing in for.

    This rule is deliberately **not** enforced by a test, and the split
    matters more than the coverage. Every phrase above has a legitimate
    use, and this repository contains legitimate uses:
    `libwebsocketd/http.go` says a particular check "is not load-bearing
    today", which is the precise technical meaning and exactly right. A
    checker that failed the build on that sentence would be switched off
    within a week, and a switched-off checker is worse than none. The em
    dash is mechanical and can fail a build; a word is a judgement and
    belongs here, where a human applies it.

    Measured when this rule was written: none of these phrases appeared
    anywhere under `docsite/content/`. The rule is a constraint on the
    next draft, not a description of a defect. It is here so that the
    next writer does not have to be told in conversation, which is how
    the em dash got in.

## 3. Never write about the documentation

This is the defect that got four pages rewritten before this standard
existed. No page and no section may open by describing how the
documentation was produced, verified, generated, or kept accurate:

> ❌ This page is generated from websocketd's own flag definitions by
> `tools/gendocs`, so it can't drift out of sync.
> ❌ Verified line-by-line against `env.go`, `main.go`, and `config.go`.
> ❌ websocketd went through one coherent hardening pass, followed by a
> second wave adding…

The reader came for the subject, not the process behind the page. Where
provenance genuinely helps a reader (a generated page, so they know not
to edit it by hand), it goes in a single line at the *foot* of the page.

Related: don't narrate the project's development history in user docs.
"Was requested many times and declined" is fine in
`understanding/design-decisions.md`, where the reader is explicitly
there for the reasoning. It is noise anywhere else.

## 4. Write as if 0.5.0 is current

No "coming soon", no "not yet released", no version-gap caveats, no
apologising for a missing binary. State what is true of the current
release, plainly.

## 5. Mode-specific shape

### Tutorial (`start/`)
- One path. No options, no "you could also", no branching.
- Every command is copy-pasteable exactly as written, in order.
- Show real output after each command, including alarming-but-normal
  output.
- Name a trap *at the moment the reader would hit it*, not in a
  preamble.
- Ends with a recap of what the reader now knows, and exactly where to
  go next.
- It must work start to finish for someone who has never seen
  websocketd. That is the only success criterion.

### How-to (`how-to/`)
- Title names the goal, not the tool: "Serve behind nginx", not "nginx".
- First sentence states the goal and the shape of the solution.
- The working solution comes first. Caveats, variations, and
  explanation come after it, or as links.
- Assumes competence. Don't teach WebSocket, Docker, or Go here.
- If the reader needs the *why*, link to `understanding/`.

### Reference (`reference/`)
- Facts only. No persuasion, no narrative, no opinion.
- Uniform structure per entry, so entries can be scanned and compared.
- Tables wherever the data is tabular.
- Completeness matters more than readability. Every flag, every
  variable, every exit code, no exceptions.
- Gotchas belong here *only* as a factual note on the entry they affect
  ("A negative value is rejected"), never as advice ("you should…").

### Explanation (`understanding/`)
- Discursive prose. No numbered steps.
- Free to discuss alternatives, trade-offs, history, and things that
  were tried and rejected.
- Answers "why", "why not", and "what if" questions.
- Never the only place a fact appears. If a reader needs the fact to do
  their job, it also belongs in reference or how-to.

## 6. Cross-linking

- Every page links onward. A page with no exits is a dead end.
- Link on the words that name the target ("see [message
  framing](/understanding/message-framing/)"), never "click here".
- A fact lives in exactly one place. Everything else links to it.

## 7. Never commit anything that identifies a machine

Good documentation shows real output. Real output comes from running real
commands on a real computer, and that computer's name, your account name,
and your home directory come along with it. The tutorial shipped the
author's actual machine name inside its example transcripts this way.

Use these placeholders instead, always:

| Instead of | Write |
|---|---|
| Your machine's name | `example-host.local` |
| Your home directory | `/Users/you/…`, or better, `$HOME/…` |
| Your account name | `you` |
| A LAN address | `127.0.0.1`, or an [RFC 5737](https://www.rfc-editor.org/rfc/rfc5737) documentation address (`192.0.2.0/24`) |

When a placeholder appears in transcript output, say so in the
surrounding prose. A reader who sees `example-host.local` in a code
block and then something else in their own terminal needs to know the
two agree. The tutorial does this where it shows the startup banner.

This is enforced, not trusted: `TestNoLocalMachineIdentifiersInRepo` in
the root package fails the build if the hostname, home directory, or
account name of the machine running the tests appears anywhere in the
repository, and flags any other real-looking `/Users/<name>` or
`/home/<name>` path. Paste output freely while drafting; the test tells
you what to scrub before it can land.

## 8. The FAQ is a signpost, not a section

`faq/` exists because the research found a handful of questions that
account for a large share of this project's entire issue history.
It contains one- or two-sentence answers and a link to the page that
owns each answer. It must never become the place a fact lives.
