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
)

// TestCrossToolConsistency simulates the full Zed -> Claude Code -> wiki round-trip.
func TestCrossToolConsistency(t *testing.T) {
	tmp := t.TempDir()
	// Keep Claude Code memory files inside the test temp dir.
	t.Setenv("HOME", tmp)

	if err := config.SaveConfig(config.PathsFor(tmp), config.Default("crosstool")); err != nil {
		t.Fatal(err)
	}
	store, err := core.OpenAt(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	// 1. Zed (via MCP wiki_remember equivalent) writes a project page.
	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(types.PageTypeProject),
		Slug:        "package-manager-decision",
		Body:        "# Package Manager Decision\n\nWe decided to migrate to pnpm for faster installs.",
	}
	if err := store.SavePage(page, types.PageTypeProject); err != nil {
		t.Fatalf("Zed write: %v", err)
	}

	// 2. EigenMemory projects the wiki into Claude Code native memory files.
	if err := ProjectMemoryProjection(store.Paths, "crosstool"); err != nil {
		t.Fatalf("project memory: %v", err)
	}

	// 3. Claude Code reads the memory file and the fact is present.
	memPath := filepath.Join(ClaudeMemoryPath("crosstool"), "project_package-manager-decision.md")
	memData, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("read claude memory file: %v", err)
	}
	if !strings.Contains(string(memData), "pnpm") {
		t.Errorf("claude memory file missing fact: %s", memData)
	}

	// Sleep so the memory file edit has a strictly newer modtime.
	time.Sleep(100 * time.Millisecond)

	// 4. Claude Code user edits the native memory file.
	updated := strings.Replace(string(memData), "pnpm", "pnpm and Yarn Berry", 1)
	if err := os.WriteFile(memPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("edit claude memory file: %v", err)
	}

	// 5. Reconcile merges the Claude Code edit back into the wiki.
	actions, err := Reconcile(store.Paths, "crosstool", false)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(actions) != 1 {
		t.Errorf("actions = %v (len %d), want 1", actions, len(actions))
	}

	// 6. Zed (via MCP wiki_recall equivalent) sees the updated fact.
	updatedPage, err := store.LoadPage(types.PageTypeProject, "package-manager-decision")
	if err != nil {
		t.Fatalf("load updated page: %v", err)
	}
	if !strings.Contains(updatedPage.Body, "Yarn Berry") {
		t.Errorf("wiki page missing claude edit: %q", updatedPage.Body)
	}

	// 7. A subsequent dry-run reconcile reports nothing to do.
	if err := ProjectMemoryProjection(store.Paths, "crosstool"); err != nil {
		t.Fatalf("re-project: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	actions2, err := Reconcile(store.Paths, "crosstool", true)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if len(actions2) != 0 {
		t.Errorf("second reconcile actions = %v (len %d), want 0", actions2, len(actions2))
	}
}
