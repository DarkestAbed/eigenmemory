package lint

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/search"
	"github.com/javi/eigenmemory/internal/types"
	"github.com/javi/eigenmemory/internal/wiki"
)

// Severity levels.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// Category tags for issues.
const (
	CategoryOrphan         = "orphan"
	CategoryBrokenLink     = "broken-link"
	CategoryStale          = "stale"
	CategoryDrift          = "index-drift"
	CategoryBrokenRelation = "broken-relation"
)

// Issue describes a single wiki health problem.
type Issue struct {
	Severity string
	Category string
	Page     string
	Message  string
}

// Report is the result of a lint pass.
type Report struct {
	Issues  []Issue
	Checked int
}

// Summary returns counts by severity.
func (r *Report) Summary() map[string]int {
	out := map[string]int{SeverityError: 0, SeverityWarning: 0, SeverityInfo: 0}
	for _, i := range r.Issues {
		out[i.Severity]++
	}
	return out
}

// Run performs a health check over the EigenMemory wiki.
func Run(paths *config.Paths, search *search.Store) (*Report, error) {
	report := &Report{}

	// Gather every page in the wiki.
	allPages := make(map[string]*types.Page) // key = "type/slug"
	var orderedKeys []string
	for _, pageType := range types.ValidPageTypes() {
		pages, err := wiki.ListPages(paths, pageType)
		if err != nil {
			return nil, fmt.Errorf("list pages %s: %w", pageType, err)
		}
		for _, p := range pages {
			key := pageKey(pageType, p.Slug)
			allPages[key] = p
			orderedKeys = append(orderedKeys, key)
			report.Checked++
		}
	}

	// Index drift: compare markdown pages to SQLite index.
	if search != nil {
		indexed, err := search.ListAllSlugs()
		if err != nil {
			return nil, fmt.Errorf("list indexed slugs: %w", err)
		}
		indexedSet := make(map[string]struct{}, len(indexed))
		for _, k := range indexed {
			indexedSet[k] = struct{}{}
		}
		for key := range allPages {
			if _, ok := indexedSet[key]; !ok {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning,
					Category: CategoryDrift,
					Page:     key,
					Message:  "page exists in markdown but is missing from the SQLite search index",
				})
			}
		}
		for key := range indexedSet {
			if _, ok := allPages[key]; !ok {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityWarning,
					Category: CategoryDrift,
					Page:     key,
					Message:  "page exists in the SQLite search index but has no markdown file",
				})
			}
		}
	}

	// Load the index catalog and extract the slugs it links to.
	linked, err := indexLinkedSlugs(paths)
	if err != nil {
		return nil, err
	}

	// Orphan check: every non-index/log page should be reachable from the index
	// or referenced by another page.
	referencedBy := linkedSlugs(nil)
	for _, p := range allPages {
		for _, link := range wiki.ExtractLinks(p.Body) {
			referencedBy[linkSlug(link)] = true
		}
	}
	for key := range allPages {
		_, slug := splitPageKey(key)
		lowerSlug := strings.ToLower(slug)
		if lowerSlug == "index" || lowerSlug == "log" {
			continue
		}
		if !linked[lowerSlug] && !referencedBy[lowerSlug] {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityWarning,
				Category: CategoryOrphan,
				Page:     key,
				Message:  "page is not linked from index.md or any other page",
			})
		}
	}

	// Broken link check and stale check.
	for key, p := range allPages {
		for _, link := range wiki.ExtractLinks(p.Body) {
			target := linkSlug(link)
			if !pageExists(allPages, target) {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityError,
					Category: CategoryBrokenLink,
					Page:     key,
					Message:  fmt.Sprintf("broken internal link to %q", link),
				})
			}
		}

		for _, rel := range p.Frontmatter.Relations {
			target := strings.ToLower(strings.TrimSpace(rel.To))
			if target != "" && !pageExists(allPages, target) {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityError,
					Category: CategoryBrokenRelation,
					Page:     key,
					Message:  fmt.Sprintf("relation %q --%s--> %q points to a page that does not exist", rel.From, rel.Type, rel.To),
				})
			}
		}

		if strings.ToLower(string(p.Frontmatter.Status)) == "stale" {
			age := time.Since(p.Frontmatter.Updated)
			if age > 30*24*time.Hour {
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityInfo,
					Category: CategoryStale,
					Page:     key,
					Message:  fmt.Sprintf("page is stale and was last updated %s ago", formatDuration(age)),
				})
			}
		}
	}

	// Sort for stable output.
	sortIssues(report.Issues)

	return report, nil
}

// indexLinkedSlugs returns the set of slugs explicitly linked from index.md.
func indexLinkedSlugs(paths *config.Paths) (map[string]bool, error) {
	idxPath := paths.IndexFile
	data, err := os.ReadFile(idxPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("read index: %w", err)
	}
	p := &types.Page{Body: string(data)}
	return linkedSlugs(wiki.ExtractLinks(p.Body)), nil
}

// linkedSlugs turns markdown link destinations into bare slugs.
func linkedSlugs(links []string) map[string]bool {
	out := make(map[string]bool, len(links))
	for _, link := range links {
		out[linkSlug(link)] = true
	}
	return out
}

// linkSlug returns the bare slug for a markdown link destination.
func linkSlug(link string) string {
	link = strings.Split(link, "#")[0]
	link = strings.TrimSuffix(link, ".md")
	link = path.Base(link)
	return strings.ToLower(strings.TrimSpace(link))
}

// pageExists reports whether a link target matches any existing page slug.
func pageExists(all map[string]*types.Page, link string) bool {
	for key := range all {
		_, slug := splitPageKey(key)
		if strings.ToLower(slug) == link {
			return true
		}
	}
	return false
}

func pageKey(pageType types.PageType, slug string) string {
	return string(pageType) + "/" + slug
}

func splitPageKey(key string) (types.PageType, string) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return "", key
	}
	return types.PageType(parts[0]), parts[1]
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

func sortIssues(issues []Issue) {
	for i := 0; i < len(issues); i++ {
		for j := i + 1; j < len(issues); j++ {
			if order(issues[i]) > order(issues[j]) {
				issues[i], issues[j] = issues[j], issues[i]
			}
		}
	}
}

func order(i Issue) string {
	return i.Category + "\t" + i.Page + "\t" + i.Message
}

// FixableIndexDrift reports whether any drift issues are present.
func (r *Report) FixableIndexDrift() bool {
	for _, i := range r.Issues {
		if i.Category == CategoryDrift {
			return true
		}
	}
	return false
}

// Orphans returns all orphan issue page keys.
func (r *Report) Orphans() []string {
	var out []string
	for _, i := range r.Issues {
		if i.Category == CategoryOrphan {
			out = append(out, i.Page)
		}
	}
	return out
}

// LinkTargetPaths returns the conventional file paths for a set of page keys.
func LinkTargetPaths(paths *config.Paths, keys []string) []string {
	var out []string
	for _, key := range keys {
		pt, slug := splitPageKey(key)
		dir, ok := map[types.PageType]string{
			types.PageTypeEntity:    "entity",
			types.PageTypeConcept:   "concept",
			types.PageTypeSummary:   "summary",
			types.PageTypeProject:   "project",
			types.PageTypeFeedback:  "feedback",
			types.PageTypeReference: "reference",
			types.PageTypeUser:      "user",
		}[pt]
		if !ok {
			continue
		}
		out = append(out, filepath.Join(paths.WikiDir, dir, slug+".md"))
	}
	return out
}
