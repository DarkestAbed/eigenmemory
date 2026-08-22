package search

const schema = `
CREATE TABLE IF NOT EXISTS pages (
	id TEXT PRIMARY KEY,
	slug TEXT NOT NULL,
	type TEXT NOT NULL,
	status TEXT NOT NULL,
	path TEXT NOT NULL,
	title TEXT,
	body TEXT,
	tags TEXT,
	sources TEXT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sources (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	sha256 TEXT NOT NULL,
	stored_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS relations (
	from_id TEXT NOT NULL,
	to_id TEXT NOT NULL,
	relation_type TEXT NOT NULL,
	provenance TEXT,
	PRIMARY KEY (from_id, to_id, relation_type)
);

CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5(
	title,
	body,
	tags,
	content='pages',
	content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS pages_ai AFTER INSERT ON pages BEGIN
	INSERT INTO pages_fts(rowid, title, body, tags)
	VALUES (new.rowid, new.title, new.body, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS pages_ad AFTER DELETE ON pages BEGIN
	INSERT INTO pages_fts(pages_fts, rowid, title, body, tags)
	VALUES ('delete', old.rowid, old.title, old.body, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS pages_au AFTER UPDATE ON pages BEGIN
	INSERT INTO pages_fts(pages_fts, rowid, title, body, tags)
	VALUES ('delete', old.rowid, old.title, old.body, old.tags);
	INSERT INTO pages_fts(rowid, title, body, tags)
	VALUES (new.rowid, new.title, new.body, new.tags);
END;

CREATE INDEX IF NOT EXISTS idx_pages_slug ON pages(slug);
CREATE INDEX IF NOT EXISTS idx_pages_type ON pages(type);
CREATE INDEX IF NOT EXISTS idx_pages_status ON pages(status);
CREATE INDEX IF NOT EXISTS idx_relations_from ON relations(from_id);
CREATE INDEX IF NOT EXISTS idx_relations_to ON relations(to_id);
`
