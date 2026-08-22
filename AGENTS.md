# EigenMemory / AGENTS.md — eigenmemory

This file is consumed by all AI agents working on this project, including Zed.

## EigenMemory MCP tools

When connected to the EigenMemory MCP server, prefer these tools:

- `wiki_recall(query)` — search the LLM Wiki before answering questions about past decisions or project context.
- `wiki_remember(fact, type, tags)` — store durable facts with proper categorization.
- `wiki_ingest(source)` — ingest a design doc, transcript, or article into the wiki.
- `wiki_lint()` — run health checks and propose fixes.
- `wiki_status()` — inspect wiki state and index health.

## Memory workflow

1. At the start of a task, recall relevant pages.
2. After making a decision or learning a durable fact, remember it.
3. Before claiming something about the project, cite the wiki or a raw source.
4. Respect the frontmatter schema in `.eigenmemory/wiki/`.

## Cross-tool consistency

Facts stored via this project's EigenMemory are shared with Claude Code. The wiki under `.eigenmemory/wiki/` is the source of truth.
