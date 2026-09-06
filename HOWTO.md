# HOWTO: Connect EigenMemory to every major coding harness

EigenMemory is a local MCP server (`eigenmemory serve --mcp`) sitting on top of a markdown wiki at
`.eigenmemory/wiki/`. Every harness below talks to the *same* server process definition — only the
config file, its format, and (for a couple of tools) an extra "native memory" step differ. This
doc collects accurate, verified-as-of-2026-09 instructions for each one.

## 0. Prerequisites (do this once per project)

```bash
curl -fsSL https://raw.githubusercontent.com/DarkestAbed/eigenmemory/main/install.sh | sh
cd path/to/your-project
eigenmemory init
```

`eigenmemory init` writes `CLAUDE.md` and `AGENTS.md` next to `.eigenmemory/`. `AGENTS.md` is the
shared instruction-file convention now read by most of the tools below (Zed, Codex CLI,
Antigravity, Cursor, Windsurf, and others); `CLAUDE.md` is Claude Code's own file and carries one
extra instruction (native-memory reconciliation) that doesn't apply anywhere else yet. You do not
need to write these by hand — just don't delete them.

Every snippet below registers the same stdio command:

```json
{ "command": "eigenmemory", "args": ["serve", "--mcp"] }
```

If `eigenmemory` isn't on `PATH` for the harness's process (GUI apps sometimes launch with a
thinner `PATH` than your shell), replace `"eigenmemory"` with the absolute path from
`which eigenmemory`.

## 1. Compatibility matrix

| Harness | Config file | Top-level key | Reads `AGENTS.md`? | Native memory reconcile | `eigenmemory setup --tool` |
|---|---|---|---|---|---|
| Claude Code | `.mcp.json` (project) | `mcpServers` | Reads `CLAUDE.md` instead | ✅ `wiki_reconcile` / `eigenmemory reconcile` | `claude` |
| Zed | `~/.config/zed/settings.json` | `context_servers` | ✅ | — (MCP tools only) | `zed` |
| Codex CLI | `~/.codex/config.toml` or `.codex/config.toml` | `mcp_servers` (TOML table) | ✅ | — (MCP tools only) | `codex` |
| Antigravity | `~/.gemini/config/mcp_config.json` or `.agents/mcp_config.json` | `mcpServers` | ✅ (`~/.gemini/AGENTS.md` + project root) | — (MCP tools only) | `antigravity` |
| Cursor | `.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global) | `mcpServers` | ✅ | — (MCP tools only) | manual |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` | `mcpServers` | ✅ | — (MCP tools only) | manual |
| Cline (VS Code ext.) | `cline_mcp_settings.json` (VS Code global storage) | `mcpServers` | check current Cline docs; historically `.clinerules` | — (MCP tools only) | manual |
| Continue.dev | `.continue/mcpServers/*.yaml` or `config.yaml` | `mcpServers` | ✅ | — (MCP tools only) | manual |
| Gemini CLI | `~/.gemini/settings.json` (or `--project` scope) | `mcpServers` | ✅ | — (MCP tools only) | manual |
| GitHub Copilot (VS Code) | `.vscode/mcp.json` | `servers` (not `mcpServers`) | partial (`.github/copilot-instructions.md`, growing `AGENTS.md` support) | — (MCP tools only) | manual |
| GitHub Copilot CLI | `.mcp.json` or `.github/mcp.json` | `mcpServers` (+ required `type`) | ✅ | — (MCP tools only) | manual |
| Amp | `~/.config/amp/settings.json` | `amp.mcpServers` | ✅ | — (MCP tools only) | manual |
| opencode | `opencode.json` | `mcp` (per-server `"type": "local"`) | ✅ | — (MCP tools only) | manual |

Only Claude Code has an eigenmemory-managed native memory directory today, so it's the only tool
`wiki_reconcile`/`eigenmemory reconcile` applies to (see [`README.md`](./README.md) and the
project's `CLAUDE.md`). Every other harness reads the wiki live through the MCP tools
(`wiki_recall`, `wiki_remember`, `wiki_query`, `wiki_ingest`, `wiki_lint`, `wiki_status`) plus
whatever `AGENTS.md`-equivalent file it honors — there's nothing tool-native to reconcile back.

## 2. Claude Code

```bash
eigenmemory setup --tool claude
```

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

Claude Code additionally gets a projected copy of the wiki under
`~/.claude/projects/<sanitized-project-path>/memory/`. Run `eigenmemory reconcile` (or the
`wiki_reconcile` MCP tool) after hand-editing those files or after Claude Code's own automatic
memory-saving writes something new, so it merges back into the wiki instead of drifting from it.

## 3. Zed

```bash
eigenmemory setup --tool zed
```

Add to `~/.config/zed/settings.json` (or the project-local `.zed/settings.json`):

```json
{
  "context_servers": {
    "eigenmemory": {
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

`command` is a plain string in Zed's current schema — not `{"path": ..., "args": [...]}`. Zed
restarts the context server automatically on save; no editor restart needed.

## 4. Codex CLI

```bash
eigenmemory setup --tool codex
```

Add to `~/.codex/config.toml` (or `.codex/config.toml` in a trusted project):

```toml
[mcp_servers.eigenmemory]
command = "eigenmemory"
args = ["serve", "--mcp"]
```

Note the snake_case `mcp_servers` table name — this is TOML, not JSON, and Codex's key differs
from most other tools' `mcpServers`. Codex already reads `AGENTS.md` (global `~/.codex/AGENTS.md`,
then every `AGENTS.md` from the repo root down to your working directory, concatenated), so the
file `eigenmemory init` wrote applies with no extra config. Codex CLI also has its own optional,
off-by-default native memory feature (`~/.codex/memories/`) — that's separate from and unrelated
to eigenmemory; there is no reconcile adapter for it.

## 5. Antigravity

```bash
eigenmemory setup --tool antigravity
```

Add to `~/.gemini/config/mcp_config.json` (global) or `.agents/mcp_config.json` (workspace-local):

```json
{
  "mcpServers": {
    "eigenmemory": {
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

Antigravity also reads `AGENTS.md` at both the global (`~/.gemini/AGENTS.md`) and project-root
scope, merging them at session start — the project-root copy `eigenmemory init` generated is
picked up automatically. You can also install/manage this through the IDE's MCP Store UI instead
of hand-editing JSON: open the agent panel menu → MCP Servers → Manage MCP Servers → View raw
config.

## 6. Cursor

Add to `.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global, all projects):

```json
{
  "mcpServers": {
    "eigenmemory": {
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

Cursor reads `AGENTS.md` as its rules file, so no extra step is needed beyond `eigenmemory init`.

## 7. Windsurf

Add to `~/.codeium/windsurf/mcp_config.json` (`%USERPROFILE%\.codeium\windsurf\mcp_config.json` on
Windows):

```json
{
  "mcpServers": {
    "eigenmemory": {
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

## 8. Cline (VS Code extension)

Cline stores MCP config in `cline_mcp_settings.json` inside VS Code's extension global storage
directory — easiest to reach via Cline's own UI: MCP Servers icon → Configure → "Edit
Configuration," which opens the file directly rather than requiring you to hunt for the path.

```json
{
  "mcpServers": {
    "eigenmemory": {
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

Cline picks up config changes live — no restart needed.

## 9. Continue.dev

Add an MCP block under `mcpServers` in your Continue `config.yaml`, or drop a
Claude-Desktop/Cursor/Cline-style JSON block into `.continue/mcpServers/eigenmemory.yaml`:

```yaml
mcpServers:
  - name: eigenmemory
    command: eigenmemory
    args: ["serve", "--mcp"]
```

## 10. Gemini CLI

```bash
gemini mcp add eigenmemory eigenmemory serve --mcp
```

or edit `~/.gemini/settings.json` directly (add `--project` scoping if you only want this wired
into one repo):

```json
{
  "mcpServers": {
    "eigenmemory": {
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

## 11. GitHub Copilot

**VS Code (Copilot Chat / agent mode)** — `.vscode/mcp.json` in the workspace. Note the top-level
key is `servers`, not `mcpServers`:

```json
{
  "servers": {
    "eigenmemory": {
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

**Copilot CLI** — does *not* read `.vscode/mcp.json`. Use `.mcp.json` (project) or
`.github/mcp.json` (repo), key `mcpServers`, and a required `type` field per server:

```json
{
  "mcpServers": {
    "eigenmemory": {
      "type": "local",
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

Copilot's own instruction file is `.github/copilot-instructions.md`; it has also been adding
`AGENTS.md` support for the coding agent. Until you've confirmed which your Copilot surface reads,
keep the durable instructions in `AGENTS.md` (eigenmemory-generated) and mirror the short version
into `.github/copilot-instructions.md` if needed.

## 12. Amp (Sourcegraph)

Edit `~/.config/amp/settings.json` (`%APPDATA%\amp\settings.json` on Windows):

```json
{
  "amp.mcpServers": {
    "eigenmemory": {
      "command": "eigenmemory",
      "args": ["serve", "--mcp"]
    }
  }
}
```

## 13. opencode

Add to `opencode.json`:

```json
{
  "mcp": {
    "eigenmemory": {
      "type": "local",
      "command": ["eigenmemory", "serve", "--mcp"]
    }
  }
}
```

## 14. Other harnesses worth watching

These either lack first-class MCP support today or their config format wasn't confirmed precisely
enough to publish a copy-paste snippet — verify against the tool's current docs before wiring
it up:

- **Aider** has no native MCP client as of 2026; the common workaround is a third-party wrapper
  (e.g. AiderDesk) that adds an MCP layer on top of it. Aider does read plain instruction files
  (`CONVENTIONS.md` via `--read`), so pointing it at `AGENTS.md` there gets you the instructions
  half even without MCP tool access.
- **JetBrains AI Assistant / Junie** supports MCP servers configured through the IDE's own
  settings UI (Settings → Tools → AI Assistant → MCP), which writes to an internal, IDE-managed
  config rather than a plain file you'd hand-edit — add `eigenmemory serve --mcp` as a stdio
  server there.
- **Warp**, **Trae**, and other newer agentic terminals/IDEs are converging on the same
  `mcpServers`-with-`command`/`args`/`env` shape used above; check for an `mcpServers` block in
  their settings before assuming a different schema.

If you get a harness working that isn't listed here (or find one of the entries above has drifted
from the tool's current docs), send a PR — `eigenmemory setup --tool <name>` is the natural home
for any of these once the format is confirmed stable, following the pattern in
`internal/cmd/setup.go`.
