package core

import (
	"fmt"
	"os"
	"time"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/search"
	"github.com/DarkestAbed/eigenmemory/internal/types"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
)

// Store is the high-level facade over the wiki filesystem and SQLite index.
type Store struct {
	Paths  *config.Paths
	Search *search.Store
}

// Open initializes the filesystem and SQLite index for the active scope.
func Open() (*Store, error) {
	_, paths, err := config.ScopeFromCWD()
	if err != nil {
		return nil, fmt.Errorf("resolve scope: %w", err)
	}

	searchStore, err := search.Open(paths)
	if err != nil {
		return nil, fmt.Errorf("open search store: %w", err)
	}

	return &Store{
		Paths:  paths,
		Search: searchStore,
	}, nil
}

// OpenAt initializes the store for an explicit root path.
func OpenAt(root string) (*Store, error) {
	paths := config.PathsFor(root)

	searchStore, err := search.Open(paths)
	if err != nil {
		return nil, fmt.Errorf("open search store: %w", err)
	}

	return &Store{
		Paths:  paths,
		Search: searchStore,
	}, nil
}

// Close releases the underlying resources.
func (s *Store) Close() error {
	if s.Search == nil {
		return nil
	}
	return s.Search.Close()
}

// SavePage writes a wiki page to disk and updates the index.
func (s *Store) SavePage(page *types.Page, pageType types.PageType) error {
	if err := wiki.SavePage(s.Paths, page, pageType); err != nil {
		return err
	}
	if err := s.Search.IndexPage(page); err != nil {
		return fmt.Errorf("index page: %w", err)
	}
	if err := s.indexBodyLinks(page); err != nil {
		return fmt.Errorf("index body links: %w", err)
	}
	return nil
}

// indexBodyLinks extracts internal links from a page's body and records them
// as untyped links_to graph edges. Body links are derived, not authored, so
// they live only in the search index — the page's frontmatter is never
// mutated. Must run after IndexPage, which clears the slug's relations before
// reinserting frontmatter relations.
func (s *Store) indexBodyLinks(page *types.Page) error {
	raw := wiki.ExtractLinks(page.Body)
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	var slugs []string
	for _, l := range raw {
		slug := wiki.LinkToSlug(l)
		if slug == "" {
			continue
		}
		if _, dup := seen[slug]; dup {
			continue
		}
		seen[slug] = struct{}{}
		slugs = append(slugs, slug)
	}
	if len(slugs) == 0 {
		return nil
	}
	return s.Search.IndexBodyLinks(page.Slug, slugs)
}

// LoadPage reads a wiki page by type and slug.
func (s *Store) LoadPage(pageType types.PageType, slug string) (*types.Page, error) {
	return wiki.LoadPage(s.Paths, pageType, slug)
}

// SaveIndex regenerates the index.md catalog.
func (s *Store) SaveIndex() error {
	return wiki.SaveIndex(s.Paths)
}

// AppendLog appends an entry to log.md.
func (s *Store) AppendLog(op wiki.Operation, subject, details string) error {
	return wiki.AppendLog(s.Paths, op, subject, details)
}

// RebuildIndex clears the SQLite index and rebuilds it from markdown.
func (s *Store) RebuildIndex() error {
	if err := s.Search.Clear(); err != nil {
		return err
	}

	for _, pageType := range types.ValidPageTypes() {
		pages, err := wiki.ListPages(s.Paths, pageType)
		if err != nil {
			return err
		}
		for _, page := range pages {
			if err := s.Search.IndexPage(page); err != nil {
				return err
			}
			if err := s.indexBodyLinks(page); err != nil {
				return err
			}
		}
	}

	sources, err := wiki.ListSources(s.Paths)
	if err != nil {
		return err
	}
	for _, src := range sources {
		// The source name is not stored on disk; recover it from the summary
		// page that references it (pages were indexed above, so the pages table
		// is populated). Index the full source content for full-text search.
		name, err := s.Search.SourceDisplayName(src.ID)
		if err != nil {
			return fmt.Errorf("lookup source name %s: %w", src.ID[:12], err)
		}
		content, err := os.ReadFile(src.Path)
		if err != nil {
			return fmt.Errorf("read source %s: %w", src.ID[:12], err)
		}
		if err := s.Search.IndexSource(src.ID, src.Path, src.SHA256, src.StoredAt.Format(time.RFC3339), name, content); err != nil {
			return err
		}
	}

	return nil
}
