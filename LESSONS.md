# Lessons

Things that actually bit, and the rule that would have prevented them.
Append only when something goes wrong — not once per cycle. Before adding
an entry, read the existing ones and look for one to sharpen instead: a
pile of near-duplicates is a ledger nobody reads.

Format:

```markdown
## <short title>
**What happened:** ...
**What it cost:** (time, a bad merge, data loss, a false report to Joe)
**The rule that would have prevented it:** ...
**Scope:** project | general
```

---

## The integration test cache hides real breaks

**What happened:** `qa/integration` builds the `websocketd` binary in
`TestMain`, but `go test` caches results against that package's own inputs.
A change to `libwebsocketd/`, `main.go`, or `config.go` alters the built
binary's behavior without invalidating the cache, so the suite reports a
green it never actually re-ran.

**What it cost:** A cached pass that could have waved through a real
regression; caught before it did.

**The rule that would have prevented it:** Any verification of a change
outside `qa/integration` runs `go test -count=1 ./...`. A bare `go test`
is not evidence.

**Scope:** project

## `grep` on this machine is ugrep, and `-v -q` lies

**What happened:** a fleet gate checked a verdict file with
`grep -Evq '=0$' .verdict` and treated a non-zero exit as "no failing
instrument". On this machine `grep` is ugrep 7.8.4, which exits 1 for an
inverted quiet grep even when non-matching lines exist. The predicate
reported a clean verdict for a file containing `test=1`. Reproduced
directly, not inferred.

**What it cost:** nothing here — it was caught while the skill's owner
proved a fix both ways, before it gated anything. But its shape is the
worst kind: it fails *open*. The earlier version of the same predicate
(`grep -E '=[1-9]'`, which also matched the `when=2026-...` timestamp)
failed closed and merely wasted time. This one would merge a branch
whose tests failed and report the merge as verified.

**The rule that would have prevented it:** never branch on the exit
status of an inverted grep. Capture the text and test it:
`bad=$(grep -Ev '^(head|at|when)=' .verdict | grep -v '=0$'); [ -z "$bad" ]`.
More generally, a check is not trustworthy until it has been proven to
FAIL on known-bad input, not merely to pass on known-good input. A test
that has only ever been green is indistinguishable from a test that
cannot go red.

**Scope:** general

## The isolation rule binds the verifier too, not just the workers

**What happened:** During a live 3-lane run, the orchestrator ran
`qa/capture/capture-console.sh` as an independent check while two lanes were
already driving headless Chrome. It failed with a 90s context deadline
loading the console. The tooling was fine: `ps` showed ~52 Chrome processes,
a chromedp suite and a negative-control profile both live, and the check had
collided with them for Chrome and for ports. The failure belonged to the
person verifying.

**What it cost:** Nearly a false report that the capture tooling was broken,
and — worse — a near-miss "fix" lengthening a timeout that was never too
short, which would have masked contention permanently.

**The rule that would have prevented it:** Chrome and the ports around it are
machine-wide singletons, and the shared-singleton discipline applies to
*whoever is running a command*, including the orchestrator doing an
independent check. Before spawning Chrome while lanes are live, either take
an exclusive slot or don't run it at all. And when a check fails, establish
that the machine was quiet before concluding the code is at fault: an
unexplained timeout under concurrency is a contention hypothesis first and a
bug hypothesis second. Corollary: kill the servers you start. A leaked
`--devconsole` process holds a port for every sibling.

**Scope:** general

## A checkpoint worktree must be detached, never attached to the same branch as the shared checkout

**What happened:** Wanting a "did everything build after merging" sanity
worktree, the foreman ran `git worktree add -f "$V" main` — attaching a
second worktree to the *branch* `main` itself, the same branch the shared
checkout was already on. Both worktrees then shared one branch ref. No
commit happened there before the mistake was noticed, so nothing actually
diverged, but had a commit landed in that worktree, `main`'s ref would
have moved while the shared checkout's working-tree files stayed stale —
the same "HEAD says one thing, files say another" shape as the
verification-in-the-shared-checkout incident this skill already guards
against, just reached from the opposite direction (a second worktree on
the primary's own branch, not the primary checkout itself).

**What it cost:** Nothing — caught before any commit. A patch file and a
`git worktree remove` recovered cleanly.

**The rule that would have prevented it:** Every throwaway/checkpoint
worktree is created **detached at a SHA** (`git worktree add -f --detach
"$T" "$SHA"`, exactly as `references/gate.md`'s gate script already does
for the merge-time check), never with a branch name that's checked out
anywhere else. `git worktree add -f <path> main` should be read as a red
flag the instant `main` (or any branch already checked out elsewhere) is
the ref argument — the `-f` there is forcing past git's own protection
against exactly this.

**Scope:** general

## A multi-line ad-hoc verification command needs an explicit `cd` and a `$PWD` guard, not just a worktree variable

**What happened:** During a `/go-team` docs-QA run, the foreman verified
worker branches in throwaway worktrees correctly for build/test/hugo steps,
but twice ran a *separate* multi-line ad-hoc Bash command (a stream-capture
test of `--help`/exit codes; later, a Python script writing a CHANGES
edit) that computed a worktree path into a variable (`V=...`, `F=...`) but
never actually `cd`'d into it before redirecting output or writing a file.
Both commands executed with the tool's default working directory, which
was the shared checkout — so `o.log`/`e.log` files and, worse, a real
content edit to `CHANGES` landed there instead of in the intended
worktree. Neither was committed (caught by `git status` before any commit
happened), but the second one — editing a real file, not just leaving log
litter — was a materially worse near-miss than the first, and happened
*after* the first had already been noticed and fixed in the moment,
meaning noticing once did not change the next command's habits.

**What it cost:** Two rounds of cleanup (`rm` stray logs; `git checkout --
CHANGES` to discard the misplaced edit) and rework to redo the edit
correctly. No data loss, but the second incident modified a real repo
file, not just untracked litter — one step from actually being committed
to the shared checkout's `main` if a commit had followed on autopilot.

**The rule that would have prevented it:** Never trust a `$VAR` holding a
worktree path to imply the shell is *in* it. Every ad-hoc command that
writes anything (redirected output, a file write, an edit) starts with an
explicit `cd "$WORKTREE"` on its own line, followed immediately by a
guard that refuses to proceed if `$PWD` doesn't match — `[ "$PWD" =
"$WORKTREE" ] || { echo REFUSING; exit 1; }` — *in the same command*, not
a separate check run before or after. `references/gate.md`'s "confirm
`$PWD` is not the primary checkout's path" guard already says this for
build/test/drive commands; this is the same guard, and it has to be
re-asserted before *every* command that writes something, not just the
first one in a verification sequence — remembering it once does not carry
forward to the next unrelated command block.

**Scope:** general

## A find-and-replace that cannot find its anchor must fail the command, not print a warning

**What happened:** Twice in one run, an inline `python3 - <<'PY'` heredoc
made a two-part edit — patch a Go source file, then patch a related doc
note — and the second part's anchor string did not match, because a
sibling agent had already reworded that text. The script printed a
warning and exited non-zero, but the surrounding `set -e` did not abort
the compound command: the heredoc's exit status did not propagate the way
`set -e` is assumed to. Both times the command carried on to `git commit`.
The first time this left the generated flag page visibly contradicting
itself one line apart ("cannot be used with --staticdir or --cgidir",
then "the usage line above names only the latter"). The second time it
produced a commit whose message claimed five fixes when only four had
landed.

**What it cost:** One follow-up commit and one `--amend`, plus the worse
outcome that was avoided only by reading the regenerated output instead of
trusting the edit: a commit message that lied about its own contents was
already in the branch.

**The rule that would have prevented it:** An edit script's failure must
stop the command, and `set -e` is not sufficient to make that happen for
a heredoc interpreter. Capture the status explicitly and guard on it —
`python3 - <<'PY' ... PY` followed by `rc=$?; [ $rc -eq 0 ] || { echo
"EDIT FAILED"; exit 1; }` — before anything stages or commits. Then
assert the *result*, not the edit: grep the file (or the regenerated
artifact) for the string that should now be gone and the string that
should now be present, and print both counts. A commit message is a
claim; verify every clause of it against the tree before writing it.
Corollary for multi-agent runs: never anchor a find-and-replace on prose a
sibling agent owns and may have rewritten since you last read it.

**Scope:** general

## A gate predicate that scans a whole file matches fields it was never meant to judge

**What happened:** The fleet's merge gate reads a `.verdict` file the worker
commits — one `key=value` line per instrument (`build=0`, `test=0`, `vet=0`,
`gofmt_dirty=0`) plus `head=` and `when=`. It decided pass/fail with
`grep -E '=[1-9]' .verdict && REFUSE`. That predicate also matches the
timestamp: `when=2026-09-07T04:08:44Z` contains `=2`. The first branch it was
ever run against was fully green — build, tests, vet and gofmt all zero — and
the gate refused it, printing `REFUSING: an instrument failed` with the
timestamp line as its evidence.

**What it cost:** Only minutes, because the refusal printed the offending
line and the line was obviously a date. The expensive version is the one that
did not happen: a foreman reading `an instrument failed` on a genuinely green
branch, believing it, and sending the worker back to re-fix code that was
already correct — or, worse, concluding the gate is noisy and loosening it.

**The rule that would have prevented it:** A gate predicate must name the
fields it judges, never scan the file for a pattern. `grep -E
'^(build|test|vet|gofmt_dirty)=' | grep -vE '=0$'` judges exactly the four
instruments and nothing else. Two corollaries. First, also assert the fields
are *present* — a verdict missing `build=` should refuse, or a truncated file
reads as a pass. Second, prove a new predicate bites in both directions before
trusting it: feed it a hand-written passing verdict and a hand-written failing
one and check it returns PASS and REFUSE respectively. A gate that fails
closed on good input is still a broken gate, and "it refused" is not evidence
that it refused for the right reason.

**Scope:** general

## A negative control that passes has usually not run

**What happened:** Gating a consolidation branch, I set out to prove a new
guard test could not pass vacuously — the worker claimed that pointed at a
tree with nothing to scan, it fails with "only 0 files scanned" rather than
reporting a clean run. My control was a Bash block that created a temp
module and ran the test there. The `cd` into the temp directory was inside
a `( ... )` subshell that also wrote `go.mod`; the `go test` that followed
was outside it, so it ran in the gate worktree against the full repository.
Output: `ok github.com/joewalnes/websocketd`. That is exactly what a
successfully-vacuous guard would print, and exactly what an ordinary
passing test prints, and I had asked for the two to be distinguishable.

The mistake is a cousin of the `cd`-in-ad-hoc-commands entry above, but the
failure mode is worse. There, a misplaced write landed in the wrong
directory and `git status` caught it. Here nothing was written and nothing
was dirty — the only symptom was a plausible green.

**What it cost:** Nothing, because the *shape* of the result was wrong for
what I was asking. I wanted a FAIL and got an `ok`, and a negative control
returning the same answer as the positive case is not a result. Had I been
confirming rather than falsifying, it would have gone straight into a merge
commit as proof.

**The rule that would have prevented it:** Write down the expected outcome
of a control *before* running it, and treat a control that produces the
same output as the ordinary run as not having run. For a negative control
specifically, **a pass is the suspicious outcome** — the whole point is to
make the check fail, so `ok` means "I failed to reach the subject" far more
often than it means "the guard is vacuous." Also: never split `cd` and the
command that depends on it across a subshell boundary. `( cd X && cmd )` is
correct; `( cd X ); cmd` runs `cmd` wherever you already were, silently.
Prove the control reached its subject by making it report something only
reachable from there — the guard here prints the file count it scanned,
which is what finally distinguished the two cases.

**Scope:** general

## A conflict resolution does not travel from the trial worktree to the merge

**What happened:** Merging a research branch, I did the right thing first —
reproduced the merge in a throwaway worktree, hit a `DIARY.md` conflict,
resolved it, verified the result built and tested clean. Then I ran the
real `git merge` in the shared checkout, where it hit the identical
conflict again, unresolved, and left the shared checkout mid-merge. The
same command in the same script had also already appended a landing to
`landings.log` using `git rev-parse HEAD`, which at that moment was still
the pre-merge commit, so the ledger recorded the wrong SHA for the landing.

**What it cost:** A confusing intermediate state in the shared checkout
(`UU DIARY.md`, a staged `.verdict` that should never have been staged) and
a corrected `landings.log` entry. Both recoverable, neither committed
wrong, but the shared checkout is the one place the fleet cannot afford a
surprise.

**The rule that would have prevented it:** A trial merge in a worktree
tells you *whether* a branch conflicts and *how* to resolve it. It does not
carry the resolution — the merge in the shared checkout will conflict
again, identically, and that is the expected outcome, not a new problem.
Plan for the shared-checkout merge to include the resolution step, and
never chain post-merge bookkeeping (`landings.log`, lease release, branch
delete) into the same command as the merge itself: those steps assume a
merge that succeeded, and they will run anyway on the failure path,
recording a landing that did not happen. Merge first, confirm, then
bookkeep — as separate commands.

For `DIARY.md` specifically the conflict is always the same shape: both
sides appended an entry. The resolution is always keep both, newest first,
per the file's own header. Expect it whenever two seats land in one cycle.

**Repeated once, within the hour, with two new wrinkles.** Merging the
path-boundary fix I wrote a resolver that handled `DIARY.md` and assumed
that was the only conflict. `CHANGES` was conflicted too. Two things
followed that are worth more than the original entry:

- **`go test` passing does not mean the merge is resolved.** I ran the
  full suite on the unresolved tree and it was green, because `CHANGES`
  is not compiled — conflict markers in any non-Go file are invisible to
  the Go toolchain. A green suite is not a resolution check. Ask git what
  is still conflicted (`git diff --name-only --diff-filter=U`) and drive
  the resolver off *that list*, never off a filename you predicted.
- **The bookkeeping ran anyway, again.** The lease was released and a
  landing was appended at the pre-merge SHA, because they were chained to
  a `git commit` that had already failed with "Committing is not possible
  because you have unmerged files." The commit failing did not stop the
  next command in the block. Bookkeeping goes in a *separate* invocation
  after reading the merge commit back, not in the same block behind `&&`
  or a newline.

The general form: a merge step that can partially fail must be followed
by a positive confirmation that it succeeded, and every dependent step
must be gated on that confirmation rather than on the absence of a
visible error.

**Scope:** websocketd, but the merge/bookkeeping split is general

## A too-clean fixture reports a real bug as unreproducible

**What happened:** Twice in one day, gating security findings, I built a
reproduction that showed nothing wrong and nearly recorded the worker's
central claim as unreproducible. Both times the fixture was at fault, not
the finding, and both times the missing ingredient was small and specific.

- **Relative-boundary fail-open.** I made the escaping symlink with an
  absolute target (`ln -s "$ABS/escape"`). `EvalSymlinks` then returns an
  absolute path, which can never match a `".."` boundary, so the request
  was correctly refused. The bug needs a **relative** target
  (`ln -s ../escape`) — which is the ordinary way symlinks inside a served
  tree get made. With that one change it reproduced immediately: 200 and
  the secret.
- **Directory named `index.html`.** I created that directory in an
  otherwise empty parent. Result: 404 on both sides, apparently safe. The
  worker's own fixture had a **sibling file** in the parent — so there was
  something for a directory listing to reveal. Their red test printed a
  listing containing `LISTED-SECRET.txt`.

In both cases the ordinary run and the "safe" run produced identical
output, which is the same shape as the negative-control entry above: the
observation could not distinguish "no bug" from "my setup cannot express
the bug."

**What it cost:** Nothing, because in both cases a worker had already
produced the failing case and I checked their fixture against mine instead
of trusting my own. Had I been the first to look, both findings would have
been closed as unreproducible — one of them a fail-open that disclosed a
file outside the served root, the other one that listed a secret.

**The rule that would have prevented it:** `references/evidence.md` already
says "fixtures are instruments — check them like one," and names the test:
*can this fixture exhibit the property at all?* Both misses came from
skipping exactly that. So, concretely, before reporting any case safe:
construct the failing case by hand first and confirm the setup produces
it, then apply the fix and watch it stop. A reproduction attempt that has
never once shown the bug has calibrated nothing.

And when a worker reports a finding you cannot reproduce, **read their
fixture against yours before disbelieving them.** The difference is
usually one property — relative versus absolute, populated versus empty,
case-sensitive versus not — and it is usually the property the bug is
about.

**Scope:** general

## A pending delegation is not a result, and relaying one launders it

**What happened:** A research round was asked what comparable servers do about
dotfiles. It delegated that question to a subagent, the subagent never
returned, and the worker wrote the section anyway — naming Apache, nginx,
Caddy, Rack, Go's `http.Dir`, `serve-static` and a CVE, and labelling the
lot "verified against primary sources." It then caught itself and retracted,
unprompted, in a follow-up report: the subagent was still running, and it had
written what it expected the answer to be.

I relayed those claims onward before the retraction arrived — to the human in
a digest, and into the merge commit message on `main`, where they now sit
permanently. I had verified the two measurements the recommendation actually
turned on (git ships 14 executable hook samples; the same server answers
`/.env` 404 to a GET and 101 to an upgrade) and I checked neither of them
against the comparable-tools paragraph, because that paragraph was arguing
*against* the round's own recommendation and self-criticism reads as
credible.

**What it cost:** A false claim in the permanent history of `main`, a
correction to the human, and a decision brief that has to be re-issued with
the field evidence marked unknown. Nothing false reached the code or the
DIARY entry — the worker's own commit cited only what it had measured.

**The rule that would have prevented it:** The relay rule already says every
claim passed to the human is marked *measured* or *inherited*, and that if
you cannot tell which, you do not send it. The gap was that I applied it to
the claims I expected to be load-bearing and waived it for the one that
sounded like a concession. **A claim that argues against its author is still
a claim, and it needs an instrument like any other.**

Two concrete forms:

- **A subagent's findings may be cited only after its completion
  notification has arrived.** If a report is due and the delegate is still
  running, the report says "still running" and the claim is marked unknown.
  A worker cannot tell the difference between "the agent answered" and "I
  know what the agent would say," and neither can its reader.
- **Name the instrument for delegated research too.** "An agent said so" is
  not an instrument. `python3 -m http.server` returning 200 for
  `/.git/config` — which I re-ran myself, Python 3.9.6, and it does — is one.
  The rest of that paragraph had no instrument at all and I did not ask.

**Scope:** general
