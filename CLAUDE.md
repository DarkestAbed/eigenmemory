# EigenMemory / CLAUDE.md — eigenmemory

This project uses [EigenMemory](https://github.com/DarkestAbed/eigenmemory), a persistent LLM Wiki for coding harnesses.

## What EigenMemory owns

The directory `.eigenmemory/` is maintained by the agent. Do not hand-edit files there unless you know what you are doing. The canonical store is:

- `.eigenmemory/wiki/` — agent-maintained markdown pages with YAML frontmatter.
- `.eigenmemory/sources/` — immutable raw sources (design docs, transcripts, web clips).
- `.eigenmemory/eigenmemory.db` — derived SQLite FTS5 index (gitignored, rebuildable).

## How to use EigenMemory

1. **Recall before answering**: when the user asks about prior decisions, run the MCP tool `wiki_recall` or read the relevant pages from `.eigenmemory/wiki/`. `wiki_query`/`wiki_recall` are keyword full-text search (FTS5), not semantic retrieval — short keyword queries are the most precise interface. A long natural-language question falls back from AND to OR ranking when AND returns nothing, so it no longer silently returns "no results," but keywords still beat prose. Results combine three sources: page full-text hits, then `(source)` hits over the **full** (untruncated) content of ingested sources with a snippet of the matching passage and the `sources/<id>` path, then one-hop graph neighbors via `[[wikilinks]]` and frontmatter `relations` (marked `(graph)`). For detail past a source's ~2KB summary digest, read the file at `.eigenmemory/sources/<id>` shown in a `(source)` hit.
2. **Remember after learning**: when you learn a durable fact (decisions, preferences, corrections, external references), store it via `wiki_remember` or by writing the appropriate wiki page. This is how pages get classified into the typed directories below.
3. **Ingest sources**: when the user provides a design doc, article, or transcript, call `wiki_ingest` or run `eigenmemory ingest <path>`. Ingest is summary-only by design — it archives an immutable copy in `sources/` plus a digest page; it intentionally does not classify the source into a typed directory. Use `wiki_remember` to extract typed facts from an ingested source.
4. **Link pages**: write `[[slug]]` wikilinks in page bodies. They are auto-indexed as untyped `links_to` graph edges at index time, so `wiki_query` can traverse them. Links inside code blocks/spans are ignored, so a verbatim source quote never becomes an edge.
5. **Maintain the wiki**: periodically run `eigenmemory lint` and apply fixes.
6. **Reconcile Claude Code memory**: changes made via `/memory` are proposals. Run `eigenmemory reconcile` to merge them back into the wiki.

## Page types

| Type | Directory | Use |
|------|-----------|-----|
| entity | `wiki/entity/` | Nouns: services, files, classes, people, decisions. |
| concept | `wiki/concept/` | Ideas, patterns, principles. |
| summary | `wiki/summary/` | Digest of a single raw source. |
| project | `wiki/project/` | Ongoing work, deadlines, milestones. |
| feedback | `wiki/feedback/` | Corrections and confirmed approaches. |
| reference | `wiki/reference/` | External links: tickets, dashboards, docs. |
| user | `wiki/user/` | Role, expertise, preferences. |

## Frontmatter standard

```yaml
---
id: <uuid>
type: entity | concept | summary | feedback | project | reference | user
status: active | stale | merged | archived
created: 2026-08-22T10:00:00Z
updated: 2026-08-22T10:00:00Z
sources:
  - source-id-1
tags: [auth, refactor]
relations:
  - { from: "slug", to: "other-slug", type: "builds_on" }
---
```

## Rules

- Always cite sources when synthesizing answers.
- Prefer updating an existing entity page over creating duplicates.
- Keep pages focused; split large topics.
- Run `eigenmemory reindex` if you manually edit many markdown files.
- Do not commit `.eigenmemory/eigenmemory.db`.
