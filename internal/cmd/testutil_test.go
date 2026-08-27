package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// isolate chdirs into dir and points HOME at it for the duration of the
// test, so that any function under test that falls back to global scope
// (~/.eigenmemory) operates on an isolated temp directory instead of the
// real machine's home directory. Both are restored via t.Cleanup/t.Setenv.
func isolate(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	t.Setenv("HOME", dir)
}

// unresolvableScope chdirs into a fresh temp dir with no .eigenmemory
// anywhere up its tree and clears $HOME, so config.ScopeFromCWD (and
// anything built on it) fails to resolve either a project or a global
// scope. This mirrors a real environment (e.g. a minimal container) where
// $HOME is unset, and exercises the "resolve scope" error path that a
// healthy test environment never reaches otherwise.
func unresolvableScope(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	t.Setenv("HOME", "")
}

// captureStdout redirects os.Stdout for the duration of fn and returns
// whatever was written to it, alongside fn's own return value.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fnErr := fn()

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String(), fnErr
}
