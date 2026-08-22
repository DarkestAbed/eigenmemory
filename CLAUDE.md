# EigenMemory / CLAUDE.md — eigenmemory

This project uses [EigenMemory](https://github.com/javi/eigenmemory), a persistent LLM Wiki for coding harnesses.

## What EigenMemory owns

The directory `.eigenmemory/` is maintained by the agent. Do not hand-edit files there unless you know what you are doing. The canonical store is:

- `.eigenmemory/wiki/` — agent-maintained markdown pages with YAML frontmatter.
- `.eigenmemory/sources/` — immutable raw sources (design docs, transcripts, web clips).
- `.eigenmemory/eigenmemory.db` — derived SQLite FTS5 index (gitignored, rebuildable).

## How to use EigenMemory

1. **Recall before answering**: when the user asks about prior decisions, run the MCP tool `wiki_recall` or read the relevant pages from `.eigenmemory/wiki/`.
2. **Remember after learning**: when you learn a durable fact (decisions, preferences, corrections, external references), store it via `wiki_remember` or by writing the appropriate wiki page.
3. **Ingest sources**: when the user provides a design doc, article, or transcript, call `wiki_ingest` or run `eigenmemory ingest <path>`.
4. **Maintain the wiki**: periodically run `eigenmemory lint` and apply fixes.
5. **Reconcile Claude Code memory**: changes made via `/memory` are proposals. Run `eigenmemory reconcile` to merge them back into the wiki.

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
