package cmd

import (
	"fmt"
	"os"

	"github.com/javi/eigenmemory/internal/core"
	"github.com/javi/eigenmemory/internal/mcp"
)

// Serve starts the EigenMemory MCP server over stdio.
func Serve(mcpMode bool) error {
	if !mcpMode {
		return fmt.Errorf("`eigenmemory serve` requires --mcp flag")
	}

	store, err := core.Open()
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Log startup to stderr so it does not interfere with stdio JSON-RPC.
	fmt.Fprintf(os.Stderr, "eigenmemory mcp server starting (root=%s)\n", store.Paths.Root)

	server := mcp.NewServer(store)
	mcp.RegisterWikiTools(server)
	mcp.RegisterWikiResources(server)

	return server.Run()
}
