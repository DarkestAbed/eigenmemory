package core

import (
	"fmt"
	"time"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/search"
	"github.com/javi/eigenmemory/internal/types"
	"github.com/javi/eigenmemory/internal/wiki"
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
	return nil
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
		}
	}

	sources, err := wiki.ListSources(s.Paths)
	if err != nil {
		return err
	}
	for _, src := range sources {
		if err := s.Search.IndexSource(src.ID, src.Path, src.SHA256, src.StoredAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}

	return nil
}
