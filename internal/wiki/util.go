package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeFileAtomic writes data to path using a temporary file and rename.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
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

// ValidateSlug rejects slugs that are empty or could escape the wiki
// directory tree when joined into a filesystem path (path separators, ".."
// segments, or anything that doesn't round-trip through Slugify unchanged).
// Apply this at every boundary where a slug is accepted from outside the
// process (MCP tool arguments, CLI flags) as well as defensively in the page
// read/write path itself.
func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug is empty")
	}
	if strings.ContainsAny(slug, `/\`) || strings.Contains(slug, "..") {
		return fmt.Errorf("invalid slug %q: must not contain path separators or \"..\"", slug)
	}
	if Slugify(slug) != slug {
		return fmt.Errorf("invalid slug %q: must be lowercase alphanumeric with hyphens", slug)
	}
	return nil
}

// StripFooters removes EigenMemory auto-appended footer lines (and any
// surrounding trailing whitespace) from the end of a markdown body.
func StripFooters(body string) string {
	lines := strings.Split(body, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	for len(lines) > 0 {
		trimmed := strings.TrimSpace(lines[len(lines)-1])
		if strings.HasPrefix(trimmed, "_Projected from `.eigenmemory/wiki/") ||
			strings.HasPrefix(trimmed, "_Synced from Claude Code memory") {
			lines = lines[:len(lines)-1]
			for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
				lines = lines[:len(lines)-1]
			}
			continue
		}
		break
	}
	return strings.Join(lines, "\n")
}
