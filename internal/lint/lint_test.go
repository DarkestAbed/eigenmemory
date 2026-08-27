package lint

import (
	"os"
	"testing"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/core"
	"github.com/javi/eigenmemory/internal/types"
	"github.com/javi/eigenmemory/internal/wiki"
)

func TestLintHealthyWiki(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("healthy")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	// Index links to the only project page.
	if err := wiki.SaveIndex(store.Paths); err != nil {
		t.Fatal(err)
	}
	// Rewrite index.md with a manual link so decision is not an orphan.
	indexBody := "# Index\n\n- [Decision](project/decision.md)\n"
	if err := os.WriteFile(store.Paths.IndexFile, []byte(indexBody), 0o644); err != nil {
		t.Fatal(err)
	}

	decision := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "decision",
		Body:        "# Decision\n\nWe decided on Go.",
	}
	if err := store.SavePage(decision, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}

	report, err := Run(store.Paths, store.Search)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Errorf("issues = %v, want none", report.Issues)
	}
}

func TestLintDetectsOrphan(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("orphan")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := wiki.SaveIndex(store.Paths); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Paths.IndexFile, []byte("# Index\n\nNo links here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	orphan := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "lonely",
		Body:        "# Lonely\n\nNo one links to me.",
	}
	if err := store.SavePage(orphan, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}

	report, err := Run(store.Paths, store.Search)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("issues = %v (len %d), want 1", report.Issues, len(report.Issues))
	}
	if report.Issues[0].Category != CategoryOrphan {
		t.Errorf("category = %s, want %s", report.Issues[0].Category, CategoryOrphan)
	}
}

func TestLintDetectsBrokenLink(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("broken")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := wiki.SaveIndex(store.Paths); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Paths.IndexFile, []byte("# Index\n\n- [Main](project/main.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "main",
		Body:        "# Main\n\nSee [missing](concept/missing.md).",
	}
	if err := store.SavePage(p, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}

	report, err := Run(store.Paths, store.Search)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("issues = %v (len %d), want 1", report.Issues, len(report.Issues))
	}
	if report.Issues[0].Category != CategoryBrokenLink {
		t.Errorf("category = %s, want %s", report.Issues[0].Category, CategoryBrokenLink)
	}
}

func TestLintDetectsIndexDrift(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("drift")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	// Write a markdown page directly, bypassing the indexer.
	p := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "drifted",
		Body:        "# Drifted\n\nNot in the index.",
	}
	if err := store.SavePage(p, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}

	// Remove the page from SQLite to simulate drift.
	if err := store.Search.RemovePage(p.Frontmatter.ID); err != nil {
		t.Fatal(err)
	}

	report, err := Run(store.Paths, store.Search)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, i := range report.Issues {
		if i.Category == CategoryDrift && i.Page == "project/drifted" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected drift issue for project/drifted, got %v", report.Issues)
	}
}

func TestLintIgnoresLogAndIndex(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("logidx")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	// index.md and log.md are never orphans; even with empty bodies they pass.
	if err := wiki.SaveIndex(store.Paths); err != nil {
		t.Fatal(err)
	}
	if err := wiki.AppendLog(store.Paths, wiki.OpInit, "test", "test"); err != nil {
		t.Fatal(err)
	}

	report, err := Run(store.Paths, store.Search)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, i := range report.Issues {
		if i.Category == CategoryOrphan && (i.Page == "summary/index" || i.Page == "summary/log") {
			t.Errorf("index/log flagged as orphan: %v", i)
		}
	}
	// Remove the drift warnings we expect because the test only saves index/log.
	for _, i := range report.Issues {
		if i.Category != CategoryDrift {
			t.Errorf("unexpected non-drift issue: %v", i)
		}
	}
}

func TestLintResolvesDriftWithFix(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("fix")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	p := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "fixme",
		Body:        "# Fixme\n\nBody.",
	}
	if err := store.SavePage(p, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}
	if err := store.Search.RemovePage(p.Frontmatter.ID); err != nil {
		t.Fatal(err)
	}

	report, err := Run(store.Paths, store.Search)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !report.FixableIndexDrift() {
		t.Fatal("expected fixable drift")
	}

	if err := store.RebuildIndex(); err != nil {
		t.Fatal(err)
	}
	report, err = Run(store.Paths, store.Search)
	if err != nil {
		t.Fatalf("Run after fix: %v", err)
	}
	for _, i := range report.Issues {
		if i.Category == CategoryDrift {
			t.Errorf("drift issue remained after rebuild: %v", i)
		}
	}
}

// TestLintDetectsBrokenRelation is a regression test for C7: the
// `relations` frontmatter field is now actually validated by lint instead
// of being silently ignored dead data.
func TestLintDetectsBrokenRelation(t *testing.T) {
	tmp := t.TempDir()
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("relations")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	if err := wiki.SaveIndex(store.Paths); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Paths.IndexFile, []byte("# Index\n\n- [Main](project/main.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := &types.Page{
		Frontmatter: types.Frontmatter{
			ID:     types.NewID(),
			Type:   types.PageTypeProject,
			Status: types.PageStatusActive,
			Relations: []types.Relation{
				{From: "main", To: "does-not-exist", Type: "implements"},
			},
		},
		Slug: "main",
		Body: "# Main\n\nNo markdown link, just a relation.",
	}
	if err := store.SavePage(p, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}

	report, err := Run(store.Paths, store.Search)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	found := false
	for _, i := range report.Issues {
		if i.Category == CategoryBrokenRelation {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a broken-relation issue, got %v", report.Issues)
	}
}

func TestLinkSlug(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"project/decision.md", "decision"},
		{"decision.md", "decision"},
		{"concept/llm-wiki-pattern", "llm-wiki-pattern"},
		{"entity/foo.md#heading", "foo"},
	}
	for _, c := range cases {
		got := linkSlug(c.in)
		if got != c.want {
			t.Errorf("linkSlug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
