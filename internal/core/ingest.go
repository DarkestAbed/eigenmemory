package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DarkestAbed/eigenmemory/internal/types"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
)

// IngestOptions controls the ingest operation.
type IngestOptions struct {
	Source string // file path or raw text
	Name   string // human-readable source name
	IsPath bool
}

// Ingest ingests a raw source into the EigenMemory wiki.
func (s *Store) Ingest(opts IngestOptions) (string, error) {
	var data []byte
	var name string
	var err error
	if opts.IsPath {
		data, err = os.ReadFile(opts.Source)
		if err != nil {
			return "", fmt.Errorf("read source file: %w", err)
		}
		name = opts.Name
		if name == "" {
			name = filepath.Base(opts.Source)
		}
	} else {
		data = []byte(opts.Source)
		name = opts.Name
		if name == "" {
			name = "inline-source"
		}
	}

	source, isNew, err := wiki.IngestSource(s.Paths, name, data)
	if err != nil {
		return "", fmt.Errorf("ingest source: %w", err)
	}
	if !isNew {
		fmt.Printf("Source already exists: %s\n", source.ID[:12])
	}
	if err := s.Search.IndexSource(source.ID, source.Path, source.SHA256, source.StoredAt.Format(time.RFC3339), name, data); err != nil {
		return "", fmt.Errorf("index source: %w", err)
	}

	// Strip a trailing ".md" (case-insensitive) before slugifying so an
	// ingested "target.md" produces slug "target", matching how the link side
	// (wiki.LinkToSlug / cleanWikilink) strips ".md". Without this, Slugify
	// turned the extension into a segment ("target.md" -> "target-md") and
	// [[target]] links could never resolve to the summary page. Only ".md" is
	// stripped (not filepath.Ext) to stay symmetric with the link side.
	base := name
	if strings.HasSuffix(strings.ToLower(name), ".md") {
		base = name[:len(name)-len(".md")]
	}
	slug := wiki.Slugify(base)
	if slug == "" {
		slug = source.ID[:12]
	}

	// Wrap the truncated digest in a fence taller than any backtick run it
	// contains, so the source's own fences can never close the wrapping block
	// and the digest renders as a verbatim quote (never as live markdown). This
	// keeps wiki.ExtractLinks(summaryBody) deterministically empty for summary
	// pages; source [[wikilinks]] are extracted separately from the full source
	// by indexSourceLinks.
	digest := truncate(string(data), 2000)
	fence := wiki.FenceFor(digest)
	body := fmt.Sprintf("# Summary: %s\n\nRaw source stored at `.eigenmemory/sources/%s`.\n\n%s\n%s\n%s\n",
		name, source.ID, fence, digest, fence)

	pageType := types.PageTypeSummary
	var page *types.Page
	if wiki.PageExists(s.Paths, pageType, slug) {
		page, err = s.LoadPage(pageType, slug)
		if err != nil {
			return "", fmt.Errorf("load summary page: %w", err)
		}
		page.Body = body
	} else {
		page = &types.Page{
			Frontmatter: types.DefaultFrontmatter(pageType),
			Slug:        slug,
			Body:        body,
		}
	}
	page.Frontmatter.Sources = uniqueStrings(append(page.Frontmatter.Sources, source.ID))
	page.Frontmatter.Tags = uniqueStrings(append(page.Frontmatter.Tags, "source", "summary"))

	if err := s.SavePage(page, pageType); err != nil {
		return "", fmt.Errorf("save summary page: %w", err)
	}
	// Extract [[wikilinks]] from the full raw source (not the truncated body)
	// and record them as graph edges. SavePage/IndexPage already cleared the
	// slug's relations and reinserted frontmatter relations, so this restores
	// the source-derived links_to edges for the current source set.
	if err := s.indexSourceLinks(page); err != nil {
		return "", fmt.Errorf("index source links: %w", err)
	}
	if err := s.SaveIndex(); err != nil {
		return "", fmt.Errorf("save index: %w", err)
	}
	if err := s.AppendLog(wiki.OpIngest, name, fmt.Sprintf("Ingested source %s (%s) at %s", source.ID[:12], name, time.Now().UTC().Format(time.RFC3339))); err != nil {
		return "", fmt.Errorf("append log: %w", err)
	}

	return fmt.Sprintf("Ingested %s → summary/%s.md (source %s)", name, slug, source.ID[:12]), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n\n... [truncated]"
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
