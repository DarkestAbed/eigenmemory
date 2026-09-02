package templates

import (
	_ "embed"
	"strings"
)

//go:embed data/CLAUDE.md.tmpl
var claudeTemplate string

//go:embed data/AGENTS.md.tmpl
var agentsTemplate string

// projectNamePlaceholder is substituted with the real project name at
// generation time. Keeping the canonical templates under data/ (rather than
// Go string literals) means this repo's own root CLAUDE.md/AGENTS.md and the
// files `eigenmemory init` writes into every other project come from the
// same source and cannot drift apart silently — see
// internal/templates/claude_md_test.go.
const projectNamePlaceholder = "{{PROJECT_NAME}}"

// CLAUDE returns the default CLAUDE.md content for an EigenMemory-enabled project.
func CLAUDE(projectName string) string {
	return strings.ReplaceAll(claudeTemplate, projectNamePlaceholder, projectName)
}

// AGENTS returns the default AGENTS.md content for an EigenMemory-enabled project.
func AGENTS(projectName string) string {
	return strings.ReplaceAll(agentsTemplate, projectNamePlaceholder, projectName)
}
