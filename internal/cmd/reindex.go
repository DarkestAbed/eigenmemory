package cmd

import (
	"fmt"
	"time"

	"github.com/javi/eigenmemory/internal/core"
	"github.com/javi/eigenmemory/internal/wiki"
)

// Reindex rebuilds the SQLite FTS5 index from the markdown wiki and
// regenerates the index.md catalog.
func Reindex() error {
	store, err := core.Open()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	if err := store.RebuildIndex(); err != nil {
		return fmt.Errorf("rebuild index: %w", err)
	}
	if err := store.SaveIndex(); err != nil {
		return fmt.Errorf("save index: %w", err)
	}
	if err := store.AppendLog(wiki.OpReindex, "manual", fmt.Sprintf("Reindexed at %s", time.Now().UTC().Format(time.RFC3339))); err != nil {
		return fmt.Errorf("append log: %w", err)
	}

	count, err := store.Search.CountPages()
	if err != nil {
		return err
	}
	fmt.Printf("Reindexed %d pages\n", count)
	return nil
}
