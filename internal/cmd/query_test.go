package cmd

import (
	"strings"
	"testing"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/core"
	"github.com/DarkestAbed/eigenmemory/internal/types"
)

func TestQuery_EmptyQuery(t *testing.T) {
	if err := Query(QueryOptions{Query: "   "}); err == nil {
		t.Fatal("expected error for an empty query")
	}
}

func TestQuery_ScopeResolutionFails(t *testing.T) {
	unresolvableScope(t)

	if err := Query(QueryOptions{Query: "anything"}); err == nil {
		t.Fatal("expected error when neither project nor global scope can be resolved")
	}
}

func TestQuery_NoWikiFound(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Query(QueryOptions{Query: "anything"}); err == nil {
		t.Fatal("expected error when no wiki exists")
	}
}

func TestQuery_NoResults(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "queryproj", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error { return Query(QueryOptions{Query: "nonexistent term"}) })
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !strings.Contains(out, "No relevant pages found") {
		t.Errorf("expected no-results message, got %q", out)
	}
}

func TestQuery_WithResults(t *testing.T) {
	tmp := t.TempDir()
	isolate(t, tmp)

	if err := Init(InitOptions{ProjectName: "queryproj2", Scope: config.ScopeProject}); err != nil {
		t.Fatal(err)
	}

	store, err := core.Open()
	if err != nil {
		t.Fatal(err)
	}
	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeEntity),
		Slug:        "auth-service",
		Body:        "# Auth Service\n\nHandles login and token refresh.",
	}
	if err := store.SavePage(page, types.PageTypeEntity); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	out, err := captureStdout(t, func() error { return Query(QueryOptions{Query: "login", Limit: 0}) })
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !strings.Contains(out, "auth-service") {
		t.Errorf("expected result in output, got %q", out)
	}
	if !strings.Contains(out, "Cite the relevant page") {
		t.Errorf("expected citation reminder in output, got %q", out)
	}
}
