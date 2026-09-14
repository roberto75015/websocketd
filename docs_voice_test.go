// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The em dash is banned in this project's live documentation. The rule was
// given twice in conversation and applied by hand both times, and both times
// it went no further than the pages open at that moment: it was never written
// into docsite/STYLE.md, so the next writer reproduced it. The rebuilt
// homepage shipped four &mdash; entities within an hour of the second
// telling, and docsite/content/ still carried 31 of the literal character
// across ten pages.
//
// An instruction that lives only in a transcript binds nobody. This test is
// the mechanism that makes it bind. The rule and the reason are in
// docsite/STYLE.md §2.11; this file only enforces the mechanical half of it.
//
// The entity forms are checked because the literal character alone is not
// enough. Searching website/index.html for "—" finds nothing and reports the
// file clean; it holds &mdash; in four places, including the <title>.

// emDashRe matches every spelling of an em dash a writer or an editor might
// produce: the character itself, and the three HTML forms that render as it.
var emDashRe = regexp.MustCompile(`(?i)\x{2014}|&mdash;|&#8212;|&#x2014;`)

// proseRoot is one place live user-facing prose lives, plus the number of
// text files the scan must find there. The floor exists so a root that moves
// or is renamed fails the build instead of silently contributing nothing: a
// guard that reaches no files reports the same clean pass as a guard that
// reaches clean files.
type proseRoot struct {
	path     string
	minFiles int
}

// proseRoots is deliberately a short explicit list rather than the whole
// repository. DIARY.md, LESSONS.md, ASKS.md, docs-archive/ and DOCS_RESEARCH/
// are records of what was written at the time and are supposed to keep their
// original wording, right or wrong. Only copy a user reads as current
// documentation is covered.
var proseRoots = []proseRoot{
	{"docsite/content", 30},
	{"docsite/STYLE.md", 1},
	{"website/index.html", 1},
	{"README.md", 1},
}

func TestNoEmDashInLiveDocs(t *testing.T) {
	root := repoRoot(t)
	skipIfNotACheckout(t, root)

	findings, scanned, err := scanProseForEmDashes(root)
	if err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("scanned %d file(s) across %d root(s)", scanned, len(proseRoots))

	if len(findings) > 0 {
		t.Errorf("%d em dash(es) in live documentation. See docsite/STYLE.md §2.11: "+
			"an em dash is nearly always two sentences that were not split, or an "+
			"aside that should have been cut. Rewrite the sentence; do not swap the "+
			"dash for a comma or a colon.\n\n%s",
			len(findings), strings.Join(findings, "\n"))
	}
}

// scanProseForEmDashes scans every configured prose root under repoRoot and
// returns one finding per em dash found outside an exempt context. It returns
// an error, rather than an empty result, if any root is missing or yields
// fewer files than its floor; both cases come from the shared traversal, which
// reports them separately.
func scanProseForEmDashes(repoRoot string) (findings []string, scanned int, err error) {
	for _, r := range proseRoots {
		files, ferr := scanRepoText(repoRoot, "prose root "+r.path, r.minFiles, underPath(r.path))
		if ferr != nil {
			return nil, scanned, ferr
		}
		for _, f := range files {
			scanned++
			for _, d := range findEmDashes(f.rel, f.content) {
				findings = append(findings, "  "+f.rel+": "+d)
			}
		}
	}
	return findings, scanned, nil
}

// findEmDashes returns a description of every em dash in content that is not
// inside an exempt context, one per finding, each naming the line it sits on.
func findEmDashes(path, content string) []string {
	masked := maskExemptRegions(path, content)
	var out []string
	for _, loc := range emDashRe.FindAllStringIndex(masked, -1) {
		out = append(out, fmt.Sprintf("%q at offset %d\n      %s",
			masked[loc[0]:loc[1]], loc[0], lineAround(content, loc[0]))) //nolint:gocritic
	}
	return out
}

var (
	// Markdown fenced code blocks. A fence is three or more backticks or
	// tildes at the start of a line.
	fenceRe = regexp.MustCompile("(?m)^[ \t]*(?:`{3,}|~{3,})")
	// Inline code spans. Never span a newline, so a stray backtick in prose
	// cannot swallow the rest of the file.
	inlineCodeRe = regexp.MustCompile("`[^`\n]*`")
	// A blockquote line: quoting someone else's words verbatim, including a
	// historical document, must not be silently re-punctuated to satisfy a
	// grep. Requoting is the writer's decision, not the checker's. This is
	// also how docsite/STYLE.md is able to show the ❌ example of the thing
	// it bans without failing its own rule.
	//
	// Leading whitespace is unbounded rather than CommonMark's three spaces,
	// because a blockquote nested inside a list item is indented to the
	// item's content column, which is where STYLE.md's own examples sit.
	blockquoteRe = regexp.MustCompile(`(?m)^[ \t]*>.*$`)
	// A URL is a literal string that has to match its target exactly.
	urlRe = regexp.MustCompile(`(?i)\bhttps?://[^\s"'<>)\]]+`)
	// HTML code and preformatted regions, and HTML comments. Go's regexp is
	// RE2 and has no backreferences, so each element gets its own pattern
	// rather than one with a \1 closing tag.
	htmlCodeRes = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<pre\b[^>]*>.*?</pre>`),
		regexp.MustCompile(`(?is)<code\b[^>]*>.*?</code>`),
		regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`),
		regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`),
	}
	htmlCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// maskExemptRegions blanks out every region an em dash is legitimately
// allowed to appear in, replacing it with spaces so that byte offsets into
// the result still index the original content. A checker that only proves it
// catches bad input is trivially satisfied by one that flags everything; the
// exempt contexts are the half that stops this one doing that.
func maskExemptRegions(path, content string) string {
	b := []byte(content)

	blank := func(re *regexp.Regexp) {
		for _, loc := range re.FindAllIndex(b, -1) {
			for i := loc[0]; i < loc[1]; i++ {
				if b[i] != '\n' {
					b[i] = ' '
				}
			}
		}
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		maskMarkdownFences(b)
		blank(inlineCodeRe)
		blank(blockquoteRe)
	case ".html", ".htm":
		blank(htmlCommentRe)
		for _, re := range htmlCodeRes {
			blank(re)
		}
	}
	blank(urlRe)

	return string(b)
}

// maskMarkdownFences blanks the body of every fenced code block, including
// the fence lines themselves. Fences toggle: an odd trailing fence blanks to
// end of file, which is the conservative reading of a malformed document.
func maskMarkdownFences(b []byte) {
	locs := fenceRe.FindAllIndex(b, -1)
	for i := 0; i+1 < len(locs); i += 2 {
		start := locs[i][0]
		end := lineEnd(b, locs[i+1][0])
		for j := start; j < end; j++ {
			if b[j] != '\n' {
				b[j] = ' '
			}
		}
	}
	if len(locs)%2 == 1 {
		for j := locs[len(locs)-1][0]; j < len(b); j++ {
			if b[j] != '\n' {
				b[j] = ' '
			}
		}
	}
}

func lineEnd(b []byte, from int) int {
	if i := strings.IndexByte(string(b[from:]), '\n'); i >= 0 {
		return from + i
	}
	return len(b)
}

// TestEmDashGuardCatchesEveryForm proves the predicate bites on each spelling
// separately. Searching only for the literal character reports
// website/index.html clean while it holds four &mdash; entities, so the
// entity forms are not decoration.
func TestEmDashGuardCatchesEveryForm(t *testing.T) {
	for _, form := range []string{"—", "&mdash;", "&MDASH;", "&#8212;", "&#x2014;", "&#X2014;"} {
		doc := "Some prose " + form + " and an aside.\n"
		if got := findEmDashes("page.md", doc); len(got) != 1 {
			t.Errorf("form %q: got %d finding(s), want 1: %v", form, len(got), got)
		}
		if got := findEmDashes("page.html", doc); len(got) != 1 {
			t.Errorf("form %q in html: got %d finding(s), want 1: %v", form, len(got), got)
		}
	}
}

// TestEmDashGuardIgnoresExemptContexts is the half that stops the guard being
// a checker that flags everything. Each case is a place an em dash is
// legitimate and must survive.
func TestEmDashGuardIgnoresExemptContexts(t *testing.T) {
	cases := []struct{ name, path, doc string }{
		{"fenced code block", "page.md",
			"Prose.\n\n```\necho \"a — b\"\n```\n\nMore prose.\n"},
		{"tilde fenced block", "page.md",
			"Prose.\n\n~~~\necho \"a — b\"\n~~~\n\nMore prose.\n"},
		{"inline code span", "page.md",
			"Write the rule as `—` when you have to name it.\n"},
		{"blockquote of a historical document", "page.md",
			"He wrote:\n\n> The trade — and it is a trade — is one process.\n\nEnd.\n"},
		{"blockquote indented inside a list item", "page.md",
			"1. **A rule.** Reason.\n\n    > ❌ a — b\n    > ✅ a. B.\n\n2. Next.\n"},
		{"url", "page.md",
			"See https://example.com/a—b for the original.\n"},
		{"html pre", "page.html",
			"<p>Prose.</p>\n<pre>echo a &mdash; b</pre>\n"},
		{"html code element", "page.html",
			"<p>Use <code>a &#8212; b</code> here.</p>\n"},
		{"html comment", "page.html",
			"<!-- was: turn any program &mdash; into a server -->\n<p>Prose.</p>\n"},
		{"html script", "page.html",
			"<script>var s = \"a — b\";</script>\n"},
	}
	for _, c := range cases {
		if got := findEmDashes(c.path, c.doc); len(got) != 0 {
			t.Errorf("%s: guard fired on an exempt context (%d finding(s)): %v", c.name, len(got), got)
		}
	}
}

// TestEmDashGuardStillSeesProseAroundExemptRegions guards the guard: masking
// a code block must not mask the paragraphs on either side of it.
func TestEmDashGuardStillSeesProseAroundExemptRegions(t *testing.T) {
	doc := "Before — an aside.\n\n```\necho \"a — b\"\n```\n\nAfter — another.\n"
	if got := findEmDashes("page.md", doc); len(got) != 2 {
		t.Errorf("got %d finding(s), want 2 (the two prose dashes, not the fenced one): %v", len(got), got)
	}
}

// TestEmDashGuardFailsOnAnEmptyTree is the negative control. Pointed at a
// checkout with nothing to scan, the guard must report an error rather than a
// clean pass. A pass here would mean every future clean run is worthless,
// because the two outcomes would be indistinguishable.
func TestEmDashGuardFailsOnAnEmptyTree(t *testing.T) {
	empty := t.TempDir()
	initTestRepo(t, empty)
	findings, scanned, err := scanProseForEmDashes(empty)
	if err == nil {
		t.Fatalf("scanning an empty checkout reported success (%d finding(s), %d file(s) scanned); "+
			"the guard can pass without reaching anything", len(findings), scanned)
	}
	// Assert on the reason, not just on failure. A control that failed for
	// some unrelated reason would look identical to one that worked.
	if !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("failed for the wrong reason, so the control proves nothing: %v", err)
	}
	t.Logf("negative control failed as required: %v", err)

	// A directory that is no checkout at all has no index to read. That has to
	// be loud too, and loud for its own reason: an enumeration that returned
	// nothing instead of failing would land here as "unreachable" and look
	// identical to the case above.
	_, _, err = scanProseForEmDashes(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "outside a git checkout") {
		t.Fatalf("scanning a directory that is not a git checkout must fail for "+
			"having no index to read, got: %v", err)
	}
}

// TestEmDashGuardFailsOnAnUnderpopulatedRoot proves the per-root floor, not
// just the missing-root case: a root that exists but has been emptied out
// must fail too.
func TestEmDashGuardFailsOnAnUnderpopulatedRoot(t *testing.T) {
	fake := t.TempDir()
	for _, r := range proseRoots {
		abs := filepath.Join(fake, r.path)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("preparing %s: %v", r.path, err)
		}
		if r.minFiles > 1 {
			// The directory root: create it, but leave it nearly empty.
			if err := os.MkdirAll(abs, 0o755); err != nil {
				t.Fatalf("preparing %s: %v", r.path, err)
			}
			if err := os.WriteFile(filepath.Join(abs, "one.md"), []byte("clean prose\n"), 0o644); err != nil {
				t.Fatalf("preparing %s: %v", r.path, err)
			}
			continue
		}
		if err := os.WriteFile(abs, []byte("clean prose\n"), 0o644); err != nil {
			t.Fatalf("preparing %s: %v", r.path, err)
		}
	}
	initTestRepo(t, fake)
	_, _, err := scanProseForEmDashes(fake)
	if err == nil {
		t.Fatal("a root holding one file passed the floor; the floor is not enforced")
	}
	// Every root exists here, so an "unreachable" error would mean the setup
	// is wrong and the floor was never reached.
	if !strings.Contains(err.Error(), "fewer than") {
		t.Fatalf("failed for the wrong reason, so the floor is untested: %v", err)
	}
	t.Logf("floor control failed as required: %v", err)
}
