<div align="center">

# 🧠 EigenMemory

### A persistent, cross-tool LLM memory layer for coding harnesses

[![CI](https://img.shields.io/github/actions/workflow/status/DarkestAbed/eigenmemory/ci.yml?branch=main&label=CI)](https://github.com/DarkestAbed/eigenmemory/actions/workflows/ci.yml)
[![Release version](https://img.shields.io/github/v/release/DarkestAbed/eigenmemory)](https://github.com/DarkestAbed/eigenmemory/releases)
![Release date](https://img.shields.io/github/release-date/DarkestAbed/eigenmemory)

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE.md)
[![GitHub stars](https://img.shields.io/github/stars/DarkestAbed/eigenmemory?style=social)](https://github.com/DarkestAbed/eigenmemory/stargazers)
![Go](https://img.shields.io/badge/language-Go-00ADD8?logo=go&logoColor=white)
![MCP](https://img.shields.io/badge/protocol-MCP-6c45d6)

## Available on

[![Install script](https://img.shields.io/badge/Install%20script-available-brightgreen)](./install.sh)
[![Go install](https://img.shields.io/badge/Go%20install-available-brightgreen)](https://pkg.go.dev/github.com/DarkestAbed/eigenmemory)
[![Binaries](https://img.shields.io/badge/Binaries-available-brightgreen)](https://github.com/DarkestAbed/eigenmemory/releases)
[![MCP server](https://img.shields.io/badge/MCP%20server-available-brightgreen)](#connect-to-claude-code)

[Quick start](#quick-start) · [Install](#install) · [Usage](#usage) · [MCP tools](#mcp-tools) · [Development](#development)

</div>

<div align="center">

<img width="100%" height="100%" alt="Image" src="https://github.com/user-attachments/assets/b2dc954c-af7c-4631-9a00-45a837ac44b4" />

</div>

EigenMemory keeps a single markdown-based **LLM Wiki** as the canonical memory store, following [Andrej Karpathy's LLM Wiki pattern](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f). EigenMemory then builds the wiki and then exposes it to Claude Code, Zed, and any MCP-speaking tool through a local stdio server. Tool-native memory files (Claude Code's `/memory`, Zed's agent memory - ***more to come***) are treated as **projections** of the wiki, not independent sources of truth — so every tool you use reads from and writes to the same knowledge base.

Use it when you want an agent to remember decisions across sessions, recall why a system was built a certain way, or stop re-asking questions you already answered last week. It runs locally, stores everything in plain markdown you can read and edit, and indexes it into SQLite FTS5 for fast keyword and graph-like search.

## Quick start

Install and initialize a wiki in your project:

```bash
curl -fsSL https://raw.githubusercontent.com/DarkestAbed/eigenmemory/main/install.sh | sh
cd path/to/your-project
eigenmemory init
```

Then point a tool at it. For Claude Code, add to `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "eigenmemory": {
      "type": "stdio",
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

That's it — the agent can now `wiki_remember` facts and `wiki_recall` them later, across sessions and across tools.

## What you can do

- **Recall before answering** — `wiki_query` / `wiki_recall` run keyword full-text search (FTS5) over the whole wiki and expand hits by one graph hop over `[[wikilinks]]` and frontmatter `relations`, so related pages surface even without matching the query terms.
- **Remember after learning** — `wiki_remember` stores a durable fact as a typed page (decision, preference, reference) with sources and tags, so the next session starts where the last one left off.
- **Ingest raw sources** — `wiki_ingest` archives a design doc, article, or transcript as an immutable copy plus a digest page, then extracts `[[wikilinks]]` from the full source as graph edges.
- **Lint for health** — `eigenmemory lint` flags broken links, orphan pages, stale entries, and index drift, and `--fix` repairs drift by rebuilding the search index.
- **Reconcile tool-native memory** — `eigenmemory reconcile` merges edits made through a tool's native memory (e.g. Claude Code's `/memory`) back into the wiki and regenerates the projection.
- **Work from your editor** — Claude Code and Zed both speak MCP, so the same wiki serves every tool without per-tool setup.

### What's in a wiki

`eigenmemory init` creates a per-project store you can read, diff, and edit by hand:

```text
.eigenmemory/
├── config.json
├── eigenmemory.db      # SQLite FTS5 index (gitignored, rebuildable)
├── sources/            # immutable raw sources
└── wiki/
    ├── index.md        # catalog of pages
    ├── log.md          # append-only activity log
    ├── entity/         # nouns: services, files, people, decisions
    ├── concept/        # ideas and patterns
    ├── summary/        # digests of raw sources
    ├── project/        # ongoing work
    ├── feedback/       # corrections and confirmed approaches
    ├── reference/      # external links
    └── user/           # role, expertise, preferences
```

Each page is markdown with YAML frontmatter (id, type, status, sources, tags, relations). `[[slug]]` wikilinks are auto-indexed as graph edges, and links inside code blocks are ignored so a verbatim quote never becomes an edge.

## Install

**Install script** (no Go required; downloads the latest release from GitHub and verifies its checksum):

```bash
curl -fsSL https://raw.githubusercontent.com/DarkestAbed/eigenmemory/main/install.sh | sh
```

Pin a version or install location:

```bash
curl -fsSL https://raw.githubusercontent.com/DarkestAbed/eigenmemory/main/install.sh | EIGENMEMORY_VERSION=v0.1.6 INSTALL_DIR=/usr/local/bin sh
```

**With Go** (any machine with Go 1.26+):

```bash
go install github.com/DarkestAbed/eigenmemory/cmd/eigenmemory@latest
```

**Manual**: grab a `tar.gz`/`zip` from the [releases page](https://github.com/DarkestAbed/eigenmemory/releases) (linux/darwin/windows × amd64/arm64) and put the binary on your `PATH`.

### Connect to Claude Code

Add to `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "eigenmemory": {
      "type": "stdio",
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

Or generate the snippet with `eigenmemory setup --tool claude`.

### Connect to Zed

Add to `~/.config/zed/settings.json`:

```json
{
  "context_servers": {
    "eigenmemory": {
      "command": {
        "path": "eigenmemory",
        "args": ["serve", "--mcp"]
      }
    }
  }
}
```

Or generate the snippet with `eigenmemory setup --tool zed`.

## Usage

- `eigenmemory init [project-name]` — create a wiki.
- `eigenmemory serve --mcp` — run the MCP server.
- `eigenmemory reindex` — rebuild the SQLite FTS5 index from markdown.
- `eigenmemory lint` — check wiki health.
- `eigenmemory lint --fix` — repair index drift by rebuilding the search index.
- `eigenmemory status` — show wiki state.
- `eigenmemory reconcile` — merge Claude Code native memory edits back into the wiki and regenerate the projection.
- `eigenmemory reconcile --dry-run` — preview proposed reconciliation actions.
- `eigenmemory query "..."` — natural-language recall with citations. Full-text hits are expanded by one graph hop over `[[wikilinks]]` and frontmatter `relations`, so related pages surface even without matching the query terms.
- `eigenmemory setup --tool claude` — print `.mcp.json` snippet for Claude Code.
- `eigenmemory setup --tool zed` — print `settings.json` snippet for Zed.

## MCP tools

| Tool | Purpose |
|------|---------|
| `wiki_recall(query)` | Search the wiki (full-text + one-hop graph expansion). |
| `wiki_remember(fact, type, tags)` | Store a durable fact. |
| `wiki_ingest(source)` | Ingest a raw source (summary-only; use `wiki_remember` to classify facts). |
| `wiki_lint()` | Run health checks. |
| `wiki_status()` | Inspect state and index health. |
| `wiki_query(question)` | Ask a natural-language question. |
| `wiki_reconcile()` | Merge tool-native memory back into the wiki. |

## Development

EigenMemory is a single Go binary backed by SQLite (via `modernc.org/sqlite`, no CGO). Build and test locally:

```bash
make build      # build the binary
make test       # run the test suite
make lint       # run golangci-lint
```

Requirements: Go 1.26+. The SQLite index is a derived, gitignored artifact — run `eigenmemory reindex` to rebuild it from markdown after a fresh checkout.

## Contributing

Bug reports, feature ideas, and code contributions are all welcome.

- [Open an issue](https://github.com/DarkestAbed/eigenmemory/issues) for a bug or concrete request.
- Read [`CLAUDE.md`](./CLAUDE.md) for the agent-maintained wiki conventions before working on the store.
- Run `make test` and `eigenmemory lint` before sending a pull request.

## Support

- [GitHub Issues](https://github.com/DarkestAbed/eigenmemory/issues)
- [`CLAUDE.md`](./CLAUDE.md) — agent-maintained wiki conventions
- [Releases](https://github.com/DarkestAbed/eigenmemory/releases)

EigenMemory is licensed under the [MIT License](./LICENSE.md).

<div align="center">

If EigenMemory saves you a session, consider [giving the project a star](https://github.com/DarkestAbed/eigenmemory/stargazers).

</div>
