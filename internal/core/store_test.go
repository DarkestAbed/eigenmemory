package core

import (
	"os"
	"testing"

	"github.com/javi/eigenmemory/internal/types"
)

func TestSavePageAndSearch(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer store.Close()

	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeEntity),
		Slug:        "auth-service",
		Body:        "# Auth Service\n\nHandles login.",
	}
	if err := store.SavePage(page, types.PageTypeEntity); err != nil {
		t.Fatalf("SavePage: %v", err)
	}

	if _, err := os.Stat(store.Paths.Database); err != nil {
		t.Errorf("database not created: %v", err)
	}

	results, err := store.Search.Search("login", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}
}

func TestRebuildIndex(t *testing.T) {
	tmp := t.TempDir()

	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeConcept),
		Slug:        "hexagonal",
		Body:        "# Hexagonal Architecture\n\nPorts and adapters pattern.",
	}

	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer store.Close()

	if err := store.SavePage(page, types.PageTypeConcept); err != nil {
		t.Fatalf("SavePage: %v", err)
	}

	if err := store.Search.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if err := store.RebuildIndex(); err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}

	count, err := store.Search.CountPages()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("CountPages after rebuild = %d, want 1", count)
	}
}
