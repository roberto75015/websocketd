// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package browser

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// The selectors below are the DOM contract handed to both the TEST and
// implementation lanes for the dev-console rebuild (ASKS.md A1). They are
// authoritative: if the shipped console uses different ids, that is an
// implementation bug against the agreed contract, not a reason to loosen
// these tests.
//
//	<html data-theme="auto|light|dark">
//	#url (text input), #connect (button, data-state = disconnected|connecting|connected)
//	#status (textContent is the human state, data-state mirrors it)
//	#send (textarea), #sendbtn (button)
//	#frames container; rows are <button class="frame" data-dir="in|out|event" data-size="N">
//	#detail with #detail-opcode, #detail-size, #detail-body
//	#clear, #theme, #counters with #count-sent/#count-recv/#count-bytes, #empty
//
// Every test in this file is expected to fail against current main: the
// decade-old console has none of these ids. That is the point — these are
// guards for the rebuild, proven to bite before the implementation lands.

// newBrowserContext gives the caller a fresh headless Chrome tab with its
// own --user-data-dir (t.TempDir(), never shared — two chromedp sessions on
// the same profile directory crash each other) and a 30s overall budget.
func newBrowserContext(t *testing.T) context.Context {
	t.Helper()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromeExecPath),
		chromedp.UserDataDir(t.TempDir()),
	)
	actx, acancel := chromedp.NewExecAllocator(context.Background(), opts...)
	t.Cleanup(acancel)
	ctx, cancel := chromedp.NewContext(actx)
	t.Cleanup(cancel)
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	t.Cleanup(timeoutCancel)
	return ctx
}

// connectConsole navigates to the console and clicks #connect, then blocks
// until #connect reports data-state="connected". A console that never opens
// a socket fails right here, loudly, rather than the caller discovering it
// later from an empty #frames.
func connectConsole(t *testing.T, ctx context.Context, url string) {
	t.Helper()
	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#connect`),
		chromedp.Click(`#connect`),
		chromedp.Poll(`document.getElementById('connect') && document.getElementById('connect').dataset.state === 'connected'`,
			nil, chromedp.WithPollingTimeout(10*time.Second)),
	); err != nil {
		t.Fatalf("connecting via the dev console UI (#connect) at %s: %v", url, err)
	}
}

// sendMessage types msg into #send and clicks #sendbtn.
//
// Both steps go through typeSafe/clickSafe rather than chromedp.SendKeys /
// chromedp.Click directly - see the comment on those two in
// harness_test.go for why a plain post-connect chromedp.SendKeys or
// chromedp.Click on this page hangs until the context deadline.
func sendMessage(t *testing.T, ctx context.Context, msg string) {
	t.Helper()
	typeSafe(t, ctx, `#send`, msg)
	clickSafe(t, ctx, `#sendbtn`)
}

// waitForEchoedFrame blocks until at least one incoming frame row appears —
// i.e. the round trip (send -> backend -> receive) actually completed, not
// just that the outgoing frame was queued.
func waitForEchoedFrame(t *testing.T, ctx context.Context) {
	t.Helper()
	if err := chromedp.Run(ctx,
		chromedp.Poll(`document.querySelectorAll('#frames .frame[data-dir="in"]').length >= 1`,
			nil, chromedp.WithPollingTimeout(10*time.Second)),
	); err != nil {
		t.Fatalf("no incoming (echoed) frame appeared in #frames within 10s: %v", err)
	}
}

// TestConsoleConnectShowsConnectedState: connect, then check both halves of
// the "unmissable connection state" requirement (ASKS.md A1) — #connect's
// data-state and #status's mirrored data-state/textContent.
func TestConsoleConnectShowsConnectedState(t *testing.T) {
	dc := startDevConsole(t)
	ctx := newBrowserContext(t)
	connectConsole(t, ctx, dc.URL())

	var connectState, statusState, statusText string
	if err := chromedp.Run(ctx,
		chromedp.AttributeValue(`#connect`, "data-state", &connectState, nil),
		chromedp.AttributeValue(`#status`, "data-state", &statusState, nil),
		chromedp.Text(`#status`, &statusText),
	); err != nil {
		t.Fatalf("reading connection state from #connect/#status: %v", err)
	}
	if connectState != "connected" {
		t.Errorf("#connect data-state = %q, want %q", connectState, "connected")
	}
	if statusState != "connected" {
		t.Errorf("#status data-state = %q, want %q", statusState, "connected")
	}
	if strings.TrimSpace(statusText) == "" {
		t.Error("#status textContent is empty; it must be a human-readable connection state")
	}
}

// TestConsoleSendAndReceiveEcho drives an actual round trip against a real
// backend process (testcmd echo): connect, send one text frame, and confirm
// both the outgoing and the echoed-back incoming frame land in #frames with
// the right direction and byte size. This is the test that actually opens a
// socket, sends a frame, and reads one back.
func TestConsoleSendAndReceiveEcho(t *testing.T) {
	dc := startDevConsole(t)
	ctx := newBrowserContext(t)
	connectConsole(t, ctx, dc.URL())

	const msg = "hello from qa/browser"
	sendMessage(t, ctx, msg)
	waitForEchoedFrame(t, ctx)

	var outDir, outSize, inDir, inSize string
	if err := chromedp.Run(ctx,
		chromedp.AttributeValue(`#frames .frame[data-dir="out"]`, "data-dir", &outDir, nil),
		chromedp.AttributeValue(`#frames .frame[data-dir="out"]`, "data-size", &outSize, nil),
		chromedp.AttributeValue(`#frames .frame[data-dir="in"]`, "data-dir", &inDir, nil),
		chromedp.AttributeValue(`#frames .frame[data-dir="in"]`, "data-size", &inSize, nil),
	); err != nil {
		t.Fatalf("reading #frames row attributes: %v", err)
	}

	wantSize := fmt.Sprintf("%d", len(msg))
	if outDir != "out" {
		t.Errorf("outgoing frame row data-dir = %q, want %q", outDir, "out")
	}
	if outSize != wantSize {
		t.Errorf("outgoing frame row data-size = %q, want %q (byte length of the sent message)", outSize, wantSize)
	}
	if inDir != "in" {
		t.Errorf("incoming frame row data-dir = %q, want %q", inDir, "in")
	}
	if inSize != wantSize {
		t.Errorf("incoming (echoed) frame row data-size = %q, want %q — testcmd's echo mode returns the exact bytes it received", inSize, wantSize)
	}
}

// TestConsoleFrameSelectionShowsDetail selects the echoed frame row and
// checks the detail pane actually reflects it — opcode, size, and (this is
// the part a stub could fake least convincingly) the payload itself.
func TestConsoleFrameSelectionShowsDetail(t *testing.T) {
	dc := startDevConsole(t)
	ctx := newBrowserContext(t)
	connectConsole(t, ctx, dc.URL())

	const msg = "detail-pane-probe-98765"
	sendMessage(t, ctx, msg)
	waitForEchoedFrame(t, ctx)

	clickSafe(t, ctx, `#frames .frame[data-dir="in"]`)

	var opcode, size, body string
	if err := chromedp.Run(ctx,
		chromedp.Text(`#detail-opcode`, &opcode),
		chromedp.Text(`#detail-size`, &size),
		chromedp.Text(`#detail-body`, &body),
	); err != nil {
		t.Fatalf("reading #detail-opcode/#detail-size/#detail-body after selecting a frame row: %v", err)
	}

	if strings.TrimSpace(opcode) == "" {
		t.Error("#detail-opcode is empty after selecting a frame row")
	}
	if strings.TrimSpace(size) == "" {
		t.Error("#detail-size is empty after selecting a frame row")
	}
	if !strings.Contains(body, msg) {
		t.Errorf("#detail-body = %q, want it to contain the selected frame's payload %q", body, msg)
	}
}

// TestConsoleClearEmptiesTranscript checks #clear actually removes the rows
// (ASKS.md A1's ring-buffer requirement is about capping growth, but the
// user-facing contract here is simpler: clicking Clear empties the visible
// transcript) and that the teaching empty state (#empty) comes back.
func TestConsoleClearEmptiesTranscript(t *testing.T) {
	dc := startDevConsole(t)
	ctx := newBrowserContext(t)
	connectConsole(t, ctx, dc.URL())
	sendMessage(t, ctx, "to-be-cleared")
	waitForEchoedFrame(t, ctx)

	clickSafe(t, ctx, `#clear`)

	var remaining int
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('#frames .frame').length`, &remaining),
	); err != nil {
		t.Fatalf("counting frame rows after clear: %v", err)
	}
	if remaining != 0 {
		t.Errorf("%d frame row(s) remain in #frames after clicking #clear, want 0", remaining)
	}

	var emptyVisible bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`(function(){var e=document.getElementById('empty'); return !!e && !!e.offsetParent;})()`, &emptyVisible),
	); err != nil {
		t.Fatalf("checking #empty visibility after clear: %v", err)
	}
	if !emptyVisible {
		t.Error("#empty (the teaching empty state) is not visible after #clear empties the transcript")
	}
}

// TestConsoleThemeTogglePersistsAcrossReload exercises ASKS.md A1's "Light
// and dark, both first-class ... an explicit light/dark override that
// persists in localStorage and can be returned to auto." It deliberately
// does not assume which widget #theme is (a cycling button vs. some other
// multi-state control isn't pinned by the DOM contract beyond the id) —
// only that clicking it changes data-theme, and that the changed value
// survives a reload.
func TestConsoleThemeTogglePersistsAcrossReload(t *testing.T) {
	dc := startDevConsole(t)
	ctx := newBrowserContext(t)

	if err := chromedp.Run(ctx,
		chromedp.Navigate(dc.URL()),
		chromedp.WaitVisible(`#theme`),
	); err != nil {
		t.Fatalf("loading the console: %v", err)
	}

	var initial string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`document.documentElement.dataset.theme`, &initial)); err != nil {
		t.Fatalf("reading initial <html data-theme>: %v", err)
	}
	if initial != "auto" && initial != "light" && initial != "dark" {
		t.Fatalf("initial <html data-theme> = %q, want one of auto|light|dark", initial)
	}

	var after string
	const maxClicks = 3
	for i := 0; i < maxClicks && (after == "" || after == initial); i++ {
		if err := chromedp.Run(ctx,
			chromedp.Click(`#theme`),
			chromedp.Evaluate(`document.documentElement.dataset.theme`, &after),
		); err != nil {
			t.Fatalf("clicking #theme (attempt %d): %v", i+1, err)
		}
	}
	if after == initial {
		t.Fatalf("clicking #theme %d times never changed <html data-theme> away from the initial %q", maxClicks, initial)
	}
	if after != "auto" && after != "light" && after != "dark" {
		t.Fatalf("<html data-theme> after toggling = %q, want one of auto|light|dark", after)
	}

	if err := chromedp.Run(ctx, chromedp.Reload()); err != nil {
		t.Fatalf("reloading the page: %v", err)
	}
	if err := chromedp.Run(ctx, chromedp.WaitVisible(`#theme`)); err != nil {
		t.Fatalf("waiting for the console to finish reloading: %v", err)
	}

	var afterReload string
	if err := chromedp.Run(ctx, chromedp.Evaluate(`document.documentElement.dataset.theme`, &afterReload)); err != nil {
		t.Fatalf("reading <html data-theme> after reload: %v", err)
	}
	if afterReload != after {
		t.Errorf("<html data-theme> after reload = %q, want the toggled value %q to persist across reload (localStorage)", afterReload, after)
	}
}

// TestConsoleHostileFramesNeverRenderAsHTML pins ASKS.md A1's "Received
// data is never rendered as HTML — textContent only. Point the console at
// someone else's server and every frame is hostile." testcmd's echo mode
// sends back exactly what it's given, so this connects to a fully
// legitimate, cooperative backend and still proves the console does not
// trust the bytes it receives over the wire: it asserts on the live DOM
// (no <img>/<script> element materializes from the payload, and no
// injected script actually ran), not on the transcript's string content —
// a page could echo the raw bytes back as inert text and still "look"
// right in a substring check while quietly having an XSS hole elsewhere.
func TestConsoleHostileFramesNeverRenderAsHTML(t *testing.T) {
	dc := startDevConsole(t)
	ctx := newBrowserContext(t)
	connectConsole(t, ctx, dc.URL())

	var titleBefore string
	if err := chromedp.Run(ctx, chromedp.Title(&titleBefore)); err != nil {
		t.Fatalf("reading document title before sending hostile frames: %v", err)
	}

	const imgPayload = `<img src=x onerror="document.title='XSS'">`
	const scriptPayload = `<script>window.__consoleTestPwned = true;</script>`

	sendMessage(t, ctx, imgPayload)
	sendMessage(t, ctx, scriptPayload)
	if err := chromedp.Run(ctx,
		chromedp.Poll(`document.querySelectorAll('#frames .frame[data-dir="in"]').length >= 2`,
			nil, chromedp.WithPollingTimeout(10*time.Second)),
	); err != nil {
		t.Fatalf("waiting for both hostile frames to echo back: %v", err)
	}
	// Give any (would-be) injected script a moment to run, and any (would-be)
	// <img onerror> a moment to fail its load and fire, before checking.
	if err := chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond)); err != nil {
		t.Fatalf("sleeping after hostile frames arrived: %v", err)
	}

	var titleAfter string
	var pwned bool
	var hostileElements int
	if err := chromedp.Run(ctx,
		chromedp.Title(&titleAfter),
		chromedp.Evaluate(`window.__consoleTestPwned === true`, &pwned),
		chromedp.Evaluate(`document.querySelectorAll('#frames img, #frames script, #detail img, #detail script, #frames iframe, #detail iframe').length`, &hostileElements),
	); err != nil {
		t.Fatalf("checking for injection after hostile frames: %v", err)
	}

	if titleAfter != titleBefore {
		t.Errorf("document.title changed from %q to %q — an <img onerror> in a received frame executed", titleBefore, titleAfter)
	}
	if pwned {
		t.Error("window.__consoleTestPwned === true — a <script> in a received frame executed")
	}
	if hostileElements != 0 {
		t.Errorf("%d live img/script/iframe element(s) found inside #frames or #detail — received frame content was parsed as HTML instead of rendered as text", hostileElements)
	}

	// Selecting the frame and reading the detail pane should show the
	// payload as literal text, confirming it landed via textContent and not
	// innerHTML (which would have collapsed/altered it during parsing).
	clickSafe(t, ctx, `#frames .frame[data-dir="in"]`)
	var body string
	if err := chromedp.Run(ctx, chromedp.Text(`#detail-body`, &body)); err != nil {
		t.Fatalf("reading #detail-body: %v", err)
	}
	if !strings.Contains(body, imgPayload) {
		t.Errorf("#detail-body = %q, want it to contain the literal payload %q as text", body, imgPayload)
	}
}

// TestConsoleKeyboardSendContract pins ASKS.md A1's "Enter to send with
// Shift+Enter for a newline."
func TestConsoleKeyboardSendContract(t *testing.T) {
	dc := startDevConsole(t)
	ctx := newBrowserContext(t)
	connectConsole(t, ctx, dc.URL())

	// Focus once (chromedp.Focus does not use NodeVisible, so it is not
	// subject to the bug typeSafe/clickSafe work around - see
	// harness_test.go). Every key after that goes to whatever is focused
	// via a selector-less chromedp.KeyEvent, with no re-query in between.
	if err := chromedp.Run(ctx, chromedp.Focus(`#send`)); err != nil {
		t.Fatalf("focusing #send: %v", err)
	}
	if err := chromedp.Run(ctx,
		chromedp.KeyEvent("line one"),
		chromedp.KeyEvent(kb.Enter, chromedp.KeyModifiers(input.ModifierShift)),
		chromedp.KeyEvent("line two"),
	); err != nil {
		t.Fatalf("typing with Shift+Enter: %v", err)
	}

	var value string
	var outCount int
	if err := chromedp.Run(ctx,
		chromedp.Value(`#send`, &value),
		chromedp.Evaluate(`document.querySelectorAll('#frames .frame[data-dir="out"]').length`, &outCount),
	); err != nil {
		t.Fatalf("reading #send value / outgoing frame count after Shift+Enter: %v", err)
	}
	if !strings.Contains(value, "\n") {
		t.Errorf("#send value = %q after Shift+Enter, want it to contain a newline (Shift+Enter inserts a newline, it must not send)", value)
	}
	if outCount != 0 {
		t.Errorf("%d outgoing frame(s) present after Shift+Enter, want 0 — Shift+Enter must not send", outCount)
	}

	if err := chromedp.Run(ctx,
		chromedp.KeyEvent(kb.Enter),
		chromedp.Poll(`document.querySelectorAll('#frames .frame[data-dir="out"]').length >= 1`,
			nil, chromedp.WithPollingTimeout(10*time.Second)),
	); err != nil {
		t.Fatalf("plain Enter did not submit #send: %v", err)
	}
}

// TestConsoleSendHistoryUpDown pins ASKS.md A1's "persistent send history on
// up/down."
func TestConsoleSendHistoryUpDown(t *testing.T) {
	dc := startDevConsole(t)
	ctx := newBrowserContext(t)
	connectConsole(t, ctx, dc.URL())

	sendMessage(t, ctx, "first-history-entry")
	waitForEchoedFrame(t, ctx)
	sendMessage(t, ctx, "second-history-entry")
	if err := chromedp.Run(ctx,
		chromedp.Poll(`document.querySelectorAll('#frames .frame[data-dir="in"]').length >= 2`,
			nil, chromedp.WithPollingTimeout(10*time.Second)),
	); err != nil {
		t.Fatalf("waiting for the second echoed frame: %v", err)
	}

	if err := chromedp.Run(ctx, chromedp.Focus(`#send`), chromedp.KeyEvent(kb.ArrowUp)); err != nil {
		t.Fatalf("pressing Up in #send: %v", err)
	}
	var afterUp1 string
	if err := chromedp.Run(ctx, chromedp.Value(`#send`, &afterUp1)); err != nil {
		t.Fatalf("reading #send after first Up: %v", err)
	}
	if afterUp1 != "second-history-entry" {
		t.Errorf("#send after one Up = %q, want the most recently sent message %q", afterUp1, "second-history-entry")
	}

	if err := chromedp.Run(ctx, chromedp.KeyEvent(kb.ArrowUp)); err != nil {
		t.Fatalf("pressing Up again in #send: %v", err)
	}
	var afterUp2 string
	if err := chromedp.Run(ctx, chromedp.Value(`#send`, &afterUp2)); err != nil {
		t.Fatalf("reading #send after second Up: %v", err)
	}
	if afterUp2 != "first-history-entry" {
		t.Errorf("#send after two Up presses = %q, want the message before that, %q", afterUp2, "first-history-entry")
	}

	if err := chromedp.Run(ctx, chromedp.KeyEvent(kb.ArrowDown)); err != nil {
		t.Fatalf("pressing Down in #send: %v", err)
	}
	var afterDown string
	if err := chromedp.Run(ctx, chromedp.Value(`#send`, &afterDown)); err != nil {
		t.Fatalf("reading #send after Down: %v", err)
	}
	if afterDown != "second-history-entry" {
		t.Errorf("#send after Up,Up,Down = %q, want Down to move forward again to %q", afterDown, "second-history-entry")
	}
}

// TestConsoleCountersTrackSentReceived pins ASKS.md A1's "session counters
// (sent, received, bytes, duration)." #count-sent/#count-recv are asserted
// exactly (unambiguous: one message sent, one echoed back). #count-bytes'
// exact aggregation rule (sent-only, received-only, or both directions
// summed) isn't specified by the DOM contract, so it is checked only for
// "present, numeric, and at least as large as one frame's payload" rather
// than an exact value, to avoid overfitting an assumption the contract
// doesn't actually make.
func TestConsoleCountersTrackSentReceived(t *testing.T) {
	dc := startDevConsole(t)
	ctx := newBrowserContext(t)
	connectConsole(t, ctx, dc.URL())

	const msg = "counter-probe"
	sendMessage(t, ctx, msg)
	waitForEchoedFrame(t, ctx)

	var sent, recv, bytesText string
	if err := chromedp.Run(ctx,
		chromedp.Text(`#count-sent`, &sent),
		chromedp.Text(`#count-recv`, &recv),
		chromedp.Text(`#count-bytes`, &bytesText),
	); err != nil {
		t.Fatalf("reading #count-sent/#count-recv/#count-bytes: %v", err)
	}

	if strings.TrimSpace(sent) != "1" {
		t.Errorf("#count-sent = %q, want %q after sending exactly one message", sent, "1")
	}
	if strings.TrimSpace(recv) != "1" {
		t.Errorf("#count-recv = %q, want %q after exactly one echoed frame", recv, "1")
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(bytesText), "%d", &n); err != nil {
		t.Fatalf("#count-bytes = %q, want a plain integer: %v", bytesText, err)
	}
	if n < len(msg) {
		t.Errorf("#count-bytes = %d, want at least %d (the byte size of the one message round-tripped)", n, len(msg))
	}
}
