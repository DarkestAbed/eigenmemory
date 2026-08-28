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

	if err := store.IndexSource("abc123", "/tmp/whatever/abc123", "abc123", "2026-08-22T00:00:00Z", "design doc", []byte("some source content")); err != nil {
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
	if err := store.IndexSource("abc123", "/tmp/whatever/abc123", "abc123", "2026-08-23T00:00:00Z", "design doc", []byte("updated content")); err != nil {
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

// TestIndexBodyLinks_PreservesFrontmatterProvenance is a regression test for
// the ON CONFLICT overwrite: if frontmatter already declares a links_to edge
// to a target, body-link indexing for the same target must not relabel that
// authored edge as body-derived. Frontmatter is authoritative.
func TestIndexBodyLinks_PreservesFrontmatterProvenance(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Page with a frontmatter links_to relation (provenance "authored") to
	// auth-service, the same target its body links.
	source := &types.Page{
		Frontmatter: types.Frontmatter{
			ID: types.NewID(),
			Relations: []types.Relation{
				{From: "auth-migration", To: "auth-service", Type: "links_to", Provenance: "authored"},
			},
		},
		Slug: "auth-migration",
		Body: "# Auth Migration\n\nDepends on [[auth-service]].\n",
		Path: tmp + "/wiki/project/auth-migration.md",
	}
	if err := store.IndexPage(source); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}
	if err := store.IndexBodyLinks("auth-migration", []string{"auth-service"}); err != nil {
		t.Fatalf("IndexBodyLinks: %v", err)
	}

	relations, err := store.ListRelations()
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(relations) != 1 {
		t.Fatalf("len(relations) = %d, want 1", len(relations))
	}
	if relations[0].Provenance != "authored" {
		t.Errorf("provenance = %q, want %q (frontmatter must not be relabeled by body link)", relations[0].Provenance, "authored")
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

func TestSearch_ORFallbackRescuesNLQuery(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	// Page contains "login" and "token" but NOT "kubernetes", "access",
	// "services", "setup". A strict-AND of all those NL tokens must miss.
	page := &types.Page{
		Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeEntity},
		Slug:        "auth-service",
		Body:        "# Auth Service\n\nHandles login and token refresh.",
		Path:        tmp + "/wiki/entity/auth-service.md",
	}
	if err := store.IndexPage(page); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}

	// Strict AND (via Search) must find nothing for the over-constrained query.
	hits, err := store.Search("how is the auth service set up for access and services", 5)
	if err != nil {
		t.Fatalf("Search AND: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("strict-AND returned %d hits, want 0 (over-constrained)", len(hits))
	}

	// OR fallback via searchPagesWithFallback must rescue the page.
	rescued, err := store.searchPagesWithFallback("how is the auth service set up for access and services", 5)
	if err != nil {
		t.Fatalf("searchPagesWithFallback: %v", err)
	}
	if len(rescued) == 0 {
		t.Errorf("OR fallback returned 0 hits, want the auth-service page rescued")
	}
}

func TestStripStopwords_AllStopwords(t *testing.T) {
	// An all-stopword query must fall back to the original tokens (no empty
	// MATCH) rather than returning an empty slice.
	got := stripStopwords([]string{"how", "is", "the", "and"})
	if len(got) == 0 {
		t.Errorf("stripStopwords of all-stopword query returned empty; want original tokens")
	}
}

func TestIndexSource_FullContentSearchable(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	// A source whose distinctive content sits well past the 2KB summary
	// truncation cutoff. The summary page body only carries the prefix.
	long := strings.Repeat("placeholder preamble line.\n", 200) // ~4.6KB
	long += "kind-based deployment section with the distinctive token xyzzy-quirk.\n"
	content := []byte("# Local K8s Cluster\n\n" + long)

	if err := store.IndexSource("deadbeef", tmp+"/sources/deadbeef", "deadbeef", "2026-08-26T00:00:00Z", "local-k8s-cluster-setup", content); err != nil {
		t.Fatalf("IndexSource: %v", err)
	}

	// Query the distinctive term that only appears past the 2KB cutoff.
	hits, err := store.searchSources("xyzzy-quirk", 5)
	if err != nil {
		t.Fatalf("searchSources: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("searchSources = %d hits, want 1 (full content must be searchable past 2KB)", len(hits))
	}
	h := hits[0]
	if h.MatchSource != "source" {
		t.Errorf("MatchSource = %q, want source", h.MatchSource)
	}
	if h.SourceID != "deadbeef" {
		t.Errorf("SourceID = %q, want deadbeef", h.SourceID)
	}
	if !strings.Contains(h.Body, "xyzzy-quirk") {
		t.Errorf("snippet missing the matched term; got %q", h.Body)
	}
	if h.Title != "local-k8s-cluster-setup" {
		t.Errorf("Title = %q, want local-k8s-cluster-setup", h.Title)
	}
}

func TestSourceDisplayName_FromSummaryPage(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	// A summary page whose sources frontmatter references the source id.
	summary := &types.Page{
		Frontmatter: types.Frontmatter{
			ID:      types.NewID(),
			Type:    types.PageTypeSummary,
			Sources: []string{"feedface"},
		},
		Slug: "design-doc",
		Body:  "# Summary: design doc\n\npreamble.\n",
		Path:  tmp + "/wiki/summary/design-doc.md",
	}
	if err := store.IndexPage(summary); err != nil {
		t.Fatalf("IndexPage summary: %v", err)
	}

	name, err := store.SourceDisplayName("feedface")
	if err != nil {
		t.Fatalf("SourceDisplayName: %v", err)
	}
	if name != "Summary: design doc" {
		t.Errorf("SourceDisplayName = %q, want %q", name, "Summary: design doc")
	}

	// Unknown source id returns empty, no error.
	unknown, err := store.SourceDisplayName("nope")
	if err != nil {
		t.Fatalf("SourceDisplayName unknown: %v", err)
	}
	if unknown != "" {
		t.Errorf("SourceDisplayName unknown = %q, want empty", unknown)
	}
}

// TestNeighbors_LegacyMdSlugFallback verifies the graph traversal fallback for
// legacy summary pages whose slug carries the "-md" segment: a relation stored
// with a bare to_id "target" must still reach the "target-md" page.
func TestNeighbors_LegacyMdSlugFallback(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	legacy := &types.Page{
		Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeSummary},
		Slug:        "target-md",
		Body:        "# target-md\n",
		Path:        tmp + "/wiki/summary/target-md.md",
	}
	if err := store.IndexPage(legacy); err != nil {
		t.Fatalf("IndexPage legacy: %v", err)
	}

	carrier := &types.Page{
		Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeProject},
		Slug:        "carrier",
		Body:        "# Carrier\n",
		Path:        tmp + "/wiki/project/carrier.md",
	}
	if err := store.IndexPage(carrier); err != nil {
		t.Fatalf("IndexPage carrier: %v", err)
	}

	// carrier links [[target]] -> bare slug "target"; the page is "target-md".
	if err := store.IndexBodyLinks("carrier", []string{"target"}); err != nil {
		t.Fatalf("IndexBodyLinks: %v", err)
	}

	neighbors, err := store.Neighbors([]string{"carrier"}, 5)
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	var found bool
	for _, n := range neighbors {
		if n.Slug == "target-md" && n.MatchSource == "graph" {
			found = true
		}
	}
	if !found {
		t.Errorf("legacy target-md neighbor not reached via -md fallback; neighbors = %+v", neighbors)
	}
}

// TestNeighbors_LegacyMdSlugFallback_ReverseSeed covers the reverse direction:
// when the seed is itself a legacy "target-md" page, carriers that stored their
// edge with the bare to_id "target" (from [[target]]) must still be reached.
func TestNeighbors_LegacyMdSlugFallback_ReverseSeed(t *testing.T) {
	tmp := t.TempDir()
	paths := config.PathsFor(tmp)

	store, err := Open(paths)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = store.Close() }()

	legacy := &types.Page{
		Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeSummary},
		Slug:        "target-md",
		Body:        "# target-md\n",
		Path:        tmp + "/wiki/summary/target-md.md",
	}
	if err := store.IndexPage(legacy); err != nil {
		t.Fatalf("IndexPage legacy: %v", err)
	}

	carrier := &types.Page{
		Frontmatter: types.Frontmatter{ID: types.NewID(), Type: types.PageTypeProject},
		Slug:        "carrier",
		Body:        "# Carrier\n",
		Path:        tmp + "/wiki/project/carrier.md",
	}
	if err := store.IndexPage(carrier); err != nil {
		t.Fatalf("IndexPage carrier: %v", err)
	}

	// carrier -> target (bare to_id). Seeding the legacy target-md page must
	// still surface carrier via the reverse bare-seed lookup.
	if err := store.IndexBodyLinks("carrier", []string{"target"}); err != nil {
		t.Fatalf("IndexBodyLinks: %v", err)
	}

	neighbors, err := store.Neighbors([]string{"target-md"}, 5)
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	var found bool
	for _, n := range neighbors {
		if n.Slug == "carrier" && n.MatchSource == "graph" {
			found = true
		}
	}
	if !found {
		t.Errorf("carrier not reached when seeding legacy target-md; neighbors = %+v", neighbors)
	}
}
