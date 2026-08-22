package adapters

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/core"
	"github.com/javi/eigenmemory/internal/types"
)

func TestReconcileOnlyUpdatesChangedFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	config.SaveConfig(config.PathsFor(tmp), config.Default("testproj"))

	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

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
