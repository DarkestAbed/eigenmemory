# EigenMemory (Claude Code plugin)

This directory is the Claude Code plugin package for [EigenMemory](https://github.com/DarkestAbed/eigenmemory) — it only declares the `eigenmemory` MCP server (`.claude-plugin/plugin.json`). It does not bundle the binary.

**Prerequisite**: the `eigenmemory` binary must be on your `PATH`. Install it with:

```bash
curl -fsSL https://raw.githubusercontent.com/DarkestAbed/eigenmemory/main/install.sh | sh
```

or `go install github.com/DarkestAbed/eigenmemory/cmd/eigenmemory@latest`. See the [repo README](../README.md) for other install methods.

Once the binary is installed, initialize a wiki in your project (`eigenmemory init`) and enable this plugin — Claude Code will start the MCP server automatically and expose `wiki_recall`, `wiki_remember`, `wiki_ingest`, `wiki_query`, `wiki_lint`, `wiki_status`, and `wiki_reconcile`.

This plugin also bundles a `memory-workflow` skill (`skills/memory-workflow/SKILL.md`) that teaches the agent when and how to use those tools — e.g. that `wiki_recall`/`wiki_query` are keyword FTS5 search rather than semantic retrieval, and that `wiki_ingest` is summary-only and needs a follow-up `wiki_remember` to classify facts. Without it, an agent using only the raw MCP tool descriptions would miss those nuances unless this project also has an `eigenmemory init`-generated `CLAUDE.md`/`AGENTS.md`.
