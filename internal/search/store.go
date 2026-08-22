package search

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/types"
)

// Store wraps the SQLite connection and indexing operations.
type Store struct {
	db    *sql.DB
	paths *config.Paths
}

// Open initializes the SQLite database at the configured path.
func Open(paths *config.Paths) (*Store, error) {
	if err := os.MkdirAll(paths.Root, 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", paths.Database+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return &Store{db: db, paths: paths}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// IndexPage inserts or updates a page in the search index.
func (s *Store) IndexPage(page *types.Page) error {
	title := extractTitle(page.Body)
	if title == "" {
		title = page.Slug
	}

	sources := strings.Join(page.Frontmatter.Sources, ",")
	tags := strings.Join(page.Frontmatter.Tags, ",")

	_, err := s.db.Exec(`
		INSERT INTO pages (id, slug, type, status, path, title, body, tags, sources, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			slug = excluded.slug,
			type = excluded.type,
			status = excluded.status,
			path = excluded.path,
			title = excluded.title,
			body = excluded.body,
			tags = excluded.tags,
			sources = excluded.sources,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at
	`,
		page.Frontmatter.ID,
		page.Slug,
		string(page.Frontmatter.Type),
		string(page.Frontmatter.Status),
		page.Path,
		title,
		page.Body,
		tags,
		sources,
		page.Frontmatter.Created.Format(time.RFC3339),
		page.Frontmatter.Updated.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("index page: %w", err)
	}
	return nil
}

// RemovePage deletes a page from the search index by ID.
func (s *Store) RemovePage(id string) error {
	_, err := s.db.Exec(`DELETE FROM pages WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("remove page: %w", err)
	}
	return nil
}

// Clear removes all indexed pages. Use with Reindex workflows.
func (s *Store) Clear() error {
	_, err := s.db.Exec(`DELETE FROM pages`)
	if err != nil {
		return fmt.Errorf("clear index: %w", err)
	}
	return nil
}

// SearchResult is a single hit from a full-text query.
type SearchResult struct {
	ID      string
	Slug    string
	Type    string
	Title   string
	Body    string
	Tags    string
	Updated string
	Rank    float64
}

// Search runs an FTS5 query and returns ranked results.
func (s *Store) Search(query string, limit int) ([]SearchResult, error) {
	ftsQuery := fts5Escape(query)

	// First query FTS5 for ranked rowids.
	rows, err := s.db.Query(`
		SELECT rowid, rank
		FROM pages_fts
		WHERE pages_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var hits []struct {
		rowid int64
		rank  float64
	}
	for rows.Next() {
		var rowid int64
		var rank float64
		if err := rows.Scan(&rowid, &rank); err != nil {
			return nil, fmt.Errorf("scan fts hit: %w", err)
		}
		hits = append(hits, struct {
			rowid int64
			rank  float64
		}{rowid, rank})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Then fetch the corresponding pages by rowid.
	var results []SearchResult
	for _, hit := range hits {
		var r SearchResult
		r.Rank = hit.rank
		err := s.db.QueryRow(`
			SELECT id, slug, type, title, body, tags, updated_at
			FROM pages
			WHERE rowid = ?
		`, hit.rowid).Scan(
			&r.ID, &r.Slug, &r.Type, &r.Title, &r.Body, &r.Tags, &r.Updated,
		)
		if err != nil {
			return nil, fmt.Errorf("fetch page for rowid %d: %w", hit.rowid, err)
		}
		results = append(results, r)
	}
	return results, nil
}

// CountPages returns the number of indexed pages.
func (s *Store) CountPages() (int, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pages`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count pages: %w", err)
	}
	return count, nil
}

// ListAllSlugs returns every indexed page as a "type/slug" key.
func (s *Store) ListAllSlugs() ([]string, error) {
	rows, err := s.db.Query(`SELECT type, slug FROM pages`)
	if err != nil {
		return nil, fmt.Errorf("list slugs: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var pageType, slug string
		if err := rows.Scan(&pageType, &slug); err != nil {
			return nil, fmt.Errorf("scan slug: %w", err)
		}
		out = append(out, pageType+"/"+slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// fts5Escape prepares a user query for the FTS5 MATCH operator.
func fts5Escape(query string) string {
	// FTS5 supports AND/OR/NOT prefixes. For simple agent queries we treat each
	// whitespace-separated token as a required term.
	tokens := strings.Fields(query)
	for i, t := range tokens {
		tokens[i] = fmt.Sprintf(`"%s"`, strings.ReplaceAll(t, `"`, `""`))
	}
	return strings.Join(tokens, " ")
}

// extractTitle returns the first H1 from a markdown body.
func extractTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}
