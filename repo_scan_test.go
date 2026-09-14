// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The repository guards in this package each ask a different question of the
// same set of files. Four questions with four failure messages is right; four
// copies of the traversal that answers them was not. The traversal lives here,
// and each predicate stays with its guard.
//
// The files considered are the ones git tracks, not the ones lying on disk.
// Every guard's contract is "nothing *committed* contains X", and a disk walk
// cannot tell a committed file from a scratch file left in the working tree:
// an ignored file at the worktree root that quoted a guard's own output turned
// a later run red for a reason unrelated to the change under test. Reading the
// index closes that, and takes .gitignore, build output and .git with it.
// Files staged but not yet committed are listed deliberately: a file about to
// be committed is the one worth checking.
//
// Two states stay in separate branches on purpose: no repository is a skip,
// a repository that reaches almost nothing is a failure. See isCheckout.

// skipExts are binary or otherwise not worth scanning as text.
var skipExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true,
	".mp4": true, ".mov": true, ".zip": true, ".gz": true, ".woff": true,
	".woff2": true, ".ttf": true, ".otf": true, ".pdf": true, ".sum": true,
}

// repoFile is one file handed to a guard: where it is, and what is in it.
type repoFile struct {
	rel     string // slash-separated, relative to the scan root
	content string
}

// repoRoot is the directory `go test` runs this package in. The package sits
// at the repository root, so that is the whole tree.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	return root
}

// isCheckout reports whether root sits in a git working tree. It is the one
// question that separates a skip from a failure, so it is asked on its own
// rather than inferred from some later command failing: a `git ls-files` that
// errors could mean no repository, and it could equally mean a broken one.
func isCheckout(root string) bool {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree").Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// skipIfNotACheckout skips the calling guard when root is not a git working
// tree. This is the only state in which a guard is allowed to go quiet, and it
// is emphatically not the floor: an empty or barely-populated checkout still
// fails, because there the guard has a subject and cannot see it.
func skipIfNotACheckout(t *testing.T, root string) {
	t.Helper()
	if !isCheckout(root) {
		t.Skipf("%s is not a git checkout, so there is nothing committed to check "+
			"here. This is expected in an extracted source tarball; the guard needs "+
			"a checkout and this is not one.", root)
	}
}

// initTestRepo turns dir into a git checkout with everything in it staged. The
// guards enumerate the index, so a control that never populated one would be
// exercising a code path the guard does not use.
func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
}

// listRepoPaths returns the root-relative paths of every file git tracks under
// root. Outside a checkout there is no index to read and the guards have no
// subject, so that is an error and not an empty result: a scan that cannot
// reach what it is guarding has to say so rather than report a clean pass.
func listRepoPaths(root string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("listing the files tracked in %s: %w; the guards "+
			"have nothing to check outside a git checkout", root, err)
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// scanRepoText returns the readable text files under root whose relative path
// want accepts, with their contents. subject names what is being scanned and
// appears only in the floor error.
//
// The floor lives here because reaching nothing is a property of the
// traversal, not of any predicate. Reaching nothing and reaching less than
// expected are reported separately so a control can assert on the reason.
func scanRepoText(root, subject string, minFiles int, want func(rel string) bool) ([]repoFile, error) {
	paths, err := listRepoPaths(root)
	if err != nil {
		return nil, err
	}

	var files []repoFile
	for _, rel := range paths {
		if !want(rel) || skipExts[strings.ToLower(filepath.Ext(rel))] {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if readErr != nil || !isProbablyText(data) {
			continue // unreadable or binary is not a guard's business
		}
		files = append(files, repoFile{rel: rel, content: string(data)})
	}

	switch {
	case len(files) == 0:
		return nil, fmt.Errorf("%s is unreachable: no readable text file is tracked under %s; "+
			"the guard cannot pass by reaching nothing", subject, root)
	case len(files) < minFiles:
		return nil, fmt.Errorf("%s yielded %d readable text file(s), fewer than the %d expected; "+
			"the scan did not reach it, so a clean result would mean nothing",
			subject, len(files), minFiles)
	}
	return files, nil
}

// underPath reports whether a root-relative path is, or is inside, p. A guard's
// roots name a single file as readily as a directory, so both forms match.
func underPath(p string) func(rel string) bool {
	return func(rel string) bool {
		return rel == p || strings.HasPrefix(rel, p+"/")
	}
}

// isProbablyText reports whether data looks like text rather than a binary
// blob, so a guard doesn't try to match strings inside a compiled artifact.
func isProbablyText(data []byte) bool {
	if len(data) > 1<<20 {
		data = data[:1<<20]
	}
	for _, b := range data {
		if b == 0 {
			return false
		}
	}
	return true
}

// lineAround returns the single line containing byte offset idx, trimmed, so a
// failure says where to look rather than only what was found.
func lineAround(content string, idx int) string {
	start := strings.LastIndexByte(content[:idx], '\n') + 1
	end := strings.IndexByte(content[idx:], '\n')
	if end < 0 {
		end = len(content)
	} else {
		end += idx
	}
	line := strings.TrimSpace(content[start:end])
	if len(line) > 160 {
		line = line[:160] + "…"
	}
	return line
}

// TestNoIndexSkipsButAnEmptyIndexFails asserts that distinction from both
// sides: skipping outside a checkout is what lets a packager build from a
// tarball, and skipping inside one that reaches nothing is how these guards
// would die unnoticed. The control itself has to survive a tarball, which the
// first draft did not — it asserted that the live repository is a checkout,
// reproducing the regression it exists to prevent.
func TestNoIndexSkipsButAnEmptyIndexFails(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git is not installed, so there is no index to reason about: %v", err)
	}

	// Case 1: no repository. The guards must skip there, which needs this to
	// be false. The tarball build depends on it.
	if isCheckout(t.TempDir()) {
		t.Error("a bare temp directory was taken for a checkout; a source tarball " +
			"would fail three guards instead of skipping them")
	}

	// Where a .git is present, git must agree that this is a checkout. The two
	// signals are independent, and a disagreement would mean every guard skips
	// while the suite reports ok. A tarball has no .git and nothing to
	// cross-check here, which is the point rather than a gap.
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil && !isCheckout(root) {
		t.Fatalf("%s holds a .git but was not taken for a checkout; every guard "+
			"would skip and the suite would prove nothing while reporting ok", root)
	}

	// Case 2: a repository whose index reaches nothing is still a repository.
	// It must not skip, it must fail the floor.
	empty := t.TempDir()
	initTestRepo(t, empty)
	if !isCheckout(empty) {
		t.Fatal("an initialised but empty checkout was not taken for one, so the " +
			"floor below is never reached and this control proves nothing")
	}
	_, err := scanRepoText(empty, "the repository", 50, func(string) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("an empty checkout must fail the floor rather than pass or skip, got: %v", err)
	}
	t.Logf("no index skips, empty index fails: %v", err)
}
