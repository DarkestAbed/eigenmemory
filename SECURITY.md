# Security Policy

## Supported versions

EigenMemory is pre-1.0. Security fixes are made against the latest release on the `main`
branch; there is no long-term support for older tags.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

- Preferred: use [GitHub Security Advisories](https://github.com/DarkestAbed/eigenmemory/security/advisories/new)
  for this repository to open a private report.
- Alternative: email **thedarkestabed@gmail.com** with a description of the issue, the
  affected version/commit, and reproduction steps if available.

We'll acknowledge reports within a few days and aim to ship a fix or mitigation before any
public disclosure. Please give us a reasonable window to do so before disclosing publicly.

## Scope and threat model

EigenMemory runs an MCP server (`eigenmemory serve --mcp`) that reads and writes files under
`.eigenmemory/` (and, via the Claude Code adapter, under `~/.claude/projects/<name>/memory/`)
on behalf of whatever LLM client is connected to it. Because tool arguments can originate from
model output (including model output influenced by content the model has read, e.g. an
ingested document or a recalled page), **all MCP tool arguments are treated as untrusted
input** — this includes `slug`, `page_type`, file paths passed to `wiki_ingest`, and the
project name resolved from a cloned repository's `.eigenmemory/config.json`.

Particularly interesting classes of report:

- Path traversal or arbitrary file read/write reachable through any MCP tool argument or
  `config.json` field.
- Anything that lets a crafted wiki page, ingested source, or `config.json` cause code
  execution, not just data manipulation.
- Resource-exhaustion issues reachable from a single untrusted MCP request (unbounded reads,
  unbounded memory growth) severe enough to be a practical denial-of-service against a local
  agent session.

Out of scope: issues that require the user to already have arbitrary code execution on their
own machine (e.g. "an attacker who can already edit your local files can edit your local
files"), and social-engineering reports.
