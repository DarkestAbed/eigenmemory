package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/core"
	"github.com/DarkestAbed/eigenmemory/internal/templates"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
)

// InitOptions controls the behaviour of the init command.
type InitOptions struct {
	ProjectName string
	Root        string
	Scope       config.Scope
	Force       bool
}

// Init creates the EigenMemory wiki structure at the requested scope.
func Init(opts InitOptions) error {
	if err := config.ValidateProjectName(opts.ProjectName); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}

	var root string
	if opts.Root != "" {
		root = opts.Root
	} else if opts.Scope == config.ScopeGlobal {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home directory: %w", err)
		}
		root = filepath.Join(home, config.GlobalDirName)
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		root = filepath.Join(cwd, config.DirName)
	}

	if info, err := os.Stat(root); err == nil && info.IsDir() {
		if !opts.Force {
			return fmt.Errorf("eigenmemory already initialized at %s; use --force to overwrite", root)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat eigenmemory root: %w", err)
	}

	paths := config.PathsFor(root)

	// Create directories.
	for _, dir := range []string{
		paths.Root,
		paths.SourcesDir,
		paths.WikiDir,
		filepath.Join(paths.WikiDir, "entity"),
		filepath.Join(paths.WikiDir, "concept"),
		filepath.Join(paths.WikiDir, "summary"),
		filepath.Join(paths.WikiDir, "project"),
		filepath.Join(paths.WikiDir, "feedback"),
		filepath.Join(paths.WikiDir, "reference"),
		filepath.Join(paths.WikiDir, "user"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// Initialize the SQLite database.
	store, err := core.OpenAt(root)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Write config.json. ClaudeProjectDir only makes sense tied to a single
	// project's working directory, so it's left empty for global scope.
	cfg := config.Default(opts.ProjectName)
	cfg.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if opts.Scope == config.ScopeProject {
		cfg.ClaudeProjectDir = config.SanitizeClaudeProjectDir(filepath.Dir(root))
	}
	if err := config.SaveConfig(paths, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	// Write schema files next to the wiki root (project scope) or inside it (global).
	if err := writeSchemaFiles(root, opts.ProjectName); err != nil {
		return err
	}

	// Seed index and log if absent.
	if err := seedIndex(paths); err != nil {
		return err
	}
	if err := wiki.AppendLog(paths, wiki.OpInit, opts.ProjectName, "Initialized EigenMemory wiki."); err != nil {
		return fmt.Errorf("append log: %w", err)
	}

	fmt.Printf("Initialized EigenMemory at %s\n", root)
	return nil
}

func seedIndex(paths *config.Paths) error {
	if _, err := os.Stat(paths.IndexFile); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	content := "# EigenMemory Index\n\nCatalog of all pages in this wiki, grouped by type.\n"
	return os.WriteFile(paths.IndexFile, []byte(content), 0o644)
}

func writeSchemaFiles(root, projectName string) error {
	// For project scope, place CLAUDE.md and AGENTS.md one directory above the
	// .eigenmemory folder so they are visible to tools and version control.
	parent := filepath.Dir(root)
	if parent == root {
		parent = root
	}

	claudePath := filepath.Join(parent, "CLAUDE.md")
	agentsPath := filepath.Join(parent, "AGENTS.md")

	if err := writeIfNotExists(claudePath, templates.CLAUDE(projectName)); err != nil {
		return fmt.Errorf("write CLAUDE.md: %w", err)
	}
	if err := writeIfNotExists(agentsPath, templates.AGENTS(projectName)); err != nil {
		return fmt.Errorf("write AGENTS.md: %w", err)
	}
	return nil
}

func writeIfNotExists(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
