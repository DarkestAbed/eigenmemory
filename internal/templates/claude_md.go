package templates

// CLAUDE returns the default CLAUDE.md content for an EigenMemory-enabled project.
func CLAUDE(projectName string) string {
	return "# EigenMemory / CLAUDE.md — " + projectName + "\n\n" +
		"This project uses [EigenMemory](https://github.com/DarkestAbed/eigenmemory), a persistent LLM Wiki for coding harnesses.\n\n" +
		"## What EigenMemory owns\n\n" +
		"The directory `.eigenmemory/` is maintained by the agent. Do not hand-edit files there unless you know what you are doing. The canonical store is:\n\n" +
		"- `.eigenmemory/wiki/` — agent-maintained markdown pages with YAML frontmatter.\n" +
		"- `.eigenmemory/sources/` — immutable raw sources (design docs, transcripts, web clips).\n" +
		"- `.eigenmemory/eigenmemory.db` — derived SQLite FTS5 index (gitignored, rebuildable).\n\n" +
		"## How to use EigenMemory\n\n" +
		"1. **Recall before answering**: when the user asks about prior decisions, run the MCP tool `wiki_recall` or read the relevant pages from `.eigenmemory/wiki/`.\n" +
		"2. **Remember after learning**: when you learn a durable fact (decisions, preferences, corrections, external references), store it via `wiki_remember` or by writing the appropriate wiki page.\n" +
		"3. **Ingest sources**: when the user provides a design doc, article, or transcript, call `wiki_ingest` or run `eigenmemory ingest <path>`.\n" +
		"4. **Maintain the wiki**: periodically run `eigenmemory lint` and apply fixes.\n" +
		"5. **Reconcile Claude Code memory**: changes made via `/memory` are proposals. Run `eigenmemory reconcile` to merge them back into the wiki.\n\n" +
		"## Page types\n\n" +
		"| Type | Directory | Use |\n" +
		"|------|-----------|-----|\n" +
		"| entity | `wiki/entity/` | Nouns: services, files, classes, people, decisions. |\n" +
		"| concept | `wiki/concept/` | Ideas, patterns, principles. |\n" +
		"| summary | `wiki/summary/` | Digest of a single raw source. |\n" +
		"| project | `wiki/project/` | Ongoing work, deadlines, milestones. |\n" +
		"| feedback | `wiki/feedback/` | Corrections and confirmed approaches. |\n" +
		"| reference | `wiki/reference/` | External links: tickets, dashboards, docs. |\n" +
		"| user | `wiki/user/` | Role, expertise, preferences. |\n\n" +
		"## Frontmatter standard\n\n" +
		"```yaml\n" +
		"---\n" +
		"id: <uuid>\n" +
		"type: entity | concept | summary | feedback | project | reference | user\n" +
		"status: active | stale | merged | archived\n" +
		"created: 2026-08-22T10:00:00Z\n" +
		"updated: 2026-08-22T10:00:00Z\n" +
		"sources:\n" +
		"  - source-id-1\n" +
		"tags: [auth, refactor]\n" +
		"relations:\n" +
		"  - { from: \"slug\", to: \"other-slug\", type: \"builds_on\" }\n" +
		"---\n" +
		"```\n\n" +
		"## Rules\n\n" +
		"- Always cite sources when synthesizing answers.\n" +
		"- Prefer updating an existing entity page over creating duplicates.\n" +
		"- Keep pages focused; split large topics.\n" +
		"- Run `eigenmemory reindex` if you manually edit many markdown files.\n" +
		"- Do not commit `.eigenmemory/eigenmemory.db`.\n"
}

// AGENTS returns the default AGENTS.md content for an EigenMemory-enabled project.
func AGENTS(projectName string) string {
	return "# EigenMemory / AGENTS.md — " + projectName + "\n\n" +
		"This file is consumed by all AI agents working on this project, including Zed.\n\n" +
		"## EigenMemory MCP tools\n\n" +
		"When connected to the EigenMemory MCP server, prefer these tools:\n\n" +
		"- `wiki_recall(query)` — search the LLM Wiki before answering questions about past decisions or project context.\n" +
		"- `wiki_remember(fact, type, tags)` — store durable facts with proper categorization.\n" +
		"- `wiki_ingest(source)` — ingest a design doc, transcript, or article into the wiki.\n" +
		"- `wiki_lint()` — run health checks and propose fixes.\n" +
		"- `wiki_status()` — inspect wiki state and index health.\n\n" +
		"## Memory workflow\n\n" +
		"1. At the start of a task, recall relevant pages.\n" +
		"2. After making a decision or learning a durable fact, remember it.\n" +
		"3. Before claiming something about the project, cite the wiki or a raw source.\n" +
		"4. Respect the frontmatter schema in `.eigenmemory/wiki/`.\n\n" +
		"## Cross-tool consistency\n\n" +
		"Facts stored via this project's EigenMemory are shared with Claude Code. The wiki under `.eigenmemory/wiki/` is the source of truth.\n"
}
