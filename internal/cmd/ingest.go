package cmd

import (
	"fmt"

	"github.com/javi/eigenmemory/internal/core"
)

// IngestOptions controls the ingest command.
type IngestOptions = core.IngestOptions

// Ingest ingests a raw source into the EigenMemory wiki.
func Ingest(opts IngestOptions) error {
	store, err := core.Open()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	result, err := store.Ingest(opts)
	if err != nil {
		return err
	}
	fmt.Println(result)
	return nil
}
