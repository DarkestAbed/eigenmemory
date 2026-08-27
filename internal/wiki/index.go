package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/types"
)

// IndexEntry represents a single row in the index catalog.
type IndexEntry struct {
	Slug     string
	Type     types.PageType
	Title    string
	OneLiner string
	Tags     []string
	Updated  string
}

// LoadIndex parses index.md into a slice of entries.
func LoadIndex(paths *config.Paths) ([]IndexEntry, error) {
	data, err := os.ReadFile(paths.IndexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []IndexEntry{}, nil
		}
		return nil, fmt.Errorf("read index: %w", err)
	}

	// Minimal parser: each bullet line looks like:
	// - [Title](entity/slug.md) — one-liner
	var entries []IndexEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-") {
			continue
		}
		entry := parseIndexLine(line)
		if entry.Slug != "" {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// SaveIndex regenerates index.md from the current wiki pages.
func SaveIndex(paths *config.Paths) error {
	var entries []IndexEntry
	for _, pageType := range types.ValidPageTypes() {
		pages, err := ListPages(paths, pageType)
		if err != nil {
			return err
		}
		for _, page := range pages {
			title := extractTitle(page.Body)
			if title == "" {
				title = page.Slug
			}
			entries = append(entries, IndexEntry{
				Slug:     page.Slug,
				Type:     page.Frontmatter.Type,
				Title:    title,
				OneLiner: extractOneLiner(page.Body),
				Tags:     page.Frontmatter.Tags,
				Updated:  page.Frontmatter.Updated.Format("2006-01-02"),
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type < entries[j].Type
		}
		return entries[i].Slug < entries[j].Slug
	})

	var sb strings.Builder
	sb.WriteString("# EigenMemory Index\n\n")
	sb.WriteString("Catalog of all pages in this wiki, grouped by type.\n\n")

	currentType := types.PageType("")
	for _, entry := range entries {
		if entry.Type != currentType {
			currentType = entry.Type
			sb.WriteString(fmt.Sprintf("\n## %s\n\n", strings.Title(string(currentType))))
		}
		dir := wikiPageTypes[entry.Type]
		tags := ""
		if len(entry.Tags) > 0 {
			tags = fmt.Sprintf(" `[%s]`", strings.Join(entry.Tags, ", "))
		}
		sb.WriteString(fmt.Sprintf("- [%s](%s/%s.md) — %s%s\n", entry.Title, dir, entry.Slug, entry.OneLiner, tags))
	}

	return writeFileAtomic(paths.IndexFile, []byte(sb.String()))
}

// parseIndexLine extracts a minimal IndexEntry from a markdown bullet.
func parseIndexLine(line string) IndexEntry {
	line = strings.TrimPrefix(line, "-")
	line = strings.TrimSpace(line)

	start := strings.Index(line, "[")
	end := strings.Index(line, "]")
	if start == -1 || end == -1 || end <= start {
		return IndexEntry{}
	}
	title := line[start+1 : end]

	linkStart := strings.Index(line[end:], "(")
	linkEnd := strings.Index(line[end:], ")")
	if linkStart == -1 || linkEnd == -1 {
		return IndexEntry{}
	}
	link := line[end+linkStart+1 : end+linkEnd]
	dir := filepath.Dir(link)
	slug := strings.TrimSuffix(filepath.Base(link), ".md")

	var pageType types.PageType
	for pt, d := range wikiPageTypes {
		if d == dir {
			pageType = pt
			break
		}
	}

	rest := ""
	if len(line) > end+linkEnd+1 {
		rest = strings.TrimSpace(strings.TrimPrefix(line[end+linkEnd+1:], "—"))
	}

	return IndexEntry{
		Slug:     slug,
		Type:     pageType,
		Title:    title,
		OneLiner: rest,
	}
}

// extractTitle returns the first H1 from a markdown body.
func extractTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

// extractOneLiner returns the first non-empty, non-heading paragraph line.
func extractOneLiner(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		if len(line) > 80 {
			line = line[:77] + "..."
		}
		return line
	}
	return ""
}
