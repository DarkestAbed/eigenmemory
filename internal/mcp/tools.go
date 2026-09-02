package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DarkestAbed/eigenmemory/internal/adapters"
	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/core"
	"github.com/DarkestAbed/eigenmemory/internal/lint"
	"github.com/DarkestAbed/eigenmemory/internal/types"
	"github.com/DarkestAbed/eigenmemory/internal/wiki"
)

// RegisterWikiTools adds all EigenMemory tools to the server.
func RegisterWikiTools(s *Server) {
	s.RegisterTool(Tool{
		Name:        "wiki_recall",
		Description: "Search the EigenMemory wiki for relevant pages. Use this before answering questions about prior decisions, project context, or stored facts.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Natural-language query to search for in the wiki.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of results to return (default 5).",
					"default":     5,
				},
			},
			"required": []string{"query"},
		},
	}, handleWikiRecall)

	s.RegisterTool(Tool{
		Name:        "wiki_remember",
		Description: "Store a durable fact in the EigenMemory wiki. Creates or updates the appropriate page and refreshes the search index.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fact": map[string]any{
					"type":        "string",
					"description": "The durable fact to remember. Will be stored as markdown body content.",
				},
				"page_type": map[string]any{
					"type":        "string",
					"description": "Page type: entity, concept, summary, project, feedback, reference, or user.",
				},
				"slug": map[string]any{
					"type":        "string",
					"description": "Optional explicit slug. If omitted, derived from the first heading or a summary of the fact.",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Optional explicit page title. If omitted, derived from the fact.",
				},
				"tags": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Tags to associate with the page.",
				},
				"sources": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Source IDs or URLs that ground this fact.",
				},
				"status": map[string]any{
					"type":        "string",
					"description": "Optional page lifecycle status: active, stale, merged, or archived. Leave unset to keep the existing status (new pages default to active).",
				},
			},
			"required": []string{"fact", "page_type"},
		},
	}, handleWikiRemember)

	s.RegisterTool(Tool{
		Name:        "wiki_ingest",
		Description: "Ingest a raw source (file path or text) into the EigenMemory wiki. Creates an immutable copy in .eigenmemory/sources/ and a summary page linked to it.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"source": map[string]any{
					"type":        "string",
					"description": "File path or raw text to ingest.",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Human-readable source name. Defaults to file basename for paths or 'inline-source' for text.",
				},
				"is_path": map[string]any{
					"type":        "boolean",
					"description": "If true, 'source' is treated as a file path. Otherwise it is treated as raw text.",
					"default":     false,
				},
			},
			"required": []string{"source"},
		},
	}, handleWikiIngest)

	s.RegisterTool(Tool{
		Name:        "wiki_reconcile",
		Description: "Synchronize Claude Code native memory files with the EigenMemory wiki. Newer edits in Claude memory are merged back into the wiki, then the projection is regenerated.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dry_run": map[string]any{
					"type":        "boolean",
					"description": "If true, only show proposed actions without applying them.",
					"default":     false,
				},
			},
		},
	}, handleWikiReconcile)

	s.RegisterTool(Tool{
		Name:        "wiki_status",
		Description: "Inspect the current EigenMemory state: scope, indexed page count, project name, and last log entry.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, handleWikiStatus)

	s.RegisterTool(Tool{
		Name:        "wiki_lint",
		Description: "Check the EigenMemory wiki for orphans, broken links, stale pages, and index drift. Optionally repair index drift.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fix": map[string]any{
					"type":        "boolean",
					"description": "If true, repair index drift by rebuilding the SQLite FTS5 index.",
					"default":     false,
				},
			},
		},
	}, handleWikiLint)

	s.RegisterTool(Tool{
		Name:        "wiki_query",
		Description: "Answer a natural-language question using the EigenMemory wiki. Returns relevant context with citations; synthesize the final answer yourself.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Natural-language question to answer from the wiki.",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of source pages to include (default 5).",
					"default":     5,
				},
			},
			"required": []string{"query"},
		},
	}, handleWikiQuery)
}

// RegisterWikiResources adds EigenMemory resources to the server.
func RegisterWikiResources(s *Server) {
	s.RegisterResource(Resource{
		URI:         "memory://index",
		Name:        "EigenMemory Index",
		Description: "Catalog of all wiki pages, grouped by type.",
		MimeType:    "text/markdown",
	}, fetchResource)

	s.RegisterResource(Resource{
		URI:         "memory://log",
		Name:        "EigenMemory Log",
		Description: "Recent append-only activity log.",
		MimeType:    "text/markdown",
	}, fetchResource)
}

func handleWikiRecall(ctx context.Context, s *Server, args json.RawMessage) (any, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse arguments: %w", err)
	}
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if params.Limit <= 0 {
		params.Limit = 5
	}

	store := s.CurrentStore()
	results, err := store.Search.SearchWithGraph(params.Query, params.Limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	var lines []string
	for i, r := range results {
		summary := wiki.StripFooters(r.Body)
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}
		marker := ""
		var extra string
		switch r.MatchSource {
		case "graph":
			marker = " (graph)"
		case "source":
			marker = " (source)"
			if r.SourceID != "" {
				extra = fmt.Sprintf("\n   Full source: .eigenmemory/sources/%s", r.SourceID)
			}
		}
		lines = append(lines, fmt.Sprintf("%d. %s (%s/%s%s)\n   %s%s", i+1, r.Title, r.Type, r.Slug, marker, summary, extra))
	}

	if len(lines) == 0 {
		return ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: "No relevant pages found."}},
		}, nil
	}

	return ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: strings.Join(lines, "\n\n")}},
	}, nil
}

func handleWikiRemember(ctx context.Context, s *Server, args json.RawMessage) (any, error) {
	var params struct {
		Fact     string   `json:"fact"`
		PageType string   `json:"page_type"`
		Slug     string   `json:"slug"`
		Title    string   `json:"title"`
		Tags     []string `json:"tags"`
		Sources  []string `json:"sources"`
		Status   string   `json:"status"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse arguments: %w", err)
	}
	if params.Fact == "" || params.PageType == "" {
		return nil, fmt.Errorf("fact and page_type are required")
	}

	pageType := types.PageType(params.PageType)
	valid := false
	for _, t := range types.ValidPageTypes() {
		if t == pageType {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("invalid page_type: %s", params.PageType)
	}

	var status types.PageStatus
	if params.Status != "" {
		status = types.PageStatus(params.Status)
		validStatus := false
		for _, st := range []types.PageStatus{types.PageStatusActive, types.PageStatusStale, types.PageStatusMerged, types.PageStatusArchived} {
			if status == st {
				validStatus = true
				break
			}
		}
		if !validStatus {
			return nil, fmt.Errorf("invalid status: %s", params.Status)
		}
	}

	store := s.CurrentStore()

	body := params.Fact
	slug := params.Slug
	title := params.Title

	if title == "" {
		title = extractFirstLine(body)
	}
	if !strings.HasPrefix(body, "# ") {
		body = "# " + title + "\n\n" + body
	}
	if slug == "" {
		slug = wiki.Slugify(title)
	}
	if err := wiki.ValidateSlug(slug); err != nil {
		return nil, fmt.Errorf("invalid slug: %w", err)
	}

	var page *types.Page
	var err error
	if wiki.PageExists(store.Paths, pageType, slug) {
		page, err = store.LoadPage(pageType, slug)
		if err != nil {
			return nil, fmt.Errorf("load existing page: %w", err)
		}
		page.Body = body
	} else {
		page = &types.Page{
			Frontmatter: types.DefaultFrontmatter(pageType),
			Slug:        slug,
			Body:        body,
		}
	}

	page.Frontmatter.Type = pageType
	page.Frontmatter.Tags = uniqueStrings(append(page.Frontmatter.Tags, params.Tags...))
	page.Frontmatter.Sources = uniqueStrings(append(page.Frontmatter.Sources, params.Sources...))
	if status != "" {
		page.Frontmatter.Status = status
	}

	if err := store.SavePage(page, pageType); err != nil {
		return nil, fmt.Errorf("save page: %w", err)
	}
	if err := store.SaveIndex(); err != nil {
		return nil, fmt.Errorf("save index: %w", err)
	}
	if err := store.AppendLog(wiki.OpRemember, page.Slug, fmt.Sprintf("Remembered via MCP at %s", time.Now().UTC().Format(time.RFC3339))); err != nil {
		return nil, fmt.Errorf("append log: %w", err)
	}

	return ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: fmt.Sprintf("Stored %s/%s.md", pageType, page.Slug)}},
	}, nil
}

func handleWikiIngest(ctx context.Context, s *Server, args json.RawMessage) (any, error) {
	var params struct {
		Source string `json:"source"`
		Name   string `json:"name"`
		IsPath bool   `json:"is_path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse arguments: %w", err)
	}
	if params.Source == "" {
		return nil, fmt.Errorf("source is required")
	}

	store := s.CurrentStore()
	result, err := store.Ingest(core.IngestOptions{
		Source: params.Source,
		Name:   params.Name,
		IsPath: params.IsPath,
	})
	if err != nil {
		return nil, err
	}

	return ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: result}},
	}, nil
}

func handleWikiReconcile(ctx context.Context, s *Server, args json.RawMessage) (any, error) {
	var params struct {
		DryRun bool `json:"dry_run"`
	}
	_ = json.Unmarshal(args, &params)

	store := s.CurrentStore()
	cfg, err := config.LoadConfig(store.Paths)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("project name not set")
	}

	claudeDir := adapters.ResolveClaudeProjectDir(cfg, store.Paths)
	if claudeDir == "" {
		return nil, fmt.Errorf("reconcile has no meaning for global-scope memory (no single Claude Code project directory is associated with it); run it from within a project scope instead")
	}

	actions, err := adapters.Reconcile(store.Paths, claudeDir, params.DryRun)
	if err != nil {
		return nil, fmt.Errorf("reconcile: %w", err)
	}

	if !params.DryRun {
		if err := adapters.ProjectMemoryProjection(store.Paths, claudeDir); err != nil {
			return nil, fmt.Errorf("project memory: %w", err)
		}
		if err := store.SaveIndex(); err != nil {
			return nil, fmt.Errorf("save index: %w", err)
		}
		if err := store.RebuildIndex(); err != nil {
			return nil, fmt.Errorf("rebuild search index: %w", err)
		}
		if err := store.AppendLog(wiki.OpReconcile, cfg.Name, fmt.Sprintf("Reconciled via MCP at %s", time.Now().UTC().Format(time.RFC3339))); err != nil {
			return nil, fmt.Errorf("append log: %w", err)
		}
	}

	var lines []string
	if params.DryRun {
		lines = append(lines, "Dry run. Proposed actions:")
	} else {
		lines = append(lines, "Reconciled:")
	}
	for _, a := range actions {
		lines = append(lines, "  - "+a)
	}

	return ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: strings.Join(lines, "\n")}},
	}, nil
}

func handleWikiStatus(ctx context.Context, s *Server, args json.RawMessage) (any, error) {
	store := s.CurrentStore()

	count, err := store.Search.CountPages()
	if err != nil {
		return nil, fmt.Errorf("count pages: %w", err)
	}
	relCount, err := store.Search.CountRelations()
	if err != nil {
		return nil, fmt.Errorf("count relations: %w", err)
	}
	srcCount, err := store.Search.CountSources()
	if err != nil {
		return nil, fmt.Errorf("count sources: %w", err)
	}

	cfg, err := config.LoadConfig(store.Paths)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	scope, _, err := config.ScopeFromCWD()
	if err != nil {
		return nil, err
	}

	entries, _ := wiki.ReadLogTail(store.Paths, 1)
	lastLog := "none"
	if len(entries) > 0 {
		lastLog = strings.SplitN(entries[0], "\n", 2)[0]
	}

	info := fmt.Sprintf("Scope: %s\nProject: %s\nIndexed pages: %d\nRelations: %d\nSources: %d\nWiki root: %s\nLast log: %s",
		scope, cfg.Name, count, relCount, srcCount, store.Paths.Root, lastLog)

	return ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: info}},
	}, nil
}

func handleWikiQuery(ctx context.Context, s *Server, args json.RawMessage) (any, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse arguments: %w", err)
	}
	if params.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if params.Limit <= 0 {
		params.Limit = 5
	}

	store := s.CurrentStore()
	results, err := store.Search.SearchWithGraph(params.Query, params.Limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		return ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: "No relevant pages found in EigenMemory."}},
		}, nil
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Context from EigenMemory for: %q\n", params.Query))
	for i, r := range results {
		body := wiki.StripFooters(r.Body)
		if len(body) > 400 {
			body = body[:400] + "..."
		}
		marker := ""
		var extra string
		switch r.MatchSource {
		case "graph":
			marker = " (graph)"
		case "source":
			marker = " (source)"
			if r.SourceID != "" {
				extra = fmt.Sprintf("\nFull source: .eigenmemory/sources/%s\n", r.SourceID)
			}
		}
		lines = append(lines, fmt.Sprintf("%d. %s (%s/%s%s)\n%s%s\n", i+1, r.Title, r.Type, r.Slug, marker, body, extra))
	}
	lines = append(lines, "Answer the question using the context above and cite the relevant page(s).")

	return ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: strings.Join(lines, "\n")}},
	}, nil
}

func handleWikiLint(ctx context.Context, s *Server, args json.RawMessage) (any, error) {
	var params struct {
		Fix bool `json:"fix"`
	}
	_ = json.Unmarshal(args, &params)

	store := s.CurrentStore()
	report, err := lint.Run(store.Paths, store.Search)
	if err != nil {
		return nil, fmt.Errorf("lint: %w", err)
	}

	if params.Fix && report.FixableIndexDrift() {
		if err := store.RebuildIndex(); err != nil {
			return nil, fmt.Errorf("rebuild index: %w", err)
		}
		report, err = lint.Run(store.Paths, store.Search)
		if err != nil {
			return nil, fmt.Errorf("lint after fix: %w", err)
		}
	}

	summary := report.Summary()
	var lines []string
	lines = append(lines, fmt.Sprintf("Checked %d pages", report.Checked))
	lines = append(lines, fmt.Sprintf("Issues: %d error(s), %d warning(s), %d info",
		summary["error"], summary["warning"], summary["info"]))
	if len(report.Issues) == 0 {
		lines = append(lines, "Wiki is healthy.")
	} else {
		lines = append(lines, "")
		for _, issue := range report.Issues {
			lines = append(lines, fmt.Sprintf("[%s] %s %s: %s",
				issue.Severity, issue.Page, issue.Category, issue.Message))
		}
	}

	return ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: strings.Join(lines, "\n")}},
	}, nil
}

func fetchResource(s *Server, uri string) (ResourceReadResult, error) {
	store := s.CurrentStore()

	var path string
	switch uri {
	case "memory://index":
		path = store.Paths.IndexFile
	case "memory://log":
		path = store.Paths.LogFile
	default:
		return ResourceReadResult{}, fmt.Errorf("unknown resource: %s", uri)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ResourceReadResult{}, fmt.Errorf("read resource: %w", err)
	}

	return ResourceReadResult{
		Contents: []ResourceContent{{
			URI:      uri,
			MimeType: "text/markdown",
			Text:     string(data),
		}},
	}, nil
}

func extractFirstLine(body string) string {
	body = strings.TrimSpace(body)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return "untitled"
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
