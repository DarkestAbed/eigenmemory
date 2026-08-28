package core

import (
	"os"
	"strings"
	"testing"

	"github.com/DarkestAbed/eigenmemory/internal/types"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
)

func TestSavePageAndSearch(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer func() { _ = store.Close() }()

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

func TestOpen_UsesScopeFromCWD(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmp)

	// No project .eigenmemory anywhere up the tree, so this falls back to
	// (an isolated) global scope, which Open() must create on demand.
	store, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := os.Stat(store.Paths.Root); err != nil {
		t.Errorf("expected store root to exist: %v", err)
	}
}

func TestOpen_ScopeResolutionFails(t *testing.T) {
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", "")

	if _, err := Open(); err == nil {
		t.Fatal("expected error when neither project nor global scope can be resolved")
	}
}

func TestLoadPage(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeEntity),
		Slug:        "loadme",
		Body:        "# Loadme\n\nBody.",
	}
	if err := store.SavePage(page, types.PageTypeEntity); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadPage(types.PageTypeEntity, "loadme")
	if err != nil {
		t.Fatalf("LoadPage: %v", err)
	}
	if loaded.Slug != "loadme" {
		t.Errorf("Slug = %q, want loadme", loaded.Slug)
	}
}

func TestSaveIndexAndAppendLog(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := store.SaveIndex(); err != nil {
		t.Fatalf("SaveIndex: %v", err)
	}
	if _, err := os.Stat(store.Paths.IndexFile); err != nil {
		t.Errorf("index.md not created: %v", err)
	}

	if err := store.AppendLog(wiki.OpInit, "test", "test details"); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}
	data, err := os.ReadFile(store.Paths.LogFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "test details") {
		t.Errorf("log.md missing appended entry")
	}
}

func TestClose_NilSearch(t *testing.T) {
	s := &Store{}
	if err := s.Close(); err != nil {
		t.Errorf("Close on a zero-value Store: %v", err)
	}
}

func TestRebuildIndex_IncludesSources(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := store.Ingest(IngestOptions{Source: "some text", Name: "src"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Search.Clear(); err != nil {
		t.Fatal(err)
	}
	if err := store.RebuildIndex(); err != nil {
		t.Fatalf("RebuildIndex: %v", err)
	}

	count, err := store.Search.CountSources()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("CountSources after rebuild = %d, want 1", count)
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
	defer func() { _ = store.Close() }()

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

func TestSavePage_IndexesBodyLinks(t *testing.T) {
	tmp := t.TempDir()
	store, err := OpenAt(tmp)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Target page that the source will wikilink to.
	target := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeEntity),
		Slug:        "auth-service",
		Body:        "# Auth Service\n\nHandles login.",
	}
	if err := store.SavePage(target, types.PageTypeEntity); err != nil {
		t.Fatalf("SavePage target: %v", err)
	}

	// Source page whose body wikilinks the target. The edge should be
	// recorded automatically without any hand-authored frontmatter relation.
	source := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "auth-migration",
		Body:        "# Auth Migration\n\nDepends on [[auth-service]].\n",
	}
	if err := store.SavePage(source, types.PageTypeProject); err != nil {
		t.Fatalf("SavePage source: %v", err)
	}

	relations, err := store.Search.ListRelations()
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	var found bool
	for _, r := range relations {
		if r.From == "auth-migration" && r.To == "auth-service" && r.Type == "links_to" {
			found = true
		}
	}
	if !found {
		t.Errorf("body link not indexed; relations = %+v", relations)
	}

	// The body link must NOT have leaked into the page's frontmatter on disk.
	loaded, err := wiki.LoadPage(store.Paths, types.PageTypeProject, "auth-migration")
	if err != nil {
		t.Fatalf("LoadPage: %v", err)
	}
	if len(loaded.Frontmatter.Relations) != 0 {
		t.Errorf("frontmatter relations mutated by body-link indexing: %+v", loaded.Frontmatter.Relations)
	}
}
