package wiki

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/types"
)

// wikiPageTypes maps page type names to their directory names.
var wikiPageTypes = map[types.PageType]string{
	types.PageTypeEntity:    "entity",
	types.PageTypeConcept:   "concept",
	types.PageTypeSummary:   "summary",
	types.PageTypeProject:   "project",
	types.PageTypeFeedback:  "feedback",
	types.PageTypeReference: "reference",
	types.PageTypeUser:      "user",
}

// LoadPage reads and parses a wiki page from disk.
func LoadPage(paths *config.Paths, pageType types.PageType, slug string) (*types.Page, error) {
	dir, ok := wikiPageTypes[pageType]
	if !ok {
		return nil, fmt.Errorf("unknown page type %q", pageType)
	}
	if err := ValidateSlug(slug); err != nil {
		return nil, err
	}
	path := filepath.Join(paths.WikiDir, dir, slug+".md")
	return LoadPageByPath(path)
}

// LoadPageByPath reads and parses a wiki page from an arbitrary path.
func LoadPageByPath(path string) (*types.Page, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read page %s: %w", path, err)
	}

	body := string(data)
	front, bodyContent, err := splitFrontmatter(body)
	if err != nil {
		return nil, fmt.Errorf("split frontmatter %s: %w", path, err)
	}

	var fm types.Frontmatter
	if front != "" {
		if err := yaml.Unmarshal([]byte(front), &fm); err != nil {
			return nil, fmt.Errorf("parse frontmatter %s: %w", path, err)
		}
	}

	slug := strings.TrimSuffix(filepath.Base(path), ".md")
	return &types.Page{
		Frontmatter: fm,
		Body:        strings.TrimSpace(bodyContent),
		Path:        path,
		Slug:        slug,
	}, nil
}

// SavePage writes a wiki page to disk and updates its timestamp.
func SavePage(paths *config.Paths, page *types.Page, pageType types.PageType) error {
	if err := ValidateSlug(page.Slug); err != nil {
		return err
	}
	// Projection/sync footers (e.g. "_Synced from Claude Code memory ..._") are
	// a read-time convenience for generated files, never part of the
	// canonical wiki body. Strip them here so no caller can accidentally
	// persist one into the store.
	page.Body = StripFooters(page.Body)
	now := nowUTC()
	page.Frontmatter.Updated = now
	if page.Frontmatter.ID == "" {
		page.Frontmatter.ID = types.NewID()
	}
	if page.Frontmatter.Created.IsZero() {
		page.Frontmatter.Created = now
	}

	dir, ok := wikiPageTypes[pageType]
	if !ok {
		return fmt.Errorf("unknown page type %q", pageType)
	}
	targetDir := filepath.Join(paths.WikiDir, dir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create page directory: %w", err)
	}

	path := filepath.Join(targetDir, page.Slug+".md")
	page.Path = path

	data, err := serializePage(page)
	if err != nil {
		return fmt.Errorf("serialize page: %w", err)
	}

	if err := writeFileAtomic(path, []byte(data)); err != nil {
		return fmt.Errorf("save page: %w", err)
	}
	return nil
}

// PageExists reports whether a page file already exists.
func PageExists(paths *config.Paths, pageType types.PageType, slug string) bool {
	dir, ok := wikiPageTypes[pageType]
	if !ok {
		return false
	}
	if err := ValidateSlug(slug); err != nil {
		return false
	}
	path := filepath.Join(paths.WikiDir, dir, slug+".md")
	_, err := os.Stat(path)
	return err == nil
}

// ListPages returns all pages of a given type.
func ListPages(paths *config.Paths, pageType types.PageType) ([]*types.Page, error) {
	dir, ok := wikiPageTypes[pageType]
	if !ok {
		return nil, fmt.Errorf("unknown page type %q", pageType)
	}
	targetDir := filepath.Join(paths.WikiDir, dir)
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list pages %s: %w", targetDir, err)
	}

	var pages []*types.Page
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		page, err := LoadPageByPath(filepath.Join(targetDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

// wikilinkRe matches Obsidian-style [[wikilink]] references, capturing the
// inner target (which may carry a "|alias" or "#heading" suffix).
var wikilinkRe = regexp.MustCompile(`\[\[([^\]\[]+)\]\]`)

// ExtractLinks parses a markdown body and returns the set of internal wiki
// links: markdown-style [text](dest) links plus Obsidian-style [[wikilink]]
// references. Links inside fenced/indented code blocks and inline code spans
// are ignored, so a verbatim source quote (e.g. an ingested document) does
// not contribute graph edges.
func ExtractLinks(body string) []string {
	seen := make(map[string]struct{})
	var links []string

	add := func(cleaned string) {
		if cleaned == "" {
			return
		}
		if _, exists := seen[cleaned]; !exists {
			seen[cleaned] = struct{}{}
			links = append(links, cleaned)
		}
	}

	source := []byte(body)
	reader := text.NewReader(source)
	md := goldmark.DefaultParser()
	doc := md.Parse(reader)

	// codeDepth tracks how deep we are inside code blocks/spans. Text and
	// link nodes nested in code are literal, not edges. Non-code text is
	// accumulated into a buffer and scanned for [[wikilinks]], because goldmark
	// may split a single [[...]] across several sibling *ast.Text nodes.
	//
	// The buffer is flushed at every non-Text boundary (a code span/block, a
	// block-level container, a link, …) so that a [[wikilink]] can only form
	// within one contiguous run of sibling Text nodes. Without this, text
	// separated by an inline code span ("[[target" + `code` + "]]") or by a
	// block boundary would stitch together into a false edge. Sibling Text
	// nodes have no intervening non-Text node in the walk, so flushing at
	// non-Text boundaries does not break the split-[[...]] case.
	codeDepth := 0
	var nonCodeText bytes.Buffer
	flush := func() {
		for _, match := range wikilinkRe.FindAllStringSubmatch(nonCodeText.String(), -1) {
			cleaned := cleanWikilink(match[1])
			if cleaned != "" && isInternalLink(cleaned) {
				add(cleaned)
			}
		}
		nonCodeText.Reset()
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		isCode := false
		switch n.(type) {
		case *ast.FencedCodeBlock, *ast.CodeBlock, *ast.CodeSpan:
			isCode = true
		}
		if entering {
			if isCode {
				codeDepth++
				// Entering a code region ends the current non-code text run.
				flush()
				return ast.WalkContinue, nil
			}
			if codeDepth > 0 {
				return ast.WalkContinue, nil
			}
			if link, ok := n.(*ast.Link); ok {
				// A link node is a boundary between sibling text runs.
				flush()
				dest := string(link.Destination)
				if isInternalLink(dest) {
					add(cleanInternalLink(dest))
				}
				return ast.WalkContinue, nil
			}
			if t, ok := n.(*ast.Text); ok {
				nonCodeText.Write(t.Value(source))
			} else {
				// Any other non-Text node (block container, etc.) is a boundary.
				flush()
			}
		} else if isCode {
			codeDepth--
		}
		return ast.WalkContinue, nil
	})
	flush()

	return links
}

// cleanWikilink normalizes the inner text of a [[wikilink]] to a link target:
// it drops a "|alias" suffix, a "#heading" suffix, and a trailing ".md", then
// trims whitespace. Unlike cleanInternalLink it does not reduce to a bare
// slug — any "type/" prefix is preserved so callers can treat it uniformly
// with markdown-link destinations.
func cleanWikilink(inner string) string {
	inner = strings.Split(inner, "|")[0]
	inner = strings.Split(inner, "#")[0]
	inner = strings.TrimSuffix(inner, ".md")
	return strings.TrimSpace(inner)
}

// LinkToSlug reduces any link target — a markdown destination like
// "entity/auth-service.md" or a wikilink target like "auth-service" — to the
// bare slug used by the relations and pages tables. It drops an alias,
// heading, and ".md" suffix, then takes the path base and lowercases.
func LinkToSlug(dest string) string {
	s := strings.Split(dest, "|")[0]
	s = strings.Split(s, "#")[0]
	s = strings.TrimSuffix(s, ".md")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.ToLower(path.Base(s))
}

// splitFrontmatter separates YAML frontmatter from markdown body.
func splitFrontmatter(content string) (string, string, error) {
	// Strip a leading UTF-8 BOM if present.
	content = strings.TrimPrefix(content, "\xef\xbb\xbf")
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return "", content, nil
	}

	rest := content[4:]
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return "", "", fmt.Errorf("frontmatter not terminated")
	}
	front := rest[:idx]
	body := rest[idx+5:]
	return front, body, nil
}

// serializePage renders a Page to its markdown representation.
func serializePage(page *types.Page) (string, error) {
	fmData, err := yaml.Marshal(page.Frontmatter)
	if err != nil {
		return "", fmt.Errorf("marshal frontmatter: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(fmData)
	buf.WriteString("---\n\n")
	buf.WriteString(strings.TrimSpace(page.Body))
	buf.WriteString("\n")
	return buf.String(), nil
}

// isInternalLink reports whether a markdown link points to another wiki page.
func isInternalLink(dest string) bool {
	if strings.Contains(dest, "://") {
		return false
	}
	if strings.HasPrefix(dest, "#") {
		return false
	}
	return true
}

// cleanInternalLink normalizes a link target to a slug.
func cleanInternalLink(dest string) string {
	dest = strings.Split(dest, "#")[0]
	dest = strings.TrimSuffix(dest, ".md")
	return strings.TrimSpace(dest)
}

var nonAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// Slugify converts a title into a safe filename slug.
func Slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// nowUTC returns the current UTC time. It is a variable to allow tests to stub.
var nowUTC = types.NowUTC
