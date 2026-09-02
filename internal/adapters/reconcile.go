package adapters

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/types"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
)

// MemoryFile represents a parsed Claude Code memory file.
type MemoryFile struct {
	Filename string
	Path     string
	ModTime  time.Time
	ID       string
	PageType types.PageType
	Slug     string
	Tags     []string
	Sources  []string
	Body     string
	// Hash is the content fingerprint recorded at the time this file was
	// last projected from the wiki (eigenmemory_hash frontmatter). Empty for
	// memory files written before this field existed.
	Hash string
	// Managed is true when the file carries eigenmemory provenance fields,
	// i.e. it was previously written by ProjectMemoryProjection. false means
	// it was written by Claude Code's own native auto-memory feature and has
	// never been reconciled into the wiki before (Slug/PageType, if set, come
	// from that native name/metadata.type frontmatter instead).
	Managed bool
}

// reconcileConflictMargin is how much newer a memory file's mtime must be
// than the wiki page's `updated` timestamp before it wins a genuine
// conflict (both sides changed since the last projection). This avoids
// false positives from sub-second clock/precision skew between a
// filesystem mtime and a YAML timestamp.
const reconcileConflictMargin = 2 * time.Second

// ScanClaudeMemory reads all memory files in the Claude Code memory
// directory identified by claudeProjectDir (see ClaudeMemoryPath).
func ScanClaudeMemory(claudeProjectDir string) ([]MemoryFile, error) {
	memDir := ClaudeMemoryPath(claudeProjectDir)
	entries, err := os.ReadDir(memDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read claude memory dir: %w", err)
	}

	var files []MemoryFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(memDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		mf, err := parseMemoryFile(path, info.ModTime())
		if err != nil {
			return nil, fmt.Errorf("parse memory file %s: %w", entry.Name(), err)
		}
		files = append(files, mf)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Filename < files[j].Filename
	})
	return files, nil
}

// Reconcile merges newer or novel Claude Code memory edits back into the
// EigenMemory wiki. claudeProjectDir identifies the Claude Code memory
// directory (see ClaudeMemoryPath / config.ResolveClaudeProjectDir) — it is
// not the eigenmemory project name.
func Reconcile(paths *config.Paths, claudeProjectDir string, dryRun bool) ([]string, error) {
	files, err := ScanClaudeMemory(claudeProjectDir)
	if err != nil {
		return nil, err
	}

	var actions []string
	for _, mf := range files {
		if mf.Filename == "MEMORY.md" {
			continue // Index file, not a source of truth.
		}
		if mf.Slug == "" {
			actions = append(actions, fmt.Sprintf("skip %s (no recognizable memory metadata)", mf.Filename))
			continue
		}

		exists := wiki.PageExists(paths, mf.PageType, mf.Slug)
		if !exists {
			if mf.Managed {
				// Previously projected by us, and the wiki page was since removed:
				// never resurrect it without explicit human review.
				actions = append(actions, fmt.Sprintf("missing wiki page for %s/%s (manual review needed)", mf.PageType, mf.Slug))
				continue
			}
			// A genuinely new native memory Claude Code's own auto-memory feature
			// wrote on its own: adopt it into the wiki instead of leaving it
			// stranded outside eigenmemory's canonical store.
			action, err := importNativeMemory(paths, mf, dryRun)
			if err != nil {
				return nil, err
			}
			actions = append(actions, action)
			continue
		}

		page, err := wiki.LoadPage(paths, mf.PageType, mf.Slug)
		if err != nil {
			return nil, fmt.Errorf("load wiki page %s/%s: %w", mf.PageType, mf.Slug, err)
		}

		// Compare content by hash rather than raw equality/mtime: a
		// filesystem mtime and a YAML timestamp are different clocks with
		// different precision, so comparing them directly (mf.ModTime.After)
		// produces false positives from clock/precision skew alone.
		cleanBody := wiki.StripFooters(page.Body)
		wikiHash := contentHash(cleanBody)
		memHash := contentHash(mf.Body)

		if memHash == wikiHash {
			continue // Already in sync (content-identical).
		}

		var update bool
		var reason string
		switch {
		case mf.Hash != "" && mf.Hash == wikiHash:
			// The wiki hasn't changed since this file was last projected;
			// the memory file was hand-edited since then.
			update = true
			reason = fmt.Sprintf("update %s/%s from %s", mf.PageType, mf.Slug, mf.Filename)
		case mf.Hash != "" && mf.Hash == memHash:
			// The memory file still matches what was last projected; the
			// wiki changed elsewhere (e.g. via wiki_remember) since then.
			// Nothing to merge — regenerating the projection will pick up
			// the wiki's current content.
			reason = fmt.Sprintf("skip %s (wiki changed since last projection)", mf.Filename)
		case mf.ModTime.Sub(page.Frontmatter.Updated) > reconcileConflictMargin:
			// No usable baseline hash (e.g. a memory file predating this
			// field) or a genuine conflict where both sides changed: fall
			// back to an explicit "meaningfully newer" mtime margin rather
			// than a raw `.After`.
			update = true
			reason = fmt.Sprintf("update %s/%s from %s (conflict: memory file is newer)", mf.PageType, mf.Slug, mf.Filename)
		default:
			reason = fmt.Sprintf("skip %s (wiki is newer)", mf.Filename)
		}

		actions = append(actions, reason)
		if update && !dryRun {
			page.Body = mf.Body
			page.Frontmatter.Tags = mergeTags(page.Frontmatter.Tags, mf.Tags)
			page.Frontmatter.Sources = mergeUnique(page.Frontmatter.Sources, mf.Sources)
			if err := wiki.SavePage(paths, page, mf.PageType); err != nil {
				return nil, fmt.Errorf("save reconciled page: %w", err)
			}
		}
	}

	return actions, nil
}

// importNativeMemory creates a new wiki page from a memory file that Claude
// Code's own auto-memory feature wrote independently of eigenmemory. A
// malformed slug is reported and skipped rather than aborting the whole
// reconcile batch, since memory file content can originate from
// unsanitized agent output (the same class of risk the slug path-traversal
// fix addressed elsewhere).
func importNativeMemory(paths *config.Paths, mf MemoryFile, dryRun bool) (string, error) {
	if err := wiki.ValidateSlug(mf.Slug); err != nil {
		return fmt.Sprintf("skip %s (invalid slug %q: %v)", mf.Filename, mf.Slug, err), nil
	}
	if dryRun {
		return fmt.Sprintf("would create %s/%s from %s (new native memory)", mf.PageType, mf.Slug, mf.Filename), nil
	}

	page := &types.Page{
		Frontmatter: types.DefaultFrontmatter(mf.PageType),
		Slug:        mf.Slug,
		Body:        mf.Body,
	}
	page.Frontmatter.Tags = mf.Tags
	page.Frontmatter.Sources = mf.Sources
	if err := wiki.SavePage(paths, page, mf.PageType); err != nil {
		return "", fmt.Errorf("create wiki page from %s: %w", mf.Filename, err)
	}
	return fmt.Sprintf("create %s/%s from %s (new native memory)", mf.PageType, mf.Slug, mf.Filename), nil
}

// nativeTypeToPageType maps Claude Code's native auto-memory metadata.type
// (the four values its own memory-writing convention uses) onto the wiki's
// PageType. An unrecognized or empty value returns ok=false so callers skip
// rather than guess.
func nativeTypeToPageType(t string) (types.PageType, bool) {
	switch types.PageType(t) {
	case types.PageTypeUser, types.PageTypeFeedback, types.PageTypeProject, types.PageTypeReference:
		return types.PageType(t), true
	default:
		return "", false
	}
}

// parseMemoryFile parses a single Claude Code memory markdown file.
func parseMemoryFile(path string, modTime time.Time) (MemoryFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MemoryFile{}, err
	}

	mf := MemoryFile{
		Filename: filepath.Base(path),
		Path:     path,
		ModTime:  modTime,
	}

	content := string(data)
	if strings.HasPrefix(content, "---") {
		// Very small frontmatter parser to avoid pulling in yaml here.
		end := strings.Index(content[3:], "\n---")
		if end != -1 {
			front := content[3 : 3+end]
			body := content[3+end+4:]
			mf.ID = extractFrontmatterValue(front, "eigenmemory_id")
			mf.Slug = extractFrontmatterValue(front, "eigenmemory_slug")
			mf.PageType = types.PageType(extractFrontmatterValue(front, "eigenmemory_type"))
			mf.Hash = extractFrontmatterValue(front, "eigenmemory_hash")
			mf.Tags = parseListValue(extractFrontmatterValue(front, "tags"))
			mf.Sources = parseListValue(extractFrontmatterValue(front, "sources"))
			mf.Body = wiki.StripFooters(strings.TrimSpace(body))
			mf.Managed = mf.ID != "" || mf.Slug != ""

			if !mf.Managed {
				// Not one of our own projections: fall back to Claude Code's
				// native auto-memory frontmatter (name / metadata.type), so
				// genuinely new native memories are recognized instead of
				// silently skipped.
				if name := extractFrontmatterValue(front, "name"); name != "" {
					if pt, ok := nativeTypeToPageType(extractFrontmatterValue(front, "type")); ok {
						mf.Slug = name
						mf.PageType = pt
					}
				}
			}
		}
	} else {
		mf.Body = strings.TrimSpace(content)
	}

	return mf, nil
}

func extractFrontmatterValue(front, key string) string {
	for _, line := range strings.Split(front, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+":"))
		}
	}
	return ""
}

func parseListValue(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func mergeTags(existing, incoming []string) []string {
	return mergeUnique(existing, incoming)
}

func mergeUnique(a, b []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range a {
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range b {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
