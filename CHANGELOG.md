# Changelog

## 0.1.0 — 2026-08-22

Initial release of EigenMemory, a persistent, cross-tool LLM memory layer for coding harnesses built on the LLM Wiki pattern.

### Added

- `eigenmemory init` — bootstrap a project or global `.eigenmemory/` wiki with SQLite FTS5 index, source directory, and `CLAUDE.md` / `AGENTS.md` maintenance instructions.
- `eigenmemory serve --mcp` — stdio MCP server exposing wiki tools to Claude Code, Zed, and any MCP client.
- MCP tools:
  - `wiki_recall` — full-text search over the wiki.
  - `wiki_remember` — store or update a durable fact.
  - `wiki_ingest` — ingest a raw source (file path or text) into `.eigenmemory/sources/` and create a summary page.
  - `wiki_query` — natural-language recall with cited context.
  - `wiki_lint` — detect orphans, broken links, stale pages, and SQLite index drift; optionally repair drift.
  - `wiki_status` — show scope, project name, indexed page count, and last log entry.
  - `wiki_reconcile` — merge Claude Code native memory edits back into the wiki and regenerate the projection.
- `eigenmemory query` — CLI natural-language recall with citations.
- `eigenmemory lint` / `eigenmemory lint --fix` — wiki health checks.
- `eigenmemory reindex` — rebuild the SQLite FTS5 index from markdown.
- `eigenmemory reconcile` / `eigenmemory reconcile --dry-run` — Claude Code memory sync.
- `eigenmemory ingest` — CLI source ingestion.
- `eigenmemory status` — project state overview.
- `eigenmemory setup --tool claude|zed` — print MCP configuration snippets.
- Claude Code adapter that projects wiki pages into `~/.claude/projects/<project>/memory/` and reconciles edits back.
- Markdown-first canonical storage with YAML frontmatter; SQLite + FTS5 as a derived, gitignored search index.
- Cross-platform release pipeline via GoReleaser and GitHub Actions.
