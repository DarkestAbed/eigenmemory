package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/core"
	"github.com/javi/eigenmemory/internal/types"
	"github.com/javi/eigenmemory/internal/wiki"
)

func TestReconcileOnlyUpdatesChangedFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("testproj")); err != nil {
		t.Fatal(err)
	}

	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	// Create a wiki page.
	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "release-process",
		Body:        "# Release Process\n\nWe use semantic versioning.",
	}
	if err := store.SavePage(page, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}

	// Generate projection.
	if err := ProjectMemoryProjection(store.Paths, "testproj"); err != nil {
		t.Fatal(err)
	}

	// Sleep to ensure modtime is newer than wiki page.
	time.Sleep(100 * time.Millisecond)

	// Edit one memory file.
	memPath := filepath.Join(ClaudeMemoryPath("testproj"), "project_release-process.md")
	data, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := string(data) + "\nExtra line.\n"
	if err := os.WriteFile(memPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}

	// Reconcile.
	actions, err := Reconcile(store.Paths, "testproj", false)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Expect only one update.
	if len(actions) != 1 {
		t.Errorf("first reconcile actions = %v (len %d), want exactly 1", actions, len(actions))
	}

	// Now regenerate projection and reconcile again; should be no new updates.
	if err := ProjectMemoryProjection(store.Paths, "testproj"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	actions2, err := Reconcile(store.Paths, "testproj", true)
	if err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if len(actions2) != 0 {
		t.Errorf("second reconcile actions = %v (len %d), want 0", actions2, len(actions2))
	}
}

// TestReconcile_SkipsWhenWikiChangedSinceProjection is a regression test for
// C5: reconcile used to compare a memory file's raw mtime against the
// wiki's `updated` timestamp, two different clocks with different
// precision. Here the wiki changes (not the memory file) after projection;
// the hash-based comparison must recognize there is nothing to merge from
// the (unmodified, now-stale) memory file, regardless of mtime ordering.
func TestReconcile_SkipsWhenWikiChangedSinceProjection(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("testproj")); err != nil {
		t.Fatal(err)
	}

	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "release-process",
		Body:        "# Release Process\n\nOriginal body.",
	}
	if err := store.SavePage(page, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}
	if err := ProjectMemoryProjection(store.Paths, "testproj"); err != nil {
		t.Fatal(err)
	}

	// The wiki changes elsewhere (e.g. via wiki_remember); the memory file
	// is left untouched.
	loaded, err := wiki.LoadPage(store.Paths, types.PageTypeProject, "release-process")
	if err != nil {
		t.Fatal(err)
	}
	loaded.Body = "# Release Process\n\nUpdated body from the wiki side."
	if err := store.SavePage(loaded, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}

	actions, err := Reconcile(store.Paths, "testproj", true)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	for _, a := range actions {
		if strings.Contains(a, "update ") {
			t.Errorf("expected no update action (wiki is the side that changed), got: %v", actions)
		}
	}
}

// TestReconcile_ConflictFallsBackToMtimeMargin covers a memory file with no
// recorded eigenmemory_hash (e.g. written before that field existed) whose
// content differs from the wiki and whose mtime is clearly (beyond the
// conflict margin) newer than the wiki's `updated` timestamp: it should win.
func TestReconcile_ConflictFallsBackToMtimeMargin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("testproj")); err != nil {
		t.Fatal(err)
	}

	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "conflict-page",
		Body:        "# Conflict Page\n\nWiki-side content.",
	}
	if err := store.SavePage(page, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}

	memDir := ClaudeMemoryPath("testproj")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "---\neigenmemory_id: x\neigenmemory_type: project\neigenmemory_slug: conflict-page\neigenmemory_updated: 2026-01-01T00:00:00Z\n---\n\nMemory-side content, no hash recorded.\n"
	memPath := filepath.Join(memDir, "project_conflict-page.md")
	if err := os.WriteFile(memPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	newer := page.Frontmatter.Updated.Add(reconcileConflictMargin + time.Second)
	if err := os.Chtimes(memPath, newer, newer); err != nil {
		t.Fatal(err)
	}

	actions, err := Reconcile(store.Paths, "testproj", true)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	found := false
	for _, a := range actions {
		if strings.Contains(a, "conflict") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a conflict-resolved-by-mtime action, got: %v", actions)
	}
}

// TestReconcile_ConflictWithinMarginSkips is the inverse: the memory file is
// newer than the wiki page but only by a hair, well inside the conflict
// margin, so it must not win (guards against reintroducing the old
// raw-.After behavior that a bare sub-second skew could trigger).
func TestReconcile_ConflictWithinMarginSkips(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("testproj")); err != nil {
		t.Fatal(err)
	}

	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "close-call",
		Body:        "# Close Call\n\nWiki-side content.",
	}
	if err := store.SavePage(page, types.PageTypeProject); err != nil {
		t.Fatal(err)
	}

	memDir := ClaudeMemoryPath("testproj")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := "---\neigenmemory_id: x\neigenmemory_type: project\neigenmemory_slug: close-call\neigenmemory_updated: 2026-01-01T00:00:00Z\n---\n\nMemory-side content, no hash recorded.\n"
	memPath := filepath.Join(memDir, "project_close-call.md")
	if err := os.WriteFile(memPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	barelyNewer := page.Frontmatter.Updated.Add(100 * time.Millisecond)
	if err := os.Chtimes(memPath, barelyNewer, barelyNewer); err != nil {
		t.Fatal(err)
	}

	actions, err := Reconcile(store.Paths, "testproj", true)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	for _, a := range actions {
		if strings.Contains(a, "conflict") {
			t.Errorf("expected no conflict-won action within the margin, got: %v", actions)
		}
	}
}
