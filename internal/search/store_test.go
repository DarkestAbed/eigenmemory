package search

import (
	"testing"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/types"
)

func TestIndexAndSearch(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	page := &types.Page{
		Frontmatter: types.Frontmatter{
			ID:   types.NewID(),
			Type: types.PageTypeEntity,
			Tags: []string{"auth", "api"},
		},
		Slug: "auth-service",
		Path: tmp + "/wiki/entity/auth-service.md",
		Body: "# Auth Service\n\nHandles login and token refresh for the API.",
	}

	if err := store.IndexPage(page); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}

	count, err := store.CountPages()
	if err != nil {
		t.Fatalf("CountPages: %v", err)
	}
	if count != 1 {
		t.Errorf("CountPages = %d, want 1", count)
	}

	results, err := store.Search("login token", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}
	if len(results) > 0 && results[0].Slug != "auth-service" {
		t.Errorf("result slug = %q, want auth-service", results[0].Slug)
	}
}

func TestRemovePage(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()

	page := &types.Page{
		Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeEntity},
		Slug:        "temp",
		Path:        tmp + "/wiki/entity/temp.md",
		Body:        "# Temp\n",
	}
	if err := store.IndexPage(page); err != nil {
		t.Fatal(err)
	}

	if err := store.RemovePage(page.Frontmatter.ID); err != nil {
		t.Fatalf("RemovePage: %v", err)
	}

	count, err := store.CountPages()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("CountPages = %d, want 0", count)
	}
}
