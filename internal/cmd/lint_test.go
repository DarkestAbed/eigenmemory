package cmd

import (
	"strings"
	"testing"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/core"
	"github.com/javi/eigenmemory/internal/types"
)

func TestLintCmd_ScopeResolutionFails(t *testing.T) {
	unresolvableScope(t)

	if err := Lint(LintOptions{}); err == nil {
		t.Fatal("expected error when neither project nor global scope can be resolved")
	}
}

func TestLintCmd_NoWikiFound(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Lint(LintOptions{}); err == nil {
		t.Fatal("expected error when no wiki exists")
	}
}

func TestLintCmd_HealthyWiki(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "lintproj", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error { return Lint(LintOptions{}) })
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if !strings.Contains(out, "Wiki is healthy.") {
		t.Errorf("expected healthy wiki message, got %q", out)
	}
}

func TestLintCmd_ReportsIssues(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "issuesproj", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	store, err := core.Open()
	if err != nil {
		t.Fatal(err)
	}
	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "main",
		Body:        "# Main\n\nSee [missing](concept/missing.md).",
	}
	if err := store.SavePage(page, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	out, err := captureStdout(t, func() error { return Lint(LintOptions{}) })
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if !strings.Contains(out, "broken-link") {
		t.Errorf("expected broken-link issue in output, got %q", out)
	}
}

func TestLintCmd_FixResolvesDrift(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "fixproj", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	store, err := core.Open()
	if err != nil {
		t.Fatal(err)
	}
	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "driftme",
		Body:        "# Driftme\n\nBody.",
	}
	if err := store.SavePage(page, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}
	if err := store.Search.RemovePage(page.Frontmatter.ID); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	out, err := captureStdout(t, func() error { return Lint(LintOptions{Fix: true}) })
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if !strings.Contains(out, "Fixing index drift") {
		t.Errorf("expected fix message, got %q", out)
	}
}
