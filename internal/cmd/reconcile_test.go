package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DarkestAbed/eigenmemory/internal/config"
)

func TestReconcileCmd_ScopeResolutionFails(t *testing.T) {
	unresolvableScope(t)

	if err := Reconcile(ReconcileOptions{}); err == nil {
		t.Fatal("expected error when neither project nor global scope can be resolved")
	}
}

func TestReconcileCmd_InvalidConfigRejected(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	paths := config.PathsFor(filepath.Join(tmp, config.DirName))
	if err := os.MkdirAll(paths.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	malicious := []byte(`{"name": "../../../etc"}`)
	if err := os.WriteFile(paths.ConfigFile, malicious, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Reconcile(ReconcileOptions{}); err == nil {
		t.Fatal("expected error for a malicious project name in config.json")
	}
}

func TestReconcileCmd_NoProjectName(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	paths := config.PathsFor(filepath.Join(tmp, config.DirName))
	if err := os.MkdirAll(paths.Root, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Reconcile(ReconcileOptions{}); err == nil {
		t.Fatal("expected error when project name is unset")
	}
}

// TestReconcileCmd_GlobalScopeRejected is a regression test: global-scope
// memory has no single associated Claude Code project directory, so
// reconcile must reject it explicitly rather than silently deriving one
// from the home directory (which would scan/project an unrelated
// ~/.claude/projects/<sanitized-home>/memory).
func TestReconcileCmd_GlobalScopeRejected(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	work := filepath.Join(tmp, "work")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := Init(InitOptions{ProjectName: "globalproj", Scope: config.ScopeGlobal}); err != nil {
		t.Fatal(err)
	}

	if err := Reconcile(ReconcileOptions{DryRun: true}); err == nil {
		t.Fatal("expected reconcile to reject global-scope memory")
	}
}

func TestReconcileCmd_DryRun(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "reconcileproj", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	if err := Reconcile(ReconcileOptions{DryRun: true}); err != nil {
		t.Fatalf("Reconcile (dry run): %v", err)
	}
}

func TestReconcileCmd_Apply(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "reconcileproj2", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	if err := Reconcile(ReconcileOptions{DryRun: false}); err != nil {
		t.Fatalf("Reconcile (apply): %v", err)
	}
}
