// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"strings"
	"testing"
)

// The docs site was planned as a docs.websocketd.com subdomain and shipped at
// websocketd.com/docs instead, because GitHub Pages allows one custom domain
// per repository. The subdomain was never registered, so every link written
// against the plan is dead. Five of them reached examples/*/README.md before
// anyone noticed, and the planning document that recommended the subdomain sat
// at the repo root reading like live guidance for weeks afterwards.
//
// Nothing else catches this: the lychee job in pages.yml runs --offline over
// the built Hugo output, so it never sees a repo markdown file and would not
// resolve an external host if it did.
//
// docs-archive/ and DIARY.md are exempt: both are historical records that are
// supposed to preserve what was recommended at the time, wrong or not.
func TestNoDeadDocsSubdomainLinks(t *testing.T) {
	// Matched with the scheme separator so that *naming* the rejected
	// subdomain in prose (pages.yml explains why it was rejected) stays legal;
	// only an actual link to it fails.
	const deadHost = "//docs.websocketd.com"

	root := repoRoot(t)
	skipIfNotACheckout(t, root)

	// Anchored at the repository root, so a file that merely shares a name
	// with an exempt one is still checked.
	exempt := func(rel string) bool {
		return rel == "DIARY.md" ||
			rel == "dead_links_test.go" || // names the pattern it looks for
			strings.HasPrefix(rel, "docs-archive/")
	}

	files, err := scanRepoText(root, "the repository", 50, func(rel string) bool {
		return !exempt(rel)
	})
	if err != nil {
		t.Fatalf("%v", err)
	}
	t.Logf("scanned %d tracked text file(s)", len(files))

	var failures []string
	for _, f := range files {
		if idx := strings.Index(f.content, deadHost); idx >= 0 {
			failures = append(failures, f.rel+":\n    "+lineAround(f.content, idx))
		}
	}

	if len(failures) > 0 {
		t.Errorf("%d file(s) link to %s, which does not exist. "+
			"The docs site is served at https://websocketd.com/docs/:\n\n%s",
			len(failures), deadHost, strings.Join(failures, "\n"))
	}
}
