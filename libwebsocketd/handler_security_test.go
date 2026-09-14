package libwebsocketd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCheckPathBoundary(t *testing.T) {
	t.Run("path within boundary", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "script.sh")
		os.WriteFile(file, []byte("#!/bin/sh"), 0755)

		err := checkPathBoundary(file, dir)
		if err != nil {
			t.Errorf("path within boundary should be allowed: %v", err)
		}
	})

	t.Run("path is boundary itself", func(t *testing.T) {
		dir := t.TempDir()
		err := checkPathBoundary(dir, dir)
		if err != nil {
			t.Errorf("path equal to boundary should be allowed: %v", err)
		}
	})

	t.Run("path outside boundary", func(t *testing.T) {
		dir := t.TempDir()
		outside := filepath.Join(os.TempDir(), "outside-boundary-test")
		os.WriteFile(outside, []byte("secret"), 0644)
		defer os.Remove(outside)

		err := checkPathBoundary(outside, dir)
		if err == nil {
			t.Error("path outside boundary should be rejected")
		}
	})

	t.Run("symlink escape", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks require admin on Windows")
		}
		dir := t.TempDir()

		// Create a file outside the boundary
		outsideDir := t.TempDir()
		outsideFile := filepath.Join(outsideDir, "secret.sh")
		os.WriteFile(outsideFile, []byte("#!/bin/sh\necho pwned"), 0755)

		// Create a symlink inside the boundary pointing outside
		symlink := filepath.Join(dir, "escape.sh")
		os.Symlink(outsideFile, symlink)

		err := checkPathBoundary(symlink, dir)
		if err == nil {
			t.Error("SECURITY: symlink escaping boundary should be rejected")
		}
	})

	t.Run("symlink within boundary", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks require admin on Windows")
		}
		dir := t.TempDir()

		// Create a real file
		realFile := filepath.Join(dir, "real.sh")
		os.WriteFile(realFile, []byte("#!/bin/sh"), 0755)

		// Create a symlink to the real file (within same dir)
		symlink := filepath.Join(dir, "link.sh")
		os.Symlink(realFile, symlink)

		err := checkPathBoundary(symlink, dir)
		if err != nil {
			t.Errorf("symlink within boundary should be allowed: %v", err)
		}
	})
}

// chdirForTest moves the process into dir for the duration of the test and
// restores the previous working directory afterwards. No test in this
// package calls t.Parallel, so a process-global chdir is safe here; do not
// add t.Parallel to a test that uses this.
func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	t.Cleanup(func() { os.Chdir(prev) })
}

// relBoundaryTree builds the fixture both relative-boundary tests use:
//
//	base/secret/loot.txt      — must never be reachable through the boundary
//	base/serve/index.html     — a legitimate file inside the boundary
//	base/serve/escape         — symlink to ../secret, i.e. out of the tree
//	base/serve/sub/           — a subdirectory to chdir into
//	base/serve-secret/loot.txt — sibling sharing a name prefix with "serve"
func relBoundaryTree(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	mkdir := func(p string) {
		if err := os.MkdirAll(filepath.Join(base, p), 0755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(p, content string) {
		if err := os.WriteFile(filepath.Join(base, p), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	mkdir("secret")
	mkdir("serve/sub")
	mkdir("serve-secret")
	write("secret/loot.txt", "TOP SECRET")
	write("serve/index.html", "hello")
	write("serve-secret/loot.txt", "TOP SECRET")
	if err := os.Symlink("../secret", filepath.Join(base, "serve", "escape")); err != nil {
		t.Fatal(err)
	}
	return base
}

// TestCheckPathBoundaryRelativeBoundary covers boundaries spelled relative to
// the process working directory — what an operator types as `--staticdir=.`
// or `--cgidir=..`.
//
// checkPathBoundary compared EvalSymlinks(path) against EvalSymlinks(boundary)
// without putting the two in a common frame of reference. EvalSymlinks
// preserves the relativeness of its argument, so with a relative boundary the
// two sides were not comparable and the answer was arbitrary rather than
// merely strict:
//
//   - boundary "." refused everything (nothing EvalSymlinks returns starts
//     with "./"), so a relative static root served 404 for every request;
//   - boundary ".." allowed everything that stayed spelled with a leading
//     "../", which a symlink pointing out of the tree still does — so the
//     boundary failed *open* and disclosed (or, under --cgidir, executed)
//     files outside the served directory.
//
// Each spelling therefore carries both directions: the file that must be
// allowed and the escape that must be refused.
func TestCheckPathBoundaryRelativeBoundary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require admin on Windows")
	}

	t.Run("boundary dot", func(t *testing.T) {
		base := relBoundaryTree(t)
		chdirForTest(t, filepath.Join(base, "serve"))

		if err := checkPathBoundary("index.html", "."); err != nil {
			t.Errorf("file inside a %q boundary should be allowed: %v", ".", err)
		}
		if err := checkPathBoundary(".", "."); err != nil {
			t.Errorf("the %q boundary itself should be allowed: %v", ".", err)
		}
		if err := checkPathBoundary(filepath.Join("escape", "loot.txt"), "."); err == nil {
			t.Error("SECURITY: symlink escaping a \".\" boundary should be rejected")
		}
	})

	t.Run("boundary dot slash", func(t *testing.T) {
		base := relBoundaryTree(t)
		chdirForTest(t, filepath.Join(base, "serve"))

		if err := checkPathBoundary("index.html", "./"); err != nil {
			t.Errorf("file inside a %q boundary should be allowed: %v", "./", err)
		}
		if err := checkPathBoundary(filepath.Join("escape", "loot.txt"), "./"); err == nil {
			t.Error("SECURITY: symlink escaping a \"./\" boundary should be rejected")
		}
	})

	t.Run("boundary dotdot", func(t *testing.T) {
		base := relBoundaryTree(t)
		chdirForTest(t, filepath.Join(base, "serve", "sub"))

		if err := checkPathBoundary(filepath.Join("..", "index.html"), ".."); err != nil {
			t.Errorf("file inside a %q boundary should be allowed: %v", "..", err)
		}
		// The fail-open: EvalSymlinks turns "../escape/loot.txt" into
		// "../../secret/loot.txt", which still carries the "../" prefix the
		// old comparison accepted.
		if err := checkPathBoundary(filepath.Join("..", "escape", "loot.txt"), ".."); err == nil {
			t.Error("SECURITY: symlink escaping a \"..\" boundary should be rejected")
		}
	})

	t.Run("boundary relative subdir", func(t *testing.T) {
		base := relBoundaryTree(t)
		chdirForTest(t, base)

		if err := checkPathBoundary(filepath.Join("serve", "index.html"), "./serve"); err != nil {
			t.Errorf("file inside a %q boundary should be allowed: %v", "./serve", err)
		}
		if err := checkPathBoundary(filepath.Join("serve", "escape", "loot.txt"), "./serve"); err == nil {
			t.Error("SECURITY: symlink escaping a \"./serve\" boundary should be rejected")
		}
		// A sibling directory whose name merely starts with the boundary's
		// name is outside it; a plain string prefix test would let it through.
		if err := checkPathBoundary(filepath.Join("serve-secret", "loot.txt"), "serve"); err == nil {
			t.Error("SECURITY: name-prefix sibling should be rejected")
		}
	})

	t.Run("mixed frames", func(t *testing.T) {
		base := relBoundaryTree(t)
		chdirForTest(t, filepath.Join(base, "serve"))
		abs := filepath.Join(base, "serve")

		// A relative path against an absolute boundary, and the reverse:
		// both name the same place and must agree with the all-absolute
		// answer rather than depending on how each side is spelled.
		if err := checkPathBoundary("index.html", abs); err != nil {
			t.Errorf("relative path inside an absolute boundary should be allowed: %v", err)
		}
		if err := checkPathBoundary(filepath.Join(abs, "index.html"), "."); err != nil {
			t.Errorf("absolute path inside a relative boundary should be allowed: %v", err)
		}
		if err := checkPathBoundary(filepath.Join("escape", "loot.txt"), abs); err == nil {
			t.Error("SECURITY: relative escape from an absolute boundary should be rejected")
		}
		if err := checkPathBoundary(filepath.Join(abs, "escape", "loot.txt"), "."); err == nil {
			t.Error("SECURITY: absolute escape from a relative boundary should be rejected")
		}
	})
}
