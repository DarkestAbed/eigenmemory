package cmd

import (
	"fmt"
	"os"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/core"
)

// Status prints the current EigenMemory state.
func Status() error {
	scope, paths, err := config.ScopeFromCWD()
	if err != nil {
		return fmt.Errorf("resolve scope: %w", err)
	}

	if _, err := os.Stat(paths.Root); err != nil {
		return fmt.Errorf("no EigenMemory wiki found at %s; run `eigenmemory init`", paths.Root)
	}

	store, err := core.OpenAt(paths.Root)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	count, err := store.Search.CountPages()
	if err != nil {
		return err
	}

	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return err
	}

	fmt.Printf("Scope:       %s\n", scope)
	fmt.Printf("Root:        %s\n", paths.Root)
	if cfg.Name != "" {
		fmt.Printf("Project:     %s\n", cfg.Name)
	}
	fmt.Printf("Indexed pages: %d\n", count)
	return nil
}
