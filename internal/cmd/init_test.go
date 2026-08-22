package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/javi/eigenmemory/internal/config"
)

func TestInit_CreatesStructure(t *testing.T) {
	tmp := t.TempDir()

	if err := Init(InitOptions{
		ProjectName: "test-project",
		Scope:       config.ScopeProject,
		Root:        filepath.Join(tmp, ".eigenmemory"),
	}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	for _, path := range []string{
		".eigenmemory/config.json",
		".eigenmemory/eigenmemory.db",
		".eigenmemory/wiki/index.md",
		".eigenmemory/wiki/log.md",
		"CLAUDE.md",
		"AGENTS.md",
	} {
		full := filepath.Join(tmp, path)
		if _, err := os.Stat(full); err != nil {
			t.Errorf("missing expected file %s: %v", path, err)
		}
	}
}

func TestInit_FailsWhenExists(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, ".eigenmemory")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	err := Init(InitOptions{
		ProjectName: "test-project",
		Scope:       config.ScopeProject,
		Root:        root,
	})
	if err == nil {
		t.Error("expected error when initializing existing eigenmemory, got nil")
	}
}
