package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DarkestAbed/eigenmemory/internal/config"
)

func TestSetup_ScopeResolutionFails(t *testing.T) {
	unresolvableScope(t)

	if err := Setup(SetupOptions{Tool: "claude"}); err == nil {
		t.Fatal("expected error when neither project nor global scope can be resolved")
	}
}

func TestSetup_NoWikiFound(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Setup(SetupOptions{Tool: "claude"}); err == nil {
		t.Fatal("expected error when no wiki exists")
	}
}

func TestSetup_NoProjectName(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	paths := config.PathsFor(filepath.Join(tmp, config.DirName))
	if err := os.MkdirAll(paths.Root, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Setup(SetupOptions{Tool: "claude"}); err == nil {
		t.Fatal("expected error when project name is unset")
	}
}

func TestSetup_Claude(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "setupproj", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error { return Setup(SetupOptions{Tool: "claude"}) })
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if !strings.Contains(out, ".mcp.json") {
		t.Errorf("expected claude snippet, got %q", out)
	}
}

func TestSetup_Zed(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "setupproj2", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error { return Setup(SetupOptions{Tool: "zed"}) })
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if !strings.Contains(out, "context_servers") {
		t.Errorf("expected zed snippet, got %q", out)
	}
}

func TestSetup_UnknownTool(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "setupproj3", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	if err := Setup(SetupOptions{Tool: "vim"}); err == nil {
		t.Fatal("expected error for an unknown tool")
	}
}
