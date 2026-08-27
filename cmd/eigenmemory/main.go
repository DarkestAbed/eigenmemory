package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/javi/eigenmemory/internal/cmd"
	"github.com/javi/eigenmemory/internal/config"
)

var rootCmd = &cobra.Command{
	Use:   "eigenmemory",
	Short: "A persistent, cross-tool LLM memory layer for coding harnesses.",
	Long: `EigenMemory implements the LLM Wiki pattern for coding tools.

It maintains a markdown-based wiki as the canonical store and exposes it
to Claude Code, Zed, and any MCP-speaking tool via a local stdio server.`,
}

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize an EigenMemory wiki in the current directory",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		name, _ := c.Flags().GetString("name")
		if name == "" {
			if len(args) > 0 {
				name = args[0]
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				name = filepathBase(cwd)
			}
		}
		force, _ := c.Flags().GetBool("force")
		global, _ := c.Flags().GetBool("global")
		scope := config.ScopeProject
		if global {
			scope = config.ScopeGlobal
		}
		return cmd.Init(cmd.InitOptions{
			ProjectName: name,
			Scope:       scope,
			Force:       force,
		})
	},
}

var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild the SQLite FTS5 index and regenerate index.md",
	RunE: func(_ *cobra.Command, _ []string) error {
		return cmd.Reindex()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show EigenMemory state and index health",
	RunE: func(_ *cobra.Command, _ []string) error {
		return cmd.Status()
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the EigenMemory MCP server over stdio",
	RunE: func(c *cobra.Command, _ []string) error {
		mcpMode, _ := c.Flags().GetBool("mcp")
		return cmd.Serve(mcpMode)
	},
}

var ingestCmd = &cobra.Command{
	Use:   "ingest <file-or-text>",
	Short: "Ingest a raw source into the EigenMemory wiki",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		isPath, _ := c.Flags().GetBool("path")
		name, _ := c.Flags().GetString("name")
		return cmd.Ingest(cmd.IngestOptions{
			Source: args[0],
			Name:   name,
			IsPath: isPath,
		})
	},
}

var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Sync Claude Code native memory with the EigenMemory wiki",
	RunE: func(c *cobra.Command, _ []string) error {
		dryRun, _ := c.Flags().GetBool("dry-run")
		return cmd.Reconcile(cmd.ReconcileOptions{DryRun: dryRun})
	},
}

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Check the EigenMemory wiki for orphans, broken links, and drift",
	RunE: func(c *cobra.Command, _ []string) error {
		fix, _ := c.Flags().GetBool("fix")
		return cmd.Lint(cmd.LintOptions{Fix: fix})
	},
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Print MCP configuration snippets for Claude Code or Zed",
	RunE: func(c *cobra.Command, _ []string) error {
		tool, _ := c.Flags().GetString("tool")
		return cmd.Setup(cmd.SetupOptions{Tool: tool})
	},
}

var queryCmd = &cobra.Command{
	Use:   "query \"...\"",
	Short: "Answer a natural-language question using the EigenMemory wiki",
	Long:  `Query searches the EigenMemory wiki and returns ranked context with citations. It does not call an LLM; synthesize the answer yourself or pipe the output to a model.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		limit, _ := c.Flags().GetInt("limit")
		return cmd.Query(cmd.QueryOptions{
			Query: args[0],
			Limit: limit,
		})
	},
}

func main() {
	initCmd.Flags().StringP("name", "n", "", "Project name (defaults to current directory name)")
	initCmd.Flags().BoolP("force", "f", false, "Overwrite an existing EigenMemory directory")
	initCmd.Flags().BoolP("global", "g", false, "Initialize global memory under ~/.eigenmemory/")
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(reindexCmd)
	rootCmd.AddCommand(statusCmd)

	serveCmd.Flags().Bool("mcp", false, "Run as an MCP server over stdio")
	rootCmd.AddCommand(serveCmd)

	setupCmd.Flags().String("tool", "", "Tool to configure: claude or zed (required)")
	_ = setupCmd.MarkFlagRequired("tool")
	rootCmd.AddCommand(setupCmd)

	ingestCmd.Flags().BoolP("path", "p", false, "Treat argument as a file path")
	ingestCmd.Flags().StringP("name", "n", "", "Human-readable source name")
	rootCmd.AddCommand(ingestCmd)

	reconcileCmd.Flags().Bool("dry-run", false, "Show proposed actions without applying them")
	rootCmd.AddCommand(reconcileCmd)

	lintCmd.Flags().Bool("fix", false, "Repair index drift by rebuilding the SQLite FTS5 index")
	rootCmd.AddCommand(lintCmd)

	queryCmd.Flags().IntP("limit", "n", 5, "Maximum number of source pages to return")
	rootCmd.AddCommand(queryCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func filepathBase(path string) string {
	for len(path) > 0 && (path[len(path)-1] == '/' || path[len(path)-1] == '\\') {
		path = path[:len(path)-1]
	}
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
