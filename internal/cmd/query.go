package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/core"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
)

// QueryOptions controls the query command.
type QueryOptions struct {
	Query string
	Limit int
}

// Query answers a natural-language question using the EigenMemory wiki.
// It returns ranked context with citations; the caller (or an LLM) synthesizes
// the final answer. No LLM provider is bundled.
func Query(opts QueryOptions) error {
	if strings.TrimSpace(opts.Query) == "" {
		return fmt.Errorf("query is required")
	}

	_, paths, err := config.ScopeFromCWD()
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
	defer func() { _ = store.Close() }()

	if opts.Limit <= 0 {
		opts.Limit = 5
	}

	results, err := store.Search.Search(opts.Query, opts.Limit)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No relevant pages found in EigenMemory.")
		return nil
	}

	fmt.Printf("Query: %q\n\n", opts.Query)
	for i, r := range results {
		body := wiki.StripFooters(r.Body)
		if len(body) > 400 {
			body = body[:400] + "..."
		}
		fmt.Printf("%d. %s (%s/%s)\n%s\n\n", i+1, r.Title, r.Type, r.Slug, body)
	}

	fmt.Println("Cite the relevant page(s) above when synthesizing your answer.")
	return nil
}
