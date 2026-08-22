package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsFor(t *testing.T) {
	root := "/tmp/test-em"
	p := PathsFor(root)

	if p.Root != root {
		t.Errorf("Root = %q, want %q", p.Root, root)
	}
	if p.ConfigFile != filepath.Join(root, ConfigFileName) {
		t.Errorf("ConfigFile = %q", p.ConfigFile)
	}
	if p.SourcesDir != filepath.Join(root, SourcesDirName) {
		t.Errorf("SourcesDir = %q", p.SourcesDir)
	}
	if p.WikiDir != filepath.Join(root, WikiDirName) {
		t.Errorf("WikiDir = %q", p.WikiDir)
	}
	if p.Database != filepath.Join(root, DatabaseFileName) {
		t.Errorf("Database = %q", p.Database)
	}
}

func TestScopeFromCWD_Project(t *testing.T) {
	tmp := t.TempDir()
	projectDir := filepath.Join(tmp, "myproject")
	emDir := filepath.Join(projectDir, DirName)
	if err := os.MkdirAll(emDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(tmp)

	scope, paths, err := ScopeFromCWD()
	if err != nil {
		t.Fatalf("ScopeFromCWD: %v", err)
	}
	if scope != ScopeProject {
		t.Errorf("scope = %q, want %q", scope, ScopeProject)
	}
	if paths.Root != emDir {
		t.Errorf("paths.Root = %q, want %q", paths.Root, emDir)
	}
}

func TestScopeFromCWD_GlobalFallback(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	scope, paths, err := ScopeFromCWD()
	if err != nil {
		t.Fatalf("ScopeFromCWD: %v", err)
	}
	if scope != ScopeGlobal {
		t.Errorf("scope = %q, want %q", scope, ScopeGlobal)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, GlobalDirName)
	if paths.Root != want {
		t.Errorf("paths.Root = %q, want %q", paths.Root, want)
	}
}
