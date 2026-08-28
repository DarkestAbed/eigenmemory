package cmd

import (
	"fmt"
	"time"

	"github.com/DarkestAbed/eigenmemory/internal/adapters"
	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/core"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
)

// ReconcileOptions controls the reconcile command.
type ReconcileOptions struct {
	DryRun bool
}

// Reconcile synchronizes Claude Code native memory with the EigenMemory wiki.
func Reconcile(opts ReconcileOptions) error {
	store, err := core.Open()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	cfg, err := config.LoadConfig(store.Paths)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Name == "" {
		return fmt.Errorf("project name not set; run `eigenmemory init`")
	}

	// First, merge any newer Claude Code memory edits back into the wiki.
	actions, err := adapters.Reconcile(store.Paths, cfg.Name, opts.DryRun)
	if err != nil {
		return fmt.Errorf("reconcile memory files: %w", err)
	}

	if opts.DryRun {
		fmt.Println("Dry run. Proposed actions:")
	} else {
		fmt.Println("Reconciled:")
	}
	for _, a := range actions {
		fmt.Printf("  - %s\n", a)
	}

	if !opts.DryRun {
		// Regenerate the projection from the (now updated) wiki.
		if err := adapters.ProjectMemoryProjection(store.Paths, cfg.Name); err != nil {
			return fmt.Errorf("project memory: %w", err)
		}
		if err := store.SaveIndex(); err != nil {
			return fmt.Errorf("save index: %w", err)
		}
		if err := store.RebuildIndex(); err != nil {
			return fmt.Errorf("rebuild search index: %w", err)
		}
		if err := store.AppendLog(wiki.OpReconcile, cfg.Name, fmt.Sprintf("Reconciled at %s", time.Now().UTC().Format(time.RFC3339))); err != nil {
			return fmt.Errorf("append log: %w", err)
		}
	}

	return nil
}
