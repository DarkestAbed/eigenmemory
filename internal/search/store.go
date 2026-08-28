package search

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/DarkestAbed/eigenmemory/internal/config"
	"github.com/DarkestAbed/eigenmemory/internal/types"
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

	db, err := sql.Open("sqlite", paths.Database)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// The modernc.org/sqlite driver does not recognize `_journal_mode` as a DSN
	// parameter (that syntax belongs to mattn/go-sqlite3), so pragmas must be
	// set explicitly after opening the connection.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set journal_mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set synchronous: %w", err)
	}

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err == nil && !strings.EqualFold(mode, "wal") {
		log.Printf("eigenmemory: warning: journal_mode is %q, expected \"wal\" (some filesystems, e.g. network mounts, don't support WAL; concurrent access may see \"database is locked\" errors)", mode)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
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
	if err := s.IndexRelations(page.Slug, page.Frontmatter.Relations); err != nil {
		return err
	}
	return nil
}

// IndexRelations replaces the stored relations originating from a page with
// the current set from its frontmatter. Call this whenever a page is
// (re)indexed so the `relations` table stays a live reflection of the wiki
// rather than dead schema.
func (s *Store) IndexRelations(fromSlug string, relations []types.Relation) error {
	if _, err := s.db.Exec(`DELETE FROM relations WHERE from_id = ?`, fromSlug); err != nil {
		return fmt.Errorf("clear relations for %s: %w", fromSlug, err)
	}
	for _, r := range relations {
		if _, err := s.db.Exec(`
			INSERT INTO relations (from_id, to_id, relation_type, provenance)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(from_id, to_id, relation_type) DO UPDATE SET provenance = excluded.provenance
		`, fromSlug, r.To, r.Type, r.Provenance); err != nil {
			return fmt.Errorf("index relation %s -> %s: %w", fromSlug, r.To, err)
		}
	}
	return nil
}

// IndexBodyLinks records the bare-slug targets of the body links on a page as
// untyped "links_to" edges with provenance "body". It is idempotent via the
// relations primary key. Callers must pass already-normalized bare slugs
// (e.g. via wiki.LinkToSlug). IndexPage clears all relations for a slug
// (including prior links_to) before inserting frontmatter relations, so call
// this after IndexPage to restore the body edges for the current body.
//
// On conflict with a same-(from,to,links_to) row already inserted from
// frontmatter, the existing row is left untouched: frontmatter is the
// authoritative source of an authored links_to edge, and a body link to the
// same target must not relabel it as body-derived.
func (s *Store) IndexBodyLinks(fromSlug string, targets []string) error {
	for _, to := range targets {
		if _, err := s.db.Exec(`
			INSERT INTO relations (from_id, to_id, relation_type, provenance)
			VALUES (?, ?, 'links_to', 'body')
			ON CONFLICT(from_id, to_id, relation_type) DO NOTHING
		`, fromSlug, to); err != nil {
			return fmt.Errorf("index body link %s -> %s: %w", fromSlug, to, err)
		}
	}
	return nil
}

// ListRelations returns every relation recorded in the index.
func (s *Store) ListRelations() ([]types.Relation, error) {
	rows, err := s.db.Query(`SELECT from_id, to_id, relation_type, provenance FROM relations`)
	if err != nil {
		return nil, fmt.Errorf("list relations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []types.Relation
	for rows.Next() {
		var r types.Relation
		var provenance sql.NullString
		if err := rows.Scan(&r.From, &r.To, &r.Type, &provenance); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		r.Provenance = provenance.String
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// CountRelations returns the number of indexed relations.
func (s *Store) CountRelations() (int, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM relations`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count relations: %w", err)
	}
	return count, nil
}

// IndexSource inserts or updates a source record in the SQLite index and
// indexes its full content for full-text search. The on-disk file under
// .eigenmemory/sources/ remains the immutable canonical copy; the `sources`
// row makes metadata queryable and the `source_docs` row (synced to
// sources_fts by triggers) makes the full content searchable past the ~2KB
// summary-page truncation.
func (s *Store) IndexSource(id, path, sha256, storedAt, name string, content []byte) error {
	_, err := s.db.Exec(`
		INSERT INTO sources (id, path, sha256, stored_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET path = excluded.path, sha256 = excluded.sha256, stored_at = excluded.stored_at
	`, id, path, sha256, storedAt)
	if err != nil {
		return fmt.Errorf("index source metadata: %w", err)
	}
	if _, err := s.db.Exec(`
		INSERT INTO source_docs (id, name, content)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET name = excluded.name, content = excluded.content
	`, id, name, string(content)); err != nil {
		return fmt.Errorf("index source content: %w", err)
	}
	return nil
}

// SourceDisplayName returns a human-readable name for a source by looking up
// the summary page that references it (the summary page title encodes the
// source name used at ingest time — the name is not stored on disk). Returns
// "" if no summary page references the source.
func (s *Store) SourceDisplayName(sourceID string) (string, error) {
	var title sql.NullString
	err := s.db.QueryRow(`
		SELECT title FROM pages
		WHERE type = 'summary' AND (',' || sources || ',' LIKE '%,' || ? || ',%')
		LIMIT 1
	`, sourceID).Scan(&title)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("lookup source name: %w", err)
	}
	return title.String, nil
}

// searchSources runs a full-text query over the full content of indexed
// sources, with the same AND-first / OR-fallback robustness as page search.
// Each hit is a SearchResult with MatchSource="source", a snippet of the
// matching passage in Body, and the full source id in SourceID.
func (s *Store) searchSources(query string, limit int) ([]SearchResult, error) {
	tokens := stripStopwords(strings.Fields(query))
	match := buildMatch(tokens, " ")
	hits, err := s.searchSourcesMatch(match, limit)
	if err != nil {
		return nil, err
	}
	if len(hits) > 0 {
		return hits, nil
	}
	return s.searchSourcesMatch(buildMatch(tokens, " OR "), limit)
}

func (s *Store) searchSourcesMatch(match string, limit int) ([]SearchResult, error) {
	if strings.TrimSpace(match) == "" || limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.Query(`
		SELECT d.id, d.name, snippet(sources_fts, 1, '**', '**', ' … ', 24), bm25(sources_fts)
		FROM sources_fts
		JOIN source_docs d ON d.rowid = sources_fts.rowid
		WHERE sources_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("search sources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []SearchResult
	for rows.Next() {
		var id, name string
		var snippet sql.NullString
		var rank float64
		if err := rows.Scan(&id, &name, &snippet, &rank); err != nil {
			return nil, fmt.Errorf("scan source hit: %w", err)
		}
		title := name
		if title == "" {
			title = id[:min(12, len(id))]
		}
		slug := id
		if len(slug) > 12 {
			slug = slug[:12]
		}
		out = append(out, SearchResult{
			ID:          id,
			Slug:        slug,
			Type:        "source",
			Title:       title,
			Body:        snippet.String,
			Rank:        rank,
			MatchSource: "source",
			SourceID:    id,
		})
	}
	return out, rows.Err()
}

// CountSources returns the number of indexed sources.
func (s *Store) CountSources() (int, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sources`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count sources: %w", err)
	}
	return count, nil
}

// RemovePage deletes a page from the search index by ID.
func (s *Store) RemovePage(id string) error {
	_, err := s.db.Exec(`DELETE FROM pages WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("remove page: %w", err)
	}
	return nil
}

// Clear removes all indexed pages, relations, sources, and source content.
// Use with Reindex workflows that rebuild the entire index from disk. Deleting
// source_docs cascades to sources_fts via triggers.
func (s *Store) Clear() error {
	if _, err := s.db.Exec(`DELETE FROM pages`); err != nil {
		return fmt.Errorf("clear index: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM relations`); err != nil {
		return fmt.Errorf("clear relations: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM sources`); err != nil {
		return fmt.Errorf("clear sources: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM source_docs`); err != nil {
		return fmt.Errorf("clear source docs: %w", err)
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
	// MatchSource is "fts" for direct page full-text hits, "source" for hits
	// from the full (untruncated) source content, and "graph" for pages pulled
	// in by one-hop relation traversal of an FTS hit. Empty for results built by
	// older callers that only ran full-text search.
	MatchSource string
	// SourceID is the full SHA-256 source id for MatchSource="source" results,
	// so callers can point at .eigenmemory/sources/<id>. Empty otherwise.
	SourceID string
}

// Search runs an FTS5 query and returns ranked results. It uses strict
// implicit-AND semantics (every non-stopword token required). For the
// recall-fallback path used by query/recall, see SearchWithGraph, which falls
// back to OR when AND yields nothing.
func (s *Store) Search(query string, limit int) ([]SearchResult, error) {
	return s.searchPagesMatch(fts5Escape(query), limit)
}

// searchPagesMatch runs a prebuilt FTS5 MATCH string against pages_fts and
// fetches the corresponding page rows. Results are MatchSource="fts".
func (s *Store) searchPagesMatch(match string, limit int) ([]SearchResult, error) {
	if strings.TrimSpace(match) == "" {
		return nil, nil
	}
	// First query FTS5 for ranked rowids.
	rows, err := s.db.Query(`
		SELECT rowid, rank
		FROM pages_fts
		WHERE pages_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
		r.MatchSource = "fts"
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

// searchPagesWithFallback runs a page query as implicit-AND first; if that
// returns nothing (a long natural-language question that ANDs too many tokens
// to zero hits), it retries as OR so the most-matching pages still surface,
// BM25-ranked. This keeps keyword queries precise while rescuing NL queries.
func (s *Store) searchPagesWithFallback(query string, limit int) ([]SearchResult, error) {
	tokens := stripStopwords(strings.Fields(query))
	hits, err := s.searchPagesMatch(buildMatch(tokens, " "), limit)
	if err != nil {
		return nil, err
	}
	if len(hits) > 0 {
		return hits, nil
	}
	return s.searchPagesMatch(buildMatch(tokens, " OR "), limit)
}

// Neighbors returns up to limit pages reachable in one hop from the given
// slugs via the relations table (either direction). Pages already in the
// seed set are excluded. Results are marked MatchSource="graph".
func (s *Store) Neighbors(seeds []string, limit int) ([]SearchResult, error) {
	if len(seeds) == 0 || limit <= 0 {
		return nil, nil
	}
	// Build placeholders for the IN clause.
	placeholders := make([]string, len(seeds))
	args := make([]any, 0, len(seeds))
	seedSet := make(map[string]bool, len(seeds))
	for i, slug := range seeds {
		placeholders[i] = "?"
		args = append(args, slug)
		seedSet[strings.ToLower(slug)] = true
	}
	inClause := strings.Join(placeholders, ",")

	// Neighbor slugs reachable in either direction. Use UNION to dedup the
	// slug column before fetching page rows.
	neighborArgs := append(append([]any{}, args...), args...)
	query := fmt.Sprintf(`
		SELECT DISTINCT p.id, p.slug, p.type, p.title, p.body, p.tags, p.updated_at
		FROM pages p
		WHERE p.slug IN (
			SELECT to_id FROM relations WHERE from_id IN (%[1]s)
			UNION
			SELECT from_id FROM relations WHERE to_id IN (%[1]s)
		)
	`, inClause)
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit*4)
	}
	rows, err := s.db.Query(query, neighborArgs...)
	if err != nil {
		return nil, fmt.Errorf("neighbors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ID, &r.Slug, &r.Type, &r.Title, &r.Body, &r.Tags, &r.Updated); err != nil {
			return nil, fmt.Errorf("scan neighbor: %w", err)
		}
		if seedSet[strings.ToLower(r.Slug)] {
			continue
		}
		r.MatchSource = "graph"
		out = append(out, r)
		if len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

// SearchWithGraph answers a query by combining three result sources, in order:
//  1. Page full-text hits (MatchSource "fts"), via searchPagesWithFallback —
//     strict AND first, OR fallback so long natural-language questions don't
//     silently return nothing.
//  2. Source full-text hits (MatchSource "source") over the full (untruncated)
//     source content — see searchSources.
//  3. One-hop graph neighbors of the page hits (MatchSource "graph").
func (s *Store) SearchWithGraph(query string, limit int) ([]SearchResult, error) {
	hits, err := s.searchPagesWithFallback(query, limit)
	if err != nil {
		return nil, err
	}

	results := hits

	if sourceHits, err := s.searchSources(query, limit); err != nil {
		return nil, fmt.Errorf("search sources: %w", err)
	} else {
		results = append(results, sourceHits...)
	}

	if len(hits) == 0 {
		return results, nil
	}
	seeds := make([]string, 0, len(hits))
	for _, h := range hits {
		seeds = append(seeds, h.Slug)
	}
	neighbors, err := s.Neighbors(seeds, limit)
	if err != nil {
		return nil, fmt.Errorf("expand graph: %w", err)
	}
	results = append(results, neighbors...)
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
	defer func() { _ = rows.Close() }()

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

// fts5Escape prepares a user query for the FTS5 MATCH operator as an implicit
// AND of every (stopword-stripped) token. Kept for strict-AND callers/tests.
func fts5Escape(query string) string {
	return buildMatch(stripStopwords(strings.Fields(query)), " ")
}

// buildMatch builds an FTS5 MATCH string from tokens joined by op: " " yields
// implicit AND (every token required), " OR " yields OR (any token, BM25-ranked).
// Each token is quoted with embedded double-quotes escaped, so a token is
// treated as a literal phrase of one word and never as FTS5 syntax.
func buildMatch(tokens []string, op string) string {
	quoted := make([]string, len(tokens))
	for i, t := range tokens {
		quoted[i] = fmt.Sprintf(`"%s"`, strings.ReplaceAll(t, `"`, `""`))
	}
	return strings.Join(quoted, op)
}

// stopwords is a small English set of low-signal tokens that over-constrain an
// implicit-AND query (long natural-language questions AND every word, so "how
// is the …" can zero out an otherwise-relevant page). Matching is case-folded.
var stopwords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "of": {}, "to": {}, "in": {},
	"on": {}, "for": {}, "is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "by": {},
	"with": {}, "how": {}, "what": {}, "when": {}, "where": {}, "which": {}, "who": {},
	"why": {}, "do": {}, "does": {}, "did": {}, "i": {}, "we": {}, "you": {}, "my": {},
	"our": {}, "it": {}, "its": {}, "this": {}, "that": {}, "from": {}, "into": {},
	"as": {}, "at": {},
}

// stripStopwords removes stopwords from tokens. If the result would be empty
// (an all-stopword query), the original tokens are returned so the caller does
// not run an empty MATCH.
func stripStopwords(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, stop := stopwords[strings.ToLower(t)]; stop {
			continue
		}
		out = append(out, t)
	}
	if len(out) == 0 {
		return tokens
	}
	return out
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
