---
description: Use the EigenMemory MCP tools (wiki_recall, wiki_query, wiki_remember, wiki_ingest, wiki_reconcile, wiki_lint, wiki_status) correctly whenever they are connected — recall before answering questions about past decisions or project context, and remember durable facts as soon as you learn them.
---

# EigenMemory workflow

When the `eigenmemory` MCP server is connected, prefer these tools over guessing or re-deriving context:

- `wiki_recall(query)` / `wiki_query(query)` — search before answering questions about prior decisions or project context. These are keyword full-text search (FTS5), **not** semantic retrieval: short keyword queries beat prose. `wiki_query` falls back from AND to OR ranking when an exact match returns nothing, so a natural-language question no longer silently returns "no results" — but keywords still win on precision.
- `wiki_remember(fact, page_type, tags)` — store a durable fact (a decision, correction, or preference) as soon as you learn it. Prefer this over waiting for the host's own automatic memory-saving to catch it, so the fact is indexed and citable immediately.
- `wiki_ingest(source)` — archive a design doc, transcript, or article. It is summary-only by design: it creates an immutable copy plus a digest page, but does **not** classify the source into a typed page. Follow up with `wiki_remember` to extract typed facts from it.
- `wiki_reconcile(dry_run)` — merge Claude Code's native `/memory` file edits back into the wiki. Only meaningful in Claude Code; if this project also has no wiki yet, `wiki_status` will say so.
- `wiki_lint()` / `wiki_status()` — check wiki health (orphans, broken links, stale pages, index drift) and inspect current state.

If `wiki_status` reports no wiki for this project, tell the user to run `eigenmemory init` before continuing — the MCP tools have nothing to operate on until then.

Always cite the wiki page (or the underlying source, for `(source)` hits) when synthesizing an answer from recalled facts.
