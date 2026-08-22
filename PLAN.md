# EigenMemory Plan

A persistent, cross-tool LLM memory layer for coding harnesses, built on the LLM Wiki pattern.

---

## 1. Executive Summary

**Goal**: Give coding AI agents (Claude Code, Zed, and any MCP-speaking tool) a shared, persistent memory that compounds over time, using Andrej Karpathy's LLM Wiki pattern as the reconciling backbone.

**Core thesis**: Instead of each tool maintaining its own isolated context, we keep a single LLM Wiki — markdown-based, source-grounded, agent-maintained — and expose it to every tool through an MCP server. Tool-native memory files are treated as **projections** of the wiki, not independent sources of truth.

**Product name**: `eigenmemory` (working title).

---

## 2. Gathered Requirements

### 2.1 Stakeholder needs

| Stakeholder | Need |
|-------------|------|
| **Developer (you)** | Long-running coding knowledge survives across sessions and across tools. |
| **Claude Code** | Read/write memory in its native format: markdown files under `~/.claude/projects/<project>/memory/`. |
| **Zed** | Access memory via MCP tools, since Zed has no native persistent memory and relies on `context_servers`. |
| **Future tools** | Any MCP client should be able to read/write this memory without re-implementation. |
| **Human-in-the-loop** | Curate sources and ask questions; the agent does bookkeeping. |

### 2.2 Functional requirements

1. **Persistent storage**: All durable facts, preferences, project context, and reference data are stored on local disk, owned by the user.
2. **Cross-tool access**: Claude Code and Zed can recall and retain the same memory.
3. **Source grounding**: Claims cite raw sources; stale sources can be detected and reconciled.
4. **Agent maintenance**: The LLM updates indexes, entity pages, cross-links, and an append-only log automatically.
5. **Reconciliation**: Changes made inside Claude Code's native memory or through the MCP server converge back into the LLM Wiki.
6. **Project + global scopes**: Per-project memory plus user-wide memory, with a unified search view.
7. **Human review**: Important writes are surfaced for approval; routine bookkeeping runs silently.
8. **Offline-first**: No cloud dependency; embeddings or search run locally if used at all.

### 2.3 Non-functional requirements

1. **Interoperability first**: Storage format must be readable by any tool without special parsing.
2. **Simple to install**: A single command installs the MCP server and wires it into Claude Code and Zed.
3. **Transparent**: A human can open the wiki in Obsidian, VS Code, or any markdown editor and understand it.
4. **Repairable**: Corruption or bad agent edits can be recovered from git history or the immutable raw-source layer.
5. **Fast enough for agent workflows**: Ingest/query should complete in under a few seconds for typical coding contexts.

---

## 3. Quality Metrics & Scenarios

### 3.1 Quality metrics

| Metric | Target | How measured |
|--------|--------|--------------|
| **Recall coverage** | ≥ 90% of stored facts relevant to a query are returned in top-5 results. | Synthetic scenario tests. |
| **Citation accuracy** | Every synthesized answer cites the source page or raw source. | Lint pass + manual spot checks. |
| **Cross-tool consistency** | A fact written via Zed is readable via Claude Code within one session restart. | Integration test. |
| **Ingest completeness** | A source document touches the index, a summary page, and all relevant entity pages. | Lint pass. |
| **Orphan rate** | < 5% of wiki pages are unlinked from the index after a lint pass. | Automated lint metric. |
| **Setup friction** | New user can install and connect both tools in < 10 minutes. | Dogfood timing. |

### 3.2 User scenarios

#### Scenario A: Cross-session project context
You start a new Claude Code session on a repo. You ask, "What were we doing with the auth refactor?" Eigenmemory recalls the entity page `auth-refactor.md`, the log entries, and the relevant source notes, answering with citations.

#### Scenario B: Tool hopping
You spend an afternoon coding in Zed. You tell the agent, "Remember that we decided to deprecate the REST v1 endpoints." The MCP server stores the fact. Later, in Claude Code, you ask, "What API decisions did we make this week?" Claude Code reads the same wiki and answers.

#### Scenario C: Ingesting a design doc
You drop a Markdown design doc into the raw sources folder. Eigenmemory ingests it: creates a summary page, updates entity pages for every service or component mentioned, links related pages, and appends a log entry. The human only had to place the file.

#### Scenario D: Lint and reconciliation
You notice two wiki pages contradict each other. You run `eigenmemory lint`. The agent surfaces the contradiction, proposes a merge or correction, and — after approval — updates the pages and the log.

#### Scenario E: New team member
A teammate clones the repo, installs eigenmemory, and runs `eigenmemory init`. The project wiki, `CLAUDE.md`, and `AGENTS.md` are already present in version control, so their Claude Code and Zed immediately share the project's accumulated context.

---

## 4. Architecture

We adopt Karpathy's three-layer model and make two key adaptations for coding tools:

1. **Tool-native memory files are projections**, not the canonical store. The canonical store is the LLM Wiki.
2. **MCP is the interoperability protocol**; an `eigenmemory` MCP server exposes the wiki to any client.

### 4.1 Three layers

```
┌─────────────────────────────────────────┐
│  Coding tools: Claude Code, Zed, ...    │
│  Each reads/writes via its native path  │
└──────────────┬──────────────────────────┘
               │ MCP tools / file watcher
┌──────────────▼──────────────────────────┐
│  Eigenmemory reconciler                 │
│  - ingest, query, lint, reconcile       │
│  - maintains LLM Wiki as source of truth  │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│  Layer 1: Raw sources (immutable)         │
│  `.eigenmemory/sources/`                  │
│  Original docs, transcripts, web clips,   │
│  code excerpts, design docs             │
├───────────────────────────────────────────┤
│  Layer 2: The wiki (agent-maintained)     │
│  `.eigenmemory/wiki/`                     │
│  index.md, log.md, entity/*.md,           │
│  concept/*.md, summary/*.md               │
├───────────────────────────────────────────┤
│  Layer 3: The schema                    │
│  `CLAUDE.md` / `AGENTS.md`                │
│  How to maintain the wiki;                │
│  also injected as tool instructions       │
└─────────────────────────────────────────┘
```

### 4.2 Storage format: markdown-first

We store the canonical wiki as plain markdown with YAML frontmatter.

**Why markdown?**
- Claude Code natively reads markdown memory files.
- Humans can edit it in Obsidian, VS Code, or any editor.
- Git diffs are meaningful.
- No binary lock-in.

**Frontmatter standard**:

```yaml
---
id: <uuid>
type: entity | concept | summary | feedback | project | reference | user
status: active | stale | merged | archived
created: 2026-08-22T10:00:00Z
updated: 2026-08-22T10:00:00Z
sources:
  - source-id-1
  - source-id-2
tags: [auth, refactor, api]
---
```

**Page types**:

| Type | Purpose | Example |
|------|---------|---------|
| `summary` | Digest of one raw source. | `summary/design-doc-auth-v2.md` |
| `entity` | A noun in the domain: person, service, file, class, decision. | `entity/auth-service.md` |
| `concept` | An idea, pattern, or principle. | `concept/hexagonal-adapter.md` |
| `project` | Ongoing work, deadlines, decisions. | `project/auth-refactor.md` |
| `feedback` | A correction or confirmed approach. | `feedback/no-global-state.md` |
| `reference` | External info: dashboards, tickets, docs. | `reference/jira-auth-1234.md` |
| `user` | Role, expertise, preferences. | `user/javi.md` |

### 4.3 MCP server

A local stdio MCP server (`eigenmemory serve`) exposes the following tools:

| Tool | Action |
|------|--------|
| `wiki_recall` | Search the wiki and return relevant pages with citations. |
| `wiki_remember` | Store a durable fact, creating/updating the right wiki pages. |
| `wiki_ingest` | Ingest a raw source into the wiki. |
| `wiki_query` | Answer a natural-language question using the wiki. |
| `wiki_lint` | Run a health check and propose fixes. |
| `wiki_status` | Show current memory state, recent writes, orphan pages. |
| `wiki_reconcile` | Synchronize tool-native memory files with the wiki. |

**Resources** (read-only, auto-injected where clients support them):
- `memory://index` — the wiki index.
- `memory://log` — recent log entries.
- `memory://rules` — the maintenance schema.

### 4.4 Reconciliation model

Tool-native memory is a **projection**:

- **Claude Code**: `~/.claude/projects/<project>/memory/` is regenerated from the wiki. The human can still use `/memory` to edit, but those edits are reconciled back.
- **Zed**: Only uses the MCP server; no native memory files to reconcile.
- **Future tools**: Each tool's native format gets an adapter.

Reconciliation rules:
1. The wiki is the source of truth.
2. Tool-native writes are treated as "proposed updates" and merged into the wiki via `wiki_remember`.
3. Conflicts are resolved by timestamp or by human approval during lint.
4. A periodic `lint` pass ensures no contradictions, orphan pages, or stale claims survive.

### 4.5 Implementation stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| **Language** | Go | Single compiled binary, fast cold start, proven in the MCP memory ecosystem (`mcp-memory`), easy cross-compilation. |
| **CLI** | `cobra` or `urfave/cli` | Idiomatic Go CLI frameworks; good testability and command discoverability. |
| **MCP transport** | JSON-RPC over stdio, implemented directly or via a small Go MCP library | The server surface is small; this keeps dependencies minimal and avoids immature SDK churn. |
| **Markdown parsing** | `goldmark` for AST; `gopkg.in/yaml.v3` for frontmatter | Battle-tested Go libraries, good Obsidian-compatible markdown support. |
| **Search** | SQLite + FTS5 as a derived full-text index over the markdown wiki | Fast, persistent, offline full-text search; still transparent because markdown remains canonical. |
| **Config** | `.eigenmemory/config.json` + `CLAUDE.md` / `AGENTS.md` | Human-editable, version-controllable. |
| **Versioning** | Git | Every project wiki is a git repo (or lives inside one). |

**Canonical vs. derived data**: Markdown files remain the canonical, human-readable source of truth; the SQLite database is a derived index. Any agent can read the wiki without SQLite, but query/recall performance depends on it. The database is rebuilt from markdown on demand (`eigenmemory reindex`) and incrementally updated on every write.

**Future upgrade path**: We can later add sqlite-vec for semantic/embedding search as an additional table, while keeping FTS5 for keyword recall.

---

## 5. Features for Initial Release (MVP)

### 5.1 Must-have

1. **`eigenmemory init`** — Create the wiki directory structure, initialize the SQLite index, and generate `CLAUDE.md` / `AGENTS.md` in a project.
2. **`eigenmemory serve --mcp`** — Run the stdio MCP server.
3. **`wiki_remember` / `wiki_recall` tools** — Store and retrieve durable facts via the SQLite-backed search index.
4. **`wiki_ingest` tool** — Ingest a raw source into the wiki (summary + entity updates + log entry) and update the SQLite index.
5. **`wiki_lint` tool** — Detect orphan pages, broken links, stale claims, contradictions, and index drift.
6. **`eigenmemory reindex`** — Rebuild the SQLite FTS index from the markdown wiki on demand.
7. **Claude Code adapter** — Generate `~/.claude/projects/<project>/memory/` files from the wiki and reconcile edits back.
8. **Zed configuration helper** — Generate the `context_servers` block for `~/.config/zed/settings.json` and the relevant `AGENTS.md` snippet.
9. **Project + global scopes** — `~/.eigenmemory/` for global memory, `.eigenmemory/` for project memory.
10. **Human approval gate** — Writes that change existing facts require confirmation; append-only log writes do not.

### 5.2 Should-have

1. **`wiki_query` tool** — Natural-language answer over the wiki with citations.
2. **Web clip ingestion** — Accept a URL or Markdown file and turn it into a source + summary.
3. **Log viewer CLI** — `eigenmemory log` prints recent activity.
4. **Status CLI** — `eigenmemory status` shows page counts, orphans, last lint time, and index health.
5. **Git integration** — Auto-commit after ingest/lint if inside a git repo (optional).

### 5.3 Won't-have (yet)

1. Vector embeddings / semantic search.
2. Multi-user real-time collaboration.
3. Cloud sync or hosted backend.
4. Complex access control.
5. Mobile clients.

---

## 6. User Stories and Tasks

### Epic 1: Bootstrap and storage foundation

**Story 1.1**: As a user, I can run `eigenmemory init` in any directory so that the wiki structure exists.

- Task 1.1.1: Define directory layout: `.eigenmemory/{config.json,sources,wiki/{index.md,log.md,entity,concept,summary,project,feedback,reference,user},eigenmemory.db}`.
- Task 1.1.2: Implement `init` command with project-name detection.
- Task 1.1.3: Create SQLite schema and FTS5 virtual table for pages and sources.
- Task 1.1.4: Generate default `CLAUDE.md` and `AGENTS.md` maintenance schema.
- Task 1.1.5: Add tests for init output and database creation.

**Story 1.2**: As a user, I can store global memory under `~/.eigenmemory/` so that facts live outside any single project.

- Task 1.2.1: Resolve scope order: global → project, with project overriding on name collisions for recall.
- Task 1.2.2: Implement scope detection from current working directory.
- Task 1.2.3: Add tests for global vs project reads and writes.

### Epic 2: Wiki primitives

**Story 2.1**: As an agent, I can read a wiki page by ID or path so that I can ground answers.

- Task 2.1.1: Implement page loader with frontmatter parsing.
- Task 2.1.2: Implement index loader (parse `index.md` table of contents / catalog).
- Task 2.1.3: Implement link extractor for cross-references.
- Task 2.1.4: Implement SQLite indexer that reads all markdown pages into FTS5 and metadata tables.

**Story 2.2**: As an agent, I can write a wiki page and have it update the index and log automatically.

- Task 2.2.1: Implement page writer with frontmatter serialization.
- Task 2.2.2: Implement index updater.
- Task 2.2.3: Implement log appender with timestamp and operation tag.
- Task 2.2.4: Ensure atomicity (write temp file + rename).
- Task 2.2.5: Incrementally update SQLite index after every markdown write.

### Epic 3: MCP server

**Story 3.1**: As a Zed user, I can add `eigenmemory` as a context server so that the agent gains memory tools.

- Task 3.1.1: Set up MCP server entry point (`eigenmemory serve --mcp`).
- Task 3.1.2: Implement `wiki_recall` tool schema and handler backed by SQLite FTS5.
- Task 3.1.3: Implement `wiki_remember` tool schema and handler that writes markdown and updates SQLite.
- Task 3.1.4: Implement `wiki_status` tool, including index health and last reindex time.
- Task 3.1.5: Add resource endpoints for `memory://index` and `memory://log`.
- Task 3.1.6: Write Zed setup docs and a settings.json generator.

**Story 3.2**: As a Claude Code user, I can add `eigenmemory` as an MCP server so that the agent uses the same memory.

- Task 3.2.1: Write Claude Code `.mcp.json` / `~/.claude.json` setup docs.
- Task 3.2.2: Test stdio transport with Claude Code.

### Epic 4: Ingest and synthesis

**Story 4.1**: As a user, I can drop a Markdown file into sources and run `eigenmemory ingest <path>` so that the wiki is updated.

- Task 4.1.1: Implement source hashing/deduplication.
- Task 4.1.2: Copy source to `.eigenmemory/sources/` with stable ID.
- Task 4.1.3: Generate summary page linked to the source.
- Task 4.1.4: Extract entities and concepts and create/update their pages.
- Task 4.1.5: Append log entry.

**Story 4.2**: As an agent, I can call `wiki_ingest` via MCP so that a source discovered during coding is preserved.

- Task 4.2.1: Expose `wiki_ingest` MCP tool.
- Task 4.2.2: Allow ingesting raw text or file path.

### Epic 5: Reconciliation with Claude Code native memory

**Story 5.1**: As a Claude Code user, my existing `~/.claude/projects/<project>/memory/` files stay in sync with the wiki.

- Task 5.1.1: Implement projection writer that maps wiki pages to Claude Code memory files (`user_*.md`, `project_*.md`, `feedback_*.md`, `reference_*.md`, plus `MEMORY.md` index).
- Task 5.1.2: Detect edits in Claude Code memory files and queue reconciliation.
- Task 5.1.3: Implement `wiki_reconcile` tool / `eigenmemory reconcile` CLI.
- Task 5.1.4: Add safety rule: never delete wiki pages just because a memory file was deleted without approval.

### Epic 6: Lint and maintenance

**Story 6.1**: As a user, I can run `eigenmemory lint` so that the wiki stays healthy.

- Task 6.1.1: Detect orphan pages (not linked from index or another page).
- Task 6.1.2: Detect broken internal links.
- Task 6.1.3: Detect missing source citations.
- Task 6.1.4: Detect pages with `status: stale` older than a threshold.
- Task 6.1.5: Surface contradictions (same entity with conflicting claims).
- Task 6.1.6: Detect index drift: markdown pages missing from SQLite or vice versa.
- Task 6.1.7: Produce a lint report and, via agent, propose fixes; optionally run `reindex` to repair drift.

**Story 6.2**: As an agent, I can call `wiki_lint` so that maintenance runs without the user typing commands.

- Task 6.2.1: Expose `wiki_lint` MCP tool.
- Task 6.2.2: Allow dry-run vs apply modes.

### Epic 7: Query and answer

**Story 7.1**: As a user, I can ask `eigenmemory query "..."` and get a cited answer from the wiki.

- Task 7.1.1: Implement SQLite FTS5 search over page titles, tags, and bodies.
- Task 7.1.2: Rank results by FTS5 rank, tag matches, and recency.
- Task 7.1.3: Build prompt context from top pages and ask the configured LLM to synthesize an answer.
- Task 7.1.4: Include citations in output.

**Story 7.2**: As an agent, I can call `wiki_query` via MCP so that I can answer questions grounded in memory.

- Task 7.2.1: Expose `wiki_query` MCP tool.
- Task 7.2.2: Stream progress / citations back to the client.

### Epic 8: Distribution and docs

**Story 8.1**: As a user, I can install a single `eigenmemory` binary and set it up in minutes.

- Task 8.1.1: Set up Go module, choose SQLite driver (`modernc.org/sqlite` for pure-Go portability or `mattn/go-sqlite3` with cgo), and release build pipeline (goreleaser or GitHub Actions) for macOS/Windows/Linux.
- Task 8.1.2: Add README with quickstart for Claude Code and Zed.
- Task 8.1.3: Add setup helper command: `eigenmemory setup --tool claude` / `--tool zed`.
- Task 8.1.4: Provide example `CLAUDE.md` and `AGENTS.md` snippets in docs.
- Task 8.1.5: Document that `.eigenmemory/eigenmemory.db` should be gitignored; markdown is the version-controlled content.

---

## 7. Implementation Roadmap

### Phase 1: Foundation (week 1)

- Set up Go module, linting (`golangci-lint`), tests (`testing` + `testify`), and SQLite driver.
- Design SQLite schema: pages, sources, relations, fts5 index.
- Implement `init` and directory layout (including `.eigenmemory/eigenmemory.db`).
- Implement page read/write, index, log, and incremental SQLite indexing.
- Implement global/project scope resolution.

### Phase 2: MCP server + basic tools (week 2)

- Implement `eigenmemory serve --mcp`.
- `wiki_recall` (SQLite FTS5), `wiki_remember` (write + index), `wiki_status`.
- Resources: `memory://index`, `memory://log`.
- Add `eigenmemory reindex` CLI command.
- Test with Zed and Claude Code.

### Phase 3: Ingest and reconciliation (week 3)

- Implement `ingest` (CLI + MCP).
- Implement Claude Code memory projection.
- Implement `reconcile`.
- Test cross-tool consistency.

### Phase 4: Lint, query, and polish (week 4)

- Implement `lint`.
- Implement `query`.
- Add setup helpers and documentation.
- Go binary release via GitHub Actions + release notes.

---

## 8. Open Decisions and Alternatives

1. **Search backend**  
   *Default*: SQLite + FTS5 as a derived index over the markdown wiki.  
   *Alternative*: Pure in-memory full-text index, or sqlite-vec for semantic search.  
   *Decision*: SQLite + FTS5 from the start for fast, persistent recall; keep sqlite-vec as a future semantic-search add-on.

2. **LLM provider for synthesis**  
   *Default*: Use the same LLM already connected via the tool (Claude Code / Zed provides the model).  
   *Alternative*: Bundle a local model.  
   *Decision*: Do not bundle a model; the tool's host is the orchestrator.

3. **Reconciliation authority**  
   *Default*: Wiki wins; tool-native edits are proposals.  
   *Alternative*: Merge by timestamp, prompt for conflicts.  
   *Decision*: Wiki wins with conflict prompting in lint.

4. **Embedding search**  
   *Default*: Not in MVP.  
   *Alternative*: Add optional sqlite-vec later.  
   *Decision*: Defer.

5. **Language choice**  
   *Default*: Go for a single compiled binary, fast cold start, and proven MCP memory-server precedent.  
   *Alternative*: TypeScript / Node.js (faster MCP SDK availability), Rust (stronger type safety, `llm-wiki-cli` precedent).  
   *Decision*: Go for the MVP; keep Rust/TS as explicit escape hatches if the Go MCP implementation becomes painful or if advanced LLM/embedding integrations dominate later.

---

## 9. Success Criteria for Initial Release

A user can:

1. Download a single `eigenmemory` binary for their OS, put it on `PATH`, and run `eigenmemory init` in a project.
2. Add the MCP server to Claude Code and Zed in under 10 minutes.
3. Tell Zed, "Remember we are moving to pnpm."
4. Later ask Claude Code, "What package manager decisions have we made?" and get the correct, cited answer.
5. Run `eigenmemory lint` and see a clean, contradiction-free report.

---

## 10. Sources

- Karpathy, *LLM Wiki*: https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f
- Claude Code memory docs: https://code.claude.com/docs/en/memory
- Claude Code `CLAUDE.md` docs: https://code.claude.com/docs/en/claude-md
- Claude Code MCP docs: https://code.claude.com/docs/en/mcp
- Zed MCP docs: https://zed.dev/docs/ai/mcp
- Zed Agent Panel docs: https://zed.dev/docs/ai/agent-panel
- Official MCP memory server: https://github.com/modelcontextprotocol/servers/tree/main/src/memory
- `mcp-memory` (Go, markdown-based): https://github.com/coah80/mcp-memory
- `llm-wiki-cli` / `lwc` (Rust, SQLite-backed): https://github.com/JanYork/llm-wiki-cli
