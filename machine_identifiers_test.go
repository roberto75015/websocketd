// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/user"
	"regexp"
	"strings"
	"testing"
)

// Documentation gets drafted by running real commands on a real machine and
// pasting the output. That is exactly how the tutorial ended up shipping the
// author's actual computer name inside its example transcripts. This test is
// the mechanism that stops it: it fails if the machine running the tests can
// be identified from anything committed to the repository.
//
// It deliberately keys off the *current* machine rather than a hardcoded list
// of forbidden strings. Whoever drafts docs — a person or an agent — does it
// on some machine, and that machine's own name and home directory are the
// values at risk of being pasted in. Checking against the live values catches
// the leak on whatever machine introduced it, and needs no maintenance.

// placeholders are the stand-ins documentation is expected to use. They must
// never be flagged, and they are what a writer should reach for instead of
// real output.
var placeholders = map[string]bool{
	"you":       true,
	"user":      true,
	"username":  true,
	"me":        true,
	"youruser":  true,
	"your-user": true,
}

// homePathRe matches an absolute home directory belonging to a named account,
// e.g. /Users/alice/... or /home/bob/... — the form a pasted shell transcript
// or an editor's absolute path takes.
var homePathRe = regexp.MustCompile(`(?:/Users|/home)/([A-Za-z0-9._-]+)`)

func TestNoLocalMachineIdentifiersInRepo(t *testing.T) {
	root := repoRoot(t)
	skipIfNotACheckout(t, root)

	// The identifying values of the machine running this test.
	var forbidden []struct{ what, value string }
	var skipped []string

	// A short name cannot identify anyone and will collide with ordinary
	// English. This machine's hostname was "Joes-Mac-Studio.local" when this
	// test was written and "Mac.localdomain" a few hours later, after a
	// network change: the short form became "Mac", which matches inside
	// "macOS", "machine", "machinery" and "BlinkMacSystemFont", failing the
	// build on twelve innocent files. Anything below this length is reported
	// as unchecked rather than matched, because a guard that fires on
	// everything gets deleted, and a guard that silently skips is worse than
	// one that says what it skipped.
	const minIdentifyingLen = 8

	consider := func(what, value string) {
		if len(value) < minIdentifyingLen {
			skipped = append(skipped, fmt.Sprintf("%s (%q) is too short to identify anything; not checked", what, value))
			return
		}
		forbidden = append(forbidden, struct{ what, value string }{what, value})
	}

	if host, err := os.Hostname(); err == nil && host != "" {
		consider("this machine's hostname", host)
		// A macOS hostname is often reported as "name.local" or "name.lan";
		// the bare name leaks just as much, so check it separately.
		if short, _, found := strings.Cut(host, "."); found && short != "" {
			consider("this machine's short hostname", short)
		}
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" && home != "/" {
		consider("this account's home directory", home)
	}

	if u, err := user.Current(); err == nil && u.Username != "" {
		// Checked only as a home-directory path segment, never bare: this
		// repository legitimately contains "joewalnes" in its module path and
		// URLs, and a bare username substring would flag every one of them.
		for _, prefix := range []string{"/Users/", "/home/"} {
			consider("this account's home path", prefix+u.Username)
		}
	}

	// Match on word boundaries rather than as a bare substring, so a real
	// identifier still cannot hide inside a longer word and an ordinary word
	// cannot be mistaken for one.
	matchers := make([]*regexp.Regexp, len(forbidden))
	for i, f := range forbidden {
		matchers[i] = regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(f.value) + `\b`)
	}

	// The scan floor is the shared traversal's: len(forbidden) == 0 below
	// covers having nothing to look *for*, and scanRepoText covers having
	// nothing to look *in*.
	files, err := scanRepoText(root, "the repository", 50, func(rel string) bool {
		// This test file names the patterns it looks for, so exempt it.
		return rel != "machine_identifiers_test.go"
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("scanned %d tracked text file(s)", len(files))

	var failures []string
	for _, f := range files {
		for i, forb := range forbidden {
			if loc := matchers[i].FindStringIndex(f.content); loc != nil {
				failures = append(failures, f.rel+": contains "+forb.what+" ("+forb.value+")\n    "+lineAround(f.content, loc[0]))
			}
		}

		for _, m := range homePathRe.FindAllStringSubmatch(f.content, -1) {
			if placeholders[strings.ToLower(m[1])] {
				continue
			}
			idx := strings.Index(f.content, m[0])
			failures = append(failures, f.rel+": contains a real-looking home directory ("+m[0]+")\n    "+lineAround(f.content, idx))
		}
	}

	// Say what was not checked, always. A guard that quietly narrows itself
	// is indistinguishable from one that passes, and this one narrows itself
	// automatically whenever the machine's name is short.
	if len(skipped) > 0 {
		t.Logf("not checked on this machine:\n  %s", strings.Join(skipped, "\n  "))
	}
	if len(forbidden) == 0 {
		t.Errorf("nothing about this machine was identifying enough to check "+
			"(%d value(s) skipped as too short). The guard passed without "+
			"testing anything, which is not the same as being clean:\n  %s",
			len(skipped), strings.Join(skipped, "\n  "))
	}

	if len(failures) > 0 {
		t.Errorf("committed files identify the machine they were written on.\n"+
			"Replace real values with a placeholder (example-host.local, /Users/you/, $HOME):\n\n%s",
			strings.Join(failures, "\n"))
	}
}
