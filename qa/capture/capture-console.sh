#!/usr/bin/env bash
#
# capture-console.sh — builds websocketd, serves the dev console, and drives
# headless Chrome to produce the screenshots and demo recording referenced
# from README.md and website/index.html (see ASKS.md A2).
#
# Re-run this any time the console changes and the assets need refreshing:
#
#   qa/capture/capture-console.sh
#
# By default it writes into website/img/console/, overwriting the committed
# assets in place. Pass a different directory to produce a proof run without
# touching the committed assets:
#
#   qa/capture/capture-console.sh /path/to/scratch/dir
#
# Requires: go, curl, python3, Chrome, and ffmpeg. Fails loudly (non-zero
# exit, message on stderr) if any of these are missing — it never produces a
# partial or zero-byte output silently.
#
# Copyright 2026 Joe Walnes and the websocketd team.
# All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -euo pipefail

CHROME="${CHROME_BIN:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
FFMPEG="${FFMPEG_BIN:-/opt/homebrew/bin/ffmpeg}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OUT_DIR="${1:-$REPO_ROOT/website/img/console}"
VIEW="${2:-pretty}"

fail() {
	echo "capture-console.sh: $*" >&2
	exit 1
}

command -v go >/dev/null 2>&1 || fail "go toolchain not found on PATH"
command -v curl >/dev/null 2>&1 || fail "curl not found on PATH"
command -v python3 >/dev/null 2>&1 || fail "python3 not found on PATH (used to allocate a free port)"
[ -x "$CHROME" ] || fail "Chrome not found/executable at: $CHROME (set CHROME_BIN to override)"
[ -x "$FFMPEG" ] || fail "ffmpeg not found/executable at: $FFMPEG (set FFMPEG_BIN to override)"

WORK="$(mktemp -d "${TMPDIR:-/tmp}/wsd-capture.XXXXXX")"
WSD_PID=""
cleanup() {
	if [ -n "$WSD_PID" ]; then
		kill "$WSD_PID" >/dev/null 2>&1 || true
		wait "$WSD_PID" 2>/dev/null || true
	fi
	rm -rf "$WORK"
}
trap cleanup EXIT

BIN="$WORK/websocketd"
echo "capture-console.sh: building websocketd -> $BIN"
(cd "$REPO_ROOT" && go build -o "$BIN" .) || fail "go build of websocketd failed"

TESTCMD_BIN="$WORK/testcmd"
echo "capture-console.sh: building qa/integration/testcmd -> $TESTCMD_BIN"
(cd "$REPO_ROOT" && go build -o "$TESTCMD_BIN" ./qa/integration/testcmd) || fail "go build of testcmd failed"

# Allocate a free TCP port — never a fixed one, so this never collides with
# a sibling agent or a developer's own websocketd instance.
PORT="$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')"
[ -n "$PORT" ] || fail "could not allocate a free port"

echo "capture-console.sh: starting websocketd --devconsole on 127.0.0.1:$PORT"
"$BIN" --port="$PORT" --devconsole "$TESTCMD_BIN" echo \
	>"$WORK/websocketd.log" 2>&1 &
WSD_PID=$!

up=0
for _ in $(seq 1 50); do
	if curl -sS -o /dev/null "http://127.0.0.1:$PORT/"; then
		up=1
		break
	fi
	if ! kill -0 "$WSD_PID" 2>/dev/null; then
		break
	fi
	sleep 0.1
done
if [ "$up" -ne 1 ]; then
	echo "capture-console.sh: websocketd never came up; log follows:" >&2
	cat "$WORK/websocketd.log" >&2
	fail "websocketd did not respond on port $PORT"
fi

mkdir -p "$OUT_DIR"
USERDATA="$WORK/chrome-profile"
mkdir -p "$USERDATA"

echo "capture-console.sh: driving Chrome against http://127.0.0.1:$PORT/ -> $OUT_DIR"
(
	cd "$SCRIPT_DIR" && go run . \
		-url "http://127.0.0.1:$PORT/" \
		-out "$OUT_DIR" \
		-userdata "$USERDATA" \
		-workdir "$WORK" \
		-chrome "$CHROME" \
		-ffmpeg "$FFMPEG" \
		-view "$VIEW"
) || fail "capture driver failed"

echo "capture-console.sh: done"
ls -la "$OUT_DIR"
