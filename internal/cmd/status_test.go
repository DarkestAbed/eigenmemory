package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/core"
)

func TestStatus_NoWikiFound(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Status(); err == nil {
		t.Fatal("expected error when no EigenMemory wiki exists")
	}
}

func TestStatus_Success(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "statusproj", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error { return Status() })
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !strings.Contains(out, "Project:     statusproj") {
		t.Errorf("expected project name in output, got %q", out)
	}
	if !strings.Contains(out, "Relations:") || !strings.Contains(out, "Sources:") {
		t.Errorf("expected relations/sources counts in output, got %q", out)
	}
}

func TestStatus_ScopeResolutionFails(t *testing.T) {
	unresolvableScope(t)

	if err := Status(); err == nil {
		t.Fatal("expected error when neither project nor global scope can be resolved")
	}
}

func TestStatus_NoProjectNameSet(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	paths := config.PathsFor(filepath.Join(tmp, config.DirName))
	if err := os.MkdirAll(paths.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(paths.Root)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	out, err := captureStdout(t, func() error { return Status() })
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if strings.Contains(out, "Project:") {
		t.Errorf("expected no Project: line when name is unset, got %q", out)
	}
}
