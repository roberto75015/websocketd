// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// pages.yml assembles the site with `cp -R website/. _site/`, so every file
// under website/ is served from the root of websocketd.com. That is easy to
// forget when a working document is left next to the page it describes:
// website/REDESIGN_PLAN.md sat there for a week and would have been published
// at https://websocketd.com/REDESIGN_PLAN.md on the next deploy. Nothing else
// looks: the lychee job walks the assembled tree, and a markdown file whose
// own links resolve is not a broken link.
//
// Planning and research documents go in docs-archive/, which is not copied
// into the artifact. website/ holds the page and its assets only.
func TestNoPlanningDocsInPublishedWebsiteDir(t *testing.T) {
	root := repoRoot(t)
	skipIfNotACheckout(t, root)

	// pages.yml publishes a fresh checkout, so what it copies is what git
	// tracks; an untracked scratch file under website/ is never served.
	paths, err := listRepoPaths(root)
	if err != nil {
		t.Fatalf("%v", err)
	}

	var found []string
	haveIndex := false
	inWebsite := underPath("website")
	for _, rel := range paths {
		switch {
		case !inWebsite(rel):
		case rel == "website/index.html":
			haveIndex = true
		case strings.EqualFold(filepath.Ext(rel), ".md"):
			found = append(found, rel)
		}
	}

	// An emptied or renamed website/ would yield nothing and report a clean
	// pass having checked nothing at all.
	if !haveIndex {
		t.Fatalf("website/index.html is not tracked; the scan reached nothing to check")
	}

	if len(found) > 0 {
		t.Errorf("%d markdown file(s) under website/ would be published at the "+
			"site root by pages.yml's `cp -R website/. _site/`:\n\n    %s\n\n"+
			"Planning and research documents go in docs-archive/.",
			len(found), strings.Join(found, "\n    "))
	}
}
