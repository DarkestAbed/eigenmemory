package cmd

import (
	"fmt"
	"os"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/core"
	"github.com/javi/eigenmemory/internal/lint"
)

// LintOptions controls the lint command.
type LintOptions struct {
	Fix bool
}

// Lint runs a health check over the EigenMemory wiki.
func Lint(opts LintOptions) error {
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

	report, err := lint.Run(paths, store.Search)
	if err != nil {
		return fmt.Errorf("lint: %w", err)
	}

	if opts.Fix && report.FixableIndexDrift() {
		fmt.Println("Fixing index drift by rebuilding SQLite FTS5 index...")
		if err := store.RebuildIndex(); err != nil {
			return fmt.Errorf("rebuild index: %w", err)
		}
		// Re-run lint so the report reflects the fix.
		report, err = lint.Run(paths, store.Search)
		if err != nil {
			return fmt.Errorf("lint after fix: %w", err)
		}
	}

	fmt.Printf("Scope: %s\n", scope)
	fmt.Printf("Root:  %s\n", paths.Root)
	fmt.Printf("Checked %d pages\n", report.Checked)

	summary := report.Summary()
	fmt.Printf("Issues: %d error(s), %d warning(s), %d info\n", summary["error"], summary["warning"], summary["info"])

	if len(report.Issues) == 0 {
		fmt.Println("Wiki is healthy.")
		return nil
	}

	fmt.Println()
	for _, issue := range report.Issues {
		fmt.Printf("[%s] %s %s: %s\n", issue.Severity, issue.Page, issue.Category, issue.Message)
	}

	return nil
}
