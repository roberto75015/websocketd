// Copyright 2013 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package libwebsocketd

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cgi"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var ErrForkNotAllowed = errors.New("too many forks active")

var upgradeRe = regexp.MustCompile(`(?i)(^|[,\s])Upgrade($|[,\s])`)

// WebsocketdServer presents http.Handler interface for requests libwebsocketd is handling.
type WebsocketdServer struct {
	Config   *Config
	Log      *LogScope
	forks    chan byte
	hostname string // cached os.Hostname(), computed once at startup

	// The CGI mount prefix (see cgiMountPrefix) depends only on --staticdir
	// and --cgidir, but deciding it costs a filesystem resolution of both,
	// and serveCGI runs ahead of the static handler for *every* request once
	// --cgidir is set. Resolved once and reused: the answer cannot change
	// without the configuration changing, and pinning it at first use also
	// means a deployment symlink flipped mid-flight cannot re-route live
	// requests out from under themselves. Lazy rather than set in the
	// constructor so a WebsocketdServer built as a plain struct literal —
	// libwebsocketd is a library — gets the same routing.
	cgiMountOnce   sync.Once
	cgiMountCached string
}

// cgiMount returns the URL prefix at which --cgidir sits inside --staticdir,
// resolving it on first use and reusing it thereafter.
func (h *WebsocketdServer) cgiMount() string {
	h.cgiMountOnce.Do(func() {
		h.cgiMountCached = cgiMountPrefix(h.Config.StaticDir, h.Config.CgiDir)
	})
	return h.cgiMountCached
}

// NewWebsocketdServer creates WebsocketdServer struct with pre-determined config, logscope and maxforks limit
func NewWebsocketdServer(config *Config, log *LogScope, maxforks int) *WebsocketdServer {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "UNKNOWN"
	}
	mux := &WebsocketdServer{
		Config:   config,
		Log:      log,
		hostname: hostname,
	}
	if maxforks > 0 {
		mux.forks = make(chan byte, maxforks)
	}
	return mux
}

func splitMimeHeader(s string) (string, string) {
	p := strings.IndexByte(s, ':')
	if p < 0 {
		return s, ""
	}
	key := textproto.CanonicalMIMEHeaderKey(s[:p])

	for p = p + 1; p < len(s); p++ {
		if s[p] != ' ' {
			break
		}
	}
	return key, s[p:]
}

func pushHeaders(h http.Header, hdrs []string) {
	for _, hstr := range hdrs {
		h.Add(splitMimeHeader(hstr))
	}
}

// isWebSocketUpgrade checks if the request is a WebSocket upgrade request.
func isWebSocketUpgrade(req *http.Request) bool {
	hdrs := req.Header
	return strings.ToLower(hdrs.Get("Upgrade")) == "websocket" &&
		upgradeRe.MatchString(hdrs.Get("Connection"))
}

// ServeHTTP muxes between WebSocket handler, CGI handler, DevConsole, Static HTML or 404.
func (h *WebsocketdServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log := h.Log.NewLevel(h.Log.LogFunc)
	log.Associate("url", h.TellURL("http", req.Host, req.RequestURI))

	if h.serveWebSocket(w, req, log) {
		return
	}

	pushHeaders(w.Header(), h.Config.Headers)
	pushHeaders(w.Header(), h.Config.HeadersHTTP)

	if h.serveDevConsole(w, req, log) {
		return
	}
	if h.serveCGI(w, req, log) {
		return
	}
	if h.serveStatic(w, req, log) {
		return
	}

	log.Access("http", "NOT FOUND")
	http.NotFound(w, req)
}

// serveWebSocket handles WebSocket upgrade requests. Returns true if handled.
func (h *WebsocketdServer) serveWebSocket(w http.ResponseWriter, req *http.Request, log *LogScope) bool {
	if h.Config.CommandName == "" && !h.Config.UsingScriptDir {
		return false
	}
	if !isWebSocketUpgrade(req) {
		return false
	}

	if h.noteForkCreated() != nil {
		log.Error("http", "Max of possible forks already active, upgrade rejected")
		http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
		return true
	}
	defer h.noteForkCompleted()

	handler, err := NewWebsocketdHandler(h, req, log)
	if err != nil {
		if err == ErrScriptNotFound {
			log.Access("session", "NOT FOUND: %s", err)
			http.Error(w, "404 Not Found", 404)
		} else {
			log.Access("session", "INTERNAL ERROR: %s", err)
			http.Error(w, "500 Internal Server Error", 500)
		}
		return true
	}

	var headers http.Header
	if len(h.Config.Headers)+len(h.Config.HeadersWs) > 0 {
		headers = http.Header(make(map[string][]string))
		pushHeaders(headers, h.Config.Headers)
		pushHeaders(headers, h.Config.HeadersWs)
	}

	upgrader := &websocket.Upgrader{
		HandshakeTimeout: h.Config.HandshakeTimeout,
		CheckOrigin: func(r *http.Request) bool {
			return checkOrigin(req, h.Config, log) == nil
		},
	}
	conn, err := upgrader.Upgrade(w, req, headers)
	if err != nil {
		// gorilla's Upgrade has already written its error response (403 for
		// a rejected origin, 400 for a bad handshake), so writing another one
		// here only made net/http log "superfluous response.WriteHeader".
		log.Access("session", "Unable to Upgrade: %s", err)
		return true
	}

	handler.accept(conn, log)
	return true
}

// serveDevConsole serves the interactive development console. Returns true if handled.
func (h *WebsocketdServer) serveDevConsole(w http.ResponseWriter, req *http.Request, log *LogScope) bool {
	if !h.Config.DevConsole {
		return false
	}
	log.Access("http", "DEVCONSOLE")
	// The body is a constant: nothing from the request is interpolated into
	// it. The console derives its WebSocket URL from location.href in the
	// browser, so the server has nothing to say about it — and with no
	// request-derived text in the page there is no reflected-injection
	// surface to escape.
	//
	// The console is routinely pointed at servers the user does not control,
	// and every frame it renders is someone else's text. The policy denies
	// everything by default and pins the page's own script and style by
	// hash, so even a defect in the page cannot turn a frame into a resource
	// load or an injected script. connect-src stays open to ws:/wss: because
	// the user types the endpoint.
	hdr := w.Header()
	hdr.Set("Content-Security-Policy", ConsoleCSP)
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Content-Type", "text/html; charset=utf-8")
	hdr.Set("ETag", ConsoleETag)
	http.ServeContent(w, req, ".html", h.Config.StartupTime, strings.NewReader(ConsoleContent))
	return true
}

// normalizeURLPath folds a request URL path into a rooted, cleaned,
// slash-separated form: the single spelling every routing decision below is
// made against. Normalizing first means "/htm/../cgi-bin/x" and
// "/cgi-bin/x" are one path as far as routing is concerned, and a path that
// climbs out of a directory can never match that directory's prefix.
//
// req.URL.Path arrives already percent-decoded, and dot segments (plus, on
// Windows, backslash separators) reach a handler verbatim. The websocketd
// binary happens to register this handler on DefaultServeMux, which
// redirects an unclean path before the handler sees it, but libwebsocketd
// is also used as a plain http.Handler with no mux in front of it — so this
// normalization is the thing actually relied on, not that redirect.
func normalizeURLPath(urlPath string) string {
	return path.Clean("/" + filepath.ToSlash(urlPath))
}

// cgiMountPrefix returns the URL path prefix at which the CGI directory
// appears inside the static directory — "/cgi-bin" for
// --staticdir=/PAGE --cgidir=/PAGE/cgi-bin — or "" when there is no such
// position.
//
// --cgidir has always mapped the request path straight into the CGI
// directory, so /hello.sh runs cgidir/hello.sh. That works until the CGI
// directory is nested inside the static root, which is the natural layout
// for a self-contained site: the URL a browser forms for the script is then
// /cgi-bin/hello.sh, which looked for cgidir/cgi-bin/hello.sh, missed, and
// fell through to the static handler — which served the script's source
// (issue #453). Deriving the prefix from where the operator already put the
// directory needs no new flag and no new spelling to learn.
//
// The derivation asks dirRelation which *directory* --cgidir names relative
// to the one --staticdir names, not which string. Deriving it lexically
// instead — filepath.Rel over the two flag values made absolute — answered
// a different question: whether the operator happened to spell the two
// flags consistently. A release symlink pointed at by one flag and not the
// other ("--staticdir=/srv/current --cgidir=/srv/releases/v1/cgi-bin") made
// one nested pair look like two unrelated trees, so no prefix was derived,
// the CGI handler did not recognize the script's URL, and a legitimate
// configuration 404ed every script at its natural URL.
//
// It stays deliberately conservative: it returns "" unless the CGI
// directory is *strictly* inside the static one. Equal directories yield no
// prefix (the direct mapping already covers every file in them), a static
// root inside the CGI directory yields none either, and anything that
// cannot be resolved yields none — falling back to the long-standing direct
// mapping rather than inventing a route. Note what identity does NOT do: a
// CGI directory that merely becomes *reachable* from the static tree through
// a symlink is not thereby inside it. Only the two directories the operator
// named are resolved; the static tree is not searched for links into the CGI
// directory, and such a URL stays refused.
func cgiMountPrefix(staticDir, cgiDir string) string {
	if staticDir == "" || cgiDir == "" {
		return ""
	}
	rel, r := dirRelation(cgiDir, staticDir)
	if r != relInside || rel == "" {
		// Not inside, unresolvable (relUnknown), or the same directory: no
		// prefix. This derivation grants a route, so anything short of a
		// definite "inside" must yield nothing.
		return ""
	}
	return "/" + rel
}

// cgiURLCandidates lists the paths to look for inside the CGI directory, in
// priority order, given the mount prefix cgiMountPrefix derived for this
// server. The request path itself comes first, so the long-standing direct
// mapping keeps winning any ambiguity; the prefix stripped off comes second,
// when one exists and the request is under it.
//
// The prefix is matched against the *normalized* path, which is what makes
// this safe: "/cgi-bin/../../outside/evil.sh" normalizes to
// "/outside/evil.sh" and no longer looks like a request under the prefix at
// all, so stripping cannot be used to re-enter the directory from above.
func cgiURLCandidates(prefix, urlPath string) []string {
	clean := normalizeURLPath(urlPath)
	candidates := []string{clean}

	if prefix == "" {
		return candidates
	}
	rest, ok := strings.CutPrefix(clean, prefix)
	if !ok || (rest != "" && rest[0] != '/') {
		// Not under the prefix. The second clause is what keeps a sibling
		// whose name merely starts with the same text ("/cgi-bindings/…")
		// out of the CGI handler.
		return candidates
	}
	if rest == "" {
		rest = "/"
	}
	return append(candidates, rest)
}

// dirRel is the answer dirRelation gives, and it has three states rather
// than two on purpose: "the filesystem did not tell me" is not the same
// answer as "definitely elsewhere", and the four call sites read the
// non-inside answers in directions that do not agree —
//
//	checkPathBoundary     refuses unless inside  — unknown refuses
//	boundedDir.Open       refuses when inside    — unknown serves
//	staticExecExclusions  skips when inside      — unknown excludes
//	cgiMountPrefix        routes when inside     — unknown routes nothing
//
// A bool collapses the last two states into one value, which is safe only
// while every caller wants the same thing from the ambiguous case. They do
// not, so there is no bool wrapper: each caller compares against relInside
// itself and states in a comment what the other two answers mean for it.
// (There was one, insideDir, and its single "unknown means outside"
// sentence was standing in for two opposite policies.)
type dirRel int

const (
	// relOutside: p definitely does not sit at or under dir.
	relOutside dirRel = iota
	// relInside: p is dir itself, or lies beneath it.
	relInside
	// relUnknown: the filesystem did not answer. Callers that permit on the
	// negative may treat this as relOutside; callers that refuse on the
	// negative must refuse here too.
	relUnknown
)

func (r dirRel) String() string {
	switch r {
	case relOutside:
		return "outside"
	case relInside:
		return "inside"
	default:
		return "unknown"
	}
}

// dirRelation resolves p and dir to real paths and reports where p sits
// relative to dir: the slash-separated relative path (empty when p *is*
// dir) and relInside when p is dir or lies beneath it; relOutside when it
// definitely lies elsewhere; relUnknown when either side cannot be
// resolved.
//
// This is the one containment answer in the package: the static handler's
// exec-directory exclusion, the CGI mount prefix and checkPathBoundary all
// end here. Directories are compared by file identity (os.SameFile), not by
// string, so a symlink into the directory, a "/./" segment, or a
// case-folding filesystem where /CGI-BIN and /cgi-bin are one directory all
// give the same answer — as does the same directory reached through more
// than one mountpoint, which is the other spelling EvalSymlinks leaves
// alone. Containment used to be resolved three ways — two by identity, one
// lexically — and every request whose spelling made them disagree fell down
// the gap between two of them (issues #453, #476).
func dirRelation(p, dir string) (string, dirRel) {
	// Both sides are made absolute before anything else. --staticdir and
	// --cgidir are used exactly as the operator typed them, so one can be
	// relative while the other is not ("--staticdir=. --cgidir=/PAGE/cgi-bin"
	// is an ordinary command line). Comparing a relative spelling against an
	// absolute one would report a file plainly inside the directory as
	// outside it — which is the answer that lets the static handler serve
	// an exec directory's source.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", relUnknown
	}
	realDir, err := filepath.EvalSymlinks(absDir)
	if err != nil {
		return "", relUnknown
	}
	di, err := os.Stat(realDir)
	if err != nil {
		return "", relUnknown
	}
	if !di.IsDir() {
		// A definite answer: nothing is inside a non-directory.
		return "", relOutside
	}
	absP, err := filepath.Abs(p)
	if err != nil {
		return "", relUnknown
	}
	real, err := filepath.EvalSymlinks(absP)
	if err != nil {
		return "", relUnknown
	}
	return dirRelationResolved(real, realDir, di)
}

// dirRelationResolved is dirRelation's walk on its own, for a caller that
// has already made both sides absolute, resolved their symlinks, and taken
// the directory's FileInfo. checkPathBoundary runs per request and has done
// all three by the time it needs this; re-entering through dirRelation
// would pay a second EvalSymlinks — an lstat per path component — on both
// sides for no new information. It is the same walk, not a second
// implementation: dirRelation is this function plus the resolution.
func dirRelationResolved(real, realDir string, di os.FileInfo) (string, dirRel) {
	// Both sides are resolved, so an ancestor that is dir can only appear at
	// or above dir's own depth, which bounds the walk. Depth is the
	// separator count: a directory always has strictly fewer separators than
	// anything beneath it, whatever the two are spelled like, where string
	// *length* only approximates that and gets it wrong the moment two
	// spellings of one directory differ in length.
	depth := strings.Count(filepath.ToSlash(real), "/")
	dirDepth := strings.Count(filepath.ToSlash(realDir), "/")
	var segs []string
	unsure := false
	for ; depth >= dirDepth; depth-- {
		fi, err := os.Stat(real)
		switch {
		case err != nil:
			// Do not abandon the walk. An ancestor that cannot be stat'ed
			// must not turn a genuine containment into a definite
			// "outside", which two of the four call sites read as
			// permission; record the doubt and keep looking for the match.
			unsure = true
		case os.SameFile(fi, di):
			for i, j := 0, len(segs)-1; i < j; i, j = i+1, j-1 {
				segs[i], segs[j] = segs[j], segs[i]
			}
			return strings.Join(segs, "/"), relInside
		}
		parent := filepath.Dir(real)
		if parent == real {
			break
		}
		segs = append(segs, filepath.Base(real))
		real = parent
	}
	if unsure {
		return "", relUnknown
	}
	return "", relOutside
}

// resolveCgiPath maps a request URL path to a file inside cgiDir, refusing
// any path that would escape the directory.
//
// req.URL.Path arrives already percent-decoded, so "../" segments and (on
// Windows) "..\" segments can reach a handler verbatim. The websocketd
// binary happens to sit behind DefaultServeMux, which redirects an unclean
// path before the handler runs, but libwebsocketd is also used as a plain
// http.Handler with no mux in front of it — so the normalization here is
// what actually keeps a request inside the directory, not that redirect
// (see normalizeURLPath). Naively joining such a path lets a request name
// any file on the host, which cgi.Handler would then execute — an
// unauthenticated RCE. We normalize the request path ourselves and require
// the result to stay within cgiDir. checkPathBoundary (applied by the
// caller) additionally defends against symlinks that point out of the
// directory.
func resolveCgiPath(cgiDir, urlPath string) (string, error) {
	// Normalize in slash space, then map to the OS separator. path.Clean
	// collapses "." and ".." lexically; a rooted clean path can never
	// retain a leading "..", so anything that tried to climb out is folded
	// back to the root and lands inside cgiDir.
	clean := normalizeURLPath(urlPath)
	if clean == "/" {
		return "", fmt.Errorf("no CGI script named in path %q", urlPath)
	}
	filePath := filepath.Join(cgiDir, filepath.FromSlash(clean))

	// Assert the invariant, do not re-decide it. The rooted clean above
	// already guarantees containment on every OS (including Windows, where
	// ToSlash folds "..\" into "../" before the clean sees it), so this
	// check is not load-bearing today — it is here to fail closed if that
	// normalization is ever weakened.
	if err := containsPath(cgiDir, filePath); err != nil {
		return "", err
	}
	return filePath, nil
}

// containsPath is an invariant assertion, not a containment predicate, and
// is the one lexical comparison left in the package. Its subject is a path
// resolveCgiPath built one line earlier as Join(cgiDir, <rooted, cleaned>),
// so filepath.Rel of the two is tautologically free of "..": no request, on
// any OS, makes it report an escape (TestResolveCgiPathAssertionNeverFires).
// It costs no syscall and catches a future weakening of normalizeURLPath,
// which is the thing actually keeping a request inside the directory.
//
// It is deliberately not routed through dirRelation, the identity predicate
// that answers containment everywhere else here: dirRelation asks the
// filesystem, and this path need not exist yet — the caller stats it
// afterwards and skips what is missing. Symlinks out of the directory,
// which no lexical check can see, are checkPathBoundary's job.
func containsPath(dir, child string) error {
	rel, err := filepath.Rel(dir, child)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %q escapes directory %q", child, dir)
	}
	return nil
}

// serveCGI executes CGI scripts from the configured directory. Returns true if handled.
func (h *WebsocketdServer) serveCGI(w http.ResponseWriter, req *http.Request, log *LogScope) bool {
	if h.Config.CgiDir == "" {
		return false
	}

	// Try the request path as given, then — when the CGI directory sits
	// inside the static root — the same path with that directory's own URL
	// prefix stripped (issue #453). Each candidate goes through the full
	// containment checks; a candidate that names nothing executable is
	// simply skipped.
	filePath := ""
	for _, candidate := range cgiURLCandidates(h.cgiMount(), req.URL.Path) {
		p, err := resolveCgiPath(h.Config.CgiDir, candidate)
		if err != nil {
			log.Access("http", "CGI: %s", err)
			continue
		}
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			continue
		}
		// Reject scripts reached through a symlink that leaves the CGI
		// directory — the lexical check above cannot see those.
		if err := checkPathBoundary(p, h.Config.CgiDir); err != nil {
			log.Access("http", "CGI: %s", err)
			continue
		}
		filePath = p
		break
	}
	if filePath == "" {
		return false
	}

	log.Associate("cgiscript", filePath)
	if h.noteForkCreated() != nil {
		log.Error("http", "Fork not allowed since maxforks amount has been reached. CGI was not run.")
		http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
		return true
	}
	defer h.noteForkCompleted()

	// Build extra environment for CGI handler.
	// Go's cgi.Handler sets standard CGI variables (RFC 3875 §4.1)
	// automatically from the HTTP request. Env provides additional
	// variables like SERVER_SOFTWARE and any passed parent env vars.
	envlen := len(h.Config.ParentEnv)
	cgienv := make([]string, envlen+1)
	if envlen > 0 {
		copy(cgienv, h.Config.ParentEnv)
	}
	cgienv[envlen] = "SERVER_SOFTWARE=" + h.Config.ServerSoftware
	cgiHandler := &cgi.Handler{
		Path: filePath,
		Env:  cgienv,
	}
	log.Access("http", "CGI")
	cgiHandler.ServeHTTP(w, req)
	return true
}

// boundedDir is an http.FileSystem that serves files from a directory but
// refuses any file reached through a symlink that leaves it, any path with
// a dotfile or dot-directory segment (.git, .env, .ssh, ...), and any
// directory that has no index.html (so http.FileServer never gets a chance
// to auto-list its contents). http.Dir alone rejects ".." traversal but
// still follows symlinks out of the root, which would disclose arbitrary
// file contents, and hands every directory straight through, dotfiles and
// listings included.
type boundedDir struct {
	root string
	// execDirs are directories whose files must never be served as static
	// content: they were configured to be *executed*, and handing back
	// their source is a disclosure, not a fallback (issue #453). Both
	// --cgidir and --dir name such a directory. Membership is decided by
	// where the file really lives, not by the URL, so a symlink from the
	// static tree into one of them is refused too.
	execDirs []string
	fs       http.FileSystem
}

// wellKnownDir is the one dotfile exemption: RFC 8615 reserves
// "/.well-known/" as the standard location for URIs meant to be served
// publicly (ACME's /.well-known/acme-challenge/<token>, security.txt, and
// friends). Refusing it would silently break a standardized, unauthenticated
// HTTP convention with no flag to opt back in. The exemption is an exact
// match on the first path segment only — it is not a prefix or a pattern,
// so a further dot segment nested inside .well-known (or a directory that
// merely starts with the same name) is still refused, and the
// symlink-boundary check below still applies to everything inside it.
const wellKnownDir = ".well-known"

// hasDotSegment reports whether any path segment of name begins with "."
// once the path has been cleaned, except the first segment if it is
// exactly wellKnownDir. path.Clean resolves "." and ".." segments away, so
// anything left starting with "." is a genuine dotfile or dot-directory
// name (.git, .env, .ssh, ...), not a traversal artifact.
func hasDotSegment(name string) bool {
	first := true
	for _, part := range strings.Split(path.Clean("/"+name), "/") {
		if part == "" {
			continue
		}
		if first {
			first = false
			if part == wellKnownDir {
				continue
			}
		}
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func (d boundedDir) Open(name string) (http.File, error) {
	// Refuse dotfiles and dot-directories outright, before touching the
	// filesystem: .git/config, .env, .ssh/id_rsa and the like are never
	// meant to be served, wherever in the tree they sit.
	if hasDotSegment(name) {
		return nil, os.ErrNotExist
	}
	// http.Dir cleans name, rejects "..", and returns proper os errors
	// (so FileServer maps missing files to 404, not 500).
	f, err := d.fs.Open(name)
	if err != nil {
		return nil, err
	}
	// The file exists; confirm its real path is still within root before
	// handing it back. filepath.Join(root, clean-name) mirrors how
	// http.Dir maps the request to disk.
	full := filepath.Join(d.root, filepath.FromSlash(path.Clean("/"+name)))
	if err := checkPathBoundary(full, d.root); err != nil {
		f.Close()
		return nil, os.ErrNotExist
	}
	// Never serve a file that lives in a directory configured for
	// execution. The CGI and WebSocket handlers run before this one and
	// take everything they recognize, but what they decline (a directory, a
	// spelling of the URL they do not route, a request with no Upgrade
	// header at all) must 404 here rather than be handed back as source.
	for _, dir := range d.execDirs {
		// Refuse on a definite "inside" only. relUnknown serves the file:
		// full resolved cleanly through checkPathBoundary just above, so
		// the doubt is about an exec directory that does not resolve —
		// a --cgidir or --dir naming nothing, which has no source to
		// disclose. See dirRel for the other three readings.
		if _, r := dirRelation(full, dir); r == relInside {
			f.Close()
			return nil, os.ErrNotExist
		}
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.IsDir() {
		// Only hand back a directory handle if it has an index.html that
		// is a regular file: that keeps http.FileServer's own
		// index-serving path working while denying it the fallback branch
		// that auto-generates a listing. Recursing through d.Open (rather
		// than opening the underlying fs directly) means the index file
		// itself is still subject to the dotfile and symlink-boundary
		// checks above.
		//
		// The not-a-directory part is load-bearing, not tidiness. Opening
		// successfully is a weaker condition than net/http's: serveFile
		// only treats the index as the response if it is not itself a
		// directory, and otherwise falls through to dirList. A directory
		// literally named index.html therefore satisfied this gate and
		// still got listed. Matching the stronger condition here means the
		// listing branch is unreachable by construction rather than by an
		// assumption about another package's control flow.
		indexName := strings.TrimSuffix(path.Clean("/"+name), "/") + "/index.html"
		idx, err := d.Open(indexName)
		if err != nil {
			f.Close()
			return nil, os.ErrNotExist
		}
		idxInfo, err := idx.Stat()
		idx.Close()
		if err != nil || idxInfo.IsDir() {
			f.Close()
			return nil, os.ErrNotExist
		}
	}
	return f, nil
}

// staticExecExclusions returns the directories the static handler must
// refuse to serve from, or nil when there is nothing to exclude.
//
// Both flags that configure execution qualify: --cgidir (issue #453) and
// --dir. The reasoning is the same for each and does not depend on which
// handler would have run the file — the static handler is the last one
// tried, so whatever the earlier handlers decline lands here, and for a
// --dir tree that is every request without an Upgrade header, which is to
// say every request a browser makes for the file.
//
// It is derived from the config on every request rather than cached on the
// server, so it cannot go stale against a hand-built WebsocketdServer that
// never went through NewWebsocketdServer — the guard is worthless if a
// caller can end up with the field unset and no exclusion at all. For the
// same reason --dir is recognized by a non-empty ScriptDir rather than by
// UsingScriptDir: the two always agree in a parsed command line, and the
// stricter of the pair is the one to key a refusal on.
//
// The one configuration a directory is skipped for is a static root at or
// inside it (--cgidir=/PAGE --staticdir=/PAGE/htm, or the oldest demo
// layout of all, --dir=. --staticdir=.): every static file is then inside
// that directory, so the exclusion would leave the static handler with
// nothing whatsoever to serve. Those layouts keep their existing behavior.
func (h *WebsocketdServer) staticExecExclusions() []string {
	if h.Config.StaticDir == "" {
		return nil
	}
	var dirs []string
	for _, dir := range []string{h.Config.CgiDir, h.Config.ScriptDir} {
		if dir == "" {
			continue
		}
		// Drop the exclusion on a definite "inside" only — the layout
		// described above, where excluding dir would leave the static
		// handler nothing at all to serve. relUnknown keeps the exclusion:
		// a directory nobody can resolve is not one to start handing files
		// out of. That is the opposite of boundedDir.Open's reading, which
		// is why dirRelation is asked here rather than a bool.
		if _, r := dirRelation(h.Config.StaticDir, dir); r == relInside {
			continue
		}
		dirs = append(dirs, dir)
	}
	return dirs
}

// serveStatic serves static files from the configured directory. Returns true if handled.
func (h *WebsocketdServer) serveStatic(w http.ResponseWriter, req *http.Request, log *LogScope) bool {
	if h.Config.StaticDir == "" {
		return false
	}
	log.Access("http", "STATIC")
	fs := boundedDir{
		root:     h.Config.StaticDir,
		execDirs: h.staticExecExclusions(),
		fs:       http.Dir(h.Config.StaticDir),
	}
	http.FileServer(fs).ServeHTTP(w, req)
	return true
}

// TellURL is a helper function that changes http to https or ws to wss in case if SSL is used
func (h *WebsocketdServer) TellURL(scheme, host, path string) string {
	if len(host) > 0 && host[0] == ':' {
		host = h.hostname + host
	}
	if h.Config.Ssl {
		return scheme + "s://" + host + path
	}
	return scheme + "://" + host + path
}

func (h *WebsocketdServer) noteForkCreated() error {
	// note that forks can be nil since the construct could've been created by
	// someone who is not using NewWebsocketdServer
	if h.forks != nil {
		select {
		case h.forks <- 1:
			return nil
		default:
			return ErrForkNotAllowed
		}
	}
	return nil
}

func (h *WebsocketdServer) noteForkCompleted() {
	if h.forks != nil {
		select {
		case <-h.forks:
			return
		default:
			// This should never happen — it means noteForkCompleted was called
			// more times than noteForkCreated. Log rather than crash the server.
			h.Log.Error("server", "noteForkCompleted called with no active forks")
			return
		}
	}
}

func checkOrigin(req *http.Request, config *Config, log *LogScope) (err error) {
	origin := req.Header.Get("Origin")
	if origin == "" || (origin == "null" && config.AllowOrigins == nil) {
		origin = "file:"
	}

	originParsed, err := url.ParseRequestURI(origin)
	if err != nil {
		log.Access("session", "Origin parsing error: %s", err)
		return err
	}

	log.Associate("origin", originParsed.String())

	if config.SameOrigin || config.AllowOrigins != nil {
		originServer, originPort, err := tellHostPort(originParsed.Host, originParsed.Scheme == "https")
		if err != nil {
			log.Access("session", "Origin hostname parsing error: %s", err)
			return err
		}
		if config.SameOrigin {
			localServer, localPort, err := tellHostPort(req.Host, req.TLS != nil)
			if err != nil {
				log.Access("session", "Request hostname parsing error: %s", err)
				return err
			}
			if originServer != localServer || originPort != localPort {
				log.Access("session", "Same origin policy mismatch")
				return fmt.Errorf("same origin policy violated")
			}
		}
		if config.AllowOrigins != nil {
			if !matchOrigin(originServer, originPort, originParsed.Scheme, config.AllowOrigins) {
				log.Access("session", "Origin is not listed in allowed list")
				return fmt.Errorf("origin list matches were not found")
			}
		}
	}
	return nil
}

// matchOrigin checks if the given origin server/port/scheme matches any entry
// in the allowed origins list. Extracted for testability.
//
// Port semantics (issue #473): an entry with an explicit port matches that
// port only. A portless entry matches only the scheme's default port (80 for
// http, 443 for https — both, if the entry carries no scheme). Appending
// ":*" opts back in to matching any port, e.g. --origin=trusted.com:*, for
// setups where every service on the host is trusted. Portless entries used
// to match any port implicitly, so a single allowlisted host also vouched
// for whatever else happened to listen on its other ports.
func matchOrigin(originServer, originPort, originScheme string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		// Strip an explicit ":*" wildcard before anything else — url.Parse
		// rejects it as an invalid port, so it must not reach the scheme
		// handling below.
		anyPort := false
		if strings.HasSuffix(allowed, ":*") {
			anyPort = true
			allowed = strings.TrimSuffix(allowed, ":*")
		}
		allowedScheme := ""
		if pos := strings.Index(allowed, "://"); pos > 0 {
			allowedURL, err := url.Parse(allowed)
			if err != nil {
				continue
			}
			if allowedURL.Scheme != originScheme {
				continue
			}
			allowedScheme = allowedURL.Scheme
			allowed = allowed[pos+3:]
		}

		// Explicit wildcard: any port on this host.
		if anyPort {
			allowServer, _, err := tellHostPort(allowed, false)
			if err == nil && allowServer == originServer {
				return true
			}
			continue
		}

		// An explicit port ("host:port") matches that port only. A missing
		// port is an error from SplitHostPort — as is a bare bracketed IPv6
		// literal — which falls through to the portless handling below.
		if host, port, err := net.SplitHostPort(allowed); err == nil {
			if port != "" && host == originServer && port == originPort {
				return true
			}
			continue
		}

		// Portless entry: the host must match and the origin port must be a
		// default port of a scheme the entry accepts (either scheme when the
		// entry itself carries none).
		allowServer, _, err := tellHostPort(allowed, false)
		if err != nil || allowServer != originServer {
			continue
		}
		if originPort == "80" && (allowedScheme == "" || allowedScheme == "http") {
			return true
		}
		if originPort == "443" && (allowedScheme == "" || allowedScheme == "https") {
			return true
		}
	}
	return false
}

func tellHostPort(host string, ssl bool) (server, port string, err error) {
	server, port, err = net.SplitHostPort(host)
	if err != nil {
		if addrerr, ok := err.(*net.AddrError); ok && strings.Contains(addrerr.Err, "missing port") {
			server = host
			if ssl {
				port = "443"
			} else {
				port = "80"
			}
			err = nil
		}
	}
	return server, port, err
}
