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
