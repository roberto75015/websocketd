// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// capture-console drives a real, headless Chrome against a running
// websocketd --devconsole instance and produces the screenshots and demo
// recording referenced from README.md and website/index.html.
//
// It is invoked by qa/capture/capture-console.sh, which builds the
// websocketd binary, starts the server, and tears it down again; this
// program only owns the browser side: load the console, connect, send a
// frame, read back the echo, select the frame, screenshot both themes, and
// assemble a short demo video of the whole sequence.
//
// This is a separate Go module (its own go.mod) precisely so the chromedp
// dependency it needs never lands in the top-level module that produces the
// shipped websocketd binary — this tool is dev/doc tooling, not part of the
// product.
//
// Selector note: the CSS selectors below target the DOM contract of the
// rebuilt console (issue #466) in libwebsocketd/console.html. A page that
// does not implement it fails this driver loudly and fast, because none of
// these ids exist there.
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// Selectors for the console's DOM contract (issue #466). See the package
// comment above.
const defaultMessage = "hello devconsole"

// squash removes all whitespace, so a payload can be compared against a
// pretty-printed rendering of itself without asserting an exact indent
// style the DOM contract never promised.
func squash(s string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
}

const (
	selURL       = "#url"
	selConnect   = "#connect" // data-state: disconnected|connecting|connected
	selStatus    = "#status"
	selSend      = "#send" // textarea: Enter sends, Shift+Enter newline
	selSendBtn   = "#sendbtn"
	selFrames    = "#frames" // rows: <button class="frame" data-dir="in|out|event" data-size="N">
	selDetail    = "#detail"
	selDetailOp  = "#detail-opcode"
	selDetailSz  = "#detail-size"
	selDetailBdy = "#detail-body"
	selClear     = "#clear"
	selTheme     = "#theme"
	selCountSent = "#count-sent"
	selCountRecv = "#count-recv"
	selCountByte = "#count-bytes"
	selEmpty     = "#empty"

	viewportWidth  = 1200
	viewportHeight = 800
)

type frame struct {
	index     int
	path      string
	timestamp time.Time
}

// recorder collects the CDP screencast frames that become the demo video.
// They arrive on chromedp's event goroutine while the main goroutine is
// driving the page, so the counter and the slice are mutex-guarded.
type recorder struct {
	dir string

	mu     sync.Mutex
	frames []frame
	index  int
}

func newRecorder(dir string) *recorder {
	return &recorder{dir: dir}
}

// listen subscribes to screencast frames for the life of ctx, writing each
// one into the recorder's directory as it arrives.
func (r *recorder) listen(ctx context.Context) {
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		sf, ok := ev.(*page.EventScreencastFrame)
		if !ok {
			return
		}
		data, err := base64.StdEncoding.DecodeString(sf.Data)
		if err != nil {
			log.Printf("capture: warning: could not decode screencast frame: %v", err)
			return
		}
		r.mu.Lock()
		r.index++
		idx := r.index
		r.mu.Unlock()

		ts := time.Now()
		if sf.Metadata != nil && sf.Metadata.Timestamp != nil {
			ts = sf.Metadata.Timestamp.Time()
		}
		path := filepath.Join(r.dir, fmt.Sprintf("frame-%06d.png", idx))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Printf("capture: warning: could not write screencast frame %d: %v", idx, err)
			return
		}
		r.mu.Lock()
		r.frames = append(r.frames, frame{index: idx, path: path, timestamp: ts})
		r.mu.Unlock()

		// Ack must happen off the event-handling goroutine (see chromedp's
		// ListenTarget docs: the callback runs synchronously and must not
		// block on further chromedp actions).
		go func(sessionID int64) {
			if err := chromedp.Run(ctx, page.ScreencastFrameAck(sessionID)); err != nil {
				log.Printf("capture: warning: could not ack screencast frame: %v", err)
			}
		}(sf.SessionID)
	})
}

// mustCollect returns every frame captured so far, oldest first, and fails
// if there are too few to build a video from.
func (r *recorder) mustCollect() []frame {
	// Give in-flight frame writes a moment to land.
	time.Sleep(300 * time.Millisecond)

	r.mu.Lock()
	captured := append([]frame(nil), r.frames...)
	r.mu.Unlock()
	sort.Slice(captured, func(i, j int) bool { return captured[i].index < captured[j].index })

	if len(captured) < 2 {
		log.Fatalf("capture: only captured %d screencast frame(s); need at least 2 to build a video — the recording mechanism did not work", len(captured))
	}
	log.Printf("capture: captured %d screencast frames spanning %s", len(captured), captured[len(captured)-1].timestamp.Sub(captured[0].timestamp))
	return captured
}

// options is the command-line configuration for one capture run. Every path
// is required and supplied by capture-console.sh, which owns the server
// under test, the scratch directory and the Chrome profile.
type options struct {
	chromePath  string
	ffmpegPath  string
	consoleURL  string
	outDir      string
	userDataDir string
	workDir     string
	framesDir   string
	message     string
	view        string
}

// parseFlags reads the command line, failing loudly if a required path is
// missing rather than starting a browser it has nowhere to put the output of.
func parseFlags() *options {
	var o options
	flag.StringVar(&o.chromePath, "chrome", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "path to Chrome binary")
	flag.StringVar(&o.ffmpegPath, "ffmpeg", "/opt/homebrew/bin/ffmpeg", "path to ffmpeg binary")
	flag.StringVar(&o.consoleURL, "url", "", "http(s) URL of the running --devconsole instance (required)")
	flag.StringVar(&o.outDir, "out", "", "directory to write console-light.png, console-dark.png, console-demo.mp4 (required)")
	flag.StringVar(&o.userDataDir, "userdata", "", "Chrome user-data-dir, must be exclusive to this run (required)")
	flag.StringVar(&o.workDir, "workdir", "", "scratch directory for raw screencast frames (required)")
	flag.StringVar(&o.message, "message", defaultMessage, "send only this message instead of the built-in demo sequence")
	flag.StringVar(&o.view, "view", "pretty", "detail-pane view to select before shooting: pretty, raw or hex")
	flag.Parse()

	if o.consoleURL == "" {
		log.Fatal("capture: -url is required")
	}
	if o.outDir == "" {
		log.Fatal("capture: -out is required")
	}
	if o.userDataDir == "" {
		log.Fatal("capture: -userdata is required")
	}
	if o.workDir == "" {
		log.Fatal("capture: -workdir is required")
	}
	return &o
}

// preflight checks the external binaries the run depends on and creates the
// directories it writes into, before any browser is started.
func (o *options) preflight() {
	mustBeExecutable("Chrome", o.chromePath)
	mustBeExecutable("ffmpeg", o.ffmpegPath)

	if err := os.MkdirAll(o.outDir, 0o755); err != nil {
		log.Fatalf("capture: creating output dir %s: %v", o.outDir, err)
	}
	o.framesDir = filepath.Join(o.workDir, "frames")
	if err := os.MkdirAll(o.framesDir, 0o755); err != nil {
		log.Fatalf("capture: creating frames dir %s: %v", o.framesDir, err)
	}
}

// startBrowser launches headless Chrome and returns the tab context every
// later phase drives, plus the teardown to defer. The context carries a
// whole-run deadline so a Chrome or a console that never responds fails
// loudly instead of hanging forever.
func startBrowser(o *options) (context.Context, func()) {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(o.chromePath),
		chromedp.UserDataDir(o.userDataDir),
		chromedp.WindowSize(viewportWidth, viewportHeight),
		chromedp.Flag("headless", "new"),
	)...)

	ctx, cancel := chromedp.NewContext(allocCtx)
	ctx, cancelTimeout := context.WithTimeout(ctx, 90*time.Second)

	return ctx, func() {
		cancelTimeout()
		cancel()
		cancelAlloc()
	}
}

// loadConsole points the browser at the running dev console, waits for the
// console's own markup to appear, and starts the screencast that becomes the
// demo video.
func loadConsole(ctx context.Context, consoleURL string) {
	log.Printf("capture: loading %s", consoleURL)
	// Fail fast (~10s) rather than eating the whole run's 90s budget if the
	// page doesn't implement the contract (e.g. the OLD console, which has
	// no #url at all). This deliberately uses chromedp.Poll's own bounded
	// timeout rather than wrapping ctx in a derived context.WithTimeout:
	// cancelling a context.WithTimeout child of a chromedp tab context was
	// observed (against the synthetic fixture, see qa/capture's proof
	// notes) to also break the PARENT context for every subsequent
	// chromedp.Run call in this process — an internal chromedp gotcha, not
	// standard library context semantics — so every bounded wait in this
	// file uses chromedp.Poll(..., WithPollingTimeout(...)) against the
	// single long-lived ctx instead of ever deriving+cancelling a
	// sub-context of it.
	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(viewportWidth, viewportHeight),
		emulation.SetEmulatedMedia().WithFeatures([]*emulation.MediaFeature{
			{Name: "prefers-color-scheme", Value: "light"},
		}),
		chromedp.Navigate(consoleURL),
		chromedp.Poll(
			fmt.Sprintf(`(() => {
				const el = document.querySelector(%q);
				if (!el) return false;
				return Boolean(el.offsetWidth || el.offsetHeight || el.getClientRects().length);
			})()`, selURL),
			nil,
			chromedp.WithPollingInterval(100*time.Millisecond),
			chromedp.WithPollingTimeout(10*time.Second),
		),
		page.StartScreencast().WithFormat(page.ScreencastFormatPng).WithEveryNthFrame(1),
	); err != nil {
		log.Fatalf("capture: could not load console at %s (looking for %s — wrong console, or contract not implemented yet?): %v", consoleURL, selURL, err)
	}

}

// connect operates the console's connect control and waits for the socket to
// be open, then logs what the status line ended up saying.
func connect(ctx context.Context) {
	// Pacing note: the Sleep calls through this sequence exist so the demo
	// video (assembled from the screencast frames captured across all of
	// it) reads as a legible few-second clip rather than a blink — each one
	// holds the UI in a state a viewer needs a moment to register (empty
	// state, connected, sent, received, selected).
	log.Printf("capture: connecting")
	if err := chromedp.Run(ctx,
		chromedp.Sleep(800*time.Millisecond),
		chromedp.SendKeys(selURL, kb.Enter, chromedp.ByQuery),
		// #connect's data-state attribute is the authoritative connection
		// signal per the contract — more precise than waiting for any
		// particular control to appear/disappear, and it also rules out a
		// false-positive "connected" read during the transient "connecting"
		// state.
		chromedp.Poll(
			fmt.Sprintf(`document.querySelector(%q)?.getAttribute('data-state') === 'connected'`, selConnect),
			nil,
			chromedp.WithPollingInterval(100*time.Millisecond),
			chromedp.WithPollingTimeout(10*time.Second),
		),
		chromedp.Sleep(700*time.Millisecond),
	); err != nil {
		log.Fatalf("capture: connect never happened (%s never reached data-state=\"connected\"): %v", selConnect, err)
	}
	var statusText string
	if err := chromedp.Run(ctx, chromedp.Text(selStatus, &statusText, chromedp.ByQuery)); err != nil {
		log.Fatalf("capture: could not read %s after connecting: %v", selStatus, err)
	}
	log.Printf("capture: connected, %s reads %q", selStatus, statusText)
}

// demoSequence is the list of messages the capture types into the console.
func demoSequence(message string) []string {
	// A single round trip left the transcript almost empty, which sells the
	// split inspector badly: the whole point is a list you scan beside a
	// detail pane. Send a short sequence instead, ending with JSON so the
	// Pretty view has structure to show. --message overrides with a single
	// message when a caller wants one specific frame.
	demo := []string{"hello devconsole", "ping", "the quick brown fox", `{"cmd":"status","uptime":41,"forks":3,"ok":true}`}
	if message != defaultMessage {
		demo = []string{message}
	}
	return demo
}

// sendMessages types each message into the composer and presses Enter.
func sendMessages(ctx context.Context, demo []string) {
	for i, m := range demo {
		log.Printf("capture: sending %d/%d %q", i+1, len(demo), m)
		// Clear the composer before typing. The console does preventDefault
		// on Enter, so a real keypress leaves nothing behind - but CDP's
		// KeyEvent also dispatches a char event, which lands a newline in
		// the textarea *after* the handler has sent and cleared it. Left
		// alone that newline prefixes the next message (a 4-byte "ping"
		// goes out as 5 bytes and the echo splits into an empty frame).
		// This compensates for the driver, not for a console defect.
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`document.querySelector(%q).value = ''`, selSend), nil),
			chromedp.SendKeys(selSend, m, chromedp.ByQuery),
			chromedp.Sleep(150*time.Millisecond),
			chromedp.KeyEvent(kb.Enter),
			chromedp.Sleep(250*time.Millisecond),
		); err != nil {
			log.Fatalf("capture: could not send %q via %s: %v", m, selSend, err)
		}
	}
}

// waitForEchoes blocks until every message has been seen going out and
// coming back, then holds long enough for the last one to render.
func waitForEchoes(ctx context.Context, want int) {
	// Wait for every echo, not just the first: a screenshot taken while
	// frames are still arriving is a race, and a short sleep would hide it.
	log.Printf("capture: waiting for all %d echoes to land in %s", want, selFrames)
	if err := chromedp.Run(ctx,
		chromedp.Poll(
			fmt.Sprintf(`document.querySelectorAll(%q).length >= %d`, selFrames+` .frame[data-dir="in"]`, want),
			nil,
			chromedp.WithPollingInterval(100*time.Millisecond),
			chromedp.WithPollingTimeout(15*time.Second),
		),
		chromedp.Poll(
			fmt.Sprintf(`document.querySelectorAll(%q).length >= %d`, selFrames+` .frame[data-dir="out"]`, want),
			nil,
			chromedp.WithPollingInterval(100*time.Millisecond),
			chromedp.WithPollingTimeout(15*time.Second),
		),
	); err != nil {
		log.Fatalf("capture: not all frames arrived in %s: %v", selFrames, err)
	}
	log.Printf("capture: all echoes received")

	log.Printf("capture: waiting to let the echo register on screen")
	if err := chromedp.Run(ctx, chromedp.Sleep(700*time.Millisecond)); err != nil {
		log.Fatalf("capture: %v", err)
	}
}

// selectLastReceivedFrame clicks the most recent inbound row so the detail
// pane has something to show in the screenshots.
func selectLastReceivedFrame(ctx context.Context) {
	log.Printf("capture: selecting the received frame")
	var clicked bool
	if err := chromedp.Run(ctx,
		// querySelectorAll + click on the last match, rather than a CSS
		// selector: the contract gives no ":last received row" pseudo-class,
		// and clicking the wrong one (or nothing) should fail fast rather
		// than have chromedp.Click poll silently until the context deadline.
		chromedp.Evaluate(fmt.Sprintf(`(() => {
			const rows = document.querySelectorAll(%q);
			if (rows.length === 0) return false;
			rows[rows.length - 1].click();
			return true;
		})()`, selFrames+` .frame[data-dir="in"]`), &clicked),
		chromedp.Sleep(900*time.Millisecond),
	); err != nil {
		log.Fatalf("capture: could not select the received frame: %v", err)
	}
	if !clicked {
		log.Fatalf("capture: no received-frame row found to select (selector %q matched nothing)", selFrames+` .frame[data-dir="in"]`)
	}
}

// selectDetailView switches the detail pane to the requested view. Pretty is
// already selected by default, so that case does nothing.
func selectDetailView(ctx context.Context, view string) {
	// The detail pane's view is part of what the screenshots document, so it
	// is selectable rather than always Pretty. aria-pressed is polled rather
	// than slept on: a click that silently misses would otherwise produce a
	// plausible screenshot of the wrong tab.
	if view != "pretty" {
		log.Printf("capture: selecting the %s view", view)
		sel := fmt.Sprintf(`#detail .dviews button[data-view=%q]`, view)
		if err := chromedp.Run(ctx,
			chromedp.Click(sel, chromedp.ByQuery),
			chromedp.Poll(
				fmt.Sprintf(`document.querySelector(%q).getAttribute('aria-pressed') === 'true'`, sel),
				nil,
				chromedp.WithPollingInterval(100*time.Millisecond),
				chromedp.WithPollingTimeout(10*time.Second),
			),
			chromedp.Sleep(400*time.Millisecond),
		); err != nil {
			log.Fatalf("capture: could not select the %q view via %s: %v", view, sel, err)
		}
	}
}

// assertDetailPane checks that selecting a frame actually populated the
// detail pane — this is the real behavioral assertion the split-inspector
// layout exists for, not just "a click happened". It checks the sent text
// round-tripped into #detail-body, and that opcode/size were filled in at
// all (their exact format isn't part of the given contract, so only
// non-emptiness is asserted for those two).
func assertDetailPane(ctx context.Context, want, view string) {
	var detailBody, detailOp, detailSz string
	if err := chromedp.Run(ctx,
		chromedp.Text(selDetailBdy, &detailBody, chromedp.ByQuery),
		chromedp.Text(selDetailOp, &detailOp, chromedp.ByQuery),
		chromedp.Text(selDetailSz, &detailSz, chromedp.ByQuery),
	); err != nil {
		log.Fatalf("capture: could not read detail pane (%s/%s/%s) after selecting a frame: %v", selDetailBdy, selDetailOp, selDetailSz, err)
	}
	// The selection step clicks the LAST received frame, so the expectation
	// is the last message of the sequence, not the first. Compared with
	// whitespace collapsed: the last demo message is JSON, and the Pretty
	// view is expected to re-indent it - that reformatting is the feature,
	// not a mismatch.
	if view == "hex" {
		// A hex dump does not contain the payload literally, so the
		// pretty/raw expectation cannot apply. Assert the dump is non-empty
		// and looks like one, rather than skipping the check entirely.
		if !strings.Contains(detailBody, "00000000") {
			log.Fatalf("capture: %s does not look like a hex dump: %q", selDetailBdy, detailBody)
		}
	} else if !strings.Contains(squash(detailBody), squash(want)) {
		log.Fatalf("capture: detail pane mismatch: selected frame carried %q, %s reads %q", want, selDetailBdy, detailBody)
	}
	if strings.TrimSpace(detailOp) == "" {
		log.Fatalf("capture: %s is empty after selecting a frame", selDetailOp)
	}
	if strings.TrimSpace(detailSz) == "" {
		log.Fatalf("capture: %s is empty after selecting a frame", selDetailSz)
	}
	log.Printf("capture: detail pane populated: opcode=%q size=%q body=%q", detailOp, detailSz, detailBody)
}

// logSessionCounters records the console's own session counters in the run
// log as evidence.
func logSessionCounters(ctx context.Context) {
	// Log-only, non-gating: the exact text format of the session counters
	// isn't part of the given contract, so these are evidence in the log,
	// not an assertion.
	var sentCount, recvCount, byteCount string
	if err := chromedp.Run(ctx,
		chromedp.Text(selCountSent, &sentCount, chromedp.ByQuery),
		chromedp.Text(selCountRecv, &recvCount, chromedp.ByQuery),
		chromedp.Text(selCountByte, &byteCount, chromedp.ByQuery),
	); err != nil {
		log.Printf("capture: note: could not read session counters (%s/%s/%s): %v", selCountSent, selCountRecv, selCountByte, err)
	} else {
		log.Printf("capture: counters: sent=%q recv=%q bytes=%q", sentCount, recvCount, byteCount)
	}
}

// stopScreencast ends the recording. The demo video covers connect -> send
// -> receive -> select and stops here, deliberately excluding the theme
// operations that follow — those are a separate concern captured as stills,
// not part of the round-trip demo.
func stopScreencast(ctx context.Context) {
	if err := chromedp.Run(ctx, page.StopScreencast()); err != nil {
		log.Fatalf("capture: could not stop screencast: %v", err)
	}
}

// shootThemeStills writes console-light.png and console-dark.png, reaching
// each theme by a different route on purpose — see the comments below.
func shootThemeStills(ctx context.Context, outDir string) {
	// --- Light screenshot: the AUTO path ---
	// data-theme is left untouched (default "auto" per the spec: follow the
	// system by default). The light rendering is produced by CDP media
	// emulation of prefers-color-scheme=light, which was already set before
	// Navigate above. This proves the "follow the system" path, not the
	// explicit-override path.
	assertDataTheme(ctx, "auto", "before the light/auto screenshot")
	shoot(ctx, "light (auto, via prefers-color-scheme emulation)", filepath.Join(outDir, "console-light.png"))

	// --- Dark screenshot: the EXPLICIT OVERRIDE path ---
	// Operates the real #theme control rather than CDP emulation, and
	// deliberately leaves the emulated media at prefers-color-scheme=light
	// (unchanged from above) while doing it: if the resulting screenshot is
	// dark, that's proof the explicit override in the spec ("persists...
	// can be returned to auto") actually wins over the system preference,
	// not just that dark CSS exists somewhere. #theme's exact control shape
	// (single cycling button vs. something else) isn't specified, so this
	// clicks it up to 4 times (enough to reach any of the 3 states from any
	// starting state under a cycling button) checking
	// document.documentElement's data-theme after each click, and fails
	// loudly — naming the assumption — if dark is never reached.
	const maxThemeClicks = 4
	reachedDark := false
	for i := 0; i < maxThemeClicks; i++ {
		if err := chromedp.Run(ctx,
			chromedp.Click(selTheme, chromedp.ByQuery),
			chromedp.Sleep(250*time.Millisecond),
		); err != nil {
			log.Fatalf("capture: could not click %s (attempt %d): %v", selTheme, i+1, err)
		}
		theme := dataTheme(ctx)
		log.Printf("capture: after clicking %s (%d/%d), data-theme=%q", selTheme, i+1, maxThemeClicks, theme)
		if theme == "dark" {
			reachedDark = true
			break
		}
	}
	if !reachedDark {
		log.Fatalf("capture: clicking %s %d times never reached data-theme=\"dark\" — the assumption that it's a single cycling auto/light/dark control does not hold for this page", selTheme, maxThemeClicks)
	}
	shoot(ctx, "dark (explicit override via #theme, prefers-color-scheme still emulated light)", filepath.Join(outDir, "console-dark.png"))
}

func main() {
	o := parseFlags()
	o.preflight()

	ctx, closeBrowser := startBrowser(o)
	defer closeBrowser()

	rec := newRecorder(o.framesDir)
	rec.listen(ctx)

	loadConsole(ctx, o.consoleURL)
	connect(ctx)

	demo := demoSequence(o.message)
	sendMessages(ctx, demo)
	waitForEchoes(ctx, len(demo))

	selectLastReceivedFrame(ctx)
	selectDetailView(ctx, o.view)
	assertDetailPane(ctx, demo[len(demo)-1], o.view)
	logSessionCounters(ctx)

	stopScreencast(ctx)
	shootThemeStills(ctx, o.outDir)

	videoPath := filepath.Join(o.outDir, "console-demo.mp4")
	encodeVideo(o.ffmpegPath, rec.mustCollect(), o.workDir, videoPath)

	verifyOutputs(o.outDir)
}

// verifyOutputs re-reads what the run just wrote. A zero-byte file is a
// failure, not a deliverable, and the sizes are the run's own receipt.
func verifyOutputs(outDir string) {
	log.Printf("capture: done")
	for _, f := range []string{"console-light.png", "console-dark.png", "console-demo.mp4"} {
		p := filepath.Join(outDir, f)
		info, err := os.Stat(p)
		if err != nil {
			log.Fatalf("capture: expected output missing: %s: %v", p, err)
		}
		if info.Size() == 0 {
			log.Fatalf("capture: output is zero bytes, which is a failure not a deliverable: %s", p)
		}
		log.Printf("capture:   %s (%d bytes)", p, info.Size())
	}
}

func mustBeExecutable(label, path string) {
	info, err := os.Stat(path)
	if err != nil {
		log.Fatalf("capture: %s not found at %s: %v", label, path, err)
	}
	if info.IsDir() {
		log.Fatalf("capture: %s path %s is a directory, not an executable", label, path)
	}
	if info.Mode()&0o111 == 0 {
		log.Fatalf("capture: %s at %s is not executable", label, path)
	}
}

// dataTheme reads <html data-theme="...">, per the contract. An empty
// string means the attribute is absent, which per the contract's own
// enumeration (auto|light|dark) would itself be a finding worth surfacing
// rather than silently treating as one of the three valid states.
func dataTheme(ctx context.Context) string {
	var theme string
	if err := chromedp.Run(ctx, chromedp.Evaluate(
		`document.documentElement.getAttribute('data-theme') || ''`, &theme,
	)); err != nil {
		log.Fatalf("capture: could not read documentElement data-theme: %v", err)
	}
	return theme
}

func assertDataTheme(ctx context.Context, want, when string) {
	got := dataTheme(ctx)
	if got != want {
		log.Fatalf("capture: expected data-theme=%q %s, got %q", want, when, got)
	}
}

// shoot captures the viewport and writes it to path. label names the shot in
// the failure messages, which are the only place a half-produced deliverable
// would otherwise show up.
func shoot(ctx context.Context, label, path string) {
	var buf []byte
	if err := chromedp.Run(ctx, chromedp.CaptureScreenshot(&buf)); err != nil {
		log.Fatalf("capture: %s screenshot failed: %v", label, err)
	}
	if len(buf) == 0 {
		log.Fatalf("capture: %s screenshot came back empty", label)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		log.Fatalf("capture: writing %s: %v", path, err)
	}
}

// encodeVideo assembles the captured screencast frames into a small,
// web-friendly demo video using ffmpeg's concat demuxer, so that each still
// frame is held on screen for exactly as long as it was actually on screen
// in the browser (per-frame CDP timestamps), rather than assuming a fixed
// frame rate. Encodes to H.264/yuv420p in an MP4 container with
// +faststart — the safest combination for a plain <video> tag with no
// server-side range-request tuning, and universally playable without a
// codec pack. No audio track exists (a UI screencast has none), so it's
// dropped explicitly with -an.
func encodeVideo(ffmpegPath string, frames []frame, workDir, outPath string) {
	listPath := filepath.Join(workDir, "frames.txt")
	var b strings.Builder
	for i, f := range frames {
		fmt.Fprintf(&b, "file '%s'\n", f.path)
		var dur time.Duration
		if i+1 < len(frames) {
			dur = frames[i+1].timestamp.Sub(f.timestamp)
		} else {
			dur = 800 * time.Millisecond // hold the final frame briefly
		}
		if dur <= 0 {
			dur = 50 * time.Millisecond
		}
		fmt.Fprintf(&b, "duration %.3f\n", dur.Seconds())
	}
	// ffmpeg's concat demuxer quirk: the duration on the last real entry is
	// ignored unless the file is repeated once more after it.
	fmt.Fprintf(&b, "file '%s'\n", frames[len(frames)-1].path)
	if err := os.WriteFile(listPath, []byte(b.String()), 0o644); err != nil {
		log.Fatalf("capture: writing ffmpeg concat list: %v", err)
	}

	cmd := exec.Command(ffmpegPath,
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-vf", "scale=800:-2:flags=lanczos,format=yuv420p",
		"-c:v", "libx264",
		"-preset", "veryslow",
		"-crf", "30",
		"-movflags", "+faststart",
		"-an",
		outPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("capture: ffmpeg failed: %v\n%s", err, out)
	}
}
