// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package browser

import (
	"net/http"
	"testing"

	"golang.org/x/net/html"
)

// requiredIDs are every id from the DOM contract in ASKS.md A1 that is
// expected to already exist in the STATIC served markup — not ids that only
// appear once JS has run and connected (e.g. .frame rows are dynamic and
// have no ids at all, so they are deliberately not in this list).
var requiredIDs = []string{
	"url", "connect", "status", "send", "sendbtn", "frames", "detail",
	"detail-opcode", "detail-size", "detail-body", "clear", "theme",
	"counters", "count-sent", "count-recv", "count-bytes", "empty",
}

// TestDevConsoleHTMLParsesAndMatchesDOMContract uses a real HTML5 parser
// (golang.org/x/net/html — available here because qa/browser is its own Go
// module; see CLAUDE.md and the go.mod in this directory for why it must
// NOT be added to the root module) to check two things a regex-based
// scanner cannot check as reliably:
//
//  1. Every id in the DOM contract exists in the static served markup, and
//     no id is duplicated (a duplicate id is legal-ish HTML5 that a real
//     parser happily accepts, and one that document.getElementById and CSS
//     #id selectors would silently and confusingly resolve to only the
//     first element).
//  2. <html> carries a data-theme attribute with a valid value, checked on
//     the parsed tree's actual attribute list rather than the source text.
//
// Note this is a different, complementary check from
// libwebsocketd/console_serving_test.go's hand-rolled tag-balance scanner,
// not a replacement for it: x/net/html's parser synthesizes an <html> node
// (with zero attributes) even when the source has no explicit <html> tag at
// all, so it cannot detect "the source omits an explicit <html> tag" the
// way the hand-rolled scanner does — HTML5 parsing is defined to recover
// from just about anything, so html.Parse returning a nil error is close to
// a vacuous check on its own. What IS load-bearing here is that the
// synthesized node's Attr list stays empty when nothing in the source
// declared it — confirmed against the actual pre-rebuild console body
// before relying on it (see the commit message for this file).
func TestDevConsoleHTMLParsesAndMatchesDOMContract(t *testing.T) {
	dc := startDevConsole(t)
	resp, err := http.Get(dc.URL())
	if err != nil {
		t.Fatalf("GET %s: %v", dc.URL(), err)
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		t.Fatalf("golang.org/x/net/html failed to parse the served console body: %v", err)
	}

	ids := map[string]int{}
	var htmlNode *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "html" && htmlNode == nil {
				htmlNode = n
			}
			for _, a := range n.Attr {
				if a.Key == "id" {
					ids[a.Val]++
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	for id, count := range ids {
		if count > 1 {
			t.Errorf("id %q appears on %d elements; ids must be unique", id, count)
		}
	}
	for _, id := range requiredIDs {
		if ids[id] == 0 {
			t.Errorf("no element with id=%q in the served static markup, want one per the DOM contract", id)
		}
	}

	if htmlNode == nil {
		t.Fatal("no <html> element in the parsed tree at all — x/net/html synthesizes one even when the source omits it, so this would mean parsing itself failed, not just a missing tag")
	}
	var theme string
	var hasTheme bool
	for _, a := range htmlNode.Attr {
		if a.Key == "data-theme" {
			theme, hasTheme = a.Val, true
		}
	}
	if !hasTheme {
		t.Error("<html> has no data-theme attribute in the served static markup")
	} else if theme != "auto" && theme != "light" && theme != "dark" {
		t.Errorf("<html data-theme> = %q, want one of auto|light|dark", theme)
	}
}
