package adapters

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/types"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
)

// contentHash returns a stable content fingerprint used to detect drift
// between a wiki page and its Claude Code memory-file projection without
// relying on filesystem mtimes (which have different precision and origin
// than the wiki's own `updated` timestamp).
func contentHash(body string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(body)))
	return hex.EncodeToString(sum[:])
}

// ClaudeMemoryPath returns the Claude Code native memory directory for a
// project, given its sanitized Claude Code project directory name (see
// config.SanitizeClaudeProjectDir / ResolveClaudeProjectDir) — NOT the
// eigenmemory project name from config.json's Name field, which is a
// different, human-chosen identifier.
func ClaudeMemoryPath(claudeProjectDir string) string {
	if claudeProjectDir == "" || config.ValidateClaudeProjectDir(claudeProjectDir) != nil {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects", claudeProjectDir, "memory")
}

// ResolveClaudeProjectDir returns the Claude Code project directory name to
// use for native memory projection/reconciliation: the value config.json
// recorded at `eigenmemory init` time, falling back to deriving it from the
// wiki root's parent directory for project-scope configs written before
// that field existed. Global-scope configs have no single Claude Code
// project directory to associate with — deriving one from the global root's
// parent (the user's home directory) would scan/project an unrelated
// ~/.claude/projects/<sanitized-home>/memory, so this returns "" for the
// global root instead of guessing; callers should treat "" as "reconcile
// does not apply here" rather than passing it on to ClaudeMemoryPath.
func ResolveClaudeProjectDir(cfg config.Config, paths *config.Paths) string {
	if cfg.ClaudeProjectDir != "" {
		return cfg.ClaudeProjectDir
	}
	if isGlobalRoot(paths.Root) {
		return ""
	}
	return config.SanitizeClaudeProjectDir(filepath.Dir(paths.Root))
}

// isGlobalRoot reports whether root is the global EigenMemory root
// (~/.eigenmemory), as opposed to a project-scoped .eigenmemory directory.
func isGlobalRoot(root string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	return filepath.Clean(root) == filepath.Clean(filepath.Join(home, config.GlobalDirName))
}

// ProjectMemoryProjection generates Claude Code memory files from the EigenMemory wiki.
func ProjectMemoryProjection(paths *config.Paths, claudeProjectDir string) error {
	memDir := ClaudeMemoryPath(claudeProjectDir)
	if memDir == "" {
		return fmt.Errorf("cannot determine Claude memory path")
	}
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return fmt.Errorf("create claude memory dir: %w", err)
	}

	// Map all wiki pages into memory files.
	var memoryFiles []string
	written := make(map[string]bool)
	projected := make(map[string]bool) // "pageType/slug" for every page projected this run
	for _, pageType := range types.ValidPageTypes() {
		pages, err := wiki.ListPages(paths, pageType)
		if err != nil {
			return err
		}
		for _, page := range pages {
			filename := memoryFilename(pageType, page.Slug)
			if filename == "" {
				continue
			}
			content := renderMemoryPage(page, pageType)
			if err := writeFileAtomic(filepath.Join(memDir, filename), []byte(content)); err != nil {
				return fmt.Errorf("write memory file %s: %w", filename, err)
			}
			memoryFiles = append(memoryFiles, filename)
			written[filename] = true
			projected[string(pageType)+"/"+page.Slug] = true
		}
	}

	if err := removeStaleProjections(memDir, written, projected); err != nil {
		return err
	}

	// Generate MEMORY.md index.
	sort.Strings(memoryFiles)
	var sb strings.Builder
	sb.WriteString("# EigenMemory Projection\n\n")
	sb.WriteString("This directory is auto-generated from `.eigenmemory/wiki/`. Do not hand-edit; run `eigenmemory reconcile` to sync changes back.\n\n")
	sb.WriteString("## Memory files\n\n")
	for _, f := range memoryFiles {
		fmt.Fprintf(&sb, "- [%s](%s)\n", f, f)
	}

	if err := writeFileAtomic(filepath.Join(memDir, "MEMORY.md"), []byte(sb.String())); err != nil {
		return fmt.Errorf("write MEMORY.md: %w", err)
	}

	return nil
}

// removeStaleProjections deletes managed memory files left behind by a
// filename-scheme change (e.g. memoryFilename's entity/concept/summary
// disambiguation): a managed file not written this run whose (pageType,
// slug) IS covered by a file that was just written is a duplicate
// projection of a page now filed under a different name — pure derived
// state, safe to delete. A managed file whose page isn't in projected at
// all is left untouched: that's the "wiki page no longer exists" case
// Reconcile already flags for manual review, not something to silently
// erase here. Unmanaged files (never projected by us) are never touched.
//
// Any edits a stale duplicate carried are expected to have already been
// folded into the wiki by a preceding Reconcile call — every caller of
// ProjectMemoryProjection in this codebase runs Reconcile first — so by the
// time this runs, deleting the duplicate loses no in-flight edits.
func removeStaleProjections(memDir string, written, projected map[string]bool) error {
	entries, err := os.ReadDir(memDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read claude memory dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "MEMORY.md" || !strings.HasSuffix(name, ".md") || written[name] {
			continue
		}
		path := filepath.Join(memDir, name)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mf, err := parseMemoryFile(path, info.ModTime())
		if err != nil {
			return fmt.Errorf("parse memory file %s: %w", name, err)
		}
		if !mf.Managed || !projected[string(mf.PageType)+"/"+mf.Slug] {
			continue
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove stale projection %s: %w", name, err)
		}
	}
	return nil
}

// memoryPrefix maps EigenMemory page types to Claude Code memory file prefixes.
// Claude Code's own native memory convention only recognizes four types
// (user, feedback, project, reference), so entity/concept/summary pages are
// all filed under the "project" prefix.
func memoryPrefix(pageType types.PageType) string {
	switch pageType {
	case types.PageTypeUser:
		return "user"
	case types.PageTypeFeedback:
		return "feedback"
	case types.PageTypeReference:
		return "reference"
	case types.PageTypeProject, types.PageTypeEntity, types.PageTypeConcept, types.PageTypeSummary:
		return "project"
	}
	return ""
}

// memoryFilename returns the Claude Code memory filename for a wiki page.
// Slugs are only unique within their own page-type directory (an entity and
// a project page can both be named "auth"), but memoryPrefix collapses
// entity/concept/summary/project onto the same "project" prefix — using
// prefix_slug.md alone would let two distinct wiki pages silently overwrite
// each other's projection. Disambiguate with the real page type whenever it
// isn't already implied by the prefix, while keeping the prefix's own
// natural type (e.g. "project_<slug>.md" for an actual project page)
// filename-compatible with prior projections.
func memoryFilename(pageType types.PageType, slug string) string {
	prefix := memoryPrefix(pageType)
	if prefix == "" {
		return ""
	}
	if string(pageType) == prefix {
		return fmt.Sprintf("%s_%s.md", prefix, slug)
	}
	return fmt.Sprintf("%s_%s_%s.md", prefix, pageType, slug)
}

// renderMemoryPage converts a wiki page into Claude Code memory file content.
func renderMemoryPage(page *types.Page, pageType types.PageType) string {
	cleanBody := wiki.StripFooters(page.Body)

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "eigenmemory_id: %s\n", page.Frontmatter.ID)
	fmt.Fprintf(&sb, "eigenmemory_type: %s\n", pageType)
	fmt.Fprintf(&sb, "eigenmemory_slug: %s\n", page.Slug)
	fmt.Fprintf(&sb, "eigenmemory_updated: %s\n", page.Frontmatter.Updated.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&sb, "eigenmemory_hash: %s\n", contentHash(cleanBody))
	if len(page.Frontmatter.Tags) > 0 {
		fmt.Fprintf(&sb, "tags: [%s]\n", strings.Join(page.Frontmatter.Tags, ", "))
	}
	if len(page.Frontmatter.Sources) > 0 {
		fmt.Fprintf(&sb, "sources: [%s]\n", strings.Join(page.Frontmatter.Sources, ", "))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(cleanBody)
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "_Projected from `.eigenmemory/wiki/%s/%s.md` via EigenMemory._\n", wikiDir(pageType), page.Slug)
	return sb.String()
}

// wikiDir maps a page type to its directory name. Duplicated from wiki package to avoid import cycle.
func wikiDir(pageType types.PageType) string {
	m := map[types.PageType]string{
		types.PageTypeEntity:    "entity",
		types.PageTypeConcept:   "concept",
		types.PageTypeSummary:   "summary",
		types.PageTypeProject:   "project",
		types.PageTypeFeedback:  "feedback",
		types.PageTypeReference: "reference",
		types.PageTypeUser:      "user",
	}
	if d, ok := m[pageType]; ok {
		return d
	}
	return ""
}

// writeFileAtomic writes data to path using a temp file and rename.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
