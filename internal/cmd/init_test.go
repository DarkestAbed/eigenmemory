package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DarkestAbed/eigenmemory/internal/config"
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

func TestInit_InvalidProjectName(t *testing.T) {
	tmp := t.TempDir()
	if err := Init(InitOptions{
		ProjectName: "../../../etc",
		Scope:       config.ScopeProject,
		Root:        filepath.Join(tmp, ".eigenmemory"),
	}); err == nil {
		t.Fatal("expected error for an invalid project name")
	}
}

func TestInit_ForceOverExisting(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, ".eigenmemory")

	if err := Init(InitOptions{ProjectName: "forceproj", Scope: config.ScopeProject, Root: root}); err != nil {
		t.Fatal(err)
	}
	// Re-running with --force must succeed and leave existing files
	// (index.md, CLAUDE.md, AGENTS.md, config.json) alone rather than error.
	if err := Init(InitOptions{ProjectName: "forceproj", Scope: config.ScopeProject, Root: root, Force: true}); err != nil {
		t.Fatalf("Init with --force: %v", err)
	}
}

func TestInit_GlobalScope(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := Init(InitOptions{ProjectName: "globalproj", Scope: config.ScopeGlobal}); err != nil {
		t.Fatalf("Init (global): %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, config.GlobalDirName)); err != nil {
		t.Errorf("expected global eigenmemory dir: %v", err)
	}
}

func TestInit_ProjectScopeFromCWD(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "cwdproj", Scope: config.ScopeProject}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, config.DirName)); err != nil {
		t.Errorf("expected .eigenmemory created under cwd: %v", err)
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
