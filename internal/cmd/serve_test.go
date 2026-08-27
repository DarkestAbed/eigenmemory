package cmd

import (
	"os"
	"testing"
)

func TestServe_RequiresMCPFlag(t *testing.T) {
	if err := Serve(false); err == nil {
		t.Fatal("expected error when --mcp flag is not set")
	}
}

func TestServe_MCPMode(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	// Give the server a stdin that returns EOF immediately, so server.Run()
	// returns right away instead of blocking on the real process stdin.
	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	_ = w.Close()
	defer func() { os.Stdin = origStdin }()

	if err := Serve(true); err != nil {
		t.Fatalf("Serve: %v", err)
	}
}
