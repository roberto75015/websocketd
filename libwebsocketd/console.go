// Copyright 2013 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libwebsocketd

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// The console is a single HTML file containing all its CSS and JS inline,
// embedded into the binary at build time. Keeping it as a real .html file
// (rather than a Go string literal) means it can be opened directly in a
// browser during development, edited with normal tooling, and — since a Go
// raw string literal cannot contain a backtick — that its JavaScript is free
// to use template literals.
//
// We can get by without jQuery or Bootstrap for this one ;).

//go:embed console.html
var consoleHTML string

// ConsoleContent is the whole console page, expanded once at init.
//
// It is deliberately request-independent. The page used to carry an {{addr}}
// placeholder substituted per request with a Host- and RequestURI-derived
// WebSocket URL, which put attacker-controlled text inside an HTML attribute
// (reflected XSS, patched once by escaping). That substitution was already
// dead weight: the console overwrites the field from location.href on load,
// so the server-side value was never seen. Removing it kills the whole
// injection class structurally rather than escaping around it, and makes the
// response a constant that can carry a strong ETag.
var ConsoleContent string

// ConsoleCSP is the Content-Security-Policy for the console response. The
// script and style hashes are computed at init from the page that is actually
// served, never written out by hand: a hand-copied hash goes stale the first
// time someone edits the file, and a stale hash means a blank console with an
// error in the devtools log. Deriving them here makes "edit the HTML" the
// whole workflow.
//
// connect-src has to allow ws: and wss: generally — the point of the console
// is that the user types the URL — but everything else is denied, so a frame
// from a hostile server has no way to become a resource load.
var ConsoleCSP string

// ConsoleETag is a strong validator over the exact bytes of ConsoleContent.
// The body no longer varies with the request, so a repeat visit can be
// answered with a 304 rather than the whole page.
var ConsoleETag string

func init() {
	ConsoleContent = strings.Replace(consoleHTML, "{{license}}", License, -1)
	ConsoleCSP = consoleCSP(ConsoleContent)
	sum := sha256.Sum256([]byte(ConsoleContent))
	ConsoleETag = `"` + hex.EncodeToString(sum[:16]) + `"`
}

// consoleCSP builds the policy from the page content.
//
// It panics if the page carries no inline <script> or <style> — the content
// is embedded at build time and cannot depend on anything at runtime, so an
// empty hash list is a broken build, not a condition to degrade around.
// Serving an unusable console quietly is the worse failure.
func consoleCSP(content string) string {
	script := inlineHashes(content, "script")
	style := inlineHashes(content, "style")
	if len(script) == 0 || len(style) == 0 {
		panic(fmt.Sprintf("console page has %d inline <script> and %d inline <style> blocks; "+
			"the CSP hashes cannot be computed (are the tags carrying attributes?)", len(script), len(style)))
	}
	return strings.Join([]string{
		"default-src 'none'",
		"script-src " + strings.Join(script, " "),
		"style-src " + strings.Join(style, " "),
		"connect-src ws: wss:",
		"frame-ancestors 'none'",
	}, "; ")
}

// inlineHashes returns a CSP sha256 source expression for the body of every
// <tag>...</tag> block in content. The tags are matched without attributes
// because that is exactly how the console writes them; a tag that grew an
// attribute would drop out of the list and trip the panic above rather than
// producing a policy that silently fails to cover it.
func inlineHashes(content, tag string) []string {
	open, closing := "<"+tag+">", "</"+tag+">"
	var out []string
	rest := content
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			return out
		}
		rest = rest[i+len(open):]
		j := strings.Index(rest, closing)
		if j < 0 {
			return out
		}
		sum := sha256.Sum256([]byte(rest[:j]))
		out = append(out, "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		rest = rest[j+len(closing):]
	}
}
