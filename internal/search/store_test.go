package search

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/types"
)

func TestIndexAndSearch(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

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

// TestIndexPage_IndexesRelations is a regression test for C7: the
// `relations` table was defined in schema but never written to.
func TestIndexPage_IndexesRelations(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	page := &types.Page{
		Frontmatter: types.Frontmatter{
			ID:   types.NewID(),
			Type: types.PageTypeProject,
			Relations: []types.Relation{
				{From: "eigenmemory", To: "llm-wiki-pattern", Type: "implements"},
				{From: "eigenmemory", To: "mcp", Type: "uses"},
			},
		},
		Slug: "eigenmemory",
		Body: "# EigenMemory\n",
	}
	if err := store.IndexPage(page); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}

	count, err := store.CountRelations()
	if err != nil {
		t.Fatalf("CountRelations: %v", err)
	}
	if count != 2 {
		t.Errorf("CountRelations = %d, want 2", count)
	}

	relations, err := store.ListRelations()
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(relations) != 2 {
		t.Fatalf("len(relations) = %d, want 2", len(relations))
	}

	// Re-indexing with a smaller relation set must replace, not accumulate.
	page.Frontmatter.Relations = []types.Relation{{From: "eigenmemory", To: "mcp", Type: "uses"}}
	if err := store.IndexPage(page); err != nil {
		t.Fatalf("re-IndexPage: %v", err)
	}
	count, err = store.CountRelations()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("CountRelations after re-index = %d, want 1", count)
	}
}

// TestIndexSource_CountsSources is a regression test for C7: the `sources`
// table was defined in schema but never written to.
func TestIndexSource_CountsSources(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.IndexSource("abc123", "/tmp/whatever/abc123", "abc123", "2026-08-22T00:00:00Z"); err != nil {
		t.Fatalf("IndexSource: %v", err)
	}

	count, err := store.CountSources()
	if err != nil {
		t.Fatalf("CountSources: %v", err)
	}
	if count != 1 {
		t.Errorf("CountSources = %d, want 1", count)
	}

	// Re-indexing the same id must upsert, not duplicate.
	if err := store.IndexSource("abc123", "/tmp/whatever/abc123", "abc123", "2026-08-23T00:00:00Z"); err != nil {
		t.Fatalf("re-IndexSource: %v", err)
	}
	count, err = store.CountSources()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("CountSources after re-index = %d, want 1", count)
	}
}

func TestRemovePage(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

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

func TestOpenEnablesWAL(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	var mode string
	if err := store.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Errorf("journal_mode = %q, want \"wal\"", mode)
	}

	var busyTimeout int
	if err := store.db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busyTimeout)
	}
}

// TestConcurrentAccessDoesNotLock is a regression test for the advertised
// (but previously non-functional) multi-process use case: e.g. Zed and
// Claude Code both talking to the same project's EigenMemory database at
// once. Before WAL + busy_timeout were actually applied, this reliably hit
// "database is locked" errors.
func TestConcurrentAccessDoesNotLock(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	writer, err := Open(paths)
	if err != nil {
		t.Fatalf("Open writer: %v", err)
	}
	defer func() { _ = writer.Close() }()

	reader, err := Open(paths)
	if err != nil {
		t.Fatalf("Open reader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	var wg sync.WaitGroup
	errs := make(chan error, 40)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			page := &types.Page{
				Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeEntity},
				Slug:        fmt.Sprintf("concurrent-%d", i),
				Path:        fmt.Sprintf("%s/wiki/entity/concurrent-%d.md", tmp, i),
				Body:        "# Concurrent\n\nwrite from another connection.",
			}
			if err := writer.IndexPage(page); err != nil {
				errs <- fmt.Errorf("write %d: %w", i, err)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			if _, err := reader.Search("concurrent", 5); err != nil {
				errs <- fmt.Errorf("read %d: %w", i, err)
			}
		}
	}()

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent access error (regression: WAL/busy_timeout not effective): %v", err)
	}
}

func BenchmarkIndexPage(b *testing.B) {
	tmp := b.TempDir()
	paths := config.PathsFor(tmp)
	store, err := Open(paths)
	if err != nil {
		b.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	page := &types.Page{
		Frontmatter: types.Frontmatter{Type: types.PageTypeEntity, Tags: []string{"auth", "api"}},
		Slug:        "auth-service",
		Path:        tmp + "/wiki/entity/auth-service.md",
		Body:        "# Auth Service\n\nHandles login and token refresh for the API.",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		page.Frontmatter.ID = types.NewID()
		if err := store.IndexPage(page); err != nil {
			b.Fatalf("IndexPage: %v", err)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	tmp := b.TempDir()
	paths := config.PathsFor(tmp)
	store, err := Open(paths)
	if err != nil {
		b.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	for i := 0; i < 200; i++ {
		page := &types.Page{
			Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeEntity, Tags: []string{"auth", "api"}},
			Slug:        fmt.Sprintf("auth-service-%d", i),
			Path:        fmt.Sprintf("%s/wiki/entity/auth-service-%d.md", tmp, i),
			Body:        "# Auth Service\n\nHandles login and token refresh for the API.",
		}
		if err := store.IndexPage(page); err != nil {
			b.Fatalf("IndexPage: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Search("login token", 5); err != nil {
			b.Fatalf("Search: %v", err)
		}
	}
}

func TestIndexBodyLinks(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	target := &types.Page{
		Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeEntity},
		Slug:       "auth-service",
		Body:       "# Auth Service\n",
		Path:       tmp + "/wiki/entity/auth-service.md",
	}
	if err := store.IndexPage(target); err != nil {
		t.Fatalf("IndexPage target: %v", err)
	}

	source := &types.Page{
		Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeProject},
		Slug:       "auth-migration",
		Body:       "# Auth Migration\n\nDepends on [[auth-service]] and [[auth-service|alias]].\n",
		Path:       tmp + "/wiki/project/auth-migration.md",
	}
	if err := store.IndexPage(source); err != nil {
		t.Fatalf("IndexPage source: %v", err)
	}
	// Body links are recorded as links_to edges; duplicates collapse to one row.
	if err := store.IndexBodyLinks("auth-migration", []string{"auth-service", "auth-service"}); err != nil {
		t.Fatalf("IndexBodyLinks: %v", err)
	}

	count, err := store.CountRelations()
	if err != nil {
		t.Fatalf("CountRelations: %v", err)
	}
	if count != 1 {
		t.Errorf("CountRelations = %d, want 1 (deduped links_to)", count)
	}

	relations, err := store.ListRelations()
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(relations) != 1 || relations[0].To != "auth-service" || relations[0].Type != "links_to" || relations[0].Provenance != "body" {
		t.Errorf("unexpected relation: %+v", relations)
	}

	// Re-running IndexPage clears all relations for the slug (including
	// links_to); IndexBodyLinks must then restore them to stay idempotent.
	if err := store.IndexPage(source); err != nil {
		t.Fatalf("re-IndexPage: %v", err)
	}
	if _, err := store.CountRelations(); err != nil {
		t.Fatal(err)
	}
	if err := store.IndexBodyLinks("auth-migration", []string{"auth-service"}); err != nil {
		t.Fatalf("re-IndexBodyLinks: %v", err)
	}
	count, err = store.CountRelations()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("CountRelations after re-index = %d, want 1", count)
	}
}

func TestSearchWithGraph_ExpandsNeighbors(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Seed page that the query will hit via FTS.
	seed := &types.Page{
		Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeProject},
		Slug:        "auth-migration",
		Body:        "# Auth Migration\n\nMigrate the login flow.",
		Path:        tmp + "/wiki/project/auth-migration.md",
	}
	if err := store.IndexPage(seed); err != nil {
		t.Fatalf("IndexPage seed: %v", err)
	}

	// Neighbor page with no overlapping FTS terms — only reachable via graph.
	neighbor := &types.Page{
		Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeEntity},
		Slug:        "token-vault",
		Body:        "# Token Vault\n\nStores refresh tokens.",
		Path:        tmp + "/wiki/entity/token-vault.md",
	}
	if err := store.IndexPage(neighbor); err != nil {
		t.Fatalf("IndexPage neighbor: %v", err)
	}

	// Edge seed -> neighbor.
	if err := store.IndexBodyLinks("auth-migration", []string{"token-vault"}); err != nil {
		t.Fatalf("IndexBodyLinks: %v", err)
	}

	results, err := store.SearchWithGraph("login flow", 5)
	if err != nil {
		t.Fatalf("SearchWithGraph: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("len(results) = %d, want at least 2 (FTS hit + graph neighbor)", len(results))
	}

	var ftsSeen, graphSeen bool
	for _, r := range results {
		switch r.MatchSource {
		case "fts":
			if r.Slug == "auth-migration" {
				ftsSeen = true
			}
		case "graph":
			if r.Slug == "token-vault" {
				graphSeen = true
			}
		}
	}
	if !ftsSeen {
		t.Errorf("FTS hit auth-migration missing from results: %+v", results)
	}
	if !graphSeen {
		t.Errorf("graph neighbor token-vault missing from results: %+v", results)
	}

	// The neighbor must not be duplicated as an FTS hit (its body has no
	// "login flow" terms).
	for _, r := range results {
		if r.Slug == "token-vault" && r.MatchSource == "fts" {
			t.Errorf("token-vault appeared as an FTS hit, should only be graph")
		}
	}
}
