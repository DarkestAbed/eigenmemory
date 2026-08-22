# EigenMemory — TODO

## Done

### Phase 1: Foundation
- [x] Go module setup with `modernc.org/sqlite`, `cobra`, `goldmark`, `gopkg.in/yaml.v3`.
- [x] Project/global scope resolution via `internal/config`.
- [x] Markdown wiki layout with YAML frontmatter under `.eigenmemory/wiki/`.
- [x] SQLite + FTS5 derived index with incremental updates on every write.
- [x] `eigenmemory init` command with `CLAUDE.md` / `AGENTS.md` generation.

### Phase 2: MCP server + basic tools
- [x] `eigenmemory serve --mcp` stdio JSON-RPC MCP server.
- [x] `wiki_recall`, `wiki_remember`, `wiki_status` tools.
- [x] `memory://index` and `memory://log` resources.
- [x] `eigenmemory reindex` command.

### Phase 3: Ingest and reconciliation
- [x] `eigenmemory ingest` and `wiki_ingest` with SHA-256 source deduplication.
- [x] Claude Code adapter projecting wiki pages into `~/.claude/projects/<project>/memory/`.
- [x] `eigenmemory reconcile` and `wiki_reconcile` merging native memory edits back into the wiki.
- [x] Fixed false-positive reconcile drift caused by projection/sync footers.
- [x] Cross-tool consistency integration test.

### Phase 4: Lint, query, and polish
- [x] `eigenmemory lint` / `wiki_lint` with orphan, broken-link, stale, and index-drift checks; `--fix` rebuilds the search index.
- [x] `wiki_query` MCP tool for natural-language recall with citations.
- [x] `eigenmemory query` CLI parity.
- [x] `eigenmemory setup --tool claude|zed` snippet helper.
- [x] Centralized footer stripping in `internal/wiki/util.go:StripFooters`.
- [x] CHANGELOG.md and annotated git tag `v0.1.0`.
- [x] GitHub Actions CI and release workflows + GoReleaser config.
- [x] README quickstart for Claude Code and Zed.
- [x] All tests passing.

## Remaining

### Must do before the release is "real"
1. **Push to GitHub and trigger CI release**
   - Add remote: `git remote add origin git@github.com:javi/eigenmemory.git`
   - Push: `git push -u origin main && git push origin v0.1.0`
   - Verify GitHub Actions builds and GoReleaser uploads binaries.

2. **End-to-end tool integration smoke test**
   - Connect `eigenmemory serve --mcp` to Claude Code via `.mcp.json`.
   - Confirm `wiki_recall` / `wiki_remember` round-trip from Claude Code.
   - Connect to Zed via `context_servers` (if Zed is available) or document the steps.

3. **Update the dogfood wiki**
   - Capture latest decisions in `.eigenmemory/wiki/`:
     - `query` CLI / `wiki_query` tool
     - `lint` model and `--fix`
     - `setup` helper
     - release pipeline and v0.1.0 tag
   - Run `eigenmemory lint && eigenmemory reconcile --dry-run` after updates.

### Should do soon
4. **Projection hygiene**
   - Prevent old projection/sync footers from ever being written into the wiki body again. Currently `StripFooters` handles it defensively; consider making `wiki.SavePage` reject or strip them at write time.

5. **Global memory scope polish**
   - Test `eigenmemory init --global` end-to-end.
   - Ensure global/project scope collisions are resolved correctly in recall.

6. **Observability / logging**
   - Add structured logging option to MCP server for debugging tool calls.
   - Surface index-drift metrics in `wiki_status`.

### Could do later (Phase 5)
7. **Vector / semantic search**
   - Optional `sqlite-vec` add-on for embedding-based recall.

8. **Git auto-commit**
   - Optional `eigenmemory lint --commit` or post-write auto-commit if inside a git repo.

9. **Multi-user / team features**
   - Merge conflict handling when multiple agents edit the same page.

10. **Packaging**
    - Homebrew formula, AUR, scoop, or `go install` documentation.

## Priorities

| Priority | Item | Why |
|----------|------|-----|
| P0 | Push to GitHub + trigger release | The v0.1.0 tag exists but binaries are not published yet. |
| P0 | Claude Code smoke test | Validate the primary use case before announcing the release. |
| P1 | Update dogfood wiki | Keep project memory accurate; demonstrate the tool eats its own dog food. |
| P1 | Footer write-time guard | Close the loop so footers cannot re-enter the canonical wiki. |
| P2 | Global scope tests | Important for users who want a user-wide memory layer. |
| P2 | MCP logging | Makes debugging integration issues much faster. |
| P3 | Vector search, git auto-commit, packaging | Post-MVP enhancements. |
