package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/core"
	"github.com/DarkestAbed/eigenmemory/internal/types"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
)

func TestResolveClaudeProjectDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	projectPaths := config.PathsFor(filepath.Join(tmp, "some-project", config.DirName))

	if got := ResolveClaudeProjectDir(config.Config{ClaudeProjectDir: "explicit-dir"}, projectPaths); got != "explicit-dir" {
		t.Errorf("with ClaudeProjectDir set, got %q, want %q", got, "explicit-dir")
	}

	want := config.SanitizeClaudeProjectDir(filepath.Join(tmp, "some-project"))
	if got := ResolveClaudeProjectDir(config.Config{}, projectPaths); got != want {
		t.Errorf("project-scope fallback: got %q, want %q", got, want)
	}

	// Regression: a global-scope config with no ClaudeProjectDir must not
	// derive one from the home directory — that would reconcile against an
	// unrelated ~/.claude/projects/<sanitized-home>/memory.
	globalPaths := config.PathsFor(filepath.Join(tmp, config.GlobalDirName))
	if got := ResolveClaudeProjectDir(config.Config{}, globalPaths); got != "" {
		t.Errorf("global-scope fallback = %q, want empty", got)
	}
}

// TestClaudeMemoryPath_AllowsIncidentalDotDot is a regression test for the
// same class of bug ValidateClaudeProjectDir guards against: a project path
// like "/work/foo..bar" sanitizes to "-work-foo..bar", which must still
// resolve to a real memory path rather than "" (which ValidateProjectName's
// blanket ".." rejection previously caused).
func TestClaudeMemoryPath_AllowsIncidentalDotDot(t *testing.T) {
	dir := config.SanitizeClaudeProjectDir("/work/foo..bar")
	if got := ClaudeMemoryPath(dir); got == "" {
		t.Errorf("ClaudeMemoryPath(%q) = \"\", want a real path", dir)
	}
}

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

// TestReconcile_ImportsNewNativeMemory covers Claude Code's own auto-memory
// feature writing a file directly (name/description/metadata.type
// frontmatter, no eigenmemory_* fields at all) with no corresponding wiki
// page yet: reconcile must adopt it into the wiki rather than silently
// skipping it as "no eigenmemory metadata".
func TestReconcile_ImportsNewNativeMemory(t *testing.T) {
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

	memDir := ClaudeMemoryPath("testproj")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	native := "---\nname: release-workflow\ndescription: \"How to ship a release\"\nmetadata:\n  node_type: memory\n  type: feedback\n  originSessionId: abc123\n  modified: 2026-08-28T22:03:57.741Z\n---\n\nThe release flow is PR, merge, tag, push.\n"
	memPath := filepath.Join(memDir, "release-workflow.md")
	if err := os.WriteFile(memPath, []byte(native), 0o644); err != nil {
		t.Fatal(err)
	}

	// Dry run must report the proposed creation without writing anything.
	dryActions, err := Reconcile(store.Paths, "testproj", true)
	if err != nil {
		t.Fatalf("Reconcile (dry run): %v", err)
	}
	if wiki.PageExists(store.Paths, types.PageTypeFeedback, "release-workflow") {
		t.Fatal("dry run must not create the wiki page")
	}
	foundProposal := false
	for _, a := range dryActions {
		if strings.Contains(a, "would create feedback/release-workflow") {
			foundProposal = true
		}
	}
	if !foundProposal {
		t.Errorf("expected a 'would create' proposal, got: %v", dryActions)
	}

	actions, err := Reconcile(store.Paths, "testproj", false)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	found := false
	for _, a := range actions {
		if strings.Contains(a, "create feedback/release-workflow") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a create action, got: %v", actions)
	}

	page, err := wiki.LoadPage(store.Paths, types.PageTypeFeedback, "release-workflow")
	if err != nil {
		t.Fatalf("expected wiki page to be created: %v", err)
	}
	if !strings.Contains(page.Body, "PR, merge, tag, push") {
		t.Errorf("page body = %q, want native memory content", page.Body)
	}
}

// TestReconcile_SkipsNativeMemoryWithUnrecognizedType covers a native memory
// file whose metadata.type isn't one of the four values Claude Code's
// auto-memory feature actually uses: it must be skipped, never guessed.
func TestReconcile_SkipsNativeMemoryWithUnrecognizedType(t *testing.T) {
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

	memDir := ClaudeMemoryPath("testproj")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	native := "---\nname: some-fact\nmetadata:\n  type: mystery\n---\n\nUnclassifiable content.\n"
	memPath := filepath.Join(memDir, "some-fact.md")
	if err := os.WriteFile(memPath, []byte(native), 0o644); err != nil {
		t.Fatal(err)
	}

	actions, err := Reconcile(store.Paths, "testproj", true)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	for _, a := range actions {
		if strings.Contains(a, "create") {
			t.Errorf("expected no create action for an unrecognized type, got: %v", actions)
		}
	}
}

// TestReconcile_SkipsInvalidNativeSlug covers a native memory file whose name
// isn't a valid wiki slug: it must be skipped with a diagnostic rather than
// aborting the whole reconcile batch.
func TestReconcile_SkipsInvalidNativeSlug(t *testing.T) {
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

	memDir := ClaudeMemoryPath("testproj")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	native := "---\nname: ../../etc/passwd\nmetadata:\n  type: project\n---\n\nMalicious slug attempt.\n"
	memPath := filepath.Join(memDir, "bad-slug.md")
	if err := os.WriteFile(memPath, []byte(native), 0o644); err != nil {
		t.Fatal(err)
	}

	actions, err := Reconcile(store.Paths, "testproj", false)
	if err != nil {
		t.Fatalf("Reconcile must not abort the batch on an invalid slug: %v", err)
	}
	found := false
	for _, a := range actions {
		if strings.Contains(a, "invalid slug") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an invalid-slug skip action, got: %v", actions)
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
