package cmd

import (
	"fmt"
	"os"

	"github.com/javi/eigenmemory/internal/config"
)

// SetupOptions controls the setup helper.
type SetupOptions struct {
	Tool string
}

// Setup prints tool-specific configuration snippets for connecting EigenMemory.
func Setup(opts SetupOptions) error {
	_, paths, err := config.ScopeFromCWD()
	if err != nil {
		return fmt.Errorf("resolve scope: %w", err)
	}

	if _, err := os.Stat(paths.Root); err != nil {
		return fmt.Errorf("no EigenMemory wiki found at %s; run `eigenmemory init`", paths.Root)
	}

	cfg, err := config.LoadConfig(paths)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Name == "" {
		return fmt.Errorf("project name not set; run `eigenmemory init`")
	}

	switch opts.Tool {
	case "claude":
		fmt.Println("Add this to your project root as `.mcp.json`:")
		fmt.Println()
		fmt.Println(`{
  "mcpServers": {
    "eigenmemory": {
      "type": "stdio",
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}`)
	case "zed":
		fmt.Println("Add this block to `~/.config/zed/settings.json`:")
		fmt.Println()
		fmt.Println(`{
  "context_servers": {
    "eigenmemory": {
      "command": {
        "path": "eigenmemory",
        "args": ["serve", "--mcp"]
      }
    }
  }
}`)
	default:
		return fmt.Errorf("unknown tool %q; supported: claude, zed", opts.Tool)
	}

	return nil
}
