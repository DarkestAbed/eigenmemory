package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DarkestAbed/eigenmemory/internal/core"
	"github.com/DarkestAbed/eigenmemory/internal/types"
)

// TestProjectMemoryProjection_NoCollisionAcrossPageTypes guards against a
// regression where entity/concept/project/summary pages all collapse onto
// the same "project_" memory-file prefix. Before memoryFilename disambiguated
// by page type, two pages with the same slug in different wiki directories
// (e.g. entity/auth.md and project/auth.md) silently overwrote each other's
// projected memory file.
func TestProjectMemoryProjection_NoCollisionAcrossPageTypes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	store, err := core.OpenAt(filepath.Join(tmp, "proj", ".eigenmemory"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	same := "auth"
	for _, pt := range []types.PageType{types.PageTypeEntity, types.PageTypeConcept, types.PageTypeSummary, types.PageTypeProject} {
		page := &types.Page{
			Frontmatter: types.DefaultFrontmatter(pt),
			Slug:        same,
			Body:        "# " + string(pt) + "\n\nContent unique to the " + string(pt) + " page.",
		}
		if err := store.SavePage(page, pt); err != nil {
			t.Fatalf("save %s page: %v", pt, err)
		}
	}

	if err := ProjectMemoryProjection(store.Paths, "collisionproj"); err != nil {
		t.Fatalf("project memory: %v", err)
	}

	memDir := ClaudeMemoryPath("collisionproj")
	entries, err := os.ReadDir(memDir)
	if err != nil {
		t.Fatal(err)
	}
	var projected []string
	for _, e := range entries {
		if e.Name() != "MEMORY.md" && strings.HasSuffix(e.Name(), ".md") {
			projected = append(projected, e.Name())
		}
	}
	if len(projected) != 4 {
		t.Fatalf("expected 4 distinct memory files for 4 same-slug pages, got %d: %v", len(projected), projected)
	}

	for _, pt := range []types.PageType{types.PageTypeEntity, types.PageTypeConcept, types.PageTypeSummary, types.PageTypeProject} {
		data, err := os.ReadFile(filepath.Join(memDir, memoryFilename(pt, same)))
		if err != nil {
			t.Fatalf("read projected file for %s: %v", pt, err)
		}
		want := "Content unique to the " + string(pt) + " page."
		if !strings.Contains(string(data), want) {
			t.Errorf("%s memory file missing its own content (got clobbered by another page type): %q", pt, data)
		}
	}
}

// TestProjectMemoryProjection_RemovesStaleLegacyDuplicate covers the gap
// Sourcery flagged in the fix above: disambiguating filenames going forward
// does nothing for an existing memory directory that already has the page
// projected under the old, collapsed filename. Without cleanup, that stale
// duplicate keeps getting reconciled as if it were an independent memory
// file, and can overwrite the canonical wiki page with outdated content.
func TestProjectMemoryProjection_RemovesStaleLegacyDuplicate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	store, err := core.OpenAt(filepath.Join(tmp, "proj", ".eigenmemory"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeEntity),
		Slug:        "auth",
		Body:        "# entity\n\nEntity page content.",
	}
	if err := store.SavePage(page, types.PageTypeEntity); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadPage(types.PageTypeEntity, "auth")
	if err != nil {
		t.Fatal(err)
	}

	memDir := ClaudeMemoryPath("migrateproj")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Simulate a pre-fix projection: memoryFilename now routes an entity
	// page to "project_entity_<slug>.md", never the collapsed
	// "project_<slug>.md" a prior version of this projection would have
	// written.
	legacyPath := filepath.Join(memDir, "project_auth.md")
	if err := os.WriteFile(legacyPath, []byte(renderMemoryPage(loaded, types.PageTypeEntity)), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ProjectMemoryProjection(store.Paths, "migrateproj"); err != nil {
		t.Fatalf("project memory: %v", err)
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("expected stale legacy projection %s to be removed after re-projecting under its new name, stat err = %v", legacyPath, err)
	}
	newPath := filepath.Join(memDir, memoryFilename(types.PageTypeEntity, "auth"))
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("expected new disambiguated projection to exist: %v", err)
	}
}

// TestProjectMemoryProjection_KeepsOrphanedManagedFileForManualReview makes
// sure the new cleanup step doesn't overreach: a managed memory file whose
// wiki page has genuinely been deleted (not renamed to a new filename) must
// stay untouched, since that's the "missing wiki page, needs manual review"
// case Reconcile already flags — silently deleting it here would erase the
// evidence Reconcile relies on to raise that flag.
func TestProjectMemoryProjection_KeepsOrphanedManagedFileForManualReview(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	store, err := core.OpenAt(filepath.Join(tmp, "proj", ".eigenmemory"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	memDir := ClaudeMemoryPath("orphanproj")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	orphanPath := filepath.Join(memDir, "project_gone.md")
	orphanContent := "---\neigenmemory_id: abc123\neigenmemory_type: project\neigenmemory_slug: gone\neigenmemory_updated: 2026-01-01T00:00:00Z\n---\n\nA page that no longer exists in the wiki.\n"
	if err := os.WriteFile(orphanPath, []byte(orphanContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ProjectMemoryProjection(store.Paths, "orphanproj"); err != nil {
		t.Fatalf("project memory: %v", err)
	}

	if _, err := os.Stat(orphanPath); err != nil {
		t.Errorf("expected orphaned managed file (page deleted from wiki, not renamed) to be left alone for manual review, got removed: %v", err)
	}
}
